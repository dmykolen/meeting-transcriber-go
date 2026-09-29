// Package service exposes the Wails-bound application API.
package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/dmykolen/meeting-transcriber-go/internal/home"
	"github.com/dmykolen/meeting-transcriber-go/internal/library"
	"github.com/dmykolen/meeting-transcriber-go/internal/listen"
	"github.com/dmykolen/meeting-transcriber-go/internal/media"
	"github.com/dmykolen/meeting-transcriber-go/internal/models"
	"github.com/dmykolen/meeting-transcriber-go/internal/store"
)

type Meetings struct {
	db      *store.DB
	lib     *library.Library
	dir     string
	mu      sync.Mutex // guards config, which Wails calls and downloads share
	config  home.Config
	started time.Time
	mcpMu   sync.RWMutex
	mcp     MCPState

	aiMu     sync.Mutex
	ai       AIState // downloads, sign-in and their problems
	fetching bool
	noServer bool

	// The listener may arrive later on a first run while models download.
	ears *listen.Recorder
}

// Listener installs the always-on recorder once it is ready.
func (m *Meetings) Listener(r *listen.Recorder) {
	m.ears = r
	r.Pause(!m.config.Listen.Enabled)
	r.Hear(m.config.Listen.System)
}

// Playing tells the listener when app-owned playback is running.
func (m *Meetings) Playing(on bool) {
	if m.ears != nil {
		m.ears.Muffle(on)
	}
}

// Listening reports the recorder state for polling UI.
func (m *Meetings) Listening() listen.Status {
	if m.ears == nil {
		return listen.Status{Phase: listen.Opening}
	}
	m.mu.Lock()
	enabled := m.config.Listen.Enabled
	m.mu.Unlock()
	if !enabled {
		return listen.Status{Phase: listen.Off}
	}
	return m.ears.Status()
}

// Summaries reports whether summaries and answers can be made right now.
func (m *Meetings) Summaries() bool { return m.lib.AI().Ready() }

// Record toggles manual recording.
func (m *Meetings) Record() {
	if m.ears != nil {
		m.ears.Toggle()
	}
}

// Hold pauses the recording in progress without ending it, or resumes it.
func (m *Meetings) Hold(on bool) {
	if m.ears != nil {
		m.ears.Hold(on)
	}
}

func New(db *store.DB, lib *library.Library, dir string, cfg home.Config) *Meetings {
	return &Meetings{db: db, lib: lib, dir: dir, config: cfg, started: time.Now()}
}

// Recent lists recordings newest first.
func (m *Meetings) Recent(limit int) ([]store.Recording, error) {
	found, err := m.db.Recent(limit)
	if err != nil {
		return nil, err
	}
	if found == nil {
		// Prefer an empty array to null for the frontend.
		return []store.Recording{}, nil
	}
	return found, nil
}

// Meeting is one recording with its transcript.
type Meeting struct {
	store.Recording
	Turns []store.Turn `json:"transcript"`
	// Wait is why a queued recording is not being processed yet (see
	// library.Waiting), and Until when it will be, for "time".
	Wait  string    `json:"wait"`
	Until time.Time `json:"until"`
}

// Open loads one recording and its transcript.
func (m *Meetings) Open(id int64) (*Meeting, error) {
	r, err := m.db.Get(id)
	if err != nil {
		return nil, fmt.Errorf("recording %d: %w", id, err)
	}
	turns, err := m.db.Turns(id)
	if err != nil {
		return nil, err
	}
	if turns == nil {
		turns = []store.Turn{}
	}
	wait, until := m.lib.Waiting(*r, time.Now())
	return &Meeting{Recording: *r, Turns: turns, Wait: wait, Until: until}, nil
}

// Rush transcribes one queued recording next, whatever the schedule says.
func (m *Meetings) Rush(id int64) error { return m.lib.Rush(id) }

// Search finds passages across every transcript.
func (m *Meetings) Search(query string) ([]store.Hit, error) {
	hits, err := m.lib.Find(context.Background(), query, 40)
	if err != nil {
		return nil, err
	}
	if hits == nil {
		return []store.Hit{}, nil
	}
	return hits, nil
}

// Answer is an LLM reply plus the passages it cites.
type Answer struct {
	Text    string      `json:"text"`
	Sources []store.Hit `json:"sources"`
}

// Ask answers a question from the transcripts.
func (m *Meetings) Ask(question string) (*Answer, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	text, hits, err := m.lib.Ask(ctx, question)
	if err != nil {
		return nil, err
	}
	return &Answer{Text: text, Sources: hits}, nil
}

// Rename changes a speaker's label throughout one meeting and teaches the
// matching voice when possible.
func (m *Meetings) Rename(id int64, from, to string) error {
	print := m.db.VoiceIn(id, from)
	if err := m.db.Rename(id, from, to); err != nil {
		return err
	}
	if len(print) == 0 {
		// The rename still stands for this meeting; there just is not enough
		// speech to learn from.
		return nil
	}
	if err := m.db.Remember(to, print, store.Source{Recording: id, Speaker: to}); err != nil {
		slog.Warn("renamed, but could not learn the voice", "name", to, "err", err)
	}
	return nil
}

// Retitle overrides the model-generated title.
func (m *Meetings) Retitle(id int64, title string) error { return m.db.Retitle(id, title) }

// ThisIsMe names the laptop owner and backfills any usable self voiceprints.
func (m *Meetings) ThisIsMe(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", errors.New("a name is needed")
	}
	recent, err := m.db.Recent(200)
	if err != nil {
		return "", err
	}
	taught := 0
	for _, r := range recent {
		print := m.db.VoiceIn(r.ID, library.Me)
		if len(print) == 0 {
			continue
		}
		if err := m.db.Remember(name, print, store.Source{Recording: r.ID, Speaker: name}); err != nil {
			return "", err
		}
		if err := m.db.Rename(r.ID, library.Me, name); err != nil {
			return "", err
		}
		if taught++; taught >= store.Keep {
			break
		}
	}
	// The setting is authoritative for microphone turns; saved voiceprints still
	// help recognise the same person on the far side of a call.
	m.lib.Owner(name)
	m.mu.Lock()
	m.config.Me = name
	err = home.Save(m.dir, m.config)
	m.mu.Unlock()
	if err != nil {
		return "", err
	}
	if taught == 0 {
		return fmt.Sprintf("Your turns are named %s from now on. Nothing recorded yet has "+
			"enough of your voice to learn it from, so that part waits for the next meeting.", name), nil
	}
	return fmt.Sprintf("Your turns are named %s from now on, and the voice was learnt from %d %s.",
		name, taught, plural(taught, "recording")), nil
}

// People lists everybody the app can recognise by voice.
func (m *Meetings) People() ([]store.Person, error) {
	people, err := m.db.People()
	if err != nil || people == nil {
		return []store.Person{}, err
	}
	return people, nil
}

// Forget drops a learned person.
func (m *Meetings) Forget(name string) error { return m.db.Forget(name) }

// PaintPerson sets or clears a person's explicit colour.
func (m *Meetings) PaintPerson(name, colour string) error { return m.db.PaintPerson(name, colour) }

// Samples lists the evidence behind a learned person.
func (m *Meetings) Samples(name string) ([]store.Source, error) {
	people, err := m.db.People()
	if err != nil {
		return nil, err
	}
	out := []store.Source{}
	for _, p := range people {
		if p.Name != name {
			continue
		}
		for _, src := range p.Sources {
			if filled, ok := m.db.Sample(src); ok {
				out = append(out, filled)
			}
		}
	}
	return out, nil
}

// Appearances lists the groups a person appears in, busiest first.
func (m *Meetings) Appearances(name string) ([]store.Group, error) {
	return m.db.Appearances(name)
}

// Waveform returns the loudness envelope of the folded playback audio.
func (m *Meetings) Waveform(id int64) ([]float32, error) {
	name := m.db.Audio(id)
	if name == "" {
		return []float32{}, nil
	}
	listen, err := media.Listenable(filepath.Join(m.dir, "recordings", name),
		filepath.Join(m.dir, "cache"))
	if err != nil {
		return []float32{}, nil
	}
	if shape := media.Shape(listen); shape != nil {
		return shape, nil
	}
	return []float32{}, nil
}

// Analytics returns transcript-derived meeting metrics.
func (m *Meetings) Analytics(id int64) (*store.Analytics, error) {
	r, err := m.db.Get(id)
	if err != nil {
		return nil, err
	}
	turns, err := m.db.Turns(id)
	if err != nil {
		return nil, err
	}
	a := store.Analyse(turns, r.Duration)
	return &a, nil
}

// Reindex fills in any missing search vectors.
func (m *Meetings) Reindex() (string, error) {
	if err := m.reindex(context.Background(), true); err != nil {
		return "", err
	}
	with, without := m.db.Indexed()
	return fmt.Sprintf("Пошук за змістом оновлено: %d уривків шукаються за змістом, %d — лише за словами.",
		with, without), nil
}

// Tidy runs the audio-retention sweep now.
func (m *Meetings) Tidy() (string, error) {
	gone, freed, err := library.Sweep(m.db, filepath.Join(m.dir, "recordings"), m.config.Keep.AudioDays)
	switch {
	case err != nil:
		return "", err
	case m.config.Keep.AudioDays <= 0:
		return "Nothing was deleted: audio is set to be kept for ever.", nil
	case gone == 0:
		return "Nothing to delete — no audio is older than that yet.", nil
	}
	return fmt.Sprintf("Deleted %d %s, %d MB. The transcripts are untouched.",
		gone, plural(gone, "recording"), freed/(1<<20)), nil
}

func plural(n int, word string) string {
	if n == 1 {
		return word
	}
	return word + "s"
}

// Groups lists library groups.
func (m *Meetings) Groups() ([]store.Group, error) { return m.db.Groups() }

// NewGroup creates a group or returns the existing one with that name.
func (m *Meetings) NewGroup(name string) (store.Group, error) { return m.db.NewGroup(name) }

// File assigns a recording to a group, or clears the assignment when group is
// zero.
func (m *Meetings) File(recording, group int64) error {
	if err := m.db.Assign(recording, group); err != nil {
		return err
	}
	if group != 0 {
		go func() {
			if err := m.lib.Advance(context.Background(), recording); err != nil {
				slog.Warn("the project document did not move", "id", recording, "err", err)
			}
		}()
	}
	return nil
}

// RebuildProject replays every meeting into a fresh document. The answer to
// "this has gone wrong".
func (m *Meetings) RebuildProject(group int64) error {
	return m.lib.Rebuild(context.Background(), group)
}

// PinItem is a person editing a line of the project document; from then on the
// model may close it but never reword it.
func (m *Meetings) PinItem(group int64, id int, text, owner, due string) error {
	return m.db.Pin(group, id, text, owner, due)
}

// TickItem ticks a line of the project document off, or puts it back.
func (m *Meetings) TickItem(group int64, id int, done bool) error {
	return m.db.Tick(group, id, done)
}

// Span returns recent recordings for the timeline.
func (m *Meetings) Span(days int) ([]store.Mark, error) {
	if days <= 0 {
		days = 365
	}
	return m.db.Span(time.Now().AddDate(0, 0, -days), time.Now())
}

// Moment returns where a line occurred within a recording.
func (m *Meetings) Moment(recording int64, text string) float64 {
	return m.db.Moment(recording, text)
}

// Standing returns one group's current standing.
func (m *Meetings) Standing(group int64) (*store.Standing, error) { return m.db.Standing(group) }

// Loose reports how many recordings belong to no group.
func (m *Meetings) Loose() (int, error) { return m.db.Loose() }

// Paint sets or clears a group's explicit colour.
func (m *Meetings) Paint(id int64, colour string) error { return m.db.Paint(id, colour) }

// RenameGroup renames a group.
func (m *Meetings) RenameGroup(id int64, name string) error { return m.db.RenameGroup(id, name) }

// DropGroup removes a group and leaves its recordings unfiled.
func (m *Meetings) DropGroup(id int64) error { return m.db.DropGroup(id) }

// InGroup lists one group's recordings.
func (m *Meetings) InGroup(group int64) ([]store.Recording, error) {
	found, err := m.db.In(group, 200)
	if err != nil || found == nil {
		return []store.Recording{}, err
	}
	return found, nil
}

// Bin lists recordings currently in the bin.
func (m *Meetings) Bin() ([]store.Recording, error) {
	found, err := m.db.Bin()
	if err != nil || found == nil {
		return []store.Recording{}, err
	}
	return found, nil
}

// Restore takes a recording back out of the bin.
func (m *Meetings) Restore(id int64) error { return m.lib.Restore(id) }

// EmptyBin permanently deletes everything currently in the bin.
func (m *Meetings) EmptyBin() (string, error) {
	gone, err := m.lib.Empty(0)
	if err != nil {
		return "", err
	}
	if gone == 0 {
		return "The bin was already empty.", nil
	}
	return fmt.Sprintf("Deleted %d %s for good.", gone, plural(gone, "recording")), nil
}

// Brief returns the cross-meeting briefing view.
func (m *Meetings) Brief(days int) (*store.Briefing, error) { return m.db.Brief(days) }

// Live returns the current live transcript.
func (m *Meetings) Live() []listen.Line {
	if m.ears == nil {
		return []listen.Line{}
	}
	if said := m.ears.Said(); said != nil {
		return said
	}
	return []listen.Line{}
}

// Again retranscribes a recording from scratch.
func (m *Meetings) Again(id int64) error { return m.lib.Again(id) }

// Summarise reruns summary generation for a recording.
func (m *Meetings) Summarise(id int64) error {
	return m.lib.Summarise(context.Background(), id)
}

// SaveNote stores a note against a meeting.
func (m *Meetings) SaveNote(id int64, note string) error {
	return m.db.SaveNote(id, note)
}

// Delete buries a meeting.
func (m *Meetings) Delete(id int64) error {
	return m.lib.Delete(id)
}

// Import copies a file into the recordings folder and queues it.
func (m *Meetings) Import(path string) (*store.Recording, error) {
	source, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer source.Close()

	// Preserve the recording's own timestamp when it looks older than now;
	// project replay order depends on it.
	when := time.Now()
	if info, err := source.Stat(); err == nil && info.ModTime().Before(when) {
		when = info.ModTime()
	}

	name := fmt.Sprintf("%s-%s", when.Format("2006-01-02T15-04-05"), filepath.Base(path))
	destination := filepath.Join(home.Recordings(m.dir), name)
	target, err := os.Create(destination)
	if err != nil {
		return nil, err
	}
	if _, err := target.ReadFrom(source); err != nil {
		target.Close()
		os.Remove(destination)
		return nil, err
	}
	target.Close()

	r, err := m.lib.Add(store.Meeting, name, when, filepath.Base(path))
	return &r, err
}

// Redate corrects when a meeting really happened.
func (m *Meetings) Redate(recording int64, when string) error {
	at, err := time.Parse(time.RFC3339, when)
	if err != nil {
		return fmt.Errorf("%q is not a date the app understands: %w", when, err)
	}
	if err := m.db.Redate(recording, at); err != nil {
		return err
	}
	// Project state depends on chronological replay order, so date changes
	// require a rebuild.
	r, err := m.db.Get(recording)
	if err != nil || r.Group == 0 {
		return err
	}
	go func() {
		if err := m.lib.Rebuild(context.Background(), r.Group); err != nil {
			slog.Warn("the project document did not rebuild after a date changed",
				"recording", recording, "err", err)
		}
	}()
	return nil
}

// Settings is the payload the settings screen reads and writes.
type Settings struct {
	Language    string `json:"language"`
	OpenAIKey   string `json:"openaiKey"`
	OpenAIModel string `json:"openaiModel"`
	Summarise   string `json:"summarise"`
	KeepNotes   bool   `json:"keepNotes"`
	Density     string `json:"density"`

	Listening bool `json:"listening"`
	// System captures machine audio alongside the microphone.
	System      bool `json:"system"`
	StartSpeech int  `json:"startSpeech"` // seconds of talking before it records
	QuietEnds   int  `json:"quietEnds"`   // seconds of silence that end it
	Preroll     int  `json:"preroll"`     // seconds it reaches back when it starts

	KeepAudioDays int `json:"keepAudioDays"` // 0 keeps recordings for ever

	AIProvider   string `json:"aiProvider"`   // "openai", "copilot" or "local"
	CopilotModel string `json:"copilotModel"` // empty lets Copilot choose
	LocalModel   string `json:"localModel"`   // a .gguf link; empty is the built-in model
	Embeddings   string `json:"embeddings"`   // "openai" or "local"

	Transcribe   string `json:"transcribe"`   // "after", "at" or "idle"
	TranscribeAt string `json:"transcribeAt"` // "19:00", for "at"

	Folder string `json:"folder"`
}

func (m *Meetings) Settings() Settings {
	m.mu.Lock()
	defer m.mu.Unlock()
	return Settings{
		AIProvider:    m.config.AI.Provider,
		CopilotModel:  m.config.AI.CopilotModel,
		LocalModel:    m.config.AI.LocalModel,
		Embeddings:    m.config.AI.Embeddings,
		Transcribe:    m.config.Queue.When,
		TranscribeAt:  m.config.Queue.At,
		Language:      m.config.Language,
		OpenAIKey:     m.config.OpenAIKey,
		OpenAIModel:   m.config.OpenAIModel,
		Summarise:     string(m.config.Summarise),
		KeepNotes:     m.config.Listen.KeepNotes,
		Density:       m.config.Density,
		Listening:     m.config.Listen.Enabled,
		System:        m.config.Listen.System,
		StartSpeech:   int(m.config.Listen.StartSpeech.Seconds()),
		QuietEnds:     int(m.config.Listen.QuietEnds.Seconds()),
		Preroll:       int(m.config.Listen.Preroll.Seconds()),
		KeepAudioDays: m.config.Keep.AudioDays,
		Folder:        m.dir,
	}
}

// SaveSettings writes settings back to disk. AI changes apply at once.
func (m *Meetings) SaveSettings(s Settings) error {
	if !slices.Contains(home.Providers, s.AIProvider) || !slices.Contains(home.Embedders, s.Embeddings) {
		return fmt.Errorf("невідомий вибір AI: %q, %q", s.AIProvider, s.Embeddings)
	}
	if s.LocalModel = strings.TrimSpace(s.LocalModel); s.LocalModel != "" {
		if _, err := models.Custom(s.LocalModel); err != nil {
			return fmt.Errorf("посилання на модель має вести на файл .gguf на Hugging Face: %w", err)
		}
	}
	if !slices.Contains(home.Whens, s.Transcribe) {
		return fmt.Errorf("невідомий вибір, коли розшифровувати: %q", s.Transcribe)
	}
	if _, err := time.Parse("15:04", s.TranscribeAt); err != nil {
		return fmt.Errorf("час розшифровки має виглядати як 19:00, а не %q", s.TranscribeAt)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	before := m.aiInputs()
	m.config.AI = home.AI{Provider: s.AIProvider, CopilotModel: s.CopilotModel,
		LocalModel: s.LocalModel, Embeddings: s.Embeddings}
	m.config.Queue = home.Queue{When: s.Transcribe, At: s.TranscribeAt}
	m.lib.Schedule(s.Transcribe, s.TranscribeAt)
	m.config.Language = s.Language
	m.config.OpenAIKey = s.OpenAIKey
	m.config.OpenAIModel = s.OpenAIModel
	m.config.Summarise = home.Choice(s.Summarise)
	m.config.Listen.KeepNotes = s.KeepNotes
	m.lib.Policy(store.When(s.Summarise))
	if m.ears != nil {
		m.ears.KeepNotes(s.KeepNotes)
	}
	m.config.Density = s.Density
	m.config.Listen.Enabled = s.Listening
	m.config.Listen.System = s.System
	m.config.Keep.AudioDays = s.KeepAudioDays
	// Pause rather than tear the recorder down; reopening devices would retrigger
	// a slow permissioned path.
	if m.ears != nil {
		m.ears.Pause(!s.Listening)
		m.ears.Hear(s.System)
	}

	// Guard UI-provided timing values before handing them to the recorder.
	m.config.Listen.StartSpeech = home.Duration{Duration: seconds(s.StartSpeech, 20, 5, 300)}
	m.config.Listen.QuietEnds = home.Duration{Duration: seconds(s.QuietEnds, 180, 15, 1800)}
	m.config.Listen.Preroll = home.Duration{Duration: seconds(s.Preroll, 300, 0, 600)}
	if m.config.Listen.Preroll.Duration > m.config.Listen.Ring.Duration {
		m.config.Listen.Ring = m.config.Listen.Preroll
	}
	if err := home.Save(m.dir, m.config); err != nil {
		return err
	}
	if m.aiInputs() != before {
		go m.ApplyAI() // after the lock is released
	}
	return nil
}

// aiInputs is every setting the AI client is built from. Callers hold m.mu.
func (m *Meetings) aiInputs() [4]any {
	return [4]any{m.config.AI, m.config.OpenAIKey, m.config.OpenAIModel, m.config.Language}
}

// seconds applies defaults and clamps UI-provided durations.
func seconds(given, fallback, low, high int) time.Duration {
	if given <= 0 {
		given = fallback
	}
	return time.Duration(min(max(given, low), high)) * time.Second
}

// RevealFolder opens the app folder in the platform file manager.
func (m *Meetings) RevealFolder() error {
	command := map[string]string{"darwin": "open", "windows": "explorer"}[runtime.GOOS]
	if command == "" {
		command = "xdg-open"
	}
	return exec.Command(command, m.dir).Start()
}

// Actions lists outstanding action items across meetings.
func (m *Meetings) Actions(includeDone bool) ([]store.Outstanding, error) {
	return m.db.Actions(includeDone)
}

// Tick marks one action item done or undone.
func (m *Meetings) Tick(id int64, index int, done bool) error {
	return m.db.TickAction(id, index, done)
}

// Markdown renders a whole meeting for export.
func (m *Meetings) Markdown(id int64) (string, error) {
	r, err := m.db.Get(id)
	if err != nil {
		return "", err
	}
	turns, err := m.db.Turns(id)
	if err != nil {
		return "", err
	}

	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n_%s · %s_\n\n", r.Title,
		r.Started.Format("2 January 2006, 15:04"), length(r.Duration))

	if s := r.Summary; s != nil {
		if s.Overview != "" {
			fmt.Fprintf(&b, "%s\n\n", s.Overview)
		}
		section(&b, "Decided", s.Decisions)
		if len(s.ActionItems) > 0 {
			b.WriteString("## To do\n\n")
			for _, a := range s.ActionItems {
				mark := " "
				if a.Done {
					mark = "x"
				}
				fmt.Fprintf(&b, "- [%s] %s", mark, a.Task)
				if a.Owner != "" {
					fmt.Fprintf(&b, " — **%s**", a.Owner)
				}
				if a.Due != "" {
					fmt.Fprintf(&b, " (%s)", a.Due)
				}
				b.WriteString("\n")
			}
			b.WriteString("\n")
		}
		section(&b, "Left open", s.OpenQuestions)
	}

	b.WriteString("## Transcript\n\n")
	for _, t := range turns {
		who := t.Speaker
		if who == "" {
			who = "—"
		}
		fmt.Fprintf(&b, "**%s** `%s` %s\n\n", who, media.Clock(t.Start), t.Text)
	}
	return b.String(), nil
}

func section(b *strings.Builder, title string, lines []string) {
	if len(lines) == 0 {
		return
	}
	fmt.Fprintf(b, "## %s\n\n", title)
	for _, l := range lines {
		fmt.Fprintf(b, "- %s\n", l)
	}
	b.WriteString("\n")
}

func length(seconds float64) string {
	m := int(seconds / 60)
	if m < 60 {
		return fmt.Sprintf("%d min", m)
	}
	return fmt.Sprintf("%d hr %d min", m/60, m%60)
}

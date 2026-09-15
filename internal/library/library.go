// Package library owns the transcription queue and transcript-derived
// artifacts.
package library

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/dmykolen/meeting-transcriber-go/internal/engine"
	"github.com/dmykolen/meeting-transcriber-go/internal/insights"
	"github.com/dmykolen/meeting-transcriber-go/internal/media"
	"github.com/dmykolen/meeting-transcriber-go/internal/store"
)

// Engine is the transcription boundary the library needs.
type Engine interface {
	Run(heard, apart []float32) (engine.Result, error)
	Solo(samples []float32, who string) (engine.Result, error)
}

// Me is the provisional label for the laptop owner.
const Me = "You"

type Library struct {
	db   *store.DB
	llm  *insights.Client
	dir  string
	wake chan struct{}

	// The engine may arrive after startup while models are still downloading.
	mu     sync.Mutex
	engine Engine
	busy   func() bool
	policy store.When
	owner  string
}

// Owner sets the laptop owner's display name.
func (l *Library) Owner(name string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.owner = name
}

func (l *Library) who() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.owner
}

// Policy sets which recordings are worth summarising.
func (l *Library) Policy(when store.When) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.policy = when
}

// Wait tells the queue when to stand aside.
func (l *Library) Wait(busy func() bool) {
	l.mu.Lock()
	l.busy = busy
	l.mu.Unlock()
}

func (l *Library) waiting() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.busy != nil && l.busy()
}

func New(db *store.DB, e Engine, llm *insights.Client, recordings string) *Library {
	return &Library{db: db, engine: e, llm: llm, dir: recordings, wake: make(chan struct{}, 1)}
}

// Use installs the engine once models have loaded and wakes the queue.
func (l *Library) Use(e Engine) {
	l.mu.Lock()
	l.engine = e
	l.mu.Unlock()
	l.Wake()
}

func (l *Library) ready() Engine {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.engine
}

// Add queues a file already in the recordings folder.
func (l *Library) Add(kind store.Kind, audio string, started time.Time, title string) (store.Recording, error) {
	if title == "" {
		title = filepath.Base(audio)
	}
	r, err := l.db.Add(store.Recording{
		Kind:    kind,
		Title:   title,
		Audio:   filepath.Base(audio),
		Started: started,
	})
	if err == nil {
		l.Wake()
	}
	return r, err
}

// Wake asks the worker to check the queue now.
func (l *Library) Wake() {
	select {
	case l.wake <- struct{}{}:
	default:
	}
}

// Run works through the queue until the context is cancelled.
func (l *Library) Run(ctx context.Context) {
	for {
		for l.ready() != nil && !l.waiting() {
			id, ok := l.next()
			if !ok {
				break
			}
			if err := l.process(ctx, id); err != nil {
				slog.Error("recording failed", "id", id, "err", err)
				_ = l.db.Fail(id, err)
			}
			if ctx.Err() != nil {
				return
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-l.wake:
		case <-time.After(30 * time.Second):
		}
	}
}

// next returns the oldest unfinished recording.
func (l *Library) next() (int64, bool) {
	recent, err := l.db.Recent(200)
	if err != nil {
		slog.Error("cannot read the queue", "err", err)
		return 0, false
	}
	for i := len(recent) - 1; i >= 0; i-- {
		switch recent[i].Status {
		case store.Done, store.Failed:
			continue
		default:
			return recent[i].ID, true
		}
	}
	return 0, false
}

// process runs the full pipeline for one recording.
func (l *Library) process(ctx context.Context, id int64) error {
	r, err := l.db.Get(id)
	if err != nil {
		return err
	}
	path := filepath.Join(l.dir, r.Audio)
	started := time.Now()

	if err := l.db.Progress(id, store.Transcribing, 0.05); err != nil {
		return err
	}
	// Folded, not averaged: averaging keeps echoed far-side speech in-band.
	samples, err := media.Voices(path)
	if err != nil {
		return fmt.Errorf("could not read the audio: %w", err)
	}
	seconds := float64(len(samples)) / media.Rate
	slog.Info("transcribing", "id", id, "minutes", seconds/60)

	// The microphone side is the local speaker; the system side is everybody
	// else.
	mic, system, stereo := media.Sides(path)
	solo := stereo && media.Loud(system) < media.Loud(mic)/64

	var read engine.Result
	if solo {
		// Skip diarization when the far side is effectively silent.
		slog.Info("one voice only: the system channel is silent", "id", id)
		read, err = l.ready().Solo(samples, Me)
	} else {
		read, err = l.ready().Run(samples, system)
	}
	turns := read.Turns
	if stereo && !solo {
		read = mine(read, mic, system)
	}
	if !solo {
		read.Turns = settle(read.Turns)
	}
	turns = read.Turns
	if err != nil && len(turns) == 0 {
		return err
	}
	if err != nil {
		slog.Warn("speakers unavailable", "id", id, "err", err)
	}

	// Apply any previously learned speaker names before writing the transcript.
	people, err := l.db.People()
	if err != nil {
		slog.Warn("could not read the voices", "err", err)
	}

	// The clusterer is intentionally biased toward over-splitting; named voices
	// can be merged back later.
	same := store.Same(read.Voices, people)
	// Never fold the microphone owner label away.
	delete(same, Me)
	if len(same) > 0 {
		for i := range turns {
			if to, split := same[turns[i].Speaker]; split {
				turns[i].Speaker = to
			}
		}
		for from := range same {
			delete(read.Voices, from)
		}
		slog.Info("clusters rejoined", "id", id, "merged", len(same))
	}

	if names := store.Recognise(enough(read.Voices, turns), people); len(names) > 0 {
		for i := range turns {
			if name, known := names[turns[i].Speaker]; known {
				turns[i].Speaker = name
			}
		}
		for label, name := range names {
			// Another sample of a known voice improves later recognition.
			if err := l.db.Remember(name, read.Voices[label],
				store.Source{Recording: id, Speaker: name}); err != nil {
				slog.Warn("could not file a voice", "name", name, "err", err)
			}
			read.Voices[name] = read.Voices[label]
			delete(read.Voices, label)
		}
		slog.Info("voices recognised", "id", id, "who", values(names))
	}

	// The microphone owner is named from settings, not recognition.
	if owner := l.who(); owner != "" {
		for i := range turns {
			if turns[i].Speaker == Me {
				turns[i].Speaker = owner
			}
		}
		if print, have := read.Voices[Me]; have {
			read.Voices[owner] = print
			delete(read.Voices, Me)
		}
	}

	// Store voiceprints under the label that ended up in the transcript.
	if err := l.db.SaveVoices(id, read.Voices); err != nil {
		slog.Warn("could not keep the voices of this recording", "id", id, "err", err)
	}

	if len(turns) == 0 {
		// Silence is still a finished recording, not a failure.
		slog.Info("nothing was said", "id", id)
	}

	if err := l.db.SaveTranscript(id, "", seconds, convert(turns)); err != nil {
		return err
	}
	slog.Info("transcribed", "id", id, "rows", len(turns),
		"pace", fmt.Sprintf("%.1fx realtime", seconds/time.Since(started).Seconds()))

	l.index(ctx, id, convert(turns))
	return l.summarise(ctx, id, turns)
}

// index cuts a transcript into searchable passages and embeds them when
// possible.
func (l *Library) index(ctx context.Context, id int64, turns []store.Turn) {
	pieces := store.Cut(id, turns)
	if len(pieces) == 0 {
		return
	}
	var vectors [][]float32
	if l.llm.Ready() {
		texts := make([]string, len(pieces))
		for i, p := range pieces {
			texts[i] = p.Text
		}
		var err error
		if vectors, err = l.llm.Embed(ctx, texts); err != nil {
			slog.Warn("indexed for keywords only", "id", id, "err", err)
		}
	}
	if err := l.db.Index(id, pieces, vectors); err != nil {
		slog.Warn("could not index the transcript", "id", id, "err", err)
	}
}

// Reindex embeds recordings that are still missing vectors.
func (l *Library) Reindex(ctx context.Context) (int, error) {
	if !l.llm.Ready() {
		return 0, errors.New("indexing needs an OpenAI key, which is in Settings")
	}
	ids, err := l.db.Stale(500)
	if err != nil {
		return 0, err
	}
	for _, id := range ids {
		turns, err := l.db.Turns(id)
		if err != nil {
			return 0, err
		}
		l.index(ctx, id, turns)
	}
	return len(ids), nil
}

// Evidence is the minimum speech duration required before assigning a learned
// name.
const Evidence = 30.0

// enough keeps only voiceprints eligible for naming.
func enough(prints map[string][]float32, turns []engine.Turn) map[string][]float32 {
	held := map[string]float64{}
	for _, t := range turns {
		held[t.Speaker] += t.End - t.Start
	}
	out := map[string][]float32{}
	for label, print := range prints {
		if label != Me && held[label] >= Evidence {
			out[label] = print
		}
	}
	return out
}

// mine folds microphone-dominant turns into one speaker.
func mine(read engine.Result, mic, system []float32) engine.Result {
	loudest, best := "", 0.0
	for i, t := range read.Turns {
		a, b := int(t.Start*media.Rate), int(t.End*media.Rate)
		if a < 0 || b > len(mic) || b <= a {
			continue
		}
		if media.Loud(mic[a:b]) > 3*media.Loud(system[a:b]) {
			if length := t.End - t.Start; length > best {
				loudest, best = read.Turns[i].Speaker, length
			}
			read.Turns[i].Speaker = Me
		}
	}
	if loudest == "" {
		return read
	}
	// Keep one voiceprint for the local speaker and drop folded-away labels.
	if print, have := read.Voices[loudest]; have {
		read.Voices[Me] = print
	}
	for label := range read.Voices {
		if label != Me && !stillUsed(read.Turns, label) {
			delete(read.Voices, label)
		}
	}
	return read
}

// Scrap is the maximum speech a disposable fragment cluster may hold.
const Scrap = 10.0

// settle absorbs tiny clusters into neighbouring substantial speakers.
func settle(turns []engine.Turn) []engine.Turn {
	held, total := map[string]float64{}, 0.0
	for _, t := range turns {
		held[t.Speaker] += t.End - t.Start
		total += t.End - t.Start
	}
	if total < 10*60 || len(held) < 3 {
		return turns
	}
	for i, t := range turns {
		if t.Speaker == "" || t.Speaker == Me || held[t.Speaker] > Scrap {
			continue
		}
		// Reassign to the nearest substantial neighbour in the contiguous stream.
		if near := neighbour(turns, i, held); near != "" {
			turns[i].Speaker = near
		}
	}
	return turns
}

func neighbour(turns []engine.Turn, at int, held map[string]float64) string {
	for step := 1; step < len(turns); step++ {
		for _, i := range [2]int{at - step, at + step} {
			if i >= 0 && i < len(turns) && held[turns[i].Speaker] > Scrap {
				return turns[i].Speaker
			}
		}
	}
	return ""
}

func stillUsed(turns []engine.Turn, label string) bool {
	for _, t := range turns {
		if t.Speaker == label {
			return true
		}
	}
	return false
}

// worth decides whether a recording should be summarised.
func (l *Library) worth(id int64, turns []engine.Turn) bool {
	l.mu.Lock()
	policy := l.policy
	l.mu.Unlock()

	if policy == store.Always {
		return true
	}
	if policy == store.Never {
		return false
	}
	// Meetings-only mode requires speech from someone other than the laptop
	// owner.
	r, err := l.db.Get(id)
	if err != nil {
		return true // rather summarise than lose one to a database error
	}
	if r.Kind == store.Meeting {
		return true
	}
	slog.Info("not summarised: nobody else was in it", "id", id, "policy", policy)
	return false
}

// Again reprocesses a finished recording from its audio.
func (l *Library) Again(id int64) error {
	r, err := l.db.Get(id)
	if err != nil {
		return err
	}
	if r.Audio == "" {
		return errors.New("the audio has been deleted, so there is nothing left to transcribe")
	}
	if _, err := os.Stat(filepath.Join(l.dir, r.Audio)); err != nil {
		return fmt.Errorf("the recording %s is not in the folder any more", r.Audio)
	}
	if err := l.db.Progress(id, store.Queued, 0); err != nil {
		return err
	}
	l.Wake()
	return nil
}

// Summarise reruns summary generation without retranscribing audio.
func (l *Library) Summarise(ctx context.Context, id int64) error {
	if !l.llm.Ready() {
		return errors.New("summaries need an OpenAI key, which is in Settings")
	}
	rows, err := l.db.Turns(id)
	if err != nil {
		return err
	}
	turns := make([]engine.Turn, len(rows))
	for i, r := range rows {
		turns[i] = engine.Turn{Start: r.Start, End: r.End, Speaker: r.Speaker, Text: r.Text}
	}
	if len(turns) == 0 {
		return errors.New("there is no transcript to summarise")
	}
	return l.summarise(ctx, id, turns)
}

func values(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for _, v := range m {
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

// summarise adds summary artifacts without failing an otherwise usable
// recording.
func (l *Library) summarise(ctx context.Context, id int64, turns []engine.Turn) error {
	if !l.llm.Ready() || len(turns) == 0 || !l.worth(id, turns) {
		return l.db.Progress(id, store.Done, 1)
	}
	if err := l.db.Progress(id, store.Summarising, 0.9); err != nil {
		return err
	}

	said := make([]insights.Turn, len(turns))
	for i, t := range turns {
		said[i] = insights.Turn{Start: t.Start, Speaker: t.Speaker, Text: t.Text}
	}
	summary, err := l.llm.Summarise(ctx, said)
	if err != nil {
		slog.Warn("no summary", "id", id, "err", err)
		return l.db.Progress(id, store.Done, 1)
	}
	if err := l.db.SaveSummary(id, translate(summary)); err != nil {
		return err
	}
	slog.Info("summarised", "id", id, "title", summary.Title, "actions", len(summary.ActionItems))
	// Advance the project document, but do not fail the recording if that step
	// breaks.
	if err := l.Advance(ctx, id); err != nil {
		slog.Warn("the project document did not move", "id", id, "err", err)
	}
	return l.db.Progress(id, store.Done, 1)
}

// Ask answers a question from the transcripts and returns its sources.
func (l *Library) Ask(ctx context.Context, question string) (string, []store.Hit, error) {
	hits, err := l.Find(ctx, question, 20)
	if err != nil {
		return "", nil, err
	}
	if len(hits) == 0 {
		return "", nil, errors.New("nothing in the transcripts covers that")
	}
	answer, err := l.llm.Answer(ctx, question, store.Passages(hits))
	return answer, hits, err
}

// Find searches both keyword and semantic indexes when available.
func (l *Library) Find(ctx context.Context, query string, limit int) ([]store.Hit, error) {
	words, err := l.db.Search(query, limit)
	if err != nil {
		return nil, err
	}
	if !l.llm.Ready() {
		return words, nil
	}
	vectors, err := l.llm.Embed(ctx, []string{query})
	if err != nil || len(vectors) == 0 {
		slog.Debug("keyword search only", "err", err)
		return words, nil
	}
	near, err := l.db.Closest(vectors[0], limit)
	if err != nil {
		return words, nil
	}
	return blend(words, near, limit), nil
}

// blend interleaves keyword and semantic hits without rescoring across systems.
func blend(words, near []store.Hit, limit int) []store.Hit {
	seen := map[string]bool{}
	out := []store.Hit{}
	key := func(h store.Hit) string { return fmt.Sprintf("%d@%.0f", h.Recording, h.Start) }

	for i := 0; len(out) < limit && (i < len(near) || i < len(words)); i++ {
		for _, list := range [][]store.Hit{near, words} {
			if i < len(list) && !seen[key(list[i])] && len(out) < limit {
				seen[key(list[i])] = true
				out = append(out, list[i])
			}
		}
	}
	return out
}

// Delete moves a recording to the bin.
func (l *Library) Delete(id int64) error { return l.db.Bury(id) }

// Restore takes a recording back out of the bin.
func (l *Library) Restore(id int64) error { return l.db.Restore(id) }

// Empty permanently deletes buried recordings older than the given age.
func (l *Library) Empty(olderThan time.Duration) (int, error) {
	buried, err := l.db.Buried(olderThan)
	if err != nil {
		return 0, err
	}
	for _, r := range buried {
		if _, err := l.db.Delete(r.ID); err != nil {
			return 0, err
		}
		if r.Audio != "" {
			_ = os.Remove(filepath.Join(l.dir, r.Audio))
			_ = os.Remove(filepath.Join(filepath.Dir(l.dir), "cache", r.Audio))
		}
	}
	if len(buried) > 0 {
		slog.Info("bin emptied", "recordings", len(buried))
	}
	return len(buried), nil
}

// convert is the engine-to-store boundary and sorts by start time.
func convert(turns []engine.Turn) []store.Turn {
	out := make([]store.Turn, len(turns))
	for i, t := range turns {
		out[i] = store.Turn{Start: t.Start, End: t.End, Speaker: t.Speaker, Text: t.Text}
	}
	sort.SliceStable(out, func(a, b int) bool { return out[a].Start < out[b].Start })
	return out
}

func translate(s *insights.Summary) *store.Summary {
	out := &store.Summary{
		Title:         s.Title,
		Overview:      s.Overview,
		Topics:        s.Topics,
		Decisions:     s.Decisions,
		OpenQuestions: s.OpenQuestions,
	}
	for _, c := range s.Chapters {
		out.Chapters = append(out.Chapters, store.Chapter{Start: c.Start, Title: c.Title, Summary: c.Summary})
	}
	for _, a := range s.ActionItems {
		out.ActionItems = append(out.ActionItems, store.Action{Task: a.Task, Owner: a.Owner, Due: a.Due})
	}
	return out
}

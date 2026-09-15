// Package listen records meetings from an always-on stereo capture loop.
package listen

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/dmykolen/meeting-transcriber-go/internal/audio"
	"github.com/dmykolen/meeting-transcriber-go/internal/home"
)

// Phase is the capture state shown in the UI.
type Phase string

const (
	Off        Phase = "off"     // listening is switched off in the settings
	Opening    Phase = "opening" // waiting for devices, often on a permission dialog
	Listening  Phase = "listening"
	Recording  Phase = "recording"
	WrappingUp Phase = "wrapping up" // it has gone quiet, and this may be the end
	Paused     Phase = "paused"
	Broken     Phase = "broken"
)

// Status is the polled UI snapshot.
type Status struct {
	Phase   Phase  `json:"phase"`
	Kind    Kind   `json:"kind"`
	Elapsed int    `json:"elapsed"` // seconds of the current recording
	Quiet   int    `json:"quiet"`   // seconds since anybody last spoke
	System  bool   `json:"system"`  // is the other side being captured at all
	Problem string `json:"problem"`
}

// Done handles a finished recording.
type Done func(path string, kind Kind, started time.Time)

// Threw reports a discarded recording.
type Threw func(seconds float64, why string)

// stamp is the human-facing recording filename layout.
const stamp = "2006-01-02 15-04"

// partial marks a file that was still being written.
const partial = ".part"

// Recorder is the capture loop.
type Recorder struct {
	dir       string
	model     string
	ring      *Ring
	detector  *Detector
	preroll   time.Duration
	keepNotes bool
	done      Done
	threw     Threw

	mic, sys *Ears
	scribe   *Scribe
	// Mic-only “someone else is here” detector used when system audio is off.
	company *Company

	// Hold quiet frames until the detector decides whether they belong.
	pending [][]int16

	// UI-facing state, read from another goroutine.
	mu     sync.Mutex
	writer *Writer
	kind   Kind
	begun  time.Time
	// The preroll must never reach back past the previous finished recording.
	ended  time.Time
	asked  bool // somebody pressed Record, rather than the app deciding
	paused bool
	// Muffle suppresses the app's own playback on the system channel.
	muffled bool
	// Changing system capture reopens devices instead of waiting for restart.
	system bool
	want   *bool // Record now / Stop, waiting to be applied by the capture loop
	snap   Status
}

// Write turns one utterance into words. Nil disables live transcript.
type Write func([]float32) (string, error)

// New wires the recorder from config. Devices are opened later in Run.
func New(recordings, vadModel string, cfg home.Listen, done Done, threw Threw, write Write, voice Voice) *Recorder {
	return &Recorder{
		dir:       recordings,
		model:     vadModel,
		ring:      NewRing(cfg.Ring.Duration),
		preroll:   cfg.Preroll.Duration,
		keepNotes: cfg.KeepNotes,
		system:    cfg.System,
		done:      done,
		threw:     threw,
		detector: NewTunedDetector(Tuning{
			StartSpeech:  cfg.StartSpeech.Duration,
			QuietMeeting: cfg.QuietEnds.Duration,
			QuietNote:    cfg.QuietEnds.Duration / 3,
		}),
		scribe:  NewScribe(write),
		company: NewCompany(voice),
		snap:    Status{Phase: Opening},
	}
}

// KeepNotes updates note retention without restart.
func (r *Recorder) KeepNotes(keep bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.keepNotes = keep
}

// Said returns the live transcript of the current recording.
func (r *Recorder) Said() []Line { return r.scribe.Said() }

// Recover removes stale partial recordings from a previous crash.
func Recover(dir string) {
	stale, _ := filepath.Glob(filepath.Join(dir, "*"+partial))
	for _, path := range stale {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		slog.Warn("discarding a recording that was interrupted mid-write",
			"file", filepath.Base(path),
			"audio", (time.Duration(info.Size()/4) * time.Second / audio.SampleRate).Round(time.Second))
		_ = os.Remove(path)
	}
}

// Run opens devices and records until ctx is cancelled.
func (r *Recorder) Run(ctx context.Context) error {
	var err error
	if r.mic, err = Listen(r.model); err != nil {
		r.fail(err)
		return err
	}
	defer r.mic.Close()
	if r.sys, err = Listen(r.model); err != nil {
		r.fail(err)
		return err
	}
	defer r.sys.Close()

	for {
		if err := r.hear(ctx); err != nil {
			return err
		}
		if ctx.Err() != nil {
			return nil
		}
		// hear returned because the system-audio setting changed.
	}
}

// hear reads one device configuration until ctx ends or the system-audio
// setting changes.
func (r *Recorder) hear(ctx context.Context) error {
	system := r.System()
	stream, err := audio.Open(ctx, system)
	if err != nil {
		r.fail(err)
		return err
	}
	defer stream.Close()
	defer r.finish("devices reopening")

	live := stream.SystemAudio
	r.mu.Lock()
	r.snap = Status{Phase: Listening, System: live}
	r.mu.Unlock()
	slog.Info("listening", "system_audio", live, "wanted", system)

	left := make([]int16, audio.FrameSize)
	right := make([]int16, audio.FrameSize)

	for {
		select {
		case <-ctx.Done():
			return nil
		case frame, ok := <-stream.Frames():
			if !ok {
				return nil
			}
			if r.System() != system {
				return nil // the setting moved; Run opens again
			}
			if r.Paused() {
				continue
			}
			for i := range audio.FrameSize {
				left[i], right[i] = frame[2*i], frame[2*i+1]
			}
			// Ignore the app's own playback when asked.
			if r.Muffled() {
				for i := range audio.FrameSize {
					right[i], frame[2*i+1] = 0, 0
				}
			}
			if err := r.step(frame, left, right, live); err != nil {
				r.fail(err)
				return err
			}
		}
	}
}

// step handles one captured frame.
func (r *Recorder) step(frame, left, right []int16, system bool) error {
	// Touch the detector from one goroutine only.
	r.mu.Lock()
	if want := r.want; want != nil {
		r.want = nil
		r.mu.Unlock()
		r.detector.Force(*want)
	} else {
		r.mu.Unlock()
	}

	speaking, mine, err := r.mic.Speaking(left)
	if err != nil {
		return err
	}
	others, theirs, err := r.sys.Speaking(right)
	if err != nil {
		return err
	}
	if r.writer != nil {
		// Only while something is being recorded: transcribing the room at
		// large, all day, would be a different and much ruder application.
		for _, u := range mine {
			r.scribe.Hear("you", u)
			// Without the system channel this is the only way to learn that
			// somebody else is here. With it, the answer is already known and
			// an embedding per utterance would be work for nothing.
			if !system && r.company.Heard(u.Samples) {
				r.detector.Company()
			}
		}
		for _, u := range theirs {
			r.scribe.Hear("them", u)
		}
	}
	r.ring.Add(frame, speaking || others)

	switch r.detector.Feed(speaking, others) {
	case Started:
		if err := r.begin(); err != nil {
			return err
		}
	case Rolled:
		if err := r.flush(nil); err != nil {
			return err
		}
		if err := r.finish("hourly split"); err != nil {
			return err
		}
		// The next file starts now, not in the past: the preroll belongs to the
		// beginning of the meeting, and it is already in the previous file.
		if err := r.open(0); err != nil {
			return err
		}
	case Ended:
		if err := r.finish("quiet"); err != nil {
			return err
		}
	}
	r.observe(system)

	// A note that turns out to have somebody on the other end was a meeting all
	// along, and the file has to follow.
	if _, kind := r.detector.Recording(); r.writer != nil && kind != r.kind {
		slog.Info("this is a conversation after all", "was", r.kind, "now", kind)
		r.mu.Lock()
		r.kind = kind
		r.mu.Unlock()
	}
	if r.writer == nil {
		return nil
	}
	if !speaking && !others {
		r.pending = append(r.pending, append([]int16(nil), frame...))
		return nil
	}
	return r.flush(frame)
}

// flush writes everything that was held back, then the frame that broke the
// silence. A pause inside a meeting belongs in the recording; the silence after
// the meeting does not, and only hindsight tells them apart.
func (r *Recorder) flush(frame []int16) error {
	for _, held := range r.pending {
		if err := r.writer.Write(held); err != nil {
			return err
		}
	}
	r.pending = r.pending[:0]
	if frame == nil {
		return nil
	}
	return r.writer.Write(frame)
}

// begin opens a recording and pours the ring into it, so that the meeting
// starts where it actually started rather than where it was noticed.
func (r *Recorder) begin() error {
	// The ring is asked first: the file name carries the start time, and the
	// ring may hold less than the preroll asks for.
	// Never reach back into a recording already filed. Two meetings minutes
	// apart otherwise share the minutes between them: the same words summarised
	// twice, two rows for one conversation, and both rows on one file.
	r.mu.Lock()
	back := reach(r.preroll, r.ended, time.Now())
	r.mu.Unlock()

	replay := r.ring.ReplaySpeech(back)
	if err := r.open(time.Duration(len(replay)) * frameDuration); err != nil {
		return err
	}
	for _, frame := range replay {
		if err := r.writer.Write(frame); err != nil {
			return err
		}
	}
	r.scribe.Start()
	slog.Info("recording started", "kind", r.kind,
		"replayed", (time.Duration(len(replay)) * frameDuration).Round(time.Second),
		"preroll_capped_by_previous", back < r.preroll)
	return nil
}

// reach is how far back a recording may start: the preroll, unless a previous
// recording ended more recently than that, in which case it stops there.
func reach(preroll time.Duration, ended, now time.Time) time.Duration {
	if ended.IsZero() {
		return preroll
	}
	return min(preroll, now.Sub(ended))
}

func (r *Recorder) open(back time.Duration) error {
	_, kind := r.detector.Recording()
	begun := time.Now().Add(-back)

	writer, err := Create(r.path(kind, begun) + partial)
	if err != nil {
		return fmt.Errorf("open a recording: %w", err)
	}
	r.mu.Lock()
	r.writer, r.kind, r.begun, r.asked = writer, kind, begun, r.detector.Forced()
	r.mu.Unlock()
	return nil
}

func (r *Recorder) path(kind Kind, at time.Time) string {
	return filepath.Join(r.dir, fmt.Sprintf("%s %s.wav", kind, at.Format(stamp)))
}

// free is a name nothing has taken. The stamp is minute-resolution, so two
// recordings that begin in the same minute would otherwise be one file — and
// the library would hold two rows, one of them playing the other's audio.
// Capping the preroll makes that rare; this makes it impossible.
func free(path string) string {
	stem := strings.TrimSuffix(path, ".wav")
	for n := 2; ; n++ {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return path
		}
		path = fmt.Sprintf("%s (%d).wav", stem, n)
	}
}

// finish closes the current recording and hands it to the library. The kind may
// have changed since it was opened, so the file is renamed to match first.
func (r *Recorder) finish(why string) error {
	r.mu.Lock()
	writer, kind, begun, asked := r.writer, r.kind, r.begun, r.asked
	r.writer = nil
	r.mu.Unlock()
	if writer == nil {
		return nil
	}

	// Whatever is still held back is the silence that ended the recording. It
	// is dropped here, which is the whole point of holding it.
	dropped := time.Duration(len(r.pending)) * frameDuration
	r.pending = r.pending[:0]

	r.scribe.Stop()
	held := writer.Duration()
	if err := writer.Close(); err != nil {
		return err
	}
	r.mu.Lock()
	r.ended = time.Now()
	r.mu.Unlock()

	final := free(r.path(kind, begun))
	if err := writer.Rename(final); err != nil {
		return err
	}
	slog.Info("recording finished", "kind", kind,
		"length", held.Round(time.Second),
		"trimmed", dropped.Round(time.Second), "why", why)

	// A recording too short to hold a sentence is a false start, not a meeting.
	if held < 5*time.Second {
		slog.Info("too short to keep", "length", held.Round(time.Second))
		_ = os.Remove(final)
		return nil
	}
	// Nobody else was in it and nobody asked for it: the app listening to a
	// phone call or to thinking aloud, hours a week transcribed for nothing.
	//
	// Only when the system channel was there to say so. Without it the app
	// cannot tell a meeting from a monologue — measured across the library,
	// the best the microphone alone manages is to lose one meeting in three —
	// and a recording it cannot judge is one it must not destroy.
	if kind == Note && !asked && !r.keepNotes && r.System() {
		slog.Info("nobody else was in it and nobody asked for it; discarded",
			"length", held.Round(time.Second))
		_ = os.Remove(final)
		if r.threw != nil {
			r.threw(held.Seconds(), "nobody else was in it")
		}
		return nil
	}
	// The VAD is recurrent, so the end of one recording would otherwise colour
	// the start of the next, and the voices heard belong to the meeting that
	// has just finished.
	r.company.Alone()
	r.mic.Forget()
	r.sys.Forget()
	r.done(final, kind, begun)
	return nil
}

// observe takes a snapshot for the window. The detector belongs to the capture
// loop and is never read from another goroutine.
func (r *Recorder) observe(system bool) {
	on, kind := r.detector.Recording()
	status := Status{Phase: Listening, System: system}
	if on {
		status = Status{
			Phase:   Recording,
			Kind:    kind,
			System:  system,
			Elapsed: int(r.detector.Elapsed().Seconds()),
			Quiet:   int(r.detector.Quiet().Seconds()),
		}
		if r.detector.Quiet() > 10*time.Second {
			status.Phase = WrappingUp
		}
	}
	r.mu.Lock()
	if r.paused {
		status = Status{Phase: Paused, System: system}
	}
	r.snap = status
	r.mu.Unlock()
}

func (r *Recorder) fail(err error) {
	slog.Error("listening stopped", "err", err)
	r.mu.Lock()
	defer r.mu.Unlock()
	r.snap = Status{Phase: Broken, Problem: err.Error()}
}

// Status is what the window shows.
func (r *Recorder) Status() Status {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.snap
}

// Toggle is the Record now / Stop button. A forced recording ignores silence,
// so an in-person meeting with long pauses is not cut short.
func (r *Recorder) Toggle() {
	on := !r.Recording()
	r.mu.Lock()
	r.want = &on
	r.mu.Unlock()
}

// Recording reports whether audio is being kept right now.
func (r *Recorder) Recording() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.snap.Phase == Recording || r.snap.Phase == WrappingUp
}

// Pause stops capture without stopping the app. Anything being recorded is
// finished first, so pausing never truncates a file mid-write.
func (r *Recorder) Pause(on bool) {
	r.mu.Lock()
	already := r.paused == on
	r.paused = on
	r.mu.Unlock()
	if already {
		return
	}
	if on {
		_ = r.finish("paused")
	}
	r.mu.Lock()
	if on {
		r.snap.Phase = Paused
	} else {
		r.snap.Phase = Listening
	}
	r.mu.Unlock()
	slog.Info("capture", "paused", on)
}

// Muffle hides the app's own playback from the listener: the tap cannot tell
// our player from a meeting, so pressing play made the app record itself. While
// the player runs the far side is silence — not detected, not transcribed, not
// written. The microphone is untouched, so dictating over a recording works.
func (r *Recorder) Muffle(on bool) {
	r.mu.Lock()
	changed := r.muffled != on
	r.muffled = on
	r.mu.Unlock()
	if changed {
		slog.Info("playback", "muffled", on)
	}
}

// Hear turns the capture of the machine's own audio on or off. It takes effect
// on the next frame: the devices are reopened rather than the app restarted.
//
// Off, the app hears only the microphone — which without headphones is
// everybody, once, so there is nothing to mix and no echo to remove. What goes
// with it is the meeting detector: speech on the system channel is the one
// signal that says somebody is talking to you, so without it every recording
// the app starts by itself is a note.
func (r *Recorder) Hear(system bool) {
	r.mu.Lock()
	changed := r.system != system
	r.system = system
	r.mu.Unlock()
	if changed {
		slog.Info("capture", "system_audio", system)
	}
}

func (r *Recorder) System() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.system
}

func (r *Recorder) Muffled() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.muffled
}

func (r *Recorder) Paused() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.paused
}

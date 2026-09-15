package listen

import (
	"time"

	"github.com/dmykolen/meeting-transcriber-go/internal/store"
)

// Kind is re-exported from store so the detector and persistence agree.
type Kind = store.Kind

const (
	Meeting = store.Meeting
	Note    = store.Note
)

// Default detector thresholds.
const (
	StartSpeech = 20 * time.Second // this much speech...
	StartWindow = 60 * time.Second // ...within this window starts a recording

	// A blip of system audio is a notification chime, not a conversation.
	MeetingAudio = 3 * time.Second

	QuietMeeting = 3 * time.Minute // silence that ends a meeting
	QuietNote    = 60 * time.Second

	RollAfter = time.Hour     // files are cut here, without a gap
	HardCap   = 3 * time.Hour // and a session cannot outlive this
)

// Tuning overrides detector thresholds; zero values keep defaults.
type Tuning struct {
	StartSpeech  time.Duration
	StartWindow  time.Duration
	MeetingAudio time.Duration
	QuietMeeting time.Duration
	QuietNote    time.Duration
}

func (t Tuning) withDefaults() Tuning {
	for _, f := range []struct {
		at  *time.Duration
		def time.Duration
	}{
		{&t.StartSpeech, StartSpeech},
		{&t.StartWindow, StartWindow},
		{&t.MeetingAudio, MeetingAudio},
		{&t.QuietMeeting, QuietMeeting},
		{&t.QuietNote, QuietNote},
	} {
		if *f.at <= 0 {
			*f.at = f.def
		}
	}
	return t
}

// Transition is what changed on this frame.
type Transition int

const (
	Continue Transition = iota
	Started             // open a file, replay the ring into it
	Rolled              // close this file and open the next; still recording
	Ended               // close the file; the session is over
)

// Detector decides when a recording starts, what kind it is, and when it ends.
type Detector struct {
	tuning    Tuning
	forced    bool // somebody pressed Record now
	stopping  bool // ...and has now pressed Stop
	recording bool
	kind      Kind
	held      int // frames in the current file
	total     int // frames since the session began
	quiet     int // consecutive silent frames

	window   []state // the rolling StartWindow
	at       int
	speaking int  // frames of the window with speech on either channel
	others   int  // frames of the window with speech on the system channel
	company  bool // a second voice was heard on the microphone alone
}

type state struct{ any, sys bool }

// NewDetector uses the defaults; NewTunedDetector takes them from the config.
func NewDetector() *Detector { return NewTunedDetector(Tuning{}) }

func NewTunedDetector(tuning Tuning) *Detector {
	t := tuning.withDefaults()
	return &Detector{tuning: t, window: make([]state, frames(t.StartWindow))}
}

// Feed advances the detector by one frame.
func (d *Detector) Feed(mic, sys bool) Transition {
	d.remember(state{any: mic || sys, sys: sys})

	if !d.recording {
		if !d.forced && d.speaking < frames(d.tuning.StartSpeech) {
			return Continue
		}
		d.recording, d.held, d.total, d.quiet = true, 0, 0, 0
		// Decide from the whole start window, not just the threshold-crossing frame.
		d.kind = Note
		if d.others >= frames(d.tuning.MeetingAudio) {
			d.kind = Meeting
		}
		if d.forced && d.others < frames(d.tuning.MeetingAudio) {
			// Pressing Record does not itself prove a second speaker exists.
			d.kind = Note
		}
		return Started
	}

	d.held++
	d.total++
	// A note that later shows a second voice becomes a meeting.
	if (sys || d.company) && d.kind == Note {
		d.kind = Meeting
	}
	if mic || sys {
		d.quiet = 0
	} else {
		d.quiet++
	}

	switch {
	case d.stopping:
		// Route manual stop through the same transition path as automatic ones.
		d.stopping, d.recording = false, false
		d.clearWindow()
		return Ended
	// Forced recording ignores silence until explicit stop or hard cap.
	case !d.forced && d.quiet >= frames(d.quietEnough()), d.total >= frames(HardCap):
		d.recording = false
		d.clearWindow()
		return Ended
	case d.held >= frames(RollAfter):
		// Roll long sessions forward without making them re-earn start evidence.
		d.held = 0
		return Rolled
	}
	return Continue
}

func (d *Detector) quietEnough() time.Duration {
	if d.kind == Meeting {
		return d.tuning.QuietMeeting
	}
	return d.tuning.QuietNote
}

// Recording reports whether audio is being kept right now, and as what.
func (d *Detector) Recording() (bool, Kind) { return d.recording, d.kind }

// Company says the mic heard a second in-room voice. It only promotes notes to
// meetings; it never demotes them.
func (d *Detector) Company() { d.company = true }

// Force records until released. Turning it off stops whatever is recording,
// however it began.
func (d *Detector) Force(on bool) {
	if !on && d.recording {
		d.stopping = true
	}
	d.forced = on
}

// Forced reports whether the current recording was asked for by a person.
func (d *Detector) Forced() bool { return d.forced }

// Held is the current file length, Elapsed the session length, and Quiet the
// current silent tail.
func (d *Detector) Held() time.Duration    { return time.Duration(d.held) * frameDuration }
func (d *Detector) Elapsed() time.Duration { return time.Duration(d.total) * frameDuration }
func (d *Detector) Quiet() time.Duration   { return time.Duration(d.quiet) * frameDuration }

func frames(d time.Duration) int { return int(d / frameDuration) }

// remember slides the start window and keeps its counts current.
func (d *Detector) remember(s state) {
	old := d.window[d.at]
	if old.any {
		d.speaking--
	}
	if old.sys {
		d.others--
	}
	if s.any {
		d.speaking++
	}
	if s.sys {
		d.others++
	}
	d.window[d.at] = s
	d.at = (d.at + 1) % len(d.window)
}

// clearWindow stops one recording's tail from leaking into the next decision.
func (d *Detector) clearWindow() {
	clear(d.window)
	d.speaking, d.others, d.at, d.company = 0, 0, 0, false
}

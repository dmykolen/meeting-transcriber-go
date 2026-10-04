package listen

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dmykolen/meeting-transcriber-go/internal/audio"
	"github.com/dmykolen/meeting-transcriber-go/internal/home"
)

// Held, a recording keeps its file but takes nothing from the microphone: the
// speech detectors are nil here and would panic if they were asked.
func TestAHeldRecordingWritesNothingAndKeepsItsFile(t *testing.T) {
	dir := t.TempDir()
	r := New(dir, "", home.Defaults().Listen, func(string, Kind, time.Time) {}, nil, nil, nil)
	r.detector.Force(true)
	r.detector.Feed(false, false)
	writer, err := Create(filepath.Join(dir, "held.wav.part"))
	if err != nil {
		t.Fatal(err)
	}
	r.writer = writer
	r.observe(true)

	r.Hold(true)
	frame := make([]int16, 2*audio.FrameSize)
	half := make([]int16, audio.FrameSize)
	for range 50 {
		if err := r.step(frame, half, half, true); err != nil {
			t.Fatal(err)
		}
	}
	if s := r.Status(); s.Phase != Held || !r.Recording() {
		t.Fatalf("phase %q, recording %v; a held recording is still a recording", s.Phase, r.Recording())
	}
	if writer.Duration() != 0 {
		t.Fatalf("%v of audio was written while held", writer.Duration())
	}

	// Stop must reach the detector even while held.
	r.Toggle()
	if r.Held() || r.want == nil || *r.want {
		t.Fatalf("held=%v want=%v; Stop was swallowed by the hold", r.Held(), r.want)
	}
	writer.Close()
	os.Remove(writer.path)
}

// A pause for a private conversation files the recording in progress and
// forgets what could carry into the next one: the ring a preroll replays, a
// Record press not yet applied. Record pressed during the pause ends it.
func TestAPauseFilesTheRecordingAndForgetsWhatCameBefore(t *testing.T) {
	dir := t.TempDir()
	r := New(dir, "", home.Defaults().Listen, func(string, Kind, time.Time) {}, nil, nil, nil)
	frame := make([]int16, 2*audio.FrameSize)
	for range 100 {
		r.ring.Add(frame, true)
	}
	r.detector.Force(true)
	r.detector.Feed(true, false)
	writer, err := Create(filepath.Join(dir, "meeting.wav.part"))
	if err != nil {
		t.Fatal(err)
	}
	r.writer = writer
	start := true
	r.want = &start

	until := time.Now().Add(time.Minute)
	r.Mute(until)
	for range 3 {
		if hushed, err := r.hush(true); err != nil || !hushed {
			t.Fatalf("hushed %v, err %v during the pause", hushed, err)
		}
	}
	if recording, _ := r.detector.Recording(); r.writer != nil || recording || r.detector.Forced() {
		t.Fatal("the recording in progress went on through the pause")
	}
	if r.ring.Held() != 0 || r.want != nil {
		t.Fatalf("kept %v of audio and Record press %v for after the pause", r.ring.Held(), r.want)
	}
	if s := r.Status(); s.Phase != Paused || !s.Until.Equal(until) {
		t.Fatalf("status %+v, want paused until %v", s, until)
	}

	r.Toggle()
	if hushed, _ := r.hush(true); hushed || r.want == nil || !*r.want {
		t.Fatalf("hushed %v, want %v: Record did not end the pause", hushed, r.want)
	}
}

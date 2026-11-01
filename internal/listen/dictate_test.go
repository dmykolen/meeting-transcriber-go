package listen

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dmykolen/meeting-transcriber-go/internal/audio"
	"github.com/dmykolen/meeting-transcriber-go/internal/home"
	"github.com/dmykolen/meeting-transcriber-go/internal/models"
)

// second is a second of audio in capture frames.
var second = int(time.Second / frameDuration)

// Pressing Note during a call's lull: the minute before it had speech on the
// system channel, which would make a recording started by Record a meeting.
// A dictated note owes nothing to what came before the press.
func TestANoteStartsWhenPressedAndStaysANote(t *testing.T) {
	r := New(t.TempDir(), "", home.Defaults().Listen, func(Filed) {}, nil, nil, nil)
	frame := make([]int16, 2*audio.FrameSize)
	for range 10 * second {
		r.ring.Add(frame, true)
		r.detector.Feed(false, true)
	}
	r.observe(true)

	if err := r.Dictate(); err != nil {
		t.Fatal(err)
	}
	r.apply()
	if r.detector.Feed(true, false) != Started {
		t.Fatal("Note did not start a recording")
	}
	if err := r.begin(); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(r.writer.path)
	if _, kind := r.detector.Recording(); kind != Note || r.kind != Note {
		t.Fatalf("the detector says %q and the file %q; a dictated note is a note", kind, r.kind)
	}
	if r.writer.Duration() != 0 {
		t.Fatalf("%v of what came before the press went into the note", r.writer.Duration())
	}
	if !r.Dictating() {
		t.Fatal("the recorder does not know the note is dictated, so it still hears the system channel")
	}
	r.observe(true)
	if s := r.Status(); !s.Dictating || s.Kind != Note || !s.Asked {
		t.Fatalf("status %+v: the window cannot tell a note is being dictated", s)
	}

	// The second press stops it, as a press of Stop would.
	if err := r.Dictate(); err != nil {
		t.Fatal(err)
	}
	r.apply()
	if r.detector.Feed(false, false) != Ended {
		t.Fatal("the second press did not end the note")
	}
}

// One microphone: while a meeting records, Note refuses instead of starting a
// second recording, and a line typed instead is filed with the meeting.
func TestNoteRefusesDuringAMeetingAndTypedLinesStay(t *testing.T) {
	cfg := home.Defaults().Listen
	cfg.KeepNotes, cfg.System = false, true
	var filed []Filed
	r := New(t.TempDir(), "", cfg, func(f Filed) { filed = append(filed, f) }, nil, nil, nil)
	r.mic, r.sys = ears(t), ears(t)
	r.detector.Force(true)
	r.detector.Feed(true, false)
	if err := r.open(0); err != nil {
		t.Fatal(err)
	}
	r.asked = false // as if the app had started it: a note nobody asked for
	r.observe(true)

	if err := r.Dictate(); err == nil || r.dictate {
		t.Fatalf("Note was taken during a recording (err %v)", err)
	}
	if err := r.Jot("Спитати Олену про доступ"); err != nil {
		t.Fatal(err)
	}
	frame := make([]int16, 2*audio.FrameSize)
	for range 6 * second {
		if err := r.writer.Write(frame); err != nil {
			t.Fatal(err)
		}
	}
	if err := r.finish("quiet"); err != nil {
		t.Fatal(err)
	}
	if len(filed) != 1 || len(filed[0].Jots) != 1 || filed[0].Jots[0].Text != "Спитати Олену про доступ" || filed[0].Dictated {
		t.Fatalf("filed %+v; the typed line, or the recording it made worth keeping, is lost", filed)
	}
	if err := r.Jot("x"); err == nil {
		t.Fatal("a line was taken with nothing recording")
	}
}

// "Buy bread" takes a second, well under the five that keep a false start out
// of the library; a dictated note is kept, unless nothing was said into it.
func TestADictatedNoteIsKeptHoweverShortButNotEmpty(t *testing.T) {
	var filed []Filed
	r := New(t.TempDir(), "", home.Defaults().Listen, func(f Filed) { filed = append(filed, f) }, nil, nil, nil)
	r.mic, r.sys = ears(t), ears(t)
	note := func(length int) {
		r.observe(true)
		if err := r.Dictate(); err != nil {
			t.Fatal(err)
		}
		r.apply()
		r.detector.Feed(true, false)
		if err := r.begin(); err != nil {
			t.Fatal(err)
		}
		frame := make([]int16, 2*audio.FrameSize)
		for range length {
			if err := r.writer.Write(frame); err != nil {
				t.Fatal(err)
			}
		}
		if err := r.finish("stopped"); err != nil {
			t.Fatal(err)
		}
		r.detector.Reset()
	}

	note(second)
	if len(filed) != 1 || !filed[0].Dictated || filed[0].Kind != Note {
		t.Fatalf("filed %+v; a one-second dictated note was thrown away", filed)
	}
	if _, err := os.Stat(filed[0].Path); err != nil || filepath.Ext(filed[0].Path) != ".wav" {
		t.Fatalf("the note's file is not there: %v", err)
	}

	note(0)
	r.observe(true)
	if len(filed) != 1 || time.Since(r.Status().Missed) > time.Second {
		t.Fatalf("filed %d, missed %v; an empty note was kept or dropped without a word", len(filed), r.Status().Missed)
	}
}

// ears loads the speech detector a finished recording resets; the tests that
// file a recording need it on this Mac.
func ears(t *testing.T) *Ears {
	dir, err := home.Dir()
	if err != nil {
		t.Skip(err)
	}
	model := models.Path(home.Models(dir), models.Speech)
	if _, err := os.Stat(model); err != nil {
		t.Skip("the speech detector is not downloaded on this Mac")
	}
	e, err := Listen(model)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(e.Close)
	return e
}

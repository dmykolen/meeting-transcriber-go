package library

import (
	"testing"

	"github.com/dmykolen/meeting-transcriber-go/internal/engine"
	"github.com/dmykolen/meeting-transcriber-go/internal/media"
)

// tone makes test audio at a fixed level.
func tone(seconds float64, level float32) []float32 {
	out := make([]float32, int(seconds*media.Rate))
	for i := range out {
		if i%2 == 0 {
			out[i] = level
		} else {
			out[i] = -level
		}
	}
	return out
}

func TestTheMicrophoneSideIsOnePerson(t *testing.T) {
	mic := tone(10, 0)
	system := tone(10, 0)
	copy(mic[0*media.Rate:2*media.Rate], tone(2, 0.5))    // you
	copy(system[2*media.Rate:4*media.Rate], tone(2, 0.5)) // them
	copy(mic[4*media.Rate:6*media.Rate], tone(2, 0.5))    // you again, mislabelled

	read := engine.Result{
		Turns: []engine.Turn{
			{Start: 0, End: 2, Speaker: "SPEAKER_00", Text: "a"},
			{Start: 2, End: 4, Speaker: "SPEAKER_01", Text: "b"},
			{Start: 4, End: 6, Speaker: "SPEAKER_02", Text: "c"},
		},
		Voices: map[string][]float32{
			"SPEAKER_00": {1, 0}, "SPEAKER_01": {0, 1}, "SPEAKER_02": {1, 0},
		},
	}
	got := mine(read, mic, system)

	if got.Turns[0].Speaker != Me || got.Turns[2].Speaker != Me {
		t.Fatalf("the microphone side was not folded together: %+v", got.Turns)
	}
	if got.Turns[1].Speaker != "SPEAKER_01" {
		t.Fatalf("the far side was taken too: %+v", got.Turns[1])
	}
	if _, kept := got.Voices[Me]; !kept {
		t.Fatal("the person sitting here has no voiceprint to be recognised by")
	}
	if _, stale := got.Voices["SPEAKER_00"]; stale {
		t.Fatal("a voiceprint was left behind under a label nothing uses")
	}
}

func TestAVoiceThroughTheSpeakersIsNotMistakenForYours(t *testing.T) {
	mic, system := tone(4, 0.05), tone(4, 0.5) // a quiet echo against the real thing
	read := engine.Result{Turns: []engine.Turn{{Start: 0, End: 4, Speaker: "SPEAKER_01"}}}

	if got := mine(read, mic, system); got.Turns[0].Speaker != "SPEAKER_01" {
		t.Fatalf("an echo was credited to you: %+v", got.Turns[0])
	}
}

func TestAFragmentClusterIsNotAParticipant(t *testing.T) {
	var turns []engine.Turn
	for i := range 100 {
		at := float64(i) * 20
		turns = append(turns,
			engine.Turn{Start: at, End: at + 10, Speaker: "SPEAKER_00", Text: "x"},
			engine.Turn{Start: at + 10, End: at + 20, Speaker: "SPEAKER_01", Text: "y"})
	}
	turns[41].Speaker = "SPEAKER_09" // ten seconds, once

	got := settle(turns)
	for _, t2 := range got {
		if t2.Speaker == "SPEAKER_09" {
			t.Fatal("a ten-second fragment is still being shown as a speaker")
		}
	}
	if got[41].Speaker != "SPEAKER_00" && got[41].Speaker != "SPEAKER_01" {
		t.Fatalf("the fragment went to nobody: %q", got[41].Speaker)
	}
}

func TestAShortRecordingKeepsEverySpeaker(t *testing.T) {
	turns := []engine.Turn{
		{Start: 0, End: 50, Speaker: "SPEAKER_00"},
		{Start: 50, End: 100, Speaker: "SPEAKER_01"},
		{Start: 100, End: 106, Speaker: "SPEAKER_02"},
	}
	if got := settle(turns); got[2].Speaker != "SPEAKER_02" {
		t.Fatalf("a brief speaker was absorbed in a short recording: %q", got[2].Speaker)
	}
}

func TestANameNeedsMoreThanAFragmentToJustifyIt(t *testing.T) {
	prints := map[string][]float32{
		"SPEAKER_00": {1, 0},
		"SPEAKER_01": {0, 1},
	}
	turns := []engine.Turn{
		{Start: 0, End: 600, Speaker: "SPEAKER_00"},   // ten minutes
		{Start: 600, End: 618, Speaker: "SPEAKER_01"}, // eighteen seconds
	}

	kept := enough(prints, turns)
	if _, in := kept["SPEAKER_00"]; !in {
		t.Fatal("somebody who spoke for ten minutes was refused a name")
	}
	if _, in := kept["SPEAKER_01"]; in {
		t.Fatal("an eighteen-second fragment is still eligible for a colleague's name")
	}
}

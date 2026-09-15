package listen

import (
	"testing"
	"time"

	"github.com/dmykolen/meeting-transcriber-go/internal/audio"
)

func frame(v int16) []int16 {
	f := make([]int16, 2*audio.FrameSize)
	for i := range f {
		f[i] = v
	}
	return f
}

// values reads the first sample of each replayed frame.
func values(frames [][]int16) []int16 {
	out := make([]int16, len(frames))
	for i, f := range frames {
		out[i] = f[0]
	}
	return out
}

func TestReplayReturnsTheMostRecentAudioOldestFirst(t *testing.T) {
	r := NewRing(10 * frameDuration)
	for i := range int16(10) {
		r.Add(frame(i), true)
	}
	got := values(r.Replay(3 * frameDuration))
	if len(got) != 3 || got[0] != 7 || got[1] != 8 || got[2] != 9 {
		t.Fatalf("replayed %v, want the last three in order", got)
	}
}

func TestTheRingOverwritesTheOldestAudio(t *testing.T) {
	r := NewRing(4 * frameDuration)
	for i := range int16(10) { // two and a half times round
		r.Add(frame(i), true)
	}
	got := values(r.Replay(time.Hour)) // more than it holds
	if len(got) != 4 || got[0] != 6 || got[3] != 9 {
		t.Fatalf("replayed %v, want frames 6..9", got)
	}
}

func TestAPartlyFilledRingReplaysOnlyWhatItHas(t *testing.T) {
	r := NewRing(time.Minute)
	for i := range int16(3) {
		r.Add(frame(i), true)
	}
	got := values(r.Replay(time.Minute))
	if len(got) != 3 || got[0] != 0 || got[2] != 2 {
		t.Fatalf("replayed %v, want the three frames it has", got)
	}
	if held := r.Held(); held != 3*frameDuration {
		t.Fatalf("Held()=%v, want %v", held, 3*frameDuration)
	}
}

func TestTheRingIsSizedForTheAudioItIsAskedToHold(t *testing.T) {
	r := NewRing(10 * time.Minute)
	for range 20000 {
		r.Add(frame(1), true)
	}
	held := r.Held()
	if held < 9*time.Minute+50*time.Second || held > 10*time.Minute {
		t.Fatalf("Held()=%v, want about ten minutes", held)
	}
}

func TestReplayingNothingIsNotAnError(t *testing.T) {
	if got := NewRing(time.Minute).Replay(time.Minute); len(got) != 0 {
		t.Fatalf("an empty ring replayed %d frames", len(got))
	}
}

func TestReplaySpeechSkipsTheEmptyRoomBeforeTheTalking(t *testing.T) {
	r := NewRing(10 * frameDuration)
	for i := range int16(6) {
		r.Add(frame(i), false) // an empty room
	}
	for i := int16(6); i < 10; i++ {
		r.Add(frame(i), true) // and then people
	}

	got := values(r.ReplaySpeech(10 * frameDuration))
	if len(got) != 4 || got[0] != 6 {
		t.Fatalf("replayed %v, want the four frames from where the talking started", got)
	}
}

func TestReplaySpeechOfASilentRingIsEmpty(t *testing.T) {
	r := NewRing(10 * frameDuration)
	for i := range int16(10) {
		r.Add(frame(i), false)
	}
	if got := r.ReplaySpeech(10 * frameDuration); len(got) != 0 {
		t.Fatalf("replayed %d frames of pure silence", len(got))
	}
}

func TestReplaySpeechKeepsPausesOnceTalkingHasStarted(t *testing.T) {
	r := NewRing(10 * frameDuration)
	r.Add(frame(0), false)
	r.Add(frame(1), true)
	r.Add(frame(2), false)
	r.Add(frame(3), true)

	got := values(r.ReplaySpeech(10 * frameDuration))
	if len(got) != 3 || got[0] != 1 || got[1] != 2 {
		t.Fatalf("replayed %v, want frames 1..3 with the pause kept", got)
	}
}

package listen

import (
	"time"

	"github.com/dmykolen/meeting-transcriber-go/internal/audio"
)

// Ring keeps the recent audio history so a recording can begin in the past.
type Ring struct {
	frames  [][]int16
	speech  []bool // was anybody talking during that frame
	next    int
	written int
}

// NewRing sizes a buffer to hold d of audio.
func NewRing(d time.Duration) *Ring {
	count := int(d / frameDuration)
	if count < 1 {
		count = 1
	}
	frames := make([][]int16, count)
	for i := range frames {
		frames[i] = make([]int16, 2*audio.FrameSize)
	}
	return &Ring{frames: frames, speech: make([]bool, count)}
}

const frameDuration = time.Duration(audio.FrameSize) * time.Second / audio.SampleRate

// Add copies one stereo frame in and remembers whether it held speech.
func (r *Ring) Add(frame []int16, speech bool) {
	copy(r.frames[r.next], frame)
	r.speech[r.next] = speech
	r.next = (r.next + 1) % len(r.frames)
	r.written++
}

// Held is how much audio the ring currently has.
func (r *Ring) Held() time.Duration {
	return time.Duration(min(r.written, len(r.frames))) * frameDuration
}

// Replay yields the most recent d of audio, oldest first.
func (r *Ring) Replay(d time.Duration) [][]int16 {
	want := min(int(d/frameDuration), min(r.written, len(r.frames)))
	out := make([][]int16, 0, want)
	// r.next is the oldest slot after wrap, and the write head before wrap.
	start := ((r.next-want)%len(r.frames) + len(r.frames)) % len(r.frames)
	for i := range want {
		out = append(out, r.frames[(start+i)%len(r.frames)])
	}
	return out
}

// ReplaySpeech is Replay starting at the first speech frame.
func (r *Ring) ReplaySpeech(d time.Duration) [][]int16 {
	want := min(int(d/frameDuration), min(r.written, len(r.frames)))
	start := ((r.next-want)%len(r.frames) + len(r.frames)) % len(r.frames)

	skip := 0
	for skip < want && !r.speech[(start+skip)%len(r.frames)] {
		skip++
	}
	out := make([][]int16, 0, want-skip)
	for i := skip; i < want; i++ {
		out = append(out, r.frames[(start+i)%len(r.frames)])
	}
	return out
}

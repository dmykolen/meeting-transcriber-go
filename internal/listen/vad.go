package listen

import (
	"fmt"

	sherpa "github.com/k2-fsa/sherpa-onnx-go-macos"

	"github.com/dmykolen/meeting-transcriber-go/internal/audio"
)

// Ears answers whether one channel currently contains speech.
type Ears struct {
	vad     *sherpa.VoiceActivityDetector
	samples []float32
}

// Listen loads one detector. The microphone and system channels each need
// their own instance.
func Listen(model string) (*Ears, error) {
	cfg := &sherpa.VadModelConfig{
		SampleRate: audio.SampleRate,
		NumThreads: 1,
		Provider:   "cpu",
	}
	cfg.SileroVad.Model = model
	cfg.SileroVad.Threshold = 0.5
	cfg.SileroVad.WindowSize = audio.FrameSize
	// These are only per-frame speech gates; meeting-end timing lives higher up.
	cfg.SileroVad.MinSpeechDuration = 0.1
	cfg.SileroVad.MinSilenceDuration = 0.25
	// Segment endings are unimportant here; callers only need speech state and
	// completed utterances.
	cfg.SileroVad.MaxSpeechDuration = 60

	// One second of buffer is enough because segments are drained every frame.
	vad := sherpa.NewVoiceActivityDetector(cfg, 1)
	if vad == nil {
		return nil, fmt.Errorf("could not load the speech detector from %s", model)
	}
	return &Ears{vad: vad, samples: make([]float32, audio.FrameSize)}, nil
}

// Speaking reports whether this frame is inside speech and returns any
// utterances that finished on it.
func (e *Ears) Speaking(frame []int16) (bool, []Utterance, error) {
	if len(frame) != audio.FrameSize {
		return false, nil, fmt.Errorf("vad: got %d samples, want %d", len(frame), audio.FrameSize)
	}
	for i, s := range frame {
		e.samples[i] = float32(s) / 32768
	}
	e.vad.AcceptWaveform(e.samples)

	var done []Utterance
	for !e.vad.IsEmpty() {
		if seg := e.vad.Front(); seg != nil && len(seg.Samples) > 0 {
			done = append(done, Utterance{
				At:      float64(seg.Start) / audio.SampleRate,
				Samples: seg.Samples,
			})
		}
		e.vad.Pop()
	}
	return e.vad.IsSpeech(), done, nil
}

// Utterance is one stretch of speech, cut where it stopped.
type Utterance struct {
	At      float64 // seconds from when this detector started listening
	Samples []float32
}

// Forget resets detector history between recordings.
func (e *Ears) Forget() { e.vad.Reset() }

func (e *Ears) Close() { sherpa.DeleteVoiceActivityDetector(e.vad) }

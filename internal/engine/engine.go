// Package engine turns recordings into timed transcript turns and speaker data.
package engine

import (
	"errors"
	"fmt"
	"sync"
)

// Turn is one transcript row.
type Turn struct {
	Start   float64 `json:"start"`
	End     float64 `json:"end"`
	Speaker string  `json:"speaker,omitempty"`
	Text    string  `json:"text"`
}

// Span is a diarized speaker segment before words are attached to it.
type Span struct {
	Start, End float64
	Speaker    int
}

// Options configures the loaded models.
type Options struct {
	// Language of the meeting, or empty to detect it.
	Language string

	// Threads for both libraries.
	Threads int

	// Transcriber is "whisper" or "parakeet".
	Transcriber string
}

// Engine holds the loaded models for the life of the process.
type Engine struct {
	models string
	opts   Options

	mu       sync.Mutex // native handles below are not reentrant
	asr      Transcriber
	speakers *speakers
	voices   *voices
}

// Transcriber is the ASR interface implemented by Whisper and Parakeet.
type Transcriber interface {
	transcribe(samples []float32) ([]Turn, error)
	close() error
}

// Open loads the configured models from disk.
func Open(modelsDir string, opts Options) (*Engine, error) {
	if opts.Threads <= 0 {
		opts.Threads = 8
	}
	e := &Engine{models: modelsDir, opts: opts}

	var err error
	if opts.Transcriber == Parakeet {
		e.asr, err = openParakeet(modelsDir, opts)
	} else {
		e.asr, err = openASR(modelsDir, opts)
	}
	if err != nil {
		return nil, fmt.Errorf("transcription: %w", err)
	}
	if e.speakers, err = openSpeakers(modelsDir, opts); err != nil {
		e.asr.close()
		return nil, fmt.Errorf("speakers: %w", err)
	}
	if e.voices, err = openVoices(modelsDir, opts); err != nil {
		e.asr.close()
		e.speakers.close()
		return nil, fmt.Errorf("voices: %w", err)
	}
	return e, nil
}

// Transcribe returns timed text without speakers.
func (e *Engine) Transcribe(samples []float32) ([]Turn, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.asr.transcribe(samples)
}

// Diarize returns numbered speaker spans.
func (e *Engine) Diarize(samples []float32) ([]Span, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.speakers.diarize(samples)
}

// Run transcribes the heard mix and diarizes the separated far-side signal when
// one exists.
func (e *Engine) Run(heard, apart []float32) (Result, error) {
	turns, err := e.Transcribe(heard)
	if err != nil {
		return Result{}, err
	}
	voices := apart
	if len(voices) == 0 {
		voices = heard // a file dropped in has no channel of its own
	}
	spans, err := e.Diarize(voices)
	if err != nil {
		// Keep a usable transcript even if speaker diarization fails.
		return Result{Turns: turns}, fmt.Errorf("transcribed, but speakers failed: %w", err)
	}

	e.mu.Lock()
	prints := e.voices.voiceprints(voices, spans)
	e.mu.Unlock()
	return Result{Turns: Attribute(turns, spans), Voices: prints}, nil
}

// Print returns the same kind of voice embedding the speaker models use.
func (e *Engine) Print(samples []float32) []float32 {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.voices.print(samples)
}

// Solo labels a known one-speaker recording without diarizing it.
func (e *Engine) Solo(samples []float32, who string) (Result, error) {
	turns, err := e.Transcribe(samples)
	if err != nil {
		return Result{}, err
	}
	for i := range turns {
		turns[i].Speaker = who
	}
	e.mu.Lock()
	print := e.voices.print(samples)
	e.mu.Unlock()

	out := Result{Turns: turns}
	if print != nil {
		out.Voices = map[string][]float32{who: print}
	}
	return out, nil
}

// Result is the processed form of one recording.
type Result struct {
	Turns []Turn

	// Voices holds one voiceprint per speaker label when there is enough speech.
	Voices map[string][]float32
}

func (e *Engine) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return errors.Join(e.asr.close(), e.speakers.close(), e.voices.close())
}

// Attribute names each turn after the speaker with the most overlap.
func Attribute(turns []Turn, spans []Span) []Turn {
	out := make([]Turn, len(turns))
	for i, t := range turns {
		out[i] = t
		best, who := 0.0, -1
		for _, s := range spans {
			if shared := overlap(t.Start, t.End, s.Start, s.End); shared > best {
				best, who = shared, s.Speaker
			}
		}
		if who >= 0 {
			out[i].Speaker = Label(who)
		}
	}
	return out
}

// Label is the placeholder name for an unnamed speaker.
func Label(speaker int) string { return fmt.Sprintf("SPEAKER_%02d", speaker) }

func overlap(aStart, aEnd, bStart, bEnd float64) float64 {
	return max(0, min(aEnd, bEnd)-max(aStart, bStart))
}

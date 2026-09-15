package listen

import (
	"log/slog"
	"strings"
	"sync"
	"time"
)

// Scribe keeps a rough live transcript while recording is still running.
type Scribe struct {
	transcribe func([]float32) (string, error)

	mu      sync.Mutex
	lines   []Line
	pending int
	started time.Time
}

// Line is one live-transcribed utterance.
type Line struct {
	At   int    `json:"at"`  // seconds into the recording
	Who  string `json:"who"` // "you" or "them" — which channel it came from
	Text string `json:"text"`
}

// Lines is how much of the live transcript is kept in memory.
const Lines = 400

// NewScribe builds a Scribe from a transcription callback.
func NewScribe(transcribe func([]float32) (string, error)) *Scribe {
	return &Scribe{transcribe: transcribe}
}

// Start clears state for a new recording.
func (s *Scribe) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lines, s.pending, s.started = nil, 0, time.Now()
}

// Hear queues an utterance without blocking the capture loop.
func (s *Scribe) Hear(who string, u Utterance) {
	s.mu.Lock()
	if s.pending > 0 || s.transcribe == nil || s.started.IsZero() {
		s.mu.Unlock()
		return
	}
	s.pending++
	at := int(time.Since(s.started).Seconds())
	s.mu.Unlock()

	go func() {
		text, err := s.transcribe(u.Samples)
		s.mu.Lock()
		defer s.mu.Unlock()
		s.pending--
		if err != nil {
			slog.Debug("live transcription skipped", "err", err)
			return
		}
		if text = strings.TrimSpace(text); text == "" {
			return
		}
		s.lines = append(s.lines, Line{At: at, Who: who, Text: text})
		if len(s.lines) > Lines {
			s.lines = s.lines[len(s.lines)-Lines:]
		}
	}()
}

// Said returns the live transcript so far, oldest first.
func (s *Scribe) Said() []Line {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Line(nil), s.lines...)
}

// Stop discards the rough live transcript.
func (s *Scribe) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lines, s.started = nil, time.Time{}
}

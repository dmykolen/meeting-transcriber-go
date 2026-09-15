package listen

import (
	"math"

	"github.com/dmykolen/meeting-transcriber-go/internal/audio"
)

// Company infers whether more than one in-room speaker has been heard when the
// system channel is unavailable.
type Company struct {
	print  Voice
	voices [][]float32 // the centre of each voice heard
	held   []int       // how many utterances each has taken in
}

// Voice turns a stretch of speech into a voiceprint.
type Voice func([]float32) []float32

// Alike is the cosine above which two utterances are treated as one person.
const Alike = 0.65

// Enough is the minimum speech needed before a voiceprint is useful.
const Enough = 5 * audio.SampleRate

func NewCompany(print Voice) *Company {
	return &Company{print: print}
}

// Heard reports whether more than one person has now been heard.
func (c *Company) Heard(samples []float32) bool {
	if c.print == nil || len(samples) < Enough {
		return len(c.voices) > 1
	}
	print := c.print(samples)
	if len(print) == 0 {
		return len(c.voices) > 1
	}

	// Compare against each running centroid rather than the first sample seen.
	best, score := -1, Alike
	for i, known := range c.voices {
		if c := cosine(print, known); c >= score {
			best, score = i, c
		}
	}
	if best < 0 {
		c.voices = append(c.voices, append([]float32(nil), print...))
		c.held = append(c.held, 1)
		return len(c.voices) > 1
	}

	c.held[best]++
	n := float32(c.held[best])
	for j := range c.voices[best] {
		c.voices[best][j] += (print[j] - c.voices[best][j]) / n
	}
	return len(c.voices) > 1
}

// Alone forgets everybody for the next recording.
func (c *Company) Alone() { c.voices, c.held = c.voices[:0], c.held[:0] }

// Voices reports how many distinct people have been heard.
func (c *Company) Voices() int { return len(c.voices) }

func cosine(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		na += float64(a[i]) * float64(a[i])
		nb += float64(b[i]) * float64(b[i])
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / math.Sqrt(na*nb)
}

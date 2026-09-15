package engine

import (
	"math"
	"slices"
)

// Helpers for preparing ASR input and filtering implausible output.

// Quiet is the target active level before transcription, in dBFS.
const Quiet = -14.0

// Rumble is the high-pass corner.
const Rumble = 80.0

// prepare high-passes and normalizes audio before ASR. It is deliberately not
// a denoiser.
func prepare(samples []float32, rate int) []float32 {
	if len(samples) == 0 {
		return samples
	}
	out := make([]float32, len(samples))

	// One-pole high-pass; this also removes DC offset.
	rc := 1 / (2 * math.Pi * Rumble)
	a := float32(rc / (rc + 1/float64(rate)))
	var prevIn, prevOut float32
	for i, s := range samples {
		prevOut = a * (prevOut + s - prevIn)
		prevIn = s
		out[i] = prevOut
	}

	// Normalize to active speech level, not peak level.
	loud := active(out)
	if loud <= 0 {
		return out
	}
	gain := float32(math.Pow(10, Quiet/20) / loud)
	// Never attenuate, and do not amplify enough to turn noise into signal.
	gain = min(max(gain, 1), 8)
	if gain == 1 {
		return out
	}
	for i := range out {
		out[i] = clamp(out[i] * gain)
	}
	return out
}

// active estimates speech loudness without averaging in silence.
func active(samples []float32) float64 {
	const window = 1600 // 100 ms
	var levels []float64
	for i := 0; i+window <= len(samples); i += window {
		var sum float64
		for _, s := range samples[i : i+window] {
			sum += float64(s) * float64(s)
		}
		levels = append(levels, math.Sqrt(sum/window))
	}
	if len(levels) == 0 {
		return 0
	}
	var loud []float64
	for _, l := range levels {
		if l > 1e-4 {
			loud = append(loud, l)
		}
	}
	if len(loud) == 0 {
		return 0
	}
	slices.Sort(loud)
	return loud[len(loud)*3/4]
}

func clamp(v float32) float32 { return max(min(v, 1), -1) }

// Doubtful is the mean token probability below which a row is treated as
// invention rather than speech.
const Doubtful = 0.35

// Brief is the token-count threshold for stricter confidence filtering.
const Brief = 3

// believable rejects rows the model is not confident it heard.
func believable(tokens []float32) bool {
	if len(tokens) == 0 {
		return true // nothing to judge on; the VAD already gated the span
	}
	var sum float64
	for _, p := range tokens {
		sum += float64(p)
	}
	mean := sum / float64(len(tokens))

	if mean < Doubtful {
		return false
	}
	// Short rows are cheap to invent and cheap to lose, so hold them to a
	// higher bar.
	return len(tokens) > Brief || mean > Doubtful*1.6
}

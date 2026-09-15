package media

import (
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
)

// Window is the fold decision interval.
const Window = Rate / 50

// Floor is the minimum loudness that still counts as speech.
const Floor = 0.002

// Louder is the margin by which the mic must beat the far side to own a window.
const Louder = 3.0

// Slide is the crossfade length when ownership changes.
const Slide = 30 * Rate / 1000

// Fold_ versions cached folded copies.
const Fold_ = "v4-"

// Fold mixes app recordings to mono by switching ownership instead of summing.
func Fold(mic, system []float32) []float32 {
	n := min(len(mic), len(system))
	out := make([]float32, n)

	// 1 is the microphone, 0 the tap. Start on the microphone.
	at := float32(1)
	step := float32(1) / float32(Slide)

	for i := 0; i < n; i += Window {
		to := min(i+Window, n)
		them, you := Loud(system[i:to]), Loud(mic[i:to])

		want := float32(1)
		if them > Floor && you < them*Louder {
			want = 0
		}
		for k := i; k < to; k++ {
			switch {
			case at < want:
				at = min(at+step, want)
			case at > want:
				at = max(at-step, want)
			}
			// Linear avoids midpoint clipping during handover.
			out[k] = mic[k]*at + system[k]*(1-at)
		}
	}
	return out
}

// Offset is how many samples the system tap trails the microphone.
func Offset(mic, system []float32) int {
	// Compare the loudest ten seconds of the far side.
	window := min(10*Rate, len(system), len(mic))
	if window < Rate {
		return 0
	}
	best, from := 0.0, 0
	for s := 0; s+window <= len(system); s += max(window/2, 1) {
		if l := Loud(system[s : s+window]); l > best {
			best, from = l, s
		}
	}
	if best < Floor {
		return 0
	}
	a, b := system[from:from+window], mic[from:from+window]

	// Coarse scan first, then refine around the winner. scan() reports how far
	// the microphone trails the tap, so a late-written tap comes back negative.
	coarse := scan(a, b, -Search, Search, Rate/1000)
	lag := -scan(a, b, coarse-Rate/1000, coarse+Rate/1000, 1)
	if lag <= 0 {
		return 0 // nothing to align
	}
	return lag
}

// Search is the largest lag considered by Offset.
const Search = Rate / 2

// scan finds the best lag by correlation.
func scan(a, b []float32, from, to, step int) int {
	bestLag, bestScore := 0, -2.0
	for lag := from; lag <= to; lag += step {
		var dot, na, nb float64
		for i := range a {
			j := i + lag
			if j < 0 || j >= len(b) {
				continue
			}
			dot += float64(a[i]) * float64(b[j])
			na += float64(a[i]) * float64(a[i])
			nb += float64(b[j]) * float64(b[j])
		}
		if na == 0 || nb == 0 {
			continue
		}
		if score := dot / (math.Sqrt(na) * math.Sqrt(nb)); score > bestScore {
			bestLag, bestScore = lag, score
		}
	}
	return bestLag
}

func sqrt(v float32) float32  { return float32(math.Sqrt(float64(v))) }
func clamp(v float32) float32 { return max(min(v, 1), -1) }

// Voices decodes audio for transcription.
func Voices(path string) ([]float32, error) {
	if mic, system, ok := Sides(path); ok {
		return Fold(mic, system), nil
	}
	return Decode(path)
}

// Mono writes a 16 kHz mono WAV.
func Mono(path string, samples []float32) error {
	body := make([]byte, 2*len(samples))
	for i, s := range samples {
		binary.LittleEndian.PutUint16(body[2*i:], uint16(int16(clamp(s)*32767)))
	}
	head := make([]byte, 0, 44)
	head = append(head, "RIFF"...)
	head = binary.LittleEndian.AppendUint32(head, uint32(36+len(body)))
	head = append(head, "WAVEfmt "...)
	head = binary.LittleEndian.AppendUint32(head, 16)
	head = binary.LittleEndian.AppendUint16(head, 1) // PCM
	head = binary.LittleEndian.AppendUint16(head, 1) // mono
	head = binary.LittleEndian.AppendUint32(head, Rate)
	head = binary.LittleEndian.AppendUint32(head, Rate*2)
	head = binary.LittleEndian.AppendUint16(head, 2)
	head = binary.LittleEndian.AppendUint16(head, 16)
	head = append(head, "data"...)
	head = binary.LittleEndian.AppendUint32(head, uint32(len(body)))
	return os.WriteFile(path, append(head, body...), 0o644)
}

// Listenable is the playback file served to the UI.
func Listenable(path, cache string) (string, error) {
	mic, system, ok := Sides(path)
	if !ok {
		return path, nil
	}
	if err := os.MkdirAll(cache, 0o755); err != nil {
		return path, err
	}
	// Encode the fold version in the cache name.
	folded := filepath.Join(cache, Fold_+filepath.Base(path))

	source, err := os.Stat(path)
	if err != nil {
		return path, err
	}
	if made, err := os.Stat(folded); err == nil && made.ModTime().After(source.ModTime()) {
		return folded, nil
	}
	if err := Mono(folded, Fold(mic, system)); err != nil {
		return path, err
	}
	return folded, nil
}

// Peaks is the number of waveform buckets drawn in the UI.
const Peaks = 300

// Shape returns a 0..1 waveform envelope for the UI.
func Shape(path string) []float32 {
	samples, err := Decode(path)
	if err != nil || len(samples) == 0 {
		return nil
	}
	bucket := max(len(samples)/Peaks, 1)
	out := make([]float32, 0, Peaks)
	loudest := float32(0)
	for at := 0; at < len(samples); at += bucket {
		v := float32(Loud(samples[at:min(at+bucket, len(samples))]))
		loudest = max(loudest, v)
		out = append(out, v)
	}
	if loudest > 0 {
		for i := range out {
			out[i] /= loudest
		}
	}
	return out
}

// Experiment 4 checks whether the tap/mic delay drifts over time.
package main

import (
	"fmt"
	"math"
	"os"

	"github.com/dmykolen/meeting-transcriber-go/internal/media"
)

const (
	window = 4 * media.Rate  // four seconds: enough speech to correlate on
	step   = 20 * media.Rate // a reading every twenty seconds
	reach  = media.Rate / 2  // search up to half a second either way
	loud   = 0.03
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: go run ./exp/04_drift <recording.wav>")
		os.Exit(2)
	}
	room, tap, ok := media.Sides(os.Args[1])
	if !ok {
		fmt.Println("not one of ours")
		os.Exit(1)
	}
	fmt.Printf("file    %s\nlength  %.1f s\n\n", os.Args[1], float64(len(tap))/media.Rate)
	fmt.Println("   time    delay   confidence   drawn against the file")
	fmt.Println("  ─────────────────────────────────────────────────────────────────")

	type point struct{ t, ms float64 }
	var seen []point

	for i := 0; i+window <= len(tap) && i+window <= len(room); i += step {
		t := tap[i : i+window]
		if rms(t) < loud {
			continue
		}
		// Search both directions. A negative answer is physically impossible
		// for a room, so if negatives keep winning the reference itself is
		// arriving late — which is its own kind of finding.
		best, at := 0.0, 0
		for lag := -reach; lag <= reach; lag += 4 {
			if c := match(room, tap, i, window, lag); c > best {
				best, at = c, lag
			}
		}
		if best < 0.15 {
			continue // nothing recognisable in this window; say nothing
		}
		ms := float64(at) / media.Rate * 1000
		seen = append(seen, point{float64(i) / media.Rate, ms})
		fmt.Printf("  %6s  %+7.1f ms   %.2f   %s\n",
			mmss(float64(i)/media.Rate), ms, best, bar(ms))
	}

	if len(seen) < 3 {
		fmt.Println("\n  too few usable windows to say anything")
		return
	}

	// A straight line through the readings. Its slope is the drift rate, and
	// parts per million is the number that names the cause.
	var sx, sy, sxx, sxy float64
	n := float64(len(seen))
	for _, p := range seen {
		sx += p.t
		sy += p.ms
		sxx += p.t * p.t
		sxy += p.t * p.ms
	}
	slope := (n*sxy - sx*sy) / (n*sxx - sx*sx) // ms of delay per second of file
	intercept := (sy - slope*sx) / n

	var spread float64
	for _, p := range seen {
		d := p.ms - (intercept + slope*p.t)
		spread += d * d
	}
	spread = math.Sqrt(spread / n)

	fmt.Printf("\n  readings          %d\n", len(seen))
	fmt.Printf("  best straight line  %.1f ms at the start, %+.4f ms per second\n", intercept, slope)
	fmt.Printf("  that is           %+.1f ppm of clock difference\n", slope*1000)
	fmt.Printf("  scatter about it  %.1f ms\n\n", spread)

	total := slope * seen[len(seen)-1].t
	switch {
	case math.Abs(total) > 20 && spread < math.Abs(total)/2:
		fmt.Printf("  DRIFT. The delay walks %.0f ms across the recording and the readings\n", total)
		fmt.Println("  sit on the line rather than scattering. The two capture devices are")
		fmt.Println("  running at slightly different speeds, so the echo is never at the")
		fmt.Println("  same place twice. No adaptive filter can converge on that, which is")
		fmt.Println("  why SpeexDSP and the app's own NLMS both removed 0.0 dB.")
		fmt.Println()
		fmt.Println("  The fix is not a better canceller. It is to resample one channel")
		fmt.Println("  onto the other's clock first, and then cancel.")
	case spread > 30:
		fmt.Println("  SCATTER, not drift. The delay jumps about without a trend, which")
		fmt.Println("  points at the capture dropping or repeating buffers rather than at")
		fmt.Println("  clock speed. Resampling would not help; the alignment has to be")
		fmt.Println("  re-established locally, block by block.")
	default:
		fmt.Println("  STEADY. The delay is essentially fixed, so drift is not the reason")
		fmt.Println("  the cancellers failed and something else is. Look at whether the tap")
		fmt.Println("  is really what was played — volume, EQ or spatial processing between")
		fmt.Println("  the tap and the speaker would make the reference non-linear.")
	}
}

func match(room, tap []float32, from, n, lag int) float64 {
	var dot, a, b float64
	for k := 0; k < n; k++ {
		j := from + k - lag
		if j < 0 || j >= len(tap) {
			continue
		}
		x, y := float64(room[from+k]), float64(tap[j])
		dot += x * y
		a += x * x
		b += y * y
	}
	if a == 0 || b == 0 {
		return 0
	}
	return math.Abs(dot) / math.Sqrt(a*b)
}

// bar draws the reading so a ramp is visible without plotting anything.
func bar(ms float64) string {
	const wide = 44
	at := int((ms + 500) / 1000 * wide)
	at = max(0, min(wide-1, at))
	out := make([]rune, wide)
	for i := range out {
		out[i] = '·'
	}
	out[wide/2] = '|' // zero
	out[at] = '●'
	return string(out)
}

func rms(a []float32) float64 {
	if len(a) == 0 {
		return 0
	}
	s := 0.0
	for _, v := range a {
		s += float64(v) * float64(v)
	}
	return math.Sqrt(s / float64(len(a)))
}

func mmss(s float64) string { return fmt.Sprintf("%d:%02d", int(s)/60, int(s)%60) }

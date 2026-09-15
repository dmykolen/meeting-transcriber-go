// Experiment 1 measures how much far-side audio is still audible twice.
package main

import (
	"fmt"
	"math"
	"os"
	"sort"

	"github.com/dmykolen/meeting-transcriber-go/internal/media"
)

// Half a second balances local detail against noisy spikes.
const block = media.Rate / 2

// Only score blocks where the far side is genuinely talking.
const loud = 0.03

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: go run ./exp/01_echo <recording.wav>")
		os.Exit(2)
	}
	path := os.Args[1]

	room, tap, ok := media.Sides(path)
	if !ok {
		fmt.Printf("%s is not one of ours (needs to be stereo mic/tap)\n", path)
		os.Exit(1)
	}
	fmt.Printf("file      %s\n", path)
	fmt.Printf("length    %.1f s at %d Hz\n\n", float64(len(tap))/media.Rate, media.Rate)

	// ── 1. the delay, block by block ─────────────────────────────────────────
	//
	// One number for a whole meeting was the first mistake: it came out at
	// -230 ms, which is a sound arriving before it was played. Print the
	// distribution instead of a mean.
	fmt.Println("── where the room copy sits behind the tap ──")
	var delays []float64
	for i := 0; i+block < len(tap) && i+block < len(room); i += block {
		if rms(tap[i:i+block]) < loud {
			continue
		}
		c, lag := best(room[i:i+block], tap[i:i+block], media.Rate/4)
		if c > 0.25 {
			delays = append(delays, float64(lag)/media.Rate*1000)
		}
	}
	if len(delays) == 0 {
		fmt.Println("  no block holds a recognisable copy — nothing to cancel")
	} else {
		sort.Float64s(delays)
		fmt.Printf("  blocks holding a copy   %d\n", len(delays))
		fmt.Printf("  delay  min %.0f  p25 %.0f  median %.0f  p75 %.0f  max %.0f ms\n",
			delays[0], at(delays, .25), at(delays, .5), at(delays, .75), delays[len(delays)-1])
		fmt.Printf("  spread  %.0f ms — %s\n\n", delays[len(delays)-1]-delays[0],
			pick(delays[len(delays)-1]-delays[0] > 60,
				"one fixed delay cannot serve this; it has to be tracked",
				"a single delay would do"))
	}

	// ── 2. what the mix leaves ───────────────────────────────────────────────
	fmt.Println("── the mix ──")
	score(media.Fold(room, tap), room, tap)

	// ── 3. the floor: what is even possible ──────────────────────────────────
	//
	// Subtract a perfectly scaled, perfectly delayed copy of the tap, one block
	// at a time, with the gain and delay solved for that block. No real filter
	// beats this, so it says how much of the problem is solvable at all and how
	// much is the room being a room.
	fmt.Println("── the floor: the best a per-block delay-and-gain can do ──")
	ideal := make([]float32, len(room))
	copy(ideal, room)
	for i := 0; i+block < len(tap) && i+block < len(room); i += block {
		t := tap[i : i+block]
		if rms(t) < loud {
			continue
		}
		_, lag := best(room[i:i+block], t, media.Rate/4)
		g := gain(room[i:i+block], tap, i, lag, block)
		for k := 0; k < block; k++ {
			j := i + k - lag
			if j >= 0 && j < len(tap) {
				ideal[i+k] = room[i+k] - float32(g)*tap[j]
			}
		}
	}
	score(add(ideal, tap), room, tap)
}

// score reports what is left of the far side in a folded mono signal, and how
// much of the person in the chair survived. Both matter: silencing the room
// entirely would score perfectly on the first and ruin the recording.
func score(out, room, tap []float32) {
	echoed, own, quiet, n := 0, 0, 0, 0
	var level, kept float64
	worst, worstAt := -99.0, 0.0

	for i := 0; i+block < len(tap) && i+block < len(out); i += block {
		t := tap[i : i+block]
		if rms(t) < loud {
			continue
		}
		n++
		res := make([]float32, block)
		for k := range res {
			res[k] = out[i+k] - t[k]
		}
		d := db(rms(res) / rms(t))
		if d < -18 {
			quiet++ // safely under; nobody hears that
			continue
		}
		c, _ := best(res, t, media.Rate/4)
		if c > 0.25 {
			echoed++
			level += d
			if d > worst {
				worst, worstAt = d, float64(i)/media.Rate
			}
		} else {
			own++
			kept += d
		}
	}
	if n == 0 {
		fmt.Println("  nothing loud enough to judge")
		return
	}
	fmt.Printf("  loud far-side blocks        %d\n", n)
	fmt.Printf("  a second copy of them       %-4d (%2.0f%%)%s\n", echoed, pc(echoed, n),
		pick(echoed > 0, fmt.Sprintf("  at %.1f dB under them", level/float64(max(echoed, 1))), ""))
	fmt.Printf("  your own voice, kept        %-4d (%2.0f%%)  at %.1f dB\n", own, pc(own, n),
		kept/float64(max(own, 1)))
	fmt.Printf("  already inaudible           %-4d (%2.0f%%)\n", quiet, pc(quiet, n))
	if echoed > 0 {
		fmt.Printf("  worst block                 %.1f dB at %s\n", worst, mmss(worstAt))
	}

	// The counts above ask "is the residual shaped like them", which a loud
	// near-end voice answers yes to by accident. This asks the only question
	// the ear asks: how LOUD is whatever copy is left, next to the original.
	// Under -25 dB nobody hears a second copy; at -13 dB everybody does.
	var loudness []float64
	over := 0
	for i := 0; i+block < len(tap) && i+block < len(out); i += block {
		t := tap[i : i+block]
		if rms(t) < loud {
			continue
		}
		res := make([]float32, block)
		for k := range res {
			res[k] = out[i+k] - t[k]
		}
		_, lag := best(res, t, media.Rate/4)
		var dot, energy float64
		for k := 0; k < block; k++ {
			if j := i + k - lag; j >= 0 && j < len(tap) {
				dot += float64(res[k]) * float64(tap[j])
				energy += float64(tap[j]) * float64(tap[j])
			}
		}
		if energy == 0 {
			continue
		}
		l := db(math.Abs(dot/energy) * math.Sqrt(energy/float64(block)) / rms(t))
		loudness = append(loudness, l)
		if l > -25 {
			over++
		}
	}
	if len(loudness) > 0 {
		sort.Float64s(loudness)
		fmt.Printf("  LEVEL of the copy left      median %.1f dB   p75 %.1f   p90 %.1f\n",
			at(loudness, .5), at(loudness, .75), at(loudness, .9))
		fmt.Printf("  audible (over -25 dB)       %d of %d (%.0f%%)\n",
			over, len(loudness), pc(over, len(loudness)))
	}
	fmt.Println()
}

// ── the small amount of arithmetic the above needs ───────────────────────────

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

func db(ratio float64) float64 { return 20 * math.Log10(ratio+1e-12) }

// best is how much of b, at its best non-negative delay, is present in a.
// Non-negative because the microphone cannot hear a sound before it is played.
func best(a, b []float32, maxLag int) (float64, int) {
	na, nb := rms(a), rms(b)
	if na == 0 || nb == 0 {
		return 0, 0
	}
	top, at := 0.0, 0
	for lag := 0; lag < maxLag; lag++ {
		s, n := 0.0, 0
		for i := 0; i+lag < len(a) && i < len(b); i++ {
			s += float64(a[i+lag]) * float64(b[i])
			n++
		}
		if n == 0 {
			continue
		}
		if c := math.Abs(s/float64(n)) / (na * nb); c > top {
			top, at = c, lag
		}
	}
	return top, at
}

// gain solves for the scale that removes as much of the delayed tap from the
// room as possible — the least-squares answer, which is the projection.
func gain(room, tap []float32, from, lag, n int) float64 {
	var dot, energy float64
	for k := 0; k < n; k++ {
		j := from + k - lag
		if j < 0 || j >= len(tap) {
			continue
		}
		dot += float64(room[k]) * float64(tap[j])
		energy += float64(tap[j]) * float64(tap[j])
	}
	if energy == 0 {
		return 0
	}
	return dot / energy
}

func add(a, b []float32) []float32 {
	out := make([]float32, min(len(a), len(b)))
	for i := range out {
		out[i] = a[i] + b[i]
	}
	return out
}

func at(sorted []float64, q float64) float64 { return sorted[int(q*float64(len(sorted)-1))] }
func pc(a, b int) float64                    { return 100 * float64(a) / float64(b) }
func mmss(s float64) string                  { return fmt.Sprintf("%d:%02d", int(s)/60, int(s)%60) }

func pick(cond bool, yes, no string) string {
	if cond {
		return yes
	}
	return no
}

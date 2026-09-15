// Experiment 2 tries FIR echo cancellation instead of ducking.
package main

import (
	"fmt"
	"math"
	"os"
	"sort"

	"github.com/dmykolen/meeting-transcriber-go/internal/media"
)

const (
	block = media.Rate / 2 // half a second
	// 64 taps at 16 kHz is 4 ms of room response after the bulk delay. Longer
	// fits more reflections and needs more data to stay well conditioned; this
	// is the value the sweep below settles on.
	taps = 64
	// How far the room copy can lag the tap. Experiment 1 measured 0-250 ms.
	reach = media.Rate / 4
	loud  = 0.03
	// Diagonal loading. Without it a block where the tap barely moves gives a
	// near-singular matrix and the solve returns a filter with enormous gain.
	ridge = 1e-4
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: go run ./exp/02_aec <recording.wav> [taps...]")
		os.Exit(2)
	}
	room, tap, ok := media.Sides(os.Args[1])
	if !ok {
		fmt.Println("not one of ours")
		os.Exit(1)
	}
	fmt.Printf("file    %s\nlength  %.1f s\n\n", os.Args[1], float64(len(tap))/media.Rate)

	fmt.Println("── what ships today ──")
	report(media.Fold(room, tap), room, tap)

	for _, n := range []int{16, 64, 192} {
		fmt.Printf("── least-squares room filter, %d taps ──\n", n)
		clean, stat := cancel(room, tap, n)
		stat.print()
		// Mixed the plain way — the tap at full level plus whatever the
		// microphone has left. No ducking anywhere.
		report(add(clean, tap), room, tap)
	}
}

type notes struct {
	blocks, fitted, refusedQuiet, refusedWorse int
	gain                                       []float64 // dB removed, per fitted block
	worstBlow                                  float64   // the most a refused fit would have added
	worstAt                                    float64
}

func (n *notes) print() {
	sort.Float64s(n.gain)
	fmt.Printf("  blocks                    %d\n", n.blocks)
	fmt.Printf("  filter fitted and used    %d (%.0f%%)\n", n.fitted, pc(n.fitted, n.blocks))
	fmt.Printf("  refused, far side quiet   %d\n", n.refusedQuiet)
	fmt.Printf("  refused, made it worse    %d%s\n", n.refusedWorse,
		iff(n.refusedWorse > 0, fmt.Sprintf("  (worst would have added %.1f dB at %s)", n.worstBlow, mmss(n.worstAt)), ""))
	if len(n.gain) > 0 {
		fmt.Printf("  echo removed  median %.1f dB   p10 %.1f   p90 %.1f   best %.1f\n",
			at(n.gain, .5), at(n.gain, .1), at(n.gain, .9), n.gain[len(n.gain)-1])
	}
}

// cancel fits a short room filter per block and subtracts what it predicts.
func cancel(room, tap []float32, n int) ([]float32, *notes) {
	out := make([]float32, len(room))
	copy(out, room)
	st := &notes{}

	for i := 0; i+block <= len(room) && i+block <= len(tap); i += block {
		st.blocks++
		near := room[i : i+block]

		// Nothing to cancel: the far side is not talking. This is also the
		// headphones case and the "meeting is one person" case, and in both the
		// right answer is to leave the microphone completely alone.
		if rms(tap[max(i-reach, 0):i+block]) < loud {
			st.refusedQuiet++
			continue
		}

		lag := delay(near, tap, i)
		w := solve(near, tap, i, lag, n)
		if w == nil {
			st.refusedQuiet++
			continue
		}

		fixed := make([]float32, block)
		for k := range fixed {
			var y float64
			for j := 0; j < n; j++ {
				at := i + k - lag - j
				if at >= 0 && at < len(tap) {
					y += w[j] * float64(tap[at])
				}
			}
			fixed[k] = near[k] - float32(y)
		}

		before, after := rms(near), rms(fixed)
		// A fit that does not clearly reduce the block is a fit that has
		// latched onto the near-end voice. Refuse it. This is the guard that
		// keeps double-talk intact: while both are speaking the filter cannot
		// explain the block, so it is not allowed to try.
		if after >= before*0.97 {
			st.refusedWorse++
			if blow := db(after / before); blow > st.worstBlow {
				st.worstBlow, st.worstAt = blow, float64(i)/media.Rate
			}
			continue
		}
		st.fitted++
		st.gain = append(st.gain, -db(after/before))
		copy(out[i:], fixed)
	}
	return out, st
}

// delay is the bulk delay of the room copy for this block: never negative,
// because sound cannot arrive before it is played.
func delay(near, tap []float32, from int) int {
	nn := rms(near)
	if nn == 0 {
		return 0
	}
	top, at := 0.0, 0
	for lag := 0; lag < reach; lag += 8 { // 0.5 ms grid, then refine
		if c := match(near, tap, from, lag); c > top {
			top, at = c, lag
		}
	}
	for lag := max(at-8, 0); lag <= at+8; lag++ {
		if c := match(near, tap, from, lag); c > top {
			top, at = c, lag
		}
	}
	return at
}

func match(near, tap []float32, from, lag int) float64 {
	var dot, a, b float64
	for k := range near {
		j := from + k - lag
		if j < 0 || j >= len(tap) {
			continue
		}
		x, y := float64(near[k]), float64(tap[j])
		dot += x * y
		a += x * x
		b += y * y
	}
	if a == 0 || b == 0 {
		return 0
	}
	return math.Abs(dot) / math.Sqrt(a*b)
}

// solve fits w minimising |near - w*tap| by the normal equations, Cholesky with
// a ridge. Textbook Wiener; nothing here is novel and that is the point.
func solve(near, tap []float32, from, lag, n int) []float64 {
	R := make([]float64, n*n)
	r := make([]float64, n)
	for k := range near {
		x := make([]float64, n)
		for j := 0; j < n; j++ {
			if at := from + k - lag - j; at >= 0 && at < len(tap) {
				x[j] = float64(tap[at])
			}
		}
		for a := 0; a < n; a++ {
			r[a] += x[a] * float64(near[k])
			for b := a; b < n; b++ {
				R[a*n+b] += x[a] * x[b]
			}
		}
	}
	power := 0.0
	for a := 0; a < n; a++ {
		power += R[a*n+a]
	}
	if power == 0 {
		return nil
	}
	load := ridge * power / float64(n)
	for a := 0; a < n; a++ {
		R[a*n+a] += load
		for b := a + 1; b < n; b++ {
			R[b*n+a] = R[a*n+b]
		}
	}
	return cholesky(R, r, n)
}

func cholesky(R, r []float64, n int) []float64 {
	L := make([]float64, n*n)
	for i := 0; i < n; i++ {
		for j := 0; j <= i; j++ {
			s := R[i*n+j]
			for k := 0; k < j; k++ {
				s -= L[i*n+k] * L[j*n+k]
			}
			if i == j {
				if s <= 0 {
					return nil // not positive definite; the block is degenerate
				}
				L[i*n+i] = math.Sqrt(s)
			} else {
				L[i*n+j] = s / L[j*n+j]
			}
		}
	}
	y := make([]float64, n)
	for i := 0; i < n; i++ {
		s := r[i]
		for k := 0; k < i; k++ {
			s -= L[i*n+k] * y[k]
		}
		y[i] = s / L[i*n+i]
	}
	w := make([]float64, n)
	for i := n - 1; i >= 0; i-- {
		s := y[i]
		for k := i + 1; k < n; k++ {
			s -= L[k*n+i] * w[k]
		}
		w[i] = s / L[i*n+i]
	}
	return w
}

// ── scoring, the same question experiment 1 asks ─────────────────────────────

// report measures the one thing the ear cares about: how loud the *coherent*
// part of the residual is — the part that is a copy of the far side.
//
// The first version of this asked "does the residual correlate with the tap"
// and classified the block yes or no. That answered the wrong question twice
// over: a quiet echo and a loud voice both cross the same threshold, so ducking
// scored well by being quiet while still echoing, and cancelling scored badly
// by keeping the voice. What matters is the *level* of the copy, so measure it
// directly — project the residual onto the delayed far side and report how far
// under the far side that projection sits.
//
// Below about -20 dB nobody hears a second copy. At -13 dB everybody does.
func report(out, room, tap []float32) {
	var echo []float64
	heard, n := 0, 0
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
		// How much of the delayed far side is in the residual, and how loud
		// that much is. The projection is the least-squares answer, so this is
		// the most generous reading of "there is no echo here" available.
		lag := delay(res, tap, i)
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
		// |g| * |tap| is the level of the copy; compare it with the far side.
		level := db(math.Abs(dot/energy) * math.Sqrt(energy/float64(block)) / rms(t))
		echo = append(echo, level)
		if level > -20 {
			heard++
		}
	}
	if n == 0 {
		fmt.Println("  nothing loud enough to judge")
		return
	}
	sort.Float64s(echo)
	fmt.Printf("  copy of the far side left in the room, relative to it:\n")
	fmt.Printf("    median %.1f dB   p75 %.1f   p90 %.1f   worst %.1f\n",
		at(echo, .5), at(echo, .75), at(echo, .9), echo[len(echo)-1])
	fmt.Printf("  blocks where it is audible (above -20 dB)   %d of %d (%.0f%%)\n\n",
		heard, n, pc(heard, n))
}

func peak(a, b []float32) (float64, int) {
	na, nb := rms(a), rms(b)
	if na == 0 || nb == 0 {
		return 0, 0
	}
	top, at := 0.0, 0
	for lag := 0; lag < reach; lag += 4 {
		s, m := 0.0, 0
		for i := 0; i+lag < len(a) && i < len(b); i++ {
			s += float64(a[i+lag]) * float64(b[i])
			m++
		}
		if m == 0 {
			continue
		}
		if c := math.Abs(s/float64(m)) / (na * nb); c > top {
			top, at = c, lag
		}
	}
	return top, at
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

func db(r float64) float64 { return 20 * math.Log10(r+1e-12) }
func pc(a, b int) float64  { return 100 * float64(a) / float64(b) }
func at(s []float64, q float64) float64 {
	return s[int(q*float64(len(s)-1))]
}
func mmss(s float64) string { return fmt.Sprintf("%d:%02d", int(s)/60, int(s)%60) }
func iff(c bool, a, b string) string {
	if c {
		return a
	}
	return b
}
func add(a, b []float32) []float32 {
	out := make([]float32, min(len(a), len(b)))
	for i := range out {
		out[i] = a[i] + b[i]
	}
	return out
}

//go:build speex

// Experiment 3 compares SpeexDSP echo cancellation with the shipping fold.
package main

/*
#cgo CFLAGS: -I/opt/homebrew/Cellar/speexdsp/1.2.1/include
#cgo LDFLAGS: -L/opt/homebrew/lib -lspeexdsp
#include <speex/speex_echo.h>
#include <speex/speex_preprocess.h>
#include <stdlib.h>
*/
import "C"

import (
	"fmt"
	"math"
	"os"
	"sort"
	"unsafe"

	"github.com/dmykolen/meeting-transcriber-go/internal/media"
)

const (
	block = media.Rate / 2
	loud  = 0.03
	reach = media.Rate / 4
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: go run ./exp/03_speex <recording.wav>")
		os.Exit(2)
	}
	room, tap, ok := media.Sides(os.Args[1])
	if !ok {
		fmt.Println("not one of ours")
		os.Exit(1)
	}
	fmt.Printf("file    %s\nlength  %.1f s\n", os.Args[1], float64(len(tap))/media.Rate)
	fmt.Printf("levels  room %.1f dBFS   tap %.1f dBFS%s\n\n",
		db(rms(room)), db(rms(tap)),
		iff(db(rms(room)) < -40 || db(rms(tap)) < -40,
			"   ← quiet; int16 conversion may be losing the tail", ""))

	fmt.Println("── what ships today: duck the microphone while they talk ──")
	report(media.Fold(room, tap), room, tap)

	// 20 ms frames. The tail sweep is the experiment: 128 ms is the usual
	// recommendation, and this room needs more than that if experiment 1 was
	// right about the spread.
	frame := media.Rate / 50
	for _, tailMs := range []int{128, 256, 400} {
		tail := media.Rate * tailMs / 1000
		fmt.Printf("── speexdsp, %d ms frame, %d ms tail (%d taps) ──\n", 1000*frame/media.Rate, tailMs, tail)
		clean, damage := speex(room, tap, frame, tail)
		fmt.Printf("  microphone-only stretches, level change  %+.1f dB %s\n",
			damage, verdict(damage))
		erle(room, clean, tap)
		report(add(clean, tap), room, tap)
	}
}

// speex runs the canceller over the whole file and reports what it did to
// stretches where the far side was silent — the edge case that decides whether
// this is safe. Nothing may be removed from the microphone when there is no
// echo to remove, and a canceller that quietly attenuates the near end there is
// worse than the echo it fixes.
func speex(room, tap []float32, frame, tail int) ([]float32, float64) {
	st := C.speex_echo_state_init(C.int(frame), C.int(tail))
	defer C.speex_echo_state_destroy(st)
	rate := C.int(media.Rate)
	C.speex_echo_ctl(st, C.SPEEX_ECHO_SET_SAMPLING_RATE, unsafe.Pointer(&rate))

	near := make([]C.spx_int16_t, frame)
	far := make([]C.spx_int16_t, frame)
	out := make([]C.spx_int16_t, frame)
	clean := make([]float32, len(room))

	var quietBefore, quietAfter float64
	var quietN int

	for i := 0; i+frame <= len(room) && i+frame <= len(tap); i += frame {
		for k := 0; k < frame; k++ {
			near[k] = C.spx_int16_t(clamp(room[i+k]) * 32767)
			far[k] = C.spx_int16_t(clamp(tap[i+k]) * 32767)
		}
		C.speex_echo_cancellation(st,
			(*C.spx_int16_t)(&near[0]), (*C.spx_int16_t)(&far[0]), (*C.spx_int16_t)(&out[0]))
		for k := 0; k < frame; k++ {
			clean[i+k] = float32(out[k]) / 32767
		}
		// The far side is silent here, so the microphone should come out
		// untouched. Measure it rather than hope.
		if rms(tap[i:i+frame]) < 0.002 && rms(room[i:i+frame]) > 0.01 {
			quietBefore += rms(room[i : i+frame])
			quietAfter += rms(clean[i : i+frame])
			quietN++
		}
	}
	if quietN == 0 {
		return clean, 0
	}
	return clean, db(quietAfter / quietBefore)
}

// erle is the number that says whether the canceller is working at all: on
// stretches where only the far side is talking, the microphone holds nothing
// but the echo, so whatever the canceller takes out is echo it removed.
//
// This is the standard measure and it should have been the first thing printed.
// The report below rewards ducking — a signal turned down 12 dB scores 12 dB
// better on "how loud is the copy" while removing nothing — so comparing a
// canceller against a ducker with that number alone is comparing two different
// questions. Echo return loss enhancement asks only about removal.
func erle(room, clean, tap []float32) {
	var gains []float64
	for i := 0; i+block <= len(tap) && i+block <= len(clean); i += block {
		// Far side talking, near side not: what is in the microphone is echo.
		if rms(tap[i:i+block]) < loud || rms(room[i:i+block]) > rms(tap[i:i+block])*0.7 {
			continue
		}
		before, after := rms(room[i:i+block]), rms(clean[i:i+block])
		if before < 1e-5 {
			continue
		}
		gains = append(gains, -db(after/before))
	}
	if len(gains) == 0 {
		fmt.Println("  ERLE  no far-side-only stretch to measure on")
		return
	}
	sort.Float64s(gains)
	fmt.Printf("  ERLE on far-side-only stretches  median %.1f dB  p25 %.1f  p75 %.1f  (%d blocks)%s\n",
		at(gains, .5), at(gains, .25), at(gains, .75), len(gains),
		iff(at(gains, .5) < 3, "   ← not converging", ""))
}

func verdict(d float64) string {
	switch {
	case d < -3:
		return "← it is eating the near end; unusable"
	case d < -1:
		return "← noticeable near-end loss"
	default:
		return "← the near end survives"
	}
}

// report measures the level of the coherent copy of the far side left in the
// mix, relative to the far side. Below about -20 dB nobody hears a second copy.
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
	fmt.Printf("  copy of the far side left, relative to it:\n")
	fmt.Printf("    median %.1f dB   p75 %.1f   p90 %.1f   worst %.1f\n",
		at(echo, .5), at(echo, .75), at(echo, .9), echo[len(echo)-1])
	fmt.Printf("  blocks where it is audible (over -20 dB)  %d of %d (%.0f%%)\n\n",
		heard, n, pc(heard, n))
}

func delay(near, tap []float32, from int) int {
	top, at := 0.0, 0
	for lag := 0; lag < reach; lag += 8 {
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

func clamp(v float32) float32           { return max(min(v, 1), -1) }
func db(r float64) float64              { return 20 * math.Log10(r+1e-12) }
func pc(a, b int) float64               { return 100 * float64(a) / float64(b) }
func at(s []float64, q float64) float64 { return s[int(q*float64(len(s)-1))] }

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

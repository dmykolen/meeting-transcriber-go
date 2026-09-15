//go:build speex

// Experiment 5 aligns the channels before echo cancellation.
package main

/*
#cgo CFLAGS: -I/opt/homebrew/Cellar/speexdsp/1.2.1/include
#cgo LDFLAGS: -L/opt/homebrew/lib -lspeexdsp
#include <speex/speex_echo.h>
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
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: go run ./exp/05_aligned <recording.wav>")
		os.Exit(2)
	}
	room, tap, ok := media.Sides(os.Args[1])
	if !ok {
		fmt.Println("not one of ours")
		os.Exit(1)
	}
	fmt.Printf("file    %s\nlength  %.1f s\n\n", os.Args[1], float64(len(tap))/media.Rate)

	fmt.Println("Shift is how far the tap is moved EARLIER before cancelling.")
	fmt.Println("ERLE is measured only where the far side talks and the room does not,")
	fmt.Println("so whatever comes out is echo that was removed and nothing else.")
	fmt.Println()
	fmt.Printf("  %6s  %8s  %8s  %8s  %s\n", "shift", "median", "p25", "p75", "")
	fmt.Println("  ─────────────────────────────────────────────────────────────")

	best, bestAt := -99.0, 0
	for _, ms := range []int{0, 100, 160, 190, 200, 210, 215, 220, 230, 260, 320} {
		shifted := earlier(tap, media.Rate*ms/1000)
		clean := speex(room, shifted, media.Rate/50, media.Rate*128/1000)
		med, p25, p75, n := erle(room, clean, shifted)
		mark := ""
		if med > best {
			best, bestAt = med, ms
			mark = "  ←"
		}
		fmt.Printf("  %4d ms  %+7.1f dB  %+7.1f  %+7.1f  (%d blocks)%s\n", ms, med, p25, p75, n, mark)
	}

	fmt.Printf("\n  best shift  %d ms, removing %.1f dB of echo\n\n", bestAt, best)
	if best < 6 {
		fmt.Println("  Still not converging. Alignment was not the whole story — look next")
		fmt.Println("  at whether the tap is really what reached the speakers.")
		return
	}

	// The whole point: what does the finished mix sound like now, and did the
	// person in the chair survive it?
	shifted := earlier(tap, media.Rate*bestAt/1000)
	clean := speex(room, shifted, media.Rate/50, media.Rate*128/1000)

	fmt.Println("── what ships today: duck the microphone while they talk ──")
	report(media.Fold(room, tap), tap)

	fmt.Println("── aligned, cancelled, mixed flat (no ducking at all) ──")
	report(add(clean, shifted), shifted)

	fmt.Printf("  microphone-only stretches, level change  %+.1f dB %s\n\n",
		untouched(room, clean, shifted), "← nothing may be removed where there is no echo")
}

// earlier moves the tap forward in time by n samples, which is the same as
// saying the file wrote it n samples late.
func earlier(tap []float32, n int) []float32 {
	if n <= 0 || n >= len(tap) {
		return tap
	}
	out := make([]float32, len(tap))
	copy(out, tap[n:])
	return out
}

func speex(room, tap []float32, frame, tail int) []float32 {
	st := C.speex_echo_state_init(C.int(frame), C.int(tail))
	defer C.speex_echo_state_destroy(st)
	rate := C.int(media.Rate)
	C.speex_echo_ctl(st, C.SPEEX_ECHO_SET_SAMPLING_RATE, unsafe.Pointer(&rate))

	near := make([]C.spx_int16_t, frame)
	far := make([]C.spx_int16_t, frame)
	out := make([]C.spx_int16_t, frame)
	clean := make([]float32, len(room))

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
	}
	return clean
}

// erle: on stretches where only the far side is talking, the microphone holds
// nothing but echo, so whatever the canceller took out is echo it removed.
func erle(room, clean, tap []float32) (med, p25, p75 float64, n int) {
	var g []float64
	for i := 0; i+block <= len(tap) && i+block <= len(clean); i += block {
		if rms(tap[i:i+block]) < loud || rms(room[i:i+block]) > rms(tap[i:i+block])*0.7 {
			continue
		}
		before, after := rms(room[i:i+block]), rms(clean[i:i+block])
		if before < 1e-5 {
			continue
		}
		g = append(g, -db(after/before))
	}
	if len(g) == 0 {
		return 0, 0, 0, 0
	}
	sort.Float64s(g)
	return at(g, .5), at(g, .25), at(g, .75), len(g)
}

// untouched: where the far side is silent there is no echo, so the microphone
// must come out exactly as it went in. A canceller that quietly attenuates the
// near end here is worse than the echo it fixes.
func untouched(room, clean, tap []float32) float64 {
	var before, after float64
	for i := 0; i+block <= len(tap) && i+block <= len(clean); i += block {
		if rms(tap[i:i+block]) > 0.002 || rms(room[i:i+block]) < 0.01 {
			continue
		}
		before += rms(room[i : i+block])
		after += rms(clean[i : i+block])
	}
	if before == 0 {
		return 0
	}
	return db(after / before)
}

// report: the level of the coherent copy of the far side left in the mix.
// Below about -20 dB nobody hears a second copy; -13 dB everybody does.
func report(out, tap []float32) {
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
		lag, _ := where(res, tap, i)
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

// where searches both directions, because this whole experiment exists because
// the answer turned out to be negative.
func where(a, b []float32, from int) (int, float64) {
	top, at := 0.0, 0
	for lag := -media.Rate / 3; lag <= media.Rate/3; lag += 8 {
		var dot, x, y float64
		for k := range a {
			j := from + k - lag
			if j < 0 || j >= len(b) {
				continue
			}
			p, q := float64(a[k]), float64(b[j])
			dot += p * q
			x += p * p
			y += q * q
		}
		if x == 0 || y == 0 {
			continue
		}
		if c := math.Abs(dot) / math.Sqrt(x*y); c > top {
			top, at = c, lag
		}
	}
	return at, top
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

func add(a, b []float32) []float32 {
	out := make([]float32, min(len(a), len(b)))
	for i := range out {
		out[i] = a[i] + b[i]
	}
	return out
}

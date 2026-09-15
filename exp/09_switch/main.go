//go:build old

// Experiment 9 records the switch-based fold that replaced summing.
//
// This measures four mixes on the same recording: the fold as it ships, the
// fold ducked far harder, and two switches. Both numbers that matter are
// printed for each — how loud the leftover copy is, and whether the person in
// the chair survived.
package main

import (
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dmykolen/meeting-transcriber-go/internal/media"
)

const (
	block = media.Rate / 2
	loud  = 0.03
	// The gain moves this often, so a switch is never a click.
	hop = media.Rate / 50
)

func main() {
	id := "74"
	if len(os.Args) > 1 {
		id = os.Args[1]
	}
	home := os.Getenv("HOME") + "/MeetingTranscriber"
	path, title := recording(home, id)

	mic, tap, ok := media.Sides(path)
	if !ok {
		fmt.Println("not one of ours")
		os.Exit(1)
	}

	out, err := os.Create(fmt.Sprintf("exp/out/D-switch-%s.txt", id))
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer out.Close()
	say := func(f string, a ...any) {
		fmt.Printf(f, a...)
		fmt.Fprintf(out, f, a...)
	}

	say("EXPERIMENT D — a switch instead of a sum\n")
	say("recording %s — %s, %.1f s\n\n", id, title, float64(len(mic))/media.Rate)
	say("Two numbers decide this:\n")
	say("  ECHO  how loud the leftover copy of the far side is, relative to them.\n")
	say("        Under -25 dB nobody hears it. At -13 dB everybody does.\n")
	say("  YOU   how much of your own voice survives, where only you are talking.\n")
	say("        0 dB is untouched. Anything under -3 dB is a recording that lost you.\n\n")

	type mix struct {
		name string
		out  []float32
		note string
	}
	mixes := []mix{
		{"fold as it ships (duck 0.25)", media.Fold(media.Cancel(mic, tap), tap, media.ForEars),
			"sum: system + mic*0.25"},
		{"fold, ducked to 0.05", media.Fold(media.Cancel(mic, tap), tap, 0.05),
			"same sum, twenty times quieter a second copy"},
		{"fold, ducked to 0.02", media.Fold(media.Cancel(mic, tap), tap, 0.02),
			"same sum, near enough to a switch"},
		{"switch, soft", switched(mic, tap, false), "one owner per instant, ramped"},
		{"switch, hard", switched(mic, tap, true), "one owner per instant, no ramp"},
	}

	// The two channels are not aligned in the file: experiment 4 measured the
	// microphone leading the tap by a steady ~230 ms. A switch decides who owns
	// each 20 ms, so a quarter of a second of error means it changes hands a
	// quarter of a second late at both ends of every sentence — still on the
	// microphone while they have already started, still on the tap after they
	// have stopped. Align first, then switch.
	for _, ms := range []int{200, 230, 260} {
		aligned := earlier(tap, media.Rate*ms/1000)
		mixes = append(mixes,
			mix{fmt.Sprintf("switch, soft, tap %d ms earlier", ms),
				switched(mic, aligned, false), "aligned, then one owner per instant"})
	}

	for _, m := range mixes {
		// Judge every mix against the far side as that mix saw it.
		against := tap
		if n := strings.Index(m.name, "tap "); n >= 0 {
			var ms int
			fmt.Sscanf(m.name[n:], "tap %d ms", &ms)
			against = earlier(tap, media.Rate*ms/1000)
		}
		echo, heard, n := leftover(m.out, against)
		yours := survived(m.out, mic, against)
		say("── %s\n", m.name)
		say("   %s\n", m.note)
		say("   ECHO  median %6.1f dB   p75 %6.1f   p90 %6.1f   audible in %d of %d (%.0f%%)\n",
			echo[len(echo)/2], echo[len(echo)*3/4], echo[len(echo)*9/10], heard, n,
			100*float64(heard)/float64(n))
		say("   YOU   %+.1f dB %s\n\n", yours, judge(yours))
	}

	say("\nWhat to read from this: ducking harder buys decibels of echo at the price\n")
	say("of your own voice, because it is still a sum. A switch pays neither price,\n")
	say("because at any instant only one channel is in the mix at all.\n")
	fmt.Printf("\nwritten to exp/out/D-switch-%s.txt\n", id)
}

// switched hands each moment to whoever owns it.
//
// The decision is made on a twenty-millisecond hop and then smoothed, so that a
// change of speaker is a crossfade of a few tens of milliseconds rather than a
// click. Hard mode skips the smoothing, to show what the smoothing is worth.
func switched(mic, tap []float32, hard bool) []float32 {
	n := min(len(mic), len(tap))
	out := make([]float32, n)

	// How fast the crossfade moves per sample, for a 30 ms slide.
	rate := float32(1) / float32(media.Rate*30/1000)
	var at float32 = 1 // 1 = the microphone owns it, 0 = the tap does

	for i := 0; i < n; i += hop {
		to := min(i+hop, n)
		them, you := media.Loud(tap[i:to]), media.Loud(mic[i:to])

		// Whose moment is this? The tap is a clean digital signal, so anything
		// audible in it is genuinely them. The microphone holds you *and* their
		// room copy, so it only counts as yours when it is clearly louder than
		// what their copy could possibly be.
		want := float32(1)
		if them > loud/3 && you < them*3 {
			want = 0 // theirs
		}
		for k := i; k < to; k++ {
			if hard {
				at = want
			} else if at < want {
				at = min(at+rate, want)
			} else if at > want {
				at = max(at-rate, want)
			}
			// A crossfade of powers, not of amplitudes: two signals at half
			// amplitude each are a dip in loudness, and the dip is audible.
			out[k] = mic[k]*sqrt(at) + tap[k]*sqrt(1-at)
		}
	}
	return out
}

func sqrt(v float32) float32 { return float32(math.Sqrt(float64(v))) }

// earlier moves the tap forward in time, which is the same as saying the file
// wrote it that much late.
func earlier(tap []float32, n int) []float32 {
	if n <= 0 || n >= len(tap) {
		return tap
	}
	out := make([]float32, len(tap))
	copy(out, tap[n:])
	return out
}

// leftover: the level of the coherent copy of the far side in the finished mix.
func leftover(out, tap []float32) (levels []float64, heard, n int) {
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
		lag := where(res, tap, i)
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
		levels = append(levels, l)
		if l > -25 {
			heard++
		}
	}
	sort.Float64s(levels)
	if len(levels) == 0 {
		levels = []float64{0}
		n = 1
	}
	return
}

// survived: where the far side is silent, the mix should be the microphone and
// nothing else. This is what a mix that fixes the echo by deleting you fails.
func survived(out, mic, tap []float32) float64 {
	var before, after float64
	for i := 0; i+block <= len(tap) && i+block <= len(out); i += block {
		if rms(tap[i:i+block]) > 0.004 || rms(mic[i:i+block]) < 0.02 {
			continue
		}
		before += rms(mic[i : i+block])
		after += rms(out[i : i+block])
	}
	if before == 0 {
		return 0
	}
	return db(after / before)
}

func judge(d float64) string {
	switch {
	case d < -3:
		return "← your own voice is being lost"
	case d < -1:
		return "← noticeably quieter than you were"
	default:
		return "← you come through as recorded"
	}
}

func where(a, b []float32, from int) int {
	top, at := 0.0, 0
	for lag := -media.Rate / 3; lag <= media.Rate/3; lag += 16 {
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
	return at
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

func recording(home, id string) (path, title string) {
	out, err := exec.Command("sqlite3", filepath.Join(home, "meetings.db"),
		"SELECT audio||char(9)||title FROM recordings WHERE id="+id).Output()
	if err != nil || len(out) == 0 {
		fmt.Println("no such recording:", id, err)
		os.Exit(1)
	}
	bits := strings.SplitN(strings.TrimSpace(string(out)), "\t", 2)
	return filepath.Join(home, "recordings", bits[0]), bits[1]
}

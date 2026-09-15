// Experiment 10 measures the main transcription-pipeline recommendations.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/dmykolen/meeting-transcriber-go/internal/engine"
	"github.com/dmykolen/meeting-transcriber-go/internal/media"
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
	samples := media.Fold(mic, tap)

	out, err := os.Create(fmt.Sprintf("exp/out/E-pipeline-%s.txt", id))
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer out.Close()
	say := func(f string, a ...any) {
		fmt.Printf(f, a...)
		fmt.Fprintf(out, f, a...)
	}

	say("EXPERIMENT E — the recommendations, measured\n")
	say("recording %s — %s, %.1f s\n\n", id, title, float64(len(samples))/media.Rate)

	eng, err := engine.Open(filepath.Join(home, "models"),
		engine.Options{Language: "uk", Threads: 8, Transcriber: "whisper"})
	if err != nil {
		say("engine: %v\n", err)
		os.Exit(1)
	}
	defer eng.Close()

	began := time.Now()
	turns, err := eng.Transcribe(samples)
	took := time.Since(began)
	if err != nil {
		say("transcribe: %v\n", err)
		os.Exit(1)
	}

	loops, worst := repeats(turns)
	rounded, lengths := timing(turns)

	say("rows                       %d\n", len(turns))
	say("took                       %s (%.1fx realtime)\n\n",
		took.Round(time.Second), float64(len(samples))/media.Rate/took.Seconds())

	say("REPETITION\n")
	say("  identical adjacent rows  %d   longest run %d\n\n", loops, worst)

	say("TIMING  (forced alignment: a boundary where the word is, not rounded)\n")
	say("  starts on a whole second %d of %d (%.0f%%)%s\n", rounded, len(turns),
		100*float64(rounded)/float64(max(len(turns), 1)),
		iff(rounded*4 < len(turns), "   ← sub-second boundaries", "   ← still rounding"))
	say("  row length  median %.1f s   p10 %.1f   p90 %.1f\n\n",
		pick(lengths, .5), pick(lengths, .1), pick(lengths, .9))

	say("SEGMENTATION  (phrase boundaries, not character counts)\n")
	ended, long := shapes(turns)
	say("  rows ending on . ! ? …   %d of %d (%.0f%%)\n", ended, len(turns),
		100*float64(ended)/float64(max(len(turns), 1)))
	say("  rows over 25 words       %d\n\n", long)

	say("THE TRANSCRIPT\n")
	for _, t := range turns {
		say("  %s  %s\n", mmss(t.Start), t.Text)
	}
	fmt.Printf("\nwritten to exp/out/E-pipeline-%s.txt\n", id)
}

func repeats(turns []engine.Turn) (total, worst int) {
	run := 1
	for i := 1; i < len(turns); i++ {
		if turns[i].Text == turns[i-1].Text {
			total++
			run++
			worst = max(worst, run)
		} else {
			run = 1
		}
	}
	if worst < 2 {
		worst = 0
	}
	return
}

// timing: how many rows begin exactly on a whole second. Whisper's segment
// timestamps land there because they are rounded to one; token times do not.
func timing(turns []engine.Turn) (int, []float64) {
	rounded := 0
	var lengths []float64
	for _, t := range turns {
		if t.Start == float64(int(t.Start)) {
			rounded++
		}
		lengths = append(lengths, t.End-t.Start)
	}
	sort.Float64s(lengths)
	return rounded, lengths
}

func shapes(turns []engine.Turn) (ended, long int) {
	for _, t := range turns {
		s := strings.TrimSpace(t.Text)
		if s != "" && strings.ContainsAny(s[len(s)-1:], ".!?…") {
			ended++
		}
		if len(strings.Fields(s)) > 25 {
			long++
		}
	}
	return
}

func pick(sorted []float64, q float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	return sorted[int(q*float64(len(sorted)-1))]
}

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

func mmss(s float64) string { return fmt.Sprintf("%3d:%05.2f", int(s)/60, s-float64(int(s)/60*60)) }

func iff(c bool, a, b string) string {
	if c {
		return a
	}
	return b
}

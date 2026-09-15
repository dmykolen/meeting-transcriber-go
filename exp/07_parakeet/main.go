// Experiment 7 compares Parakeet with Whisper on the same audio.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"unicode"

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

	out, err := os.Create(fmt.Sprintf("exp/out/B-parakeet-%s.txt", id))
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer out.Close()
	say := func(f string, a ...any) {
		fmt.Printf(f, a...)
		fmt.Fprintf(out, f, a...)
	}

	say("EXPERIMENT B — Parakeet against Whisper on the same audio\n")
	say("recording %s — %s\n", id, title)
	say("audio %.1f s, the same mono the app feeds its models\n\n", float64(len(samples))/media.Rate)

	for _, which := range []string{"parakeet", "whisper"} {
		say("════════════════════════════════════════════════════════════════\n")
		say("%s\n\n", which)

		eng, err := engine.Open(filepath.Join(home, "models"), engine.Options{
			Language: "uk", Threads: 8, Transcriber: which,
		})
		if err != nil {
			say("  could not open: %v\n\n", err)
			continue
		}

		began := time.Now()
		// Run, not Transcribe: this is the pipeline the app runs, so the
		// speakers come out with the words and the diarization can be read
		// beside the transcript rather than guessed at separately.
		result, err := eng.Run(samples, nil)
		took := time.Since(began)
		eng.Close()
		if err != nil {
			say("  failed: %v\n\n", err)
			continue
		}

		loops, worst, where := repeats(result.Turns)
		cyr, lat, ru := alphabet(result.Turns)
		say("  rows %d   loops %d   worst run %d×  %s\n", len(result.Turns), loops, worst, where)
		say("  letters: cyrillic %d, latin %d   russian-only letters (ыъэё): %d %s\n",
			cyr, lat, ru, verdict(ru, cyr))
		say("  speakers found: %s\n", speakers(result.Turns))
		say("  took %s (%.1fx realtime)\n\n", took.Round(time.Second),
			float64(len(samples))/media.Rate/took.Seconds())

		for _, t := range result.Turns {
			say("  %s  %-18s  %s\n", mmss(t.Start), cut(t.Speaker, 18), t.Text)
		}
		say("\n")
	}
	fmt.Printf("\nwritten to exp/out/B-parakeet-%s.txt\n", id)
}

// alphabet counts what script the words came out in. A Ukrainian meeting
// transcribed with ы, ъ, э or ё in it was transcribed as Russian, which is the
// failure Parakeet has to be watched for.
func alphabet(turns []engine.Turn) (cyrillic, latin, russian int) {
	for _, t := range turns {
		for _, r := range t.Text {
			switch {
			case strings.ContainsRune("ыъэёЫЪЭЁ", r):
				russian++
				cyrillic++
			case unicode.Is(unicode.Cyrillic, r):
				cyrillic++
			case unicode.Is(unicode.Latin, r):
				latin++
			}
		}
	}
	return
}

func verdict(russian, cyrillic int) string {
	if cyrillic == 0 {
		return ""
	}
	if share := float64(russian) / float64(cyrillic); share > 0.005 {
		return "← this is Russian, not Ukrainian"
	}
	return "← Ukrainian"
}

func speakers(turns []engine.Turn) string {
	seen := map[string]int{}
	var order []string
	for _, t := range turns {
		who := t.Speaker
		if who == "" {
			who = "—"
		}
		if seen[who] == 0 {
			order = append(order, who)
		}
		seen[who]++
	}
	var out []string
	for _, w := range order {
		out = append(out, fmt.Sprintf("%s×%d", w, seen[w]))
	}
	return strings.Join(out, "  ")
}

func repeats(turns []engine.Turn) (total, worst int, where string) {
	run := 1
	for i := 1; i < len(turns); i++ {
		if turns[i].Text == turns[i-1].Text {
			total++
			run++
			if run > worst {
				worst, where = run, fmt.Sprintf("at %s %q", mmss(turns[i].Start), cut(turns[i].Text, 40))
			}
		} else {
			run = 1
		}
	}
	if worst < 2 {
		return total, 0, "none"
	}
	return total, worst, where
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

func cut(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n-1]) + "…"
	}
	return s
}

func mmss(s float64) string { return fmt.Sprintf("%3d:%02d", int(s)/60, int(s)%60) }

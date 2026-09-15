// Experiment 13 tests microphone-only meeting detection.
package main

import (
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/dmykolen/meeting-transcriber-go/internal/engine"
	"github.com/dmykolen/meeting-transcriber-go/internal/listen"
	"github.com/dmykolen/meeting-transcriber-go/internal/media"
)

// Utterances are cut the way the recorder cuts them, so the experiment sees
// what the detector would see.
const (
	quiet  = 0.012           // under this nobody is talking
	hop    = media.Rate / 50 // 20 ms, the recorder's frame
	gap    = 25              // frames of silence that end an utterance
	enough = 3 * media.Rate / 2
)

// Long is the minimum utterance the sweep tries as an alternative to enough. A
// voiceprint from a second and a half is mostly room.
var lengths = []int{3 * media.Rate / 2, 3 * media.Rate, 5 * media.Rate}

type recording struct {
	id    int
	kind  string
	title string
	audio string
}

func main() {
	home := os.Getenv("HOME") + "/MeetingTranscriber"
	rows := library(home)
	if len(rows) == 0 {
		fmt.Println("nothing in the library")
		os.Exit(1)
	}

	eng, err := engine.Open(filepath.Join(home, "models"),
		engine.Options{Language: "uk", Threads: 8, Transcriber: "whisper"})
	if err != nil {
		fmt.Println("engine:", err)
		os.Exit(1)
	}
	defer eng.Close()

	out, err := os.Create("exp/out/H-company.txt")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer out.Close()
	say := func(f string, a ...any) {
		fmt.Printf(f, a...)
		fmt.Fprintf(out, f, a...)
	}

	say("EXPERIMENT H — meeting or monologue, from the microphone alone\n\n")
	say("Ground truth is the system channel's own verdict on each recording.\n")
	say("The detector never sees it: it is given the microphone and nothing else.\n\n")

	// Voiceprints are the expensive part, so they are computed once per
	// recording and the thresholds are swept over the same vectors.
	type held struct {
		kind   string
		title  string
		prints [][][]float32 // one set per minimum utterance length
	}
	var all []held
	meetings, notes := 0, 0

	for _, r := range rows {
		mic, _, ok := media.Sides(filepath.Join(home, "recordings", r.audio))
		if !ok {
			continue
		}
		byLength := make([][][]float32, len(lengths))
		for _, u := range utterances(mic) {
			p := eng.Print(u)
			if len(p) == 0 {
				continue
			}
			for i, least := range lengths {
				if len(u) >= least {
					byLength[i] = append(byLength[i], p)
				}
			}
		}
		if len(byLength[0]) == 0 {
			continue
		}
		all = append(all, held{r.kind, r.title, byLength})
		if r.kind == "meeting" {
			meetings++
		} else {
			notes++
		}
		fmt.Printf("\r  read %d recordings…", len(all))
	}
	fmt.Print("\r")
	say("recordings with usable audio  %d   (%d meetings, %d notes)\n\n",
		len(all), meetings, notes)

	say("  %-5s %-6s %-8s  %-20s  %-20s  %s\n",
		"len", "alike", "support", "meetings kept", "notes left alone", "both right")
	say("  %s\n", strings.Repeat("─", 78))

	best, bestScore := "", -1.0
	for li, least := range lengths {
		for _, alike := range []float64{0.45, 0.55, 0.65, 0.75} {
			for _, support := range []int{1, 2, 3} {
				kept, calm := 0, 0
				for _, h := range all {
					there := company(h.prints[li], alike, support)
					if h.kind == "meeting" && there {
						kept++
					}
					if h.kind != "meeting" && !there {
						calm++
					}
				}
				// A meeting lost is data destroyed; a note called a meeting is a
				// summary nobody wanted. Weighted three to one, in that order.
				score := 3*share(kept, meetings) + share(calm, notes)
				mark := ""
				if score > bestScore {
					bestScore, best = score, fmt.Sprintf("at least %.1fs, alike %.2f, support %d",
						float64(least)/media.Rate, alike, support)
					mark = "  ←"
				}
				say("  %-5.1f %-6.2f %-8d  %3d of %-3d (%3.0f%%)   %3d of %-3d (%3.0f%%)   %3.0f%%%s\n",
					float64(least)/media.Rate, alike, support,
					kept, meetings, share(kept, meetings),
					calm, notes, share(calm, notes),
					share(kept+calm, meetings+notes), mark)
			}
		}
	}
	say("\n  best on this library: %s\n", best)

	say("\nRead the middle column first. A meeting the detector misses becomes a\n")
	say("note, and notes are discarded, so that column is data destroyed. The\n")
	say("right-hand column is only money: a monologue called a meeting gets a\n")
	say("summary nobody asked for.\n\n")
	say("The threshold in internal/listen/company.go is %.2f.\n", listen.Alike)
	fmt.Println("\nwritten to exp/out/H-company.txt")
}

// company reports whether a second person is really there.
//
// The first version asked only whether some utterance failed to match anybody
// seen so far, and on the library that called 79% of monologues meetings: one
// person across twenty minutes does not sound identical to themselves, and a
// single odd utterance was enough to invent a second speaker.
//
// So a voice has to be corroborated. support is how many utterances must land
// in the second cluster before it counts as a person rather than as a moment
// where somebody turned their head.
func company(prints [][]float32, alike float64, support int) bool {
	// Against the centre of each cluster, not against whichever member was
	// seen first. One person's utterances scatter around their own mean, so
	// comparing to a single sample makes the answer depend on which sample
	// happened to arrive first — which is what put 79% of monologues in the
	// meeting column on the first attempt.
	var known [][]float32
	var held []int
	for _, p := range prints {
		best, score := -1, alike
		for i, k := range known {
			if c := cosine(p, k); c >= score {
				best, score = i, c
			}
		}
		if best < 0 {
			known = append(known, append([]float32(nil), p...))
			held = append(held, 1)
			continue
		}
		held[best]++
		// Running mean: the cluster moves towards everything it has taken in.
		n := float32(held[best])
		for j := range known[best] {
			known[best][j] += (p[j] - known[best][j]) / n
		}
	}
	// The biggest cluster is whoever talked most. Anybody else needs support.
	best := 0
	for i, n := range held {
		if n > held[best] {
			best = i
		}
	}
	for i, n := range held {
		if i != best && n >= support {
			return true
		}
	}
	return false
}

// utterances cuts the microphone into stretches of speech, roughly as the VAD
// does. Energy rather than Silero: the point is to hand the embedder a person
// talking, and the exact boundary does not change who they are.
func utterances(mic []float32) [][]float32 {
	var out [][]float32
	start, silence := -1, 0
	for i := 0; i+hop <= len(mic); i += hop {
		if media.Loud(mic[i:i+hop]) > quiet {
			if start < 0 {
				start = i
			}
			silence = 0
			continue
		}
		if start < 0 {
			continue
		}
		if silence++; silence >= gap {
			if i-start >= enough {
				out = append(out, mic[start:i])
			}
			start, silence = -1, 0
		}
	}
	if start >= 0 && len(mic)-start >= enough {
		out = append(out, mic[start:])
	}
	return out
}

func cosine(a, b []float32) float64 {
	var dot, na, nb float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		na += float64(a[i]) * float64(a[i])
		nb += float64(b[i]) * float64(b[i])
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / math.Sqrt(na*nb)
}

func share(a, b int) float64 {
	if b == 0 {
		return 0
	}
	return 100 * float64(a) / float64(b)
}

func library(home string) []recording {
	out, err := exec.Command("sqlite3", filepath.Join(home, "meetings.db"),
		`SELECT id||char(9)||kind||char(9)||audio||char(9)||substr(title,1,40)
		 FROM recordings WHERE audio <> '' AND duration > 60 ORDER BY id`).Output()
	if err != nil {
		fmt.Println("library:", err)
		return nil
	}
	var rows []recording
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		b := strings.Split(line, "\t")
		if len(b) < 4 {
			continue
		}
		id, _ := strconv.Atoi(b[0])
		rows = append(rows, recording{id, b[1], b[3], b[2]})
	}
	return rows
}

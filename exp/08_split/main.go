// Experiment 8 transcribes the microphone and system channels separately.
package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/dmykolen/meeting-transcriber-go/internal/engine"
	"github.com/dmykolen/meeting-transcriber-go/internal/media"
)

type said struct {
	start, end float64
	who, text  string
	from       string // "mic" or "tap"
}

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

	out, err := os.Create(fmt.Sprintf("exp/out/C-split-%s.txt", id))
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer out.Close()
	say := func(f string, a ...any) {
		fmt.Printf(f, a...)
		fmt.Fprintf(out, f, a...)
	}

	say("EXPERIMENT C — the two channels transcribed separately\n")
	say("recording %s — %s\n", id, title)
	say("audio %.1f s\n\n", float64(len(mic))/media.Rate)

	models := filepath.Join(home, "models")
	opts := engine.Options{Language: "uk", Threads: 8, Transcriber: "whisper"}

	// ── the microphone: one person, no diarization needed ────────────────────
	say("── microphone (the owner, by construction) ──\n")
	began := time.Now()
	eng, err := engine.Open(models, opts)
	if err != nil {
		say("engine: %v\n", err)
		os.Exit(1)
	}
	mine, err := eng.Transcribe(mic)
	if err != nil {
		say("microphone: %v\n", err)
	}
	say("  rows %d   took %s\n", len(mine), time.Since(began).Round(time.Second))
	say("  no clustering ran on this side at all\n\n")

	// ── the tap: everybody else, on audio with no room in it ─────────────────
	say("── system tap (everybody on the call) ──\n")
	began = time.Now()
	theirs, err := eng.Run(tap, nil)
	if err != nil {
		say("tap: %v\n", err)
	}
	eng.Close()
	say("  rows %d   took %s\n", len(theirs.Turns), time.Since(began).Round(time.Second))
	say("  speakers: %s\n\n", tally(theirs.Turns))

	// ── the two, interleaved ─────────────────────────────────────────────────
	var all []said
	for _, t := range mine {
		all = append(all, said{t.Start, t.End, "Me", t.Text, "mic"})
	}
	for _, t := range theirs.Turns {
		who := t.Speaker
		if who == "" {
			who = "?"
		}
		all = append(all, said{t.Start, t.End, who, t.Text, "tap"})
	}
	sort.Slice(all, func(i, j int) bool { return all[i].start < all[j].start })

	say("── the meeting, both sides, in order ──\n")
	loops, worst, where := repeats(all)
	say("  rows %d   loops %d   worst run %d×  %s\n\n", len(all), loops, worst, where)
	for _, s := range all {
		say("  %s  %-4s %-12s  %s\n", mmss(s.start), s.from, cut(s.who, 12), s.text)
	}

	// ── scored against what the owner heard ──────────────────────────────────
	say("\n── against exp/truth/%s.txt ──\n", id)
	truth, err := load(fmt.Sprintf("exp/truth/%s.txt", id))
	if err != nil {
		say("  no ground truth for this recording (%v)\n", err)
		return
	}
	score(say, truth, all)
	fmt.Printf("\nwritten to exp/out/C-split-%s.txt\n", id)
}

type window struct {
	from, to float64
	who      string
}

func load(path string) ([]window, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []window
	scan := bufio.NewScanner(f)
	for scan.Scan() {
		line := strings.TrimSpace(scan.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		bits := strings.Fields(line)
		if len(bits) < 3 {
			continue
		}
		from, _ := strconv.ParseFloat(bits[0], 64)
		to, _ := strconv.ParseFloat(bits[1], 64)
		out = append(out, window{from, to, strings.Join(bits[2:], " ")})
	}
	return out, scan.Err()
}

// score asks the only question the owner asked: in the stretches he named, who
// does the app think was talking? A stretch he says is one unenrolled person
// must come back as one label, and it must not be somebody the app knows.
func score(say func(string, ...any), truth []window, all []said) {
	for _, w := range truth {
		count := map[string]float64{}
		for _, s := range all {
			overlap := min(s.end, w.to) - max(s.start, w.from)
			if overlap > 0 {
				count[s.who] += overlap
			}
		}
		if len(count) == 0 {
			say("  %s–%s  %-18s → nothing transcribed there\n", mmss(w.from), mmss(w.to), w.who)
			continue
		}
		type share struct {
			who string
			sec float64
		}
		var order []share
		total := 0.0
		for k, v := range count {
			order = append(order, share{k, v})
			total += v
		}
		sort.Slice(order, func(i, j int) bool { return order[i].sec > order[j].sec })
		var parts []string
		for _, o := range order {
			parts = append(parts, fmt.Sprintf("%s %.0f%%", o.who, 100*o.sec/total))
		}
		say("  %s–%s  said to be %-18s → app says %s\n",
			mmss(w.from), mmss(w.to), w.who, strings.Join(parts, ", "))
	}
	say("\n  What to look for: the two Сергій stretches must agree with each other\n")
	say("  and must NOT be a name the app already knows. Today they both come\n")
	say("  back as Kovalenko Ostap, which is a stranger folded into somebody\n")
	say("  enrolled — the one mistake internal/engine/diarize.go says must never\n")
	say("  happen, because nothing downstream can undo it.\n")
}

func tally(turns []engine.Turn) string {
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

func repeats(all []said) (total, worst int, where string) {
	run := 1
	for i := 1; i < len(all); i++ {
		if all[i].text == all[i-1].text && all[i].from == all[i-1].from {
			total++
			run++
			if run > worst {
				worst, where = run, fmt.Sprintf("at %s %q", mmss(all[i].start), cut(all[i].text, 40))
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

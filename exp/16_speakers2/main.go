// Experiment 16 scores speaker attribution against a timed reference.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	sherpa "github.com/k2-fsa/sherpa-onnx-go-macos"
	"github.com/dmykolen/meeting-transcriber-go/internal/media"
)

const (
	audio = "exp/truth/meet-test.m4a"
	truth = "exp/truth/meet-test-whisperx.json"
	frame = 0.1 // the grid both answers are laid on
)

// ─────────────────────────────────────────────────────────────── ground truth

type heard struct {
	from, to float64
	who      string
}

func reference(path string) []heard {
	raw, err := os.ReadFile(path)
	if err != nil {
		die(err)
	}
	var doc struct {
		Segments []struct {
			Speaker string `json:"speaker"`
			Words   []struct {
				Start   *float64 `json:"start"`
				End     *float64 `json:"end"`
				Speaker string   `json:"speaker"`
			} `json:"words"`
		} `json:"segments"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		die(err)
	}
	var out []heard
	last := ""
	for _, s := range doc.Segments {
		for _, w := range s.Words {
			if w.Start == nil || w.End == nil {
				continue
			}
			who := w.Speaker
			if who == "" {
				who = s.Speaker
			}
			if who == "" {
				who = last
			}
			last = who
			out = append(out, heard{*w.Start, *w.End, who})
		}
	}
	return out
}

// grid is the reference as one label per 100 ms of speech. Silence is left out:
// a diarizer is not being asked about the gaps.
func grid(ref []heard) map[int]string {
	out := map[int]string{}
	for _, w := range ref {
		for t := int(w.from / frame); t <= int(w.to/frame); t++ {
			out[t] = w.who
		}
	}
	return out
}

// ───────────────────────────────────────────────────────────────── the score

// agreement lays our spans on the reference's grid and reports how much of the
// speech we name the same way, once each of our speakers is read as whichever
// of theirs it best stands for.
//
// The mapping is one to one and chosen by exhaustion rather than greedily:
// three reference speakers against at most a handful of ours is a few hundred
// possibilities, and a greedy mapping quietly flatters a diarizer that found
// too few people.
func agreement(spans []sherpa.OfflineSpeakerDiarizationSegment, want map[int]string) (float64, float64, int) {
	ours := map[int]map[string]int{}
	covered := map[int]bool{}
	for _, s := range spans {
		for t := int(float64(s.Start) / frame); t <= int(float64(s.End)/frame); t++ {
			who, ok := want[t]
			if !ok {
				continue
			}
			covered[t] = true
			if ours[s.Speaker] == nil {
				ours[s.Speaker] = map[string]int{}
			}
			ours[s.Speaker][who]++
		}
	}
	if len(want) == 0 {
		return 0, 0, 0
	}
	coverage := 100 * float64(len(covered)) / float64(len(want))

	var mine []int
	for id := range ours {
		mine = append(mine, id)
	}
	sort.Ints(mine)
	var theirs []string
	seen := map[string]bool{}
	for _, who := range want {
		if !seen[who] {
			seen[who] = true
			theirs = append(theirs, who)
		}
	}
	sort.Strings(theirs)

	best := 0
	var walk func(at int, used map[int]bool, sum int)
	walk = func(at int, used map[int]bool, sum int) {
		if at == len(theirs) {
			best = max(best, sum)
			return
		}
		walk(at+1, used, sum) // this reference speaker matched by nobody
		for _, id := range mine {
			if used[id] {
				continue
			}
			used[id] = true
			walk(at+1, used, sum+ours[id][theirs[at]])
			delete(used, id)
		}
	}
	walk(0, map[int]bool{}, 0)
	return 100 * float64(best) / float64(len(want)), coverage, len(mine)
}

// ──────────────────────────────────────────────────────────────────── sweep

type setup struct {
	segmentation string
	embedding    string
	threshold    float32
	clusters     int
}

func main() {
	models := os.Getenv("HOME") + "/MeetingTranscriber/models"

	out, err := os.Create("exp/out/K-speakers.txt")
	if err != nil {
		die(err)
	}
	defer out.Close()
	say := func(f string, a ...any) {
		fmt.Printf(f, a...)
		fmt.Fprintf(out, f, a...)
	}

	samples, err := media.Voices(audio)
	if err != nil {
		die(err)
	}
	ref := reference(truth)
	want := grid(ref)

	say("EXPERIMENT K — who is talking\n\n")
	say("audio      %s   %.1f s\n", filepath.Base(audio), float64(len(samples))/media.Rate)
	say("reference  %d words of speech, %d speakers, %.1f s of it\n\n",
		len(ref), speakers(ref), float64(len(want))*frame)

	segs := map[string]string{
		"pyannote-3.0": filepath.Join(models, "segmentation", "model.onnx"),
		"reverb-v2":    filepath.Join(models, "sherpa-onnx-reverb-diarization-v2", "model.onnx"),
	}
	embs := map[string]string{
		"campplus-zh_en": filepath.Join(models, "3dspeaker_speech_campplus_sv_zh_en_16k-common_advanced.onnx"),
		"eres2netv2":     filepath.Join(models, "3dspeaker_speech_eres2netv2_sv_zh-cn_16k-common.onnx"),
		"titanet-large":  filepath.Join(models, "nemo_en_titanet_large.onnx"),
	}
	for name, path := range segs {
		if _, err := os.Stat(path); err != nil {
			say("  missing segmentation %s (%s) — skipped\n", name, path)
			delete(segs, name)
		}
	}
	for name, path := range embs {
		if _, err := os.Stat(path); err != nil {
			say("  missing embedding %s — skipped\n", name)
			delete(embs, name)
		}
	}

	say("  %-13s %-15s %-9s %-9s %-8s %-9s %-6s %s\n",
		"segmentation", "embedding", "threshold", "speakers", "agree", "coverage", "took", "")
	say("  %s\n", strings.Repeat("─", 92))

	bestScore, bestName := -1.0, ""
	for _, segName := range sorted(segs) {
		for _, embName := range sorted(embs) {
			for _, th := range []float32{0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1.0, 1.1} {
				run(say, samples, want, setup{segs[segName], embs[embName], th, -1},
					segName, embName, fmt.Sprintf("%.2f", th), &bestScore, &bestName)
			}
			// The same models told how many people are in the room. Not a
			// setting anything can ship — a recording does not announce it —
			// but it separates "cannot tell these voices apart" from "picked
			// the wrong number of them".
			run(say, samples, want, setup{segs[segName], embs[embName], 0, 3},
				segName, embName, "told 3", &bestScore, &bestName)
			say("\n")
		}
	}
	say("  best: %s at %.0f%%\n", bestName, bestScore)
	say("\n  agree     of the reference's speech, how much we name the same way\n")
	say("  coverage  of the reference's speech, how much our spans cover at all\n")
	say("  the app ships pyannote-3.0 + campplus-zh_en at 0.90\n")
	fmt.Println("\nwritten to exp/out/K-speakers.txt")
}

func run(say func(string, ...any), samples []float32, want map[int]string, s setup,
	segName, embName, shown string, bestScore *float64, bestName *string) {
	var c sherpa.OfflineSpeakerDiarizationConfig
	c.Segmentation.Pyannote.Model = s.segmentation
	c.Segmentation.NumThreads = 8
	c.Embedding.Model = s.embedding
	c.Embedding.NumThreads = 8
	c.Clustering.NumClusters = s.clusters
	c.Clustering.Threshold = s.threshold
	c.MinDurationOn = 0.3
	c.MinDurationOff = 0.5

	began := time.Now()
	sd := sherpa.NewOfflineSpeakerDiarization(&c)
	if sd == nil {
		say("  %-13s %-15s %-9s  would not load\n", segName, embName, shown)
		return
	}
	defer sherpa.DeleteOfflineSpeakerDiarization(sd)

	spans := sd.Process(samples)
	agree, coverage, found := agreement(spans, want)
	mark := ""
	if agree > *bestScore {
		*bestScore, *bestName = agree, fmt.Sprintf("%s + %s at %s", segName, embName, shown)
		mark = "  ←"
	}
	say("  %-13s %-15s %-9s %-9d %-7.0f%% %-8.0f%% %-6.1fs%s\n",
		segName, embName, shown, found, agree, coverage, time.Since(began).Seconds(), mark)
}

func sorted[T any](m map[string]T) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func speakers(ref []heard) int {
	seen := map[string]bool{}
	for _, w := range ref {
		seen[w.who] = true
	}
	return len(seen)
}

func die(err error) {
	fmt.Println("error:", err)
	os.Exit(1)
}

// Experiment 11 compares diarization setups against a speaker reference.
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

	sherpa "github.com/k2-fsa/sherpa-onnx-go-macos"

	"github.com/dmykolen/meeting-transcriber-go/internal/engine"
	"github.com/dmykolen/meeting-transcriber-go/internal/media"
)

type window struct {
	from, to float64
	who      string
}

type run struct {
	name          string
	segmentation  string
	embedding     string
	tapOnly       bool
	threshold     float32
	minOn, minOff float32
}

func main() {
	id := "74"
	if len(os.Args) > 1 {
		id = os.Args[1]
	}
	home := os.Getenv("HOME") + "/MeetingTranscriber"
	models := filepath.Join(home, "models")
	path, title := recording(home, id)

	mic, tap, ok := media.Sides(path)
	if !ok {
		fmt.Println("not one of ours")
		os.Exit(1)
	}
	mixed := media.Fold(mic, tap)

	truth, err := load(fmt.Sprintf("exp/truth/%s.txt", id))
	if err != nil {
		fmt.Println("no ground truth:", err)
		os.Exit(1)
	}

	out, err := os.Create(fmt.Sprintf("exp/out/F-speakers-%s.txt", id))
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer out.Close()
	say := func(f string, a ...any) {
		fmt.Printf(f, a...)
		fmt.Fprintf(out, f, a...)
	}

	pyannote := filepath.Join(models, "segmentation", "model.onnx")
	reverb := filepath.Join(models, "sherpa-onnx-reverb-diarization-v2", "model.onnx")
	camPP := filepath.Join(models, "embedding.onnx")
	zhEn := filepath.Join(models, "3dspeaker_speech_campplus_sv_zh_en_16k-common_advanced.onnx")
	eres := filepath.Join(models, "3dspeaker_speech_eres2netv2_sv_zh-cn_16k-common.onnx")

	runs := []run{
		{name: "as it ships", segmentation: pyannote, embedding: camPP, threshold: engine.Threshold},
		{name: "+ zh_en embedding", segmentation: pyannote, embedding: zhEn, threshold: engine.Threshold},
		{name: "+ ERes2NetV2", segmentation: pyannote, embedding: eres, threshold: engine.Threshold},
		{name: "reverb segmentation", segmentation: reverb, embedding: camPP, threshold: engine.Threshold},
		{name: "reverb + zh_en", segmentation: reverb, embedding: zhEn, threshold: engine.Threshold},
		{name: "tap only, as it ships", segmentation: pyannote, embedding: camPP, tapOnly: true, threshold: engine.Threshold},
		{name: "tap only + zh_en", segmentation: pyannote, embedding: zhEn, tapOnly: true, threshold: engine.Threshold},
		{name: "tap only, tighter (0.7)", segmentation: pyannote, embedding: zhEn, tapOnly: true, threshold: 0.7},
		{name: "tap only, shorter floor", segmentation: pyannote, embedding: zhEn, tapOnly: true, threshold: engine.Threshold, minOn: 0.15, minOff: 0.25},
	}

	say("EXPERIMENT F — segmentation, embedding and channel, against the owner's ear\n")
	say("recording %s — %s, %.1f s\n\n", id, title, float64(len(mixed))/media.Rate)
	say("He says:\n")
	for _, w := range truth {
		say("  %s–%s  %s\n", mmss(w.from), mmss(w.to), w.who)
	}
	say("\nThe two Сергій windows must agree with each other and must NOT share a\n")
	say("label with the Kovalenko window. Everything else is detail.\n\n")

	for _, r := range runs {
		audio := mixed
		if r.tapOnly {
			audio = tap
		}
		began := time.Now()
		spans, n, err := diarize(r, audio)
		if err != nil {
			say("── %-24s FAILED: %v\n\n", r.name, err)
			continue
		}
		say("── %s\n", r.name)
		say("   %d speakers found, %s (%.0fx realtime)\n", n,
			time.Since(began).Round(time.Second),
			float64(len(audio))/media.Rate/time.Since(began).Seconds())
		score(say, truth, spans)
	}
	fmt.Printf("\nwritten to exp/out/F-speakers-%s.txt\n", id)
}

type span struct {
	from, to float64
	who      int
}

func diarize(r run, samples []float32) ([]span, int, error) {
	var c sherpa.OfflineSpeakerDiarizationConfig
	c.Segmentation.Pyannote.Model = r.segmentation
	c.Segmentation.NumThreads = 8
	c.Embedding.Model = r.embedding
	c.Embedding.NumThreads = 8
	c.Clustering.NumClusters = -1
	c.Clustering.Threshold = r.threshold
	c.MinDurationOn = 0.3
	c.MinDurationOff = 0.5
	if r.minOn > 0 {
		c.MinDurationOn = r.minOn
	}
	if r.minOff > 0 {
		c.MinDurationOff = r.minOff
	}

	sd := sherpa.NewOfflineSpeakerDiarization(&c)
	if sd == nil {
		return nil, 0, fmt.Errorf("models would not load")
	}
	defer sherpa.DeleteOfflineSpeakerDiarization(sd)

	segments := sd.Process(samples)
	seen := map[int]bool{}
	out := make([]span, 0, len(segments))
	for _, s := range segments {
		out = append(out, span{float64(s.Start), float64(s.End), s.Speaker})
		seen[s.Speaker] = true
	}
	return out, len(seen), nil
}

// score answers only the questions the owner asked.
func score(say func(string, ...any), truth []window, spans []span) {
	dominant := make([]int, len(truth))
	for i, w := range truth {
		held := map[int]float64{}
		for _, s := range spans {
			if overlap := min(s.to, w.to) - max(s.from, w.from); overlap > 0 {
				held[s.who] += overlap
			}
		}
		total, top, best := 0.0, -1, 0.0
		for who, sec := range held {
			total += sec
			if sec > best {
				best, top = sec, who
			}
		}
		dominant[i] = top
		share := 0.0
		if total > 0 {
			share = 100 * best / total
		}
		say("   %s–%s  %-18s → speaker %-3s holds %.0f%%\n",
			mmss(w.from), mmss(w.to), w.who, label(top), share)
	}

	// The two questions that matter, stated as verdicts rather than numbers.
	var same, other []int
	for i, w := range truth {
		if strings.EqualFold(w.who, "Сергій") {
			same = append(same, dominant[i])
		} else {
			other = append(other, dominant[i])
		}
	}
	agree := len(same) > 1 && allEqual(same)
	apart := true
	for _, a := range same {
		for _, b := range other {
			if a == b {
				apart = false
			}
		}
	}
	say("   the stranger is one speaker      %s\n", tick(agree))
	say("   and not the enrolled one         %s\n\n", tick(apart))
}

func allEqual(a []int) bool {
	for _, v := range a[1:] {
		if v != a[0] {
			return false
		}
	}
	return true
}

func tick(ok bool) string {
	if ok {
		return "yes"
	}
	return "NO"
}

func label(n int) string {
	if n < 0 {
		return "—"
	}
	return strconv.Itoa(n)
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
		b := strings.Fields(line)
		if len(b) < 3 {
			continue
		}
		from, _ := strconv.ParseFloat(b[0], 64)
		to, _ := strconv.ParseFloat(b[1], 64)
		out = append(out, window{from, to, strings.Join(b[2:], " ")})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].from < out[j].from })
	return out, scan.Err()
}

func recording(home, id string) (path, title string) {
	out, err := exec.Command("sqlite3", filepath.Join(home, "meetings.db"),
		"SELECT audio||char(9)||title FROM recordings WHERE id="+id).Output()
	if err != nil || len(out) == 0 {
		fmt.Println("no such recording:", id, err)
		os.Exit(1)
	}
	b := strings.SplitN(strings.TrimSpace(string(out)), "\t", 2)
	return filepath.Join(home, "recordings", b[0]), b[1]
}

func mmss(s float64) string { return fmt.Sprintf("%d:%02d", int(s)/60, int(s)%60) }

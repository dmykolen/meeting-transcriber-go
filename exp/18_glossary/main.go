// Experiment 18 measures glossary prompting versus repetition cost.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	whisper "github.com/ggerganov/whisper.cpp/bindings/go/pkg/whisper"
	"github.com/dmykolen/meeting-transcriber-go/internal/media"
)

const specials = 50257

// Glossary is the owner's own vocabulary, counted out of his library rather
// than guessed: the Latin-script words his meetings actually contain.
const Glossary = "Northwind, Azure, OpenAI, DevOps, CRM, Jira, Confluence, GitLab, " +
	"Kafka, Oracle, SQLModel, Kubernetes, VPN, API, LLM, VectorDB, IDM, " +
	"Active Directory, Teams, Excel, роадмап, спринт, деплой, реліз."

type setup struct {
	name    string
	context int
	prompt  string
	how     string
}

type clip struct {
	name  string
	audio string
	truth string
}

type refWord struct {
	text string
	at   float64
}

type row struct {
	start, end float64
	text       string
}

func main() {
	models := os.Getenv("HOME") + "/MeetingTranscriber/models"

	out, err := os.Create("exp/out/M-glossary.txt")
	if err != nil {
		die(err)
	}
	defer out.Close()
	say := func(f string, a ...any) {
		fmt.Printf(f, a...)
		fmt.Fprintf(out, f, a...)
	}

	clips := []clip{
		{"meet-test", "exp/truth/meet-test.m4a", "exp/truth/meet-test-whisperx.json"},
		{"daily", os.Getenv("HOME") + "/MeetingTranscriber/recordings/daily-standup.m4a",
			"exp/truth/wx78/daily-standup.json"},
	}
	setups := []setup{
		{"ships", 0, "", "no context, no glossary — what ships"},
		{"ctx-64", 64, "", "context back on, still no glossary — what context alone costs"},
		{"ctx-224", 224, "", "more context, no glossary"},
		{"gloss-64", 64, Glossary, "the glossary, in a small window"},
		{"gloss-224", 224, Glossary, "the glossary, in a window big enough to hold it"},
	}

	say("EXPERIMENT M — a glossary, and the context it costs\n\n")
	say("glossary: %s\n\n", Glossary)

	for _, c := range clips {
		samples, err := media.Voices(c.audio)
		if err != nil {
			say("  %s: %v\n\n", c.name, err)
			continue
		}
		ref := reference(c.truth)
		if len(ref) == 0 {
			say("  %s: no reference\n\n", c.name)
			continue
		}
		length := float64(len(samples)) / media.Rate
		say("%s\n%s   %.0f s, %d reference words\n%s\n",
			strings.Repeat("=", 78), c.name, length, len(ref), strings.Repeat("=", 78))
		say("  %-11s %-6s %-10s %-8s %-8s %-8s %s\n",
			"setup", "rows", "agreement", "median", "repeats", "longest", "pace")
		say("  %s\n", strings.Repeat("─", 76))

		for _, s := range setups {
			began := time.Now()
			rows := transcribe(filepath.Join(models, "whisper.bin"), models, samples, s)
			took := time.Since(began).Seconds()

			share, median := judge(rows, ref)
			repeats, longest := loops(rows)
			say("  %-11s %-6d %-9.0f%% %-8.2f %-8d %-8d %.0fx\n",
				s.name, len(rows), share, median, repeats, longest, length/took)
			dump(c.name, s.name, rows)
		}
		say("\n")
	}
	say("  agreement  of the reference's words, how many we wrote the same\n")
	say("  repeats    rows whose text repeats the row before them\n")
	say("  longest    the longest run of one row repeating — the failure mode\n\n")
	for _, s := range setups {
		say("  %-11s %s\n", s.name, s.how)
	}
	fmt.Println("\nwritten to exp/out/M-glossary.txt")
}

// loops counts the thing that made this app set the context to zero: a row that
// says what the row before it said, over and over.
func loops(rows []row) (int, int) {
	repeats, run, longest := 0, 1, 1
	for i := 1; i < len(rows); i++ {
		if strings.EqualFold(strings.TrimSpace(rows[i].text), strings.TrimSpace(rows[i-1].text)) {
			repeats++
			run++
			longest = max(longest, run)
			continue
		}
		run = 1
	}
	return repeats, longest
}

func transcribe(model, models string, samples []float32, s setup) []row {
	m, err := whisper.New(model)
	if err != nil {
		die(err)
	}
	defer m.Close()
	ctx, err := m.NewContext()
	if err != nil {
		die(err)
	}
	ctx.SetThreads(8)
	ctx.SetLanguage("uk")
	ctx.SetMaxContext(s.context)
	if s.prompt != "" {
		ctx.SetInitialPrompt(s.prompt)
	}
	ctx.SetVAD(true)
	ctx.SetVADModelPath(filepath.Join(models, "vad.bin"))
	ctx.SetVADThreshold(0.5)
	ctx.SetVADMinSpeechMs(250)
	ctx.SetVADMinSilenceMs(300)
	ctx.SetVADSpeechPadMs(200)
	if err := ctx.Process(prepare(samples), nil, nil, nil); err != nil {
		die(err)
	}
	var out []row
	for {
		seg, err := ctx.NextSegment()
		if err == io.EOF {
			break
		}
		if err != nil {
			die(err)
		}
		if text := strings.TrimSpace(seg.Text); text != "" && believable(seg.Tokens) {
			out = append(out, row{seg.Start.Seconds(), seg.End.Seconds(), text})
		}
	}
	return out
}

func prepare(samples []float32) []float32 {
	out := make([]float32, len(samples))
	rc := 1 / (2 * math.Pi * 80.0)
	a := float32(rc / (rc + 1/16000.0))
	var in, prev float32
	for i, s := range samples {
		prev = a * (prev + s - in)
		in = s
		out[i] = prev
	}
	loud := active(out)
	if loud <= 0 {
		return out
	}
	gain := min(max(float32(math.Pow(10, -14.0/20)/loud), 1), 8)
	if gain == 1 {
		return out
	}
	for i := range out {
		out[i] = max(min(out[i]*gain, 1), -1)
	}
	return out
}

func active(samples []float32) float64 {
	const window = 1600
	var loud []float64
	for i := 0; i+window <= len(samples); i += window {
		var sum float64
		for _, s := range samples[i : i+window] {
			sum += float64(s) * float64(s)
		}
		if l := math.Sqrt(sum / window); l > 1e-4 {
			loud = append(loud, l)
		}
	}
	if len(loud) == 0 {
		return 0
	}
	slices.Sort(loud)
	return loud[len(loud)*3/4]
}

func believable(tokens []whisper.Token) bool {
	var sum float64
	n := 0
	for _, t := range tokens {
		if t.Id < specials {
			sum += float64(t.P)
			n++
		}
	}
	if n == 0 {
		return true
	}
	mean := sum / float64(n)
	if mean < 0.35 {
		return false
	}
	return n > 3 || mean > 0.35*1.6
}

func judge(rows []row, ref []refWord) (float64, float64) {
	var mine []refWord
	for _, r := range rows {
		ws := words(r.text)
		for i, w := range ws {
			share := 0.0
			if len(ws) > 1 {
				share = float64(i) / float64(len(ws))
			}
			mine = append(mine, refWord{w, r.start + share*(r.end-r.start)})
		}
	}
	pairs := align(mine, ref)
	if len(pairs) == 0 {
		return 0, 0
	}
	errs := make([]float64, 0, len(pairs))
	for _, p := range pairs {
		errs = append(errs, math.Abs(mine[p[0]].at-ref[p[1]].at))
	}
	slices.Sort(errs)
	return 100 * float64(len(pairs)) / float64(len(ref)), errs[len(errs)/2]
}

func align(mine, ref []refWord) [][2]int {
	n, m := len(mine), len(ref)
	if n == 0 || m == 0 {
		return nil
	}
	const gap, hit, miss = -1, 2, -1
	grid := make([]int32, (n+1)*(m+1))
	at := func(i, j int) int { return i*(m+1) + j }
	for i := 1; i <= n; i++ {
		grid[at(i, 0)] = int32(i * gap)
	}
	for j := 1; j <= m; j++ {
		grid[at(0, j)] = int32(j * gap)
	}
	for i := 1; i <= n; i++ {
		for j := 1; j <= m; j++ {
			s := int32(miss)
			if mine[i-1].text == ref[j-1].text {
				s = hit
			}
			grid[at(i, j)] = max(grid[at(i-1, j-1)]+s,
				max(grid[at(i-1, j)]+gap, grid[at(i, j-1)]+gap))
		}
	}
	var pairs [][2]int
	for i, j := n, m; i > 0 && j > 0; {
		s := int32(miss)
		same := mine[i-1].text == ref[j-1].text
		if same {
			s = hit
		}
		switch {
		case grid[at(i, j)] == grid[at(i-1, j-1)]+s:
			if same {
				pairs = append(pairs, [2]int{i - 1, j - 1})
			}
			i, j = i-1, j-1
		case grid[at(i, j)] == grid[at(i-1, j)]+gap:
			i--
		default:
			j--
		}
	}
	slices.Reverse(pairs)
	return pairs
}

func reference(path string) []refWord {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var doc struct {
		Segments []struct {
			Words []struct {
				Word  string   `json:"word"`
				Start *float64 `json:"start"`
			} `json:"words"`
		} `json:"segments"`
	}
	if json.Unmarshal(raw, &doc) != nil {
		return nil
	}
	var out []refWord
	clock := 0.0
	for _, s := range doc.Segments {
		for _, w := range s.Words {
			if w.Start != nil {
				clock = *w.Start
			}
			if n := normal(w.Word); n != "" {
				out = append(out, refWord{n, clock})
			}
		}
	}
	return out
}

func normal(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r >= 0x400 && r <= 0x4ff {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func words(text string) []string {
	var out []string
	for _, w := range strings.Fields(text) {
		if n := normal(w); n != "" {
			out = append(out, n)
		}
	}
	return out
}

func dump(clip, name string, rows []row) {
	f, err := os.Create("exp/out/M-" + clip + "-" + name + ".txt")
	if err != nil {
		return
	}
	defer f.Close()
	for _, r := range rows {
		fmt.Fprintf(f, "%7.2f %7.2f  %s\n", r.start, r.end, r.text)
	}
}

func die(err error) {
	fmt.Println("error:", err)
	os.Exit(1)
}

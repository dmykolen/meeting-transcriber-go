// Experiment 15 compares row-placement strategies against word-level reference.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	whisper "github.com/ggerganov/whisper.cpp/bindings/go/pkg/whisper"
	"github.com/dmykolen/meeting-transcriber-go/internal/engine"
	"github.com/dmykolen/meeting-transcriber-go/internal/media"
)

const (
	audio    = "exp/truth/meet-test.m4a"
	truth    = "exp/truth/meet-test-whisperx.json"
	specials = 50257
	breath   = 0.45
)

// ─────────────────────────────────────────────────────────────── ground truth

type refWord struct {
	text    string
	at      float64
	speaker string
}

func reference(path string) []refWord {
	raw, err := os.ReadFile(path)
	if err != nil {
		die(err)
	}
	var doc struct {
		Segments []struct {
			Speaker string `json:"speaker"`
			Words   []struct {
				Word    string   `json:"word"`
				Start   *float64 `json:"start"`
				Speaker string   `json:"speaker"`
			} `json:"words"`
		} `json:"segments"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		die(err)
	}
	var out []refWord
	last, clock := "", 0.0
	for _, s := range doc.Segments {
		for _, w := range s.Words {
			who := w.Speaker
			if who == "" {
				who = s.Speaker
			}
			if who == "" {
				who = last
			}
			last = who
			at := clock
			if w.Start != nil {
				at, clock = *w.Start, *w.Start
			}
			if n := normal(w.Word); n != "" {
				out = append(out, refWord{n, at, who})
			}
		}
	}
	return out
}

// normal strips a word down to what two transcripts of one recording can be
// expected to agree on.
func normal(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r >= 0x400 && r <= 0x4ff {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// ───────────────────────────────────────────────────────── what a variant says

// said is one word as a variant places it: the row it is in, and the moment
// that row implies it was spoken. A row carries a start and an end and nothing
// between, so a word's moment is read off the row in proportion — which is
// exactly what the reader does when it highlights along.
type said struct {
	text string
	at   float64
	who  string
}

func spread(rows []engine.Turn) []said {
	var out []said
	for _, r := range rows {
		ws := strings.Fields(r.Text)
		var keep []string
		for _, w := range ws {
			if n := normal(w); n != "" {
				keep = append(keep, n)
			}
		}
		for i, w := range keep {
			share := 0.0
			if len(keep) > 1 {
				share = float64(i) / float64(len(keep))
			}
			out = append(out, said{w, r.Start + share*(r.End-r.Start), r.Speaker})
		}
	}
	return out
}

// ────────────────────────────────────────────────────────────────── the score

type score struct {
	rows, words, agreed int
	median, p90, worst  float64
	lean                float64 // signed median: negative is early
	speaker             float64
}

// judge aligns the two transcripts word by word and asks, of every word both
// heard, how far apart the two put it.
func judge(rows []engine.Turn, ref []refWord) (score, []string) {
	mine := spread(rows)
	pairs := align(mine, ref)

	s := score{rows: len(rows), words: len(mine), agreed: len(pairs)}
	if len(pairs) == 0 {
		return s, nil
	}

	seen := map[string]map[string]int{}
	errs := make([]float64, 0, len(pairs))
	for _, p := range pairs {
		errs = append(errs, mine[p[0]].at-ref[p[1]].at)
		who, theirs := mine[p[0]].who, ref[p[1]].speaker
		if who == "" || theirs == "" {
			continue
		}
		if seen[who] == nil {
			seen[who] = map[string]int{}
		}
		seen[who][theirs]++
	}
	signed := slices.Clone(errs)
	slices.Sort(signed)
	s.lean = signed[len(signed)/2]

	for i := range errs {
		errs[i] = math.Abs(errs[i])
	}
	slices.Sort(errs)
	s.median = errs[len(errs)/2]
	s.p90 = errs[min(len(errs)*9/10, len(errs)-1)]
	s.worst = errs[len(errs)-1]
	s.speaker = agree(seen)

	var notes []string
	for i, p := range pairs {
		if i%1 == 0 {
			notes = append(notes, fmt.Sprintf("%8.2f  %8.2f  %+7.2f  %-10s %-10s %s",
				mine[p[0]].at, ref[p[1]].at, mine[p[0]].at-ref[p[1]].at,
				mine[p[0]].who, ref[p[1]].speaker, mine[p[0]].text))
		}
	}
	return s, notes
}

// align is Needleman-Wunsch over the two word sequences.
//
// Not a forward search for each row: this meeting says "окей" five times and
// "віддати все девопсом для середовища" twice, and a forward search happily
// matches the second to the first and calls the row two seconds late. A global
// alignment cannot, because it has to account for every word in both.
func align(mine []said, ref []refWord) [][2]int {
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

// agree is how often a variant's speaker labels mean the same thing as the
// reference's, once each of ours is read as whichever of theirs it most often
// coincides with.
func agree(seen map[string]map[string]int) float64 {
	type cell struct {
		ours, theirs string
		n            int
	}
	var cells []cell
	total := 0
	for ours, row := range seen {
		for theirs, n := range row {
			cells = append(cells, cell{ours, theirs, n})
			total += n
		}
	}
	if total == 0 {
		return 0
	}
	sort.Slice(cells, func(a, b int) bool { return cells[a].n > cells[b].n })
	tookOurs, tookTheirs, right := map[string]bool{}, map[string]bool{}, 0
	for _, c := range cells {
		if tookOurs[c.ours] || tookTheirs[c.theirs] {
			continue
		}
		tookOurs[c.ours], tookTheirs[c.theirs] = true, true
		right += c.n
	}
	return 100 * float64(right) / float64(total)
}

// ────────────────────────────────────────────────────────────────────── main

type kind struct {
	name string
	how  string
	vad  bool
	cut  bool // ask whisper.cpp to break the segments up itself
	tok  bool // ask for per-token times, which we may no longer need
	rows func([]whisper.Segment) []engine.Turn
}

func main() {
	home := os.Getenv("HOME") + "/MeetingTranscriber"
	models := filepath.Join(home, "models")

	out, err := os.Create("exp/out/J-rows.txt")
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
	say("EXPERIMENT J — putting a row where the words are\n\n")
	say("audio      %s   %.1f s\n", filepath.Base(audio), float64(len(samples))/media.Rate)
	say("reference  %s   %d words, %d speakers\n", filepath.Base(truth), len(ref), speakers(ref))
	say("scoring    the two transcripts are aligned word by word; for every word\n")
	say("           both heard, how far apart do they put it\n\n")

	eng, err := engine.Open(models, engine.Options{Language: "uk", Threads: 8, Transcriber: "whisper"})
	if err != nil {
		die(err)
	}
	defer eng.Close()
	spans, err := eng.Diarize(samples)
	if err != nil {
		die(err)
	}
	say("diarization  %d spans, %d speakers — one pass, shared by every variant\n\n",
		len(spans), clusters(spans))

	model, err := whisper.New(filepath.Join(models, "whisper.bin"))
	if err != nil {
		die(err)
	}
	defer model.Close()

	say("  %-9s %-6s %-8s %-8s %-8s %-8s %-9s %s\n",
		"variant", "rows", "words", "median", "p90", "worst", "speakers", "lean")
	say("  %s\n", strings.Repeat("─", 92))

	// The shipping path itself, not a copy: whatever internal/engine does now,
	// including the confidence filter, which the hand-rolled variants skip.
	shipped, err := eng.Transcribe(samples)
	if err != nil {
		die(err)
	}
	{
		named := engine.Attribute(shipped, spans)
		s, notes := judge(named, ref)
		say("  %-9s %-6d %-8d %-8.2f %-8.2f %-8.2f %-8.0f%% %+.2f s   ← internal/engine as it now is\n",
			"shipping", s.rows, s.agreed, s.median, s.p90, s.worst, s.speaker, s.lean)
		dump("shipping", named, notes)

		near := engine.Attribute(shipped, spans)
		blank := 0
		for _, r := range named {
			if r.Speaker == "" {
				blank++
			}
		}
		nearest(near, spans)
		s2, notes2 := judge(near, ref)
		say("  %-9s %-6d %-8d %-8.2f %-8.2f %-8.2f %-8.0f%% %+.2f s   ← and with %d nameless rows given the nearest speaker\n",
			"nearest", s2.rows, s2.agreed, s2.median, s2.p90, s2.worst, s2.speaker, s2.lean, blank)
		dump("nearest", near, notes2)
	}

	kinds := []kind{
		{"current", "rows built from raw token times — what ships", true, false, true, asCurrent},
		{"segments", "one row per whisper segment, nothing split", true, false, true, asSegments},
		{"remap", "split as now, token times stretched onto the segment's span", true, false, true, asRemap},
		{"split", "whisper.cpp splits the segments (max_len 90, on word)", true, true, true, asSegments},
		{"novad", "no VAD at all, so token times need no mapping", false, false, true, asCurrent},
		{"plain", "one row per segment, and no per-token times asked for at all", true, false, false, asSegments},
	}

	for _, k := range kinds {
		segs := decode(model, samples, models, k)
		named := engine.Attribute(k.rows(segs), spans)
		s, notes := judge(named, ref)
		say("  %-9s %-6d %-8d %-8.2f %-8.2f %-8.2f %-8.0f%% %+.2f s\n",
			k.name, s.rows, s.agreed, s.median, s.p90, s.worst, s.speaker, s.lean)
		dump(k.name, named, notes)
	}

	say("\n  words     words both transcripts heard, and so could be compared\n")
	say("  median    how far apart the two put a word, in seconds\n")
	say("  p90       the same for the worst tenth of words\n")
	say("  speakers  how often our label for a word means what theirs does\n")
	say("  lean      signed median: negative is early, which is the drift\n\n")
	for _, k := range kinds {
		say("  %-9s %s\n", k.name, k.how)
	}
	say("\n  rows and word-by-word detail in exp/out/J-rows-<variant>.txt\n")
	fmt.Println("\nwritten to exp/out/J-rows.txt")
}

// decode runs the model and hands back every segment, so the variants differ
// only in what they make of them.
func decode(model whisper.Model, samples []float32, models string, k kind) []whisper.Segment {
	ctx, err := model.NewContext()
	if err != nil {
		die(err)
	}
	ctx.SetThreads(8)
	ctx.SetLanguage("uk")
	ctx.SetMaxContext(0)
	ctx.SetTokenTimestamps(k.tok)
	if k.vad {
		ctx.SetVAD(true)
		ctx.SetVADModelPath(filepath.Join(models, "vad.bin"))
		ctx.SetVADThreshold(0.5)
		ctx.SetVADMinSpeechMs(250)
		ctx.SetVADMinSilenceMs(300)
		ctx.SetVADSpeechPadMs(200)
	}
	if k.cut {
		ctx.SetMaxSegmentLength(90)
		ctx.SetSplitOnWord(true)
	}
	if err := ctx.Process(clean(samples), nil, nil, nil); err != nil {
		die(err)
	}
	var out []whisper.Segment
	for {
		s, err := ctx.NextSegment()
		if err == io.EOF {
			break
		}
		if err != nil {
			die(err)
		}
		out = append(out, s)
	}
	return out
}

// clean is internal/engine's prepare, copied because it is unexported: a
// one-pole high-pass and a level, so the variants see what the app sees.
func clean(samples []float32) []float32 {
	out := make([]float32, len(samples))
	rc := 1 / (2 * math.Pi * 80.0)
	a := float32(rc / (rc + 1/16000.0))
	var in, prev float32
	for i, s := range samples {
		prev = a * (prev + s - in)
		in = s
		out[i] = prev
	}
	return out
}

func asSegments(segs []whisper.Segment) []engine.Turn {
	var out []engine.Turn
	for _, s := range segs {
		if text := strings.TrimSpace(s.Text); text != "" {
			out = append(out, engine.Turn{Start: s.Start.Seconds(), End: s.End.Seconds(), Text: text})
		}
	}
	return out
}

func asCurrent(segs []whisper.Segment) []engine.Turn { return phrases(segs, false) }
func asRemap(segs []whisper.Segment) []engine.Turn   { return phrases(segs, true) }

type spokenWord struct {
	text      string
	at, until float64
}

// phrases is internal/engine's phrases, with the one change under test: when
// remap is set, every token time is stretched from the segment's raw span onto
// the segment's mapped one before a row is built out of it.
func phrases(segs []whisper.Segment, remap bool) []engine.Turn {
	var out []engine.Turn
	for _, segment := range segs {
		whole := strings.TrimSpace(segment.Text)
		if whole == "" {
			continue
		}
		from, to := segment.Start.Seconds(), segment.End.Seconds()
		ws := spoken(segment.Tokens)
		if len(ws) < 2 {
			out = append(out, engine.Turn{Start: from, End: to, Text: whole})
			continue
		}
		if remap {
			ws = stretch(ws, from, to)
		}

		var text strings.Builder
		start, end := ws[0].at, ws[0].at
		flush := func() {
			if s := strings.TrimSpace(text.String()); s != "" {
				out = append(out, engine.Turn{Start: start, End: max(end, start), Text: s})
			}
			text.Reset()
		}
		for i, w := range ws {
			if text.Len() == 0 {
				start = w.at
			}
			text.WriteString(w.text)
			end = w.until
			gap := 0.0
			if i+1 < len(ws) {
				gap = ws[i+1].at - w.until
			}
			if i+1 == len(ws) || ends(w.text) || gap > breath {
				flush()
			}
		}
		flush()
	}
	return out
}

// stretch maps a segment's word times from whatever clock the tokens are on
// onto the one the segment itself is on. Both ends are known to be right;
// everything between them is placed in proportion.
func stretch(ws []spokenWord, from, to float64) []spokenWord {
	raw0, raw1 := ws[0].at, ws[len(ws)-1].until
	if raw1-raw0 < 0.05 || to-from < 0.05 {
		return ws
	}
	k := (to - from) / (raw1 - raw0)
	out := make([]spokenWord, len(ws))
	for i, w := range ws {
		out[i] = spokenWord{w.text, from + (w.at-raw0)*k, from + (w.until-raw0)*k}
	}
	return out
}

func spoken(tokens []whisper.Token) []spokenWord {
	var out []spokenWord
	clock := 0.0
	for _, t := range tokens {
		if t.Id >= specials || strings.TrimSpace(t.Text) == "" && t.Text != " " {
			continue
		}
		at := max(t.Start.Seconds(), clock)
		until := max(t.End.Seconds(), at)
		clock = until
		if len(out) == 0 || strings.HasPrefix(t.Text, " ") {
			out = append(out, spokenWord{t.Text, at, until})
			continue
		}
		last := &out[len(out)-1]
		last.text += t.Text
		last.until = until
	}
	return out
}

func ends(token string) bool {
	t := strings.TrimSpace(token)
	return t != "" && strings.ContainsAny(t[len(t)-1:], ".!?…")
}

// ─────────────────────────────────────────────────────────────────── plumbing

func dump(name string, rows []engine.Turn, notes []string) {
	f, err := os.Create("exp/out/J-rows-" + name + ".txt")
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintln(f, "ROWS")
	for _, r := range rows {
		fmt.Fprintf(f, "%7.2f %7.2f  %-12s %s\n", r.Start, r.End, r.Speaker, r.Text)
	}
	fmt.Fprintf(f, "\nWORD BY WORD, against the reference\n%8s  %8s  %7s  %-10s %-10s %s\n",
		"ours", "theirs", "error", "our who", "their who", "word")
	for _, n := range notes {
		fmt.Fprintln(f, n)
	}
}

// nearest gives a row that no speaker span overlaps the speaker of the span
// closest to it in time. A row with nobody's name on it is not humility, it is
// a hole in the transcript, and the person who was talking either side of it is
// a far better guess than nothing.
func nearest(rows []engine.Turn, spans []engine.Span) {
	for i := range rows {
		if rows[i].Speaker != "" || len(spans) == 0 {
			continue
		}
		best, gap := -1, math.Inf(1)
		mid := (rows[i].Start + rows[i].End) / 2
		for _, s := range spans {
			d := math.Max(0, math.Max(s.Start-mid, mid-s.End))
			if d < gap {
				best, gap = s.Speaker, d
			}
		}
		if best >= 0 {
			rows[i].Speaker = engine.Label(best)
		}
	}
}

func speakers(ref []refWord) int {
	seen := map[string]bool{}
	for _, w := range ref {
		seen[w.speaker] = true
	}
	return len(seen)
}

func clusters(spans []engine.Span) int {
	seen := map[int]bool{}
	for _, s := range spans {
		seen[s.Speaker] = true
	}
	return len(seen)
}

func die(err error) {
	fmt.Println("error:", err)
	os.Exit(1)
}

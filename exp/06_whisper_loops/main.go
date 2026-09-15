// Experiment 6 probes settings that stop Whisper repetition loops.
//
// This runs the same recording through several settings and counts the loops,
// so the choice is made on the meeting rather than on the documentation.
package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	whisper "github.com/ggerganov/whisper.cpp/bindings/go/pkg/whisper"

	"github.com/dmykolen/meeting-transcriber-go/internal/media"
)

type setting struct {
	name string
	// maxContext 0 stops the decoder seeing its own previous output.
	// -1 leaves whisper.cpp's default alone.
	maxContext  int
	entropy     float32 // 0 = leave alone. Default 2.4; raise to reject loops.
	logprob     float32 // 0 = leave alone. Default -1.0; raise to reject junk.
	fallback    float32 // 0 = leave alone. Temperature step for a second try.
	beam        int     // 0 = leave alone
	description string
}

var tried = []setting{
	{name: "as it ships", maxContext: -1,
		description: "whatever the app does today"},
	{name: "no self-context", maxContext: 0,
		description: "the decoder never sees its own previous output"},
	{name: "temperature fallback", maxContext: -1, fallback: 0.2, entropy: 2.4, logprob: -1.0,
		description: "re-decode a window that looks like a loop"},
	{name: "both", maxContext: 0, fallback: 0.2, entropy: 2.4, logprob: -1.0,
		description: "no self-context and a fallback behind it"},
	{name: "both, beam 5", maxContext: 0, fallback: 0.2, entropy: 2.4, logprob: -1.0, beam: 5,
		description: "and search rather than take the first path"},
}

func main() {
	id := "74"
	if len(os.Args) > 1 {
		id = os.Args[1]
	}
	home := os.Getenv("HOME") + "/MeetingTranscriber"
	path, title := recording(home, id)
	fmt.Printf("recording %s — %s\n%s\n\n", id, title, path)

	// The mono the models are actually given today.
	mic, tap, ok := media.Sides(path)
	if !ok {
		fmt.Println("not one of ours")
		os.Exit(1)
	}
	samples := media.Fold(mic, tap)
	fmt.Printf("audio  %.1f s\n\n", float64(len(samples))/media.Rate)

	model, err := whisper.New(filepath.Join(home, "models", "whisper.bin"))
	if err != nil {
		fmt.Println("whisper:", err)
		os.Exit(1)
	}
	defer model.Close()

	out, err := os.Create(fmt.Sprintf("exp/out/A-whisper-%s.txt", id))
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer out.Close()
	say := func(f string, a ...any) {
		fmt.Printf(f, a...)
		fmt.Fprintf(out, f, a...)
	}

	say("EXPERIMENT A — Whisper settings against its own repetition\n")
	say("recording %s — %s\n\n", id, title)

	for _, s := range tried {
		say("════════════════════════════════════════════════════════════════\n")
		say("%s — %s\n", s.name, s.description)
		say("  max_text_ctx %s   entropy %s   logprob %s   fallback %s   beam %s\n\n",
			shown(float32(s.maxContext), s.maxContext >= 0),
			shown(s.entropy, s.entropy != 0), shown(s.logprob, s.logprob != 0),
			shown(s.fallback, s.fallback != 0), shown(float32(s.beam), s.beam != 0))

		began := time.Now()
		turns, err := run(model, samples, s, filepath.Join(home, "models", "vad.bin"))
		if err != nil {
			say("  FAILED: %v\n\n", err)
			continue
		}
		took := time.Since(began)

		loops, worst, where := repeats(turns)
		say("  rows %d   loops %d   worst run %d×  %s\n", len(turns), loops, worst, where)
		say("  took %s (%.1fx realtime)\n\n", took.Round(time.Second),
			float64(len(samples))/media.Rate/took.Seconds())
		for _, t := range turns {
			say("  %s  %s\n", mmss(t.Start), t.Text)
		}
		say("\n")
	}
	fmt.Printf("\nwritten to exp/out/A-whisper-%s.txt\n", id)
}

func run(model whisper.Model, samples []float32, s setting, vad string) ([]Turn, error) {
	ctx, err := model.NewContext()
	if err != nil {
		return nil, err
	}
	ctx.SetThreads(8)
	ctx.SetLanguage("uk")
	// The same VAD the app uses. Without it Whisper writes fluent text over
	// silence, which is a different invention from the one being chased here.
	ctx.SetVAD(true)
	ctx.SetVADModelPath(vad)
	ctx.SetVADThreshold(0.5)
	ctx.SetVADMinSpeechMs(250)
	ctx.SetVADMinSilenceMs(300)
	ctx.SetVADSpeechPadMs(200)

	if s.maxContext >= 0 {
		ctx.SetMaxContext(s.maxContext)
	}
	if s.entropy != 0 {
		ctx.SetEntropyThold(s.entropy)
	}
	if s.logprob != 0 {
		ctx.SetTokenSumThreshold(s.logprob)
	}
	if s.fallback != 0 {
		ctx.SetTemperatureFallback(s.fallback)
	}
	if s.beam != 0 {
		ctx.SetBeamSize(s.beam)
	}

	if err := ctx.Process(samples, nil, nil, nil); err != nil {
		return nil, err
	}
	var turns []Turn
	for {
		seg, err := ctx.NextSegment()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if text := strings.TrimSpace(seg.Text); text != "" {
			turns = append(turns, Turn{seg.Start.Seconds(), seg.End.Seconds(), text})
		}
	}
	return turns, nil
}

type Turn struct {
	Start, End float64
	Text       string
}

// repeats counts rows identical to the one before them, and finds the longest
// run — which is the number that says whether the loop is gone or merely rarer.
func repeats(turns []Turn) (total, worst int, where string) {
	run := 1
	for i := 1; i < len(turns); i++ {
		if turns[i].Text == turns[i-1].Text {
			total++
			run++
			if run > worst {
				worst, where = run, fmt.Sprintf("at %s %q", mmss(turns[i].Start), cut(turns[i].Text))
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
	// Read it straight out of the library so the experiment and the app are
	// always looking at the same file.
	out, err := query(home, "SELECT audio||char(9)||title FROM recordings WHERE id="+id)
	if err != nil || out == "" {
		fmt.Println("no such recording:", id, err)
		os.Exit(1)
	}
	bits := strings.SplitN(strings.TrimSpace(out), "\t", 2)
	return filepath.Join(home, "recordings", bits[0]), bits[1]
}

func shown(v float32, on bool) string {
	if !on {
		return "—"
	}
	return fmt.Sprintf("%g", v)
}

func cut(s string) string {
	if len(s) > 40 {
		return s[:40] + "…"
	}
	return s
}

func mmss(s float64) string { return fmt.Sprintf("%3d:%02d", int(s)/60, int(s)%60) }

// query runs one SQL statement against the library. sqlite3 rather than a
// driver because an experiment should not drag a dependency in for one string.
func query(home, sql string) (string, error) {
	out, err := exec.Command("sqlite3", filepath.Join(home, "meetings.db"), sql).Output()
	return string(out), err
}

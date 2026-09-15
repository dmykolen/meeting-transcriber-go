// Experiment 14 measures why transcript rows drift from the audio clock.
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	whisper "github.com/ggerganov/whisper.cpp/bindings/go/pkg/whisper"
	"github.com/dmykolen/meeting-transcriber-go/internal/media"
)

const specials = 50257

func main() {
	path := "exp/truth/meet-test.m4a"
	if len(os.Args) > 1 {
		path = os.Args[1]
	}
	home := os.Getenv("HOME") + "/MeetingTranscriber"
	models := filepath.Join(home, "models")

	out, err := os.Create("exp/out/I-timeline.txt")
	if err != nil {
		die(err)
	}
	defer out.Close()
	say := func(f string, a ...any) {
		fmt.Printf(f, a...)
		fmt.Fprintf(out, f, a...)
	}

	samples, err := media.Voices(path)
	if err != nil {
		die(err)
	}
	length := float64(len(samples)) / media.Rate
	say("EXPERIMENT I — the transcript's clock against the audio's clock\n\n")
	say("file      %s\n", filepath.Base(path))
	say("audio     %.2f s  (%d samples at %d Hz)\n\n", length, len(samples), media.Rate)

	for _, vad := range []bool{true, false} {
		run(say, models, samples, length, vad)
	}
	fmt.Println("\nwritten to exp/out/I-timeline.txt")
}

func run(say func(string, ...any), models string, samples []float32, length float64, vad bool) {
	label := "VAD OFF"
	if vad {
		label = "VAD ON  (what the app ships)"
	}
	say("%s\n%s\n%s\n", strings.Repeat("=", 78), label, strings.Repeat("=", 78))

	model, err := whisper.New(filepath.Join(models, "whisper.bin"))
	if err != nil {
		die(err)
	}
	defer model.Close()

	ctx, err := model.NewContext()
	if err != nil {
		die(err)
	}
	ctx.SetThreads(8)
	ctx.SetLanguage("uk")
	ctx.SetMaxContext(0)
	ctx.SetTokenTimestamps(true)
	if vad {
		ctx.SetVAD(true)
		ctx.SetVADModelPath(filepath.Join(models, "vad.bin"))
		ctx.SetVADThreshold(0.5)
		ctx.SetVADMinSpeechMs(250)
		ctx.SetVADMinSilenceMs(300)
		ctx.SetVADSpeechPadMs(200)
	}
	if err := ctx.Process(samples, nil, nil, nil); err != nil {
		die(err)
	}

	say("\n  %-8s %-8s   %-8s %-8s   %-7s  %s\n",
		"seg t0", "seg t1", "tok t0", "tok t1", "gap", "text")
	say("  %s\n", strings.Repeat("─", 100))

	var lastSegEnd, lastTokEnd float64
	var back int
	prev := -1.0
	for {
		segment, err := ctx.NextSegment()
		if err == io.EOF {
			break
		}
		if err != nil {
			die(err)
		}
		segStart, segEnd := segment.Start.Seconds(), segment.End.Seconds()
		tokStart, tokEnd, ok := span(segment.Tokens)
		if !ok {
			continue
		}
		if segStart < prev {
			back++
		}
		prev = segStart

		say("  %-8.2f %-8.2f   %-8.2f %-8.2f   %+7.2f  %s\n",
			segStart, segEnd, tokStart, tokEnd, segStart-tokStart, short(segment.Text))
		lastSegEnd, lastTokEnd = segEnd, tokEnd
	}

	say("\n  last segment ends at   %.2f s   (audio is %.2f s)\n", lastSegEnd, length)
	say("  last token   ends at   %.2f s\n", lastTokEnd)
	say("  the two clocks differ by %.2f s\n", lastSegEnd-lastTokEnd)
	say("  segments out of order: %d\n\n", back)
}

// span is the first and last time the real words of a segment carry, with the
// control tokens left out — they are what phrases() would build a row from.
func span(tokens []whisper.Token) (float64, float64, bool) {
	from, to, ok := 0.0, 0.0, false
	for _, t := range tokens {
		if t.Id >= specials {
			continue
		}
		if !ok {
			from, ok = t.Start.Seconds(), true
		}
		to = max(to, t.End.Seconds())
	}
	return from, to, ok
}

func short(s string) string {
	s = strings.TrimSpace(s)
	if len([]rune(s)) > 52 {
		return string([]rune(s)[:52]) + "…"
	}
	return s
}

func die(err error) {
	fmt.Println("error:", err)
	os.Exit(1)
}

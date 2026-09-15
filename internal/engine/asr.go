package engine

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	whisper "github.com/ggerganov/whisper.cpp/bindings/go/pkg/whisper"
)

// asr wraps whisper.cpp.
type asr struct {
	model whisper.Model
	vad   string
	opts  Options
}

func openASR(dir string, opts Options) (*asr, error) {
	path := filepath.Join(dir, "whisper.bin")
	model, err := whisper.New(path)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &asr{model: model, vad: filepath.Join(dir, "vad.bin"), opts: opts}, nil
}

func (a *asr) close() error {
	if a.model == nil {
		return nil
	}
	return a.model.Close()
}

// Rate is the ASR sample rate.
const Rate = 16000

// transcribe runs Whisper with VAD and keeps one output row per mapped segment.
func (a *asr) transcribe(samples []float32) ([]Turn, error) {
	ctx, err := a.model.NewContext()
	if err != nil {
		return nil, err
	}
	ctx.SetThreads(uint(a.opts.Threads))
	ctx.SetLanguage(language(a.opts.Language))

	// Zero disables previous-text carry-over and avoids measured repetition loops.
	ctx.SetMaxContext(0)

	ctx.SetVAD(true)
	ctx.SetVADModelPath(a.vad)
	ctx.SetVADThreshold(0.5)
	ctx.SetVADMinSpeechMs(250)
	ctx.SetVADMinSilenceMs(300)
	// Pad each detected speech span so the VAD does not clip words.
	ctx.SetVADSpeechPadMs(200)

	if err := ctx.Process(prepare(samples, Rate), nil, nil, nil); err != nil {
		return nil, err
	}

	var turns []Turn
	for {
		segment, err := ctx.NextSegment()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		text := strings.TrimSpace(segment.Text)
		if text == "" || !believable(odds(segment.Tokens)) {
			continue
		}
		turns = append(turns, Turn{
			Start: segment.Start.Seconds(),
			End:   segment.End.Seconds(),
			Text:  text,
		})
	}
	return turns, nil
}

// odds extracts token confidences for believable().
func odds(tokens []whisper.Token) []float32 {
	out := make([]float32, 0, len(tokens))
	for _, t := range tokens {
		if t.Id < specials {
			out = append(out, t.P)
		}
	}
	return out
}

// specials is the first non-lexical whisper token id.
const specials = 50257

// language maps the empty string to Whisper's auto-detect value.
func language(l string) string {
	if l == "" {
		return "auto"
	}
	return l
}

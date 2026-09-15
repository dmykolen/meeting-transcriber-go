package engine

import (
	"errors"
	"path/filepath"
	"strings"

	sherpa "github.com/k2-fsa/sherpa-onnx-go-macos"
)

// parakeet is the optional sherpa-onnx Parakeet recognizer.
type parakeet struct {
	rec *sherpa.OfflineRecognizer
}

// Parakeet is the engine name used in settings and on disk.
const Parakeet = "parakeet"

func openParakeet(dir string, opts Options) (*parakeet, error) {
	models := filepath.Join(dir, Parakeet)

	var c sherpa.OfflineRecognizerConfig
	c.ModelConfig.Transducer.Encoder = filepath.Join(models, "encoder.int8.onnx")
	c.ModelConfig.Transducer.Decoder = filepath.Join(models, "decoder.int8.onnx")
	c.ModelConfig.Transducer.Joiner = filepath.Join(models, "joiner.int8.onnx")
	c.ModelConfig.Tokens = filepath.Join(models, "tokens.txt")
	c.ModelConfig.ModelType = "nemo_transducer"
	c.ModelConfig.NumThreads = opts.Threads
	c.ModelConfig.Provider = "cpu"
	c.DecodingMethod = "greedy_search"

	rec := sherpa.NewOfflineRecognizer(&c)
	if rec == nil {
		return nil, errors.New("the Parakeet model would not load; it may still be downloading")
	}
	return &parakeet{rec: rec}, nil
}

func (p *parakeet) close() error {
	if p.rec != nil {
		sherpa.DeleteOfflineRecognizer(p.rec)
		p.rec = nil
	}
	return nil
}

// Chunk bounds Parakeet memory use and gives timestamps coarse anchors.
const Chunk = 120.0

// transcribe returns one timestamped row per chunk.
func (p *parakeet) transcribe(samples []float32) ([]Turn, error) {
	if p.rec == nil {
		return nil, errors.New("no transcription model")
	}
	var turns []Turn
	step := int(Chunk * 16000)

	for at := 0; at < len(samples); at += step {
		to := min(at+step, len(samples))
		stream := sherpa.NewOfflineStream(p.rec)
		stream.AcceptWaveform(16000, samples[at:to])
		p.rec.Decode(stream)
		text := strings.TrimSpace(stream.GetResult().Text)
		sherpa.DeleteOfflineStream(stream)

		if text != "" {
			turns = append(turns, Turn{
				Start: float64(at) / 16000,
				End:   float64(to) / 16000,
				Text:  text,
			})
		}
	}
	return turns, nil
}

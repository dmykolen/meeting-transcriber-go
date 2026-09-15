package engine

import (
	"errors"
	"path/filepath"
	"sort"

	sherpa "github.com/k2-fsa/sherpa-onnx-go-macos"
)

// Enough is the minimum speech duration worth storing as a voiceprint.
const Enough = 4.0

// voices extracts comparable voice vectors.
type voices struct {
	ex *sherpa.SpeakerEmbeddingExtractor
}

func openVoices(dir string, opts Options) (*voices, error) {
	ex := sherpa.NewSpeakerEmbeddingExtractor(&sherpa.SpeakerEmbeddingExtractorConfig{
		Model:      filepath.Join(dir, "embedding.onnx"),
		NumThreads: opts.Threads,
		Provider:   "cpu",
	})
	if ex == nil {
		return nil, errors.New("the voice model would not load")
	}
	return &voices{ex: ex}, nil
}

// print returns one vector for a single speaker clip.
func (v *voices) print(samples []float32) []float32 {
	if v.ex == nil || float64(len(samples))/16000 < Enough {
		return nil
	}
	stream := v.ex.CreateStream()
	defer sherpa.DeleteOnlineStream(stream)

	stream.AcceptWaveform(16000, samples)
	stream.InputFinished()
	if !v.ex.IsReady(stream) {
		return nil
	}
	return v.ex.Compute(stream)
}

func (v *voices) close() error {
	if v.ex != nil {
		sherpa.DeleteSpeakerEmbeddingExtractor(v.ex)
		v.ex = nil
	}
	return nil
}

// voiceprints extracts one vector per speaker from their longest stretches.
func (v *voices) voiceprints(samples []float32, spans []Span) map[string][]float32 {
	bySpeaker := map[int][]Span{}
	for _, s := range spans {
		bySpeaker[s.Speaker] = append(bySpeaker[s.Speaker], s)
	}

	prints := map[string][]float32{}
	for speaker, mine := range bySpeaker {
		sort.Slice(mine, func(a, b int) bool {
			return mine[a].End-mine[a].Start > mine[b].End-mine[b].Start
		})
		var clip []float32
		for _, s := range mine {
			from, to := int(s.Start*16000), int(s.End*16000)
			if from < 0 || to > len(samples) || to <= from {
				continue
			}
			clip = append(clip, samples[from:to]...)
			if float64(len(clip))/16000 >= 3*Enough {
				break
			}
		}
		if print := v.print(clip); print != nil {
			prints[Label(speaker)] = print
		}
	}
	return prints
}

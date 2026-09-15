package engine

import (
	"errors"
	"path/filepath"

	sherpa "github.com/k2-fsa/sherpa-onnx-go-macos"
)

// Threshold is deliberately loose: later voiceprint matching can merge splits,
// but an early merge destroys the distinction.
const Threshold = 0.90

// speakers wraps sherpa-onnx diarization.
type speakers struct {
	sd *sherpa.OfflineSpeakerDiarization
}

func openSpeakers(dir string, opts Options) (*speakers, error) {
	var c sherpa.OfflineSpeakerDiarizationConfig
	c.Segmentation.Pyannote.Model = filepath.Join(dir, "segmentation", "model.onnx")
	c.Segmentation.NumThreads = opts.Threads
	c.Embedding.Model = filepath.Join(dir, "embedding.onnx")
	c.Embedding.NumThreads = opts.Threads
	c.Clustering.NumClusters = -1 // meetings do not announce participant counts
	c.Clustering.Threshold = Threshold
	c.MinDurationOn = 0.3
	c.MinDurationOff = 0.5

	sd := sherpa.NewOfflineSpeakerDiarization(&c)
	if sd == nil {
		return nil, errors.New("the speaker models would not load")
	}
	return &speakers{sd: sd}, nil
}

func (s *speakers) diarize(samples []float32) ([]Span, error) {
	if s.sd == nil {
		return nil, errors.New("no speaker model")
	}
	segments := s.sd.Process(samples)
	spans := make([]Span, len(segments))
	for i, seg := range segments {
		spans[i] = Span{Start: float64(seg.Start), End: float64(seg.End), Speaker: seg.Speaker}
	}
	return spans, nil
}

func (s *speakers) close() error {
	if s.sd != nil {
		sherpa.DeleteOfflineSpeakerDiarization(s.sd)
		s.sd = nil
	}
	return nil
}

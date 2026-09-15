// Package models fetches model files and helper binaries on first run.
package models

import (
	"archive/tar"
	"compress/bzip2"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"
)

// A Model is one downloadable asset.
type Model struct {
	Name  string // what a person sees while it downloads
	Key   string // the folder or file it becomes, under models/
	URL   string
	Bytes int64  // for the progress bar, and to notice a truncated download
	Check string // sha256 of the download, when it is worth pinning
	Run   bool   // an executable rather than weights: unpacked into bin/, +x
}

// Everything the app needs to work offline. Sizes are compressed download
// sizes.
var (
	// Whisper ships quantized to keep download size down with little measured
	// loss against the full model.
	Whisper = Model{
		Name:  "Transcription",
		Key:   "whisper.bin",
		URL:   "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-large-v3-turbo-q5_0.bin",
		Bytes: 574_000_000,
	}

	// Whisper's own VAD.
	Vad = Model{
		Name:  "Speech detection",
		Key:   "vad.bin",
		URL:   "https://huggingface.co/ggml-org/whisper-vad/resolve/main/ggml-silero-v5.1.2.bin",
		Bytes: 885_000,
	}

	// Silero VAD for the always-on listener.
	Speech = Model{
		Name:  "Listening",
		Key:   "silero_vad.onnx",
		URL:   "https://github.com/k2-fsa/sherpa-onnx/releases/download/asr-models/silero_vad.onnx",
		Bytes: 643_854,
	}

	// ffmpeg covers containers the platform decoders do not handle.
	FFmpeg = Model{
		Name:  "Format support",
		Key:   "ffmpeg",
		URL:   "https://github.com/eugeneware/ffmpeg-static/releases/download/b6.1.1/ffmpeg-darwin-arm64.gz",
		Bytes: 19_246_198,
		Check: "8923876afa8db5585022d7860ec7e589af192f441c56793971276d450ed3bbfa",
		Run:   true,
	}

	// Parakeet is optional and fetched only when selected.
	ParakeetModel = Model{
		Name:  "Parakeet",
		Key:   "parakeet",
		URL:   "https://github.com/k2-fsa/sherpa-onnx/releases/download/asr-models/sherpa-onnx-nemo-parakeet-tdt-0.6b-v3-int8.tar.bz2",
		Bytes: 636_000_000,
	}

	Segmentation = Model{
		Name:  "Speaker segmentation",
		Key:   "segmentation",
		URL:   "https://github.com/k2-fsa/sherpa-onnx/releases/download/speaker-segmentation-models/sherpa-onnx-pyannote-segmentation-3-0.tar.bz2",
		Bytes: 7_000_000,
	}

	// Embedding is the measured-best speaker model for the current pipeline.
	Embedding = Model{
		Name:  "Speaker voices",
		Key:   "embedding.onnx",
		URL:   "https://github.com/k2-fsa/sherpa-onnx/releases/download/speaker-recongition-models/3dspeaker_speech_campplus_sv_zh_en_16k-common_advanced.onnx",
		Bytes: 28_300_000,
	}
)

// Optional returns extra assets required by a chosen transcriber.
func Optional(transcriber string) Set {
	if transcriber == "parakeet" {
		return Set{ParakeetModel}
	}
	return nil
}

// Required returns the assets a working install needs, fetched small-first.
func Required() Set { return Set{Vad, Speech, FFmpeg, Segmentation, Embedding, Whisper} }

// Progress is reported to the first-run screen.
type Progress struct {
	Model      string
	Done, Size int64
	Finished   bool
	Err        error
}

// Fraction is 0..1, or 0 when the size is not known.
func (p Progress) Fraction() float64 {
	if p.Size <= 0 {
		return 0
	}
	return min(float64(p.Done)/float64(p.Size), 1)
}

// Set is a group of downloadable assets.
type Set []Model

// Missing returns the assets not already present on disk.
func (s Set) Missing(dir string) Set {
	var todo Set
	for _, m := range s {
		if !Have(dir, m) {
			todo = append(todo, m)
		}
	}
	return todo
}

// Size is the total remaining download.
func (s Set) Size() int64 {
	var total int64
	for _, m := range s {
		total += m.Bytes
	}
	return total
}

// Have reports whether this exact model is already unpacked.
func Have(dir string, m Model) bool {
	from, err := os.ReadFile(filepath.Join(dir, ".have-"+m.Key))
	return err == nil && string(from) == m.URL
}

// Path is where a model ended up on disk.
func Path(dir string, m Model) string {
	if m.Run {
		return filepath.Join(Tools(dir), m.Key)
	}
	return filepath.Join(dir, m.Key)
}

// Tools is the folder downloaded executables live in.
func Tools(dir string) string { return filepath.Join(filepath.Dir(dir), "bin") }

// Fetch downloads and unpacks everything missing, reporting progress on the
// channel until close.
func Fetch(ctx context.Context, dir string, set Set, report chan<- Progress) {
	defer close(report)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		report <- Progress{Err: err}
		return
	}
	for _, m := range set {
		if err := fetch(ctx, dir, m, report); err != nil {
			report <- Progress{Model: m.Name, Err: err}
			return
		}
		report <- Progress{Model: m.Name, Done: m.Bytes, Size: m.Bytes, Finished: true}
	}
}

func fetch(ctx context.Context, dir string, m Model, report chan<- Progress) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.URL, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("%s: %w", m.Name, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: the download returned %s", m.Name, resp.Status)
	}

	size := resp.ContentLength
	if size <= 0 {
		size = m.Bytes
	}
	counted := &counter{reader: resp.Body}

	// Hash as bytes arrive; one downloaded asset is executed.
	var digest io.Reader = counted
	sum := sha256.New()
	if m.Check != "" {
		digest = io.TeeReader(counted, sum)
	}

	// Watch the download stream here so unpacking still moves the progress bar.
	done := make(chan struct{})
	go func() {
		tick := time.NewTicker(200 * time.Millisecond)
		defer tick.Stop()
		for {
			select {
			case <-done:
				return
			case <-tick.C:
				report <- Progress{Model: m.Name, Done: counted.read.Load(), Size: size}
			}
		}
	}()
	defer close(done)

	if err := unpack(digest, dir, m); err != nil {
		return fmt.Errorf("%s: %w", m.Name, err)
	}
	if got := hex.EncodeToString(sum.Sum(nil)); m.Check != "" && got != m.Check {
		_ = os.RemoveAll(Path(dir, m))
		return fmt.Errorf("%s: the download does not match its published checksum "+
			"(expected %s, got %s) and has been discarded", m.Name, m.Check[:12], got[:12])
	}
	// Write the marker last; Have() uses it as the finished signal.
	return os.WriteFile(filepath.Join(dir, ".have-"+m.Key), []byte(m.URL), 0o644)
}

// unpack writes a single file straight through or expands an archive into the
// model's own folder.
func unpack(r io.Reader, dir string, m Model) error {
	switch {
	case m.Run:
		gz, err := gzip.NewReader(r)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(Tools(dir), 0o755); err != nil {
			return err
		}
		if err := writeFile(Path(dir, m), gz); err != nil {
			return err
		}
		return os.Chmod(Path(dir, m), 0o755)
	case !strings.HasSuffix(m.URL, ".tar.bz2"):
		return writeFile(filepath.Join(dir, m.Key), r)
	}

	into := filepath.Join(dir, m.Key)
	if err := os.RemoveAll(into); err != nil {
		return err
	}
	archive := tar.NewReader(bzip2.NewReader(r))
	for {
		header, err := archive.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		if header.Typeflag != tar.TypeReg {
			continue
		}
		// Drop the archive's top-level directory and refuse path traversal.
		name := strip(header.Name)
		if name == "" || strings.Contains(name, "..") {
			continue
		}
		if err := writeFile(filepath.Join(into, name), archive); err != nil {
			return err
		}
	}
}

func strip(name string) string {
	if _, rest, found := strings.Cut(filepath.Clean(name), string(filepath.Separator)); found {
		return rest
	}
	return ""
}

func writeFile(path string, r io.Reader) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(file, r)
	return err
}

// counter counts bytes for the progress bar.
type counter struct {
	reader io.Reader
	read   atomic.Int64
}

func (c *counter) Read(p []byte) (int, error) {
	n, err := c.reader.Read(p)
	c.read.Add(int64(n))
	return n, err
}

// Sum is the sha256 of a file.
func Sum(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	h := sha256.New()
	if _, err := io.Copy(h, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

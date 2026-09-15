package audio

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

// handshake bounds how long the tap may stay silent before the app falls back
// to mic-only capture.
const handshake = 10 * time.Second

// macOS system audio comes from the external audiotee CLI. The subprocess
// boundary keeps Objective-C tap failures out of this process.
type systemAudio struct {
	cmd    *exec.Cmd
	out    chan []int16
	closed sync.Once
}

func (s *systemAudio) Samples() <-chan []int16 { return s.out }

func (s *systemAudio) Close() error {
	s.closed.Do(func() {
		if s.cmd.Process != nil {
			_ = s.cmd.Process.Kill()
		}
		_ = s.cmd.Wait()
	})
	return nil
}

// wireFormat is the metadata line audiotee prints before audio starts.
type wireFormat struct {
	Encoding   string `json:"encoding"`
	SampleRate int    `json:"sample_rate"`
	Channels   int    `json:"channels_per_frame"`
	Bits       int    `json:"bits_per_channel"`
	IsFloat    bool   `json:"is_float"`
}

func (w wireFormat) check() error {
	switch {
	case w.Encoding != "pcm_s16le" || w.IsFloat:
		return fmt.Errorf("audiotee returned %q, want pcm_s16le", w.Encoding)
	case w.SampleRate != SampleRate:
		return fmt.Errorf("audiotee returned %d Hz, want %d", w.SampleRate, SampleRate)
	case w.Channels != 1:
		return fmt.Errorf("audiotee returned %d channels, want 1", w.Channels)
	case w.Bits != 16:
		return fmt.Errorf("audiotee returned %d bits, want 16", w.Bits)
	}
	return nil
}

func openSystemAudio() (Device, error) {
	bin, err := findAudiotee()
	if err != nil {
		return nil, err
	}

	cmd := exec.Command(bin, "--sample-rate", fmt.Sprint(SampleRate))
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start audiotee: %w", err)
	}

	sys := &systemAudio{cmd: cmd, out: make(chan []int16, 128)}
	format := make(chan error, 1)
	go watchAudiotee(stderr, format)

	// Wait for the declared wire format before trusting any samples.
	select {
	case err := <-format:
		if err != nil {
			_ = sys.Close()
			return nil, err
		}
	case <-time.After(handshake):
		_ = sys.Close()
		return nil, fmt.Errorf("audiotee said nothing for %v; carrying on without system audio", handshake)
	}
	go sys.read(stdout)
	return sys, nil
}

// watchAudiotee parses the JSON-lines log and reports the declared format once.
func watchAudiotee(stderr io.Reader, format chan<- error) {
	type line struct {
		Type string          `json:"message_type"`
		Data json.RawMessage `json:"data"`
	}
	scan := bufio.NewScanner(stderr)
	scan.Buffer(make([]byte, 0, 8192), 1<<20)
	announced := false

	for scan.Scan() {
		var msg line
		if json.Unmarshal(scan.Bytes(), &msg) != nil {
			continue
		}
		switch msg.Type {
		case "metadata":
			var w wireFormat
			if err := json.Unmarshal(msg.Data, &w); err != nil {
				format <- fmt.Errorf("audiotee metadata: %w", err)
				return
			}
			if err := w.check(); err != nil {
				format <- err
				return
			}
			slog.Info("system audio tap open", "rate", w.SampleRate, "encoding", w.Encoding)
			format <- nil
			announced = true
		case "error":
			slog.Error("audiotee", "detail", string(msg.Data))
		}
	}
	if !announced {
		format <- errors.New("audiotee exited before declaring a format")
	}
}

func (s *systemAudio) read(stdout io.Reader) {
	defer close(s.out)
	buf := make([]byte, FrameSize*2)

	for {
		n, err := io.ReadFull(stdout, buf)
		if n >= 2 {
			block := make([]int16, n/2)
			for i := range block {
				block[i] = int16(binary.LittleEndian.Uint16(buf[2*i:]))
			}
			select {
			case s.out <- block:
			default:
				slog.Warn("system audio buffer full; dropped a block")
			}
		}
		if err != nil {
			if !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
				slog.Error("system audio read failed", "err", err)
			}
			return
		}
	}
}

// findAudiotee looks beside the executable first, which is where the .app
// bundle puts it, so a bundled daemon never depends on the user's PATH.
func findAudiotee() (string, error) {
	if bin := os.Getenv("MT_AUDIOTEE"); bin != "" {
		return bin, nil
	}
	self, err := os.Executable()
	if err == nil {
		for _, candidate := range []string{
			filepath.Join(filepath.Dir(self), "audiotee"),
			filepath.Join(filepath.Dir(self), "..", "Resources", "audiotee"),
		} {
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				return filepath.Abs(candidate)
			}
		}
	}
	if bin, err := exec.LookPath("audiotee"); err == nil {
		return bin, nil
	}
	return "", errors.New("audiotee not found; set MT_AUDIOTEE or place it beside the binary")
}

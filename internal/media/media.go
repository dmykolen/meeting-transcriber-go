// Package media decodes recordings into the audio shapes the rest of the app
// expects.
package media

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Rate is the canonical downstream sample rate.
const Rate = 16000

// Tools points to app-managed helper binaries when present.
var Tools string

// Sides splits one of our stereo WAVs into aligned microphone and system
// channels.
func Sides(path string) (mic, system []float32, ok bool) {
	if !strings.EqualFold(filepath.Ext(path), ".wav") {
		return nil, nil, false
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, false
	}
	format, body, err := chunks(raw)
	if err != nil || format.channels != 2 || format.bits != 16 || int(format.rate) != Rate {
		return nil, nil, false
	}
	frames := len(body) / 4
	mic, system = make([]float32, frames), make([]float32, frames)
	for i := range frames {
		mic[i] = float32(int16(binary.LittleEndian.Uint16(body[4*i:]))) / 32768
		system[i] = float32(int16(binary.LittleEndian.Uint16(body[4*i+2:]))) / 32768
	}
	return mic, ahead(system, Offset(mic, system)), true
}

// ahead shifts a channel earlier in time.
func ahead(samples []float32, n int) []float32 {
	if n <= 0 || n >= len(samples) {
		return samples
	}
	out := make([]float32, len(samples))
	copy(out, samples[n:])
	return out
}

// Clock renders a recording position.
func Clock(seconds float64) string {
	s := int(seconds)
	if s >= 3600 {
		return fmt.Sprintf("%d:%02d:%02d", s/3600, s/60%60, s%60)
	}
	return fmt.Sprintf("%d:%02d", s/60, s%60)
}

// Loud is RMS loudness.
func Loud(samples []float32) float64 {
	if len(samples) == 0 {
		return 0
	}
	var sum float64
	for _, s := range samples {
		sum += float64(s) * float64(s)
	}
	return math.Sqrt(sum / float64(len(samples)))
}

// Decode reads a file and returns mono samples at Rate.
func Decode(path string) ([]float32, error) {
	if strings.EqualFold(filepath.Ext(path), ".wav") {
		samples, err := decodeWAV(path)
		if err == nil {
			return samples, nil
		}
		// Fall through to slower decoders for unusual WAVs.
	}

	var failed []string
	for _, decode := range []struct {
		name string
		run  func(string) ([]float32, error)
	}{
		{"afconvert", viaAfconvert},
		{"ffmpeg", viaFFmpeg},
	} {
		samples, err := decode.run(path)
		if err == nil {
			return samples, nil
		}
		failed = append(failed, decode.name+": "+trim(err.Error()))
	}
	return nil, fmt.Errorf("%s could not be read (%s)", filepath.Base(path), strings.Join(failed, "; "))
}

// trim keeps decoder errors short enough for the UI.
func trim(s string) string {
	if line := strings.TrimSpace(strings.Split(s, "\n")[0]); len(line) > 90 {
		return line[:90] + "…"
	} else {
		return line
	}
}

// viaAfconvert uses the converter that ships with macOS.
func viaAfconvert(path string) ([]float32, error) {
	if runtime.GOOS != "darwin" {
		return nil, errors.New("only on macOS")
	}
	out, err := os.CreateTemp("", "mt-*.wav")
	if err != nil {
		return nil, err
	}
	out.Close()
	defer os.Remove(out.Name())

	cmd := exec.Command("/usr/bin/afconvert",
		"-f", "WAVE", "-d", fmt.Sprintf("LEI16@%d", Rate), "-c", "1", path, out.Name())
	if raw, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("%s", strings.TrimSpace(string(raw)))
	}
	return decodeWAV(out.Name())
}

// viaFFmpeg is the catch-all decoder.
func viaFFmpeg(path string) ([]float32, error) { return decodeWith(path, "ffmpeg") }

// decodeWAV handles ordinary PCM WAV files in-process.
func decodeWAV(path string) ([]float32, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	format, data, err := chunks(raw)
	if err != nil {
		return nil, err
	}
	if format.audioFormat != 1 || format.bits != 16 {
		return nil, fmt.Errorf("wav: format %d at %d bits is not plain 16-bit PCM", format.audioFormat, format.bits)
	}
	if format.rate%Rate != 0 {
		return nil, fmt.Errorf("wav: %d Hz does not divide down to %d", format.rate, Rate)
	}

	step := int(format.rate / Rate)
	channels := int(format.channels)
	frames := len(data) / 2 / channels
	out := make([]float32, 0, frames/step)

	// Mix channels and decimate. Most app-owned WAVs are already 16 kHz.
	for f := 0; f < frames; f += step {
		var sum float32
		for c := range channels {
			at := (f*channels + c) * 2
			sum += float32(int16(binary.LittleEndian.Uint16(data[at:]))) / 32768
		}
		out = append(out, sum/float32(channels))
	}
	return out, nil
}

type wavFormat struct {
	audioFormat uint16
	channels    uint16
	rate        uint32
	bits        uint16
}

// chunks walks the RIFF structure instead of assuming a 44-byte header.
func chunks(raw []byte) (wavFormat, []byte, error) {
	var format wavFormat
	if len(raw) < 12 || string(raw[0:4]) != "RIFF" || string(raw[8:12]) != "WAVE" {
		return format, nil, errors.New("wav: not a RIFF/WAVE file")
	}
	seen := false
	for at := 12; at+8 <= len(raw); {
		id := string(raw[at : at+4])
		size := int(binary.LittleEndian.Uint32(raw[at+4 : at+8]))
		body := at + 8
		if body+size > len(raw) {
			size = len(raw) - body // a truncated final chunk is still readable
		}
		switch id {
		case "fmt ":
			if size < 16 {
				return format, nil, errors.New("wav: short format chunk")
			}
			format = wavFormat{
				audioFormat: binary.LittleEndian.Uint16(raw[body:]),
				channels:    binary.LittleEndian.Uint16(raw[body+2:]),
				rate:        binary.LittleEndian.Uint32(raw[body+4:]),
				bits:        binary.LittleEndian.Uint16(raw[body+14:]),
			}
			seen = true
		case "data":
			if !seen {
				return format, nil, errors.New("wav: data before format")
			}
			return format, raw[body : body+size], nil
		}
		at = body + size + size%2 // chunks are word-aligned
	}
	return format, nil, errors.New("wav: no data chunk")
}

// decodeWith runs an external decoder and reads raw samples from stdout.
func decodeWith(path, tool string) ([]float32, error) {
	bin, err := find(tool)
	if err != nil {
		return nil, fmt.Errorf("not installed")
	}
	cmd := exec.Command(bin,
		"-nostdin", "-loglevel", "error",
		"-i", path,
		"-f", "f32le", "-acodec", "pcm_f32le",
		"-ac", "1", "-ar", fmt.Sprint(Rate),
		"-")
	out, err := cmd.Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && len(exit.Stderr) > 0 {
			return nil, fmt.Errorf("%s: %s", filepath.Base(path), strings.TrimSpace(string(exit.Stderr)))
		}
		return nil, fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	samples := make([]float32, len(out)/4)
	for i := range samples {
		samples[i] = math.Float32frombits(binary.LittleEndian.Uint32(out[4*i:]))
	}
	if len(samples) == 0 {
		return nil, fmt.Errorf("%s has no audio we can read", filepath.Base(path))
	}
	return samples, nil
}

// find prefers the app's own copy over PATH.
func find(tool string) (string, error) {
	if Tools != "" {
		if candidate := filepath.Join(Tools, tool); usable(candidate) {
			return candidate, nil
		}
	}
	if self, err := os.Executable(); err == nil {
		for _, candidate := range []string{
			filepath.Join(filepath.Dir(self), tool),
			filepath.Join(filepath.Dir(self), "..", "Resources", tool),
		} {
			if usable(candidate) {
				return filepath.Abs(candidate)
			}
		}
	}
	return exec.LookPath(tool)
}

func usable(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir() && info.Mode()&0o111 != 0
}

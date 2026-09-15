package media

import (
	"bytes"
	"encoding/binary"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// wav writes a test WAV in a conventional layout.
func wav(t *testing.T, channels int, rate int, samples []int16, extraChunk bool) string {
	t.Helper()
	body := make([]byte, len(samples)*2)
	for i, s := range samples {
		binary.LittleEndian.PutUint16(body[2*i:], uint16(s))
	}

	var out []byte
	put := func(b ...byte) { out = append(out, b...) }
	u32 := func(v uint32) { b := make([]byte, 4); binary.LittleEndian.PutUint32(b, v); out = append(out, b...) }
	u16 := func(v uint16) { b := make([]byte, 2); binary.LittleEndian.PutUint16(b, v); out = append(out, b...) }

	put([]byte("RIFF")...)
	u32(0) // patched below
	put([]byte("WAVE")...)

	put([]byte("fmt ")...)
	u32(16)
	u16(1)                           // PCM
	u16(uint16(channels))            //
	u32(uint32(rate))                //
	u32(uint32(rate * channels * 2)) // byte rate
	u16(uint16(channels * 2))        // block align
	u16(16)                          // bits

	if extraChunk {
		put([]byte("LIST")...)
		u32(4)
		put([]byte("INFO")...)
	}

	put([]byte("data")...)
	u32(uint32(len(body)))
	out = append(out, body...)
	binary.LittleEndian.PutUint32(out[4:], uint32(len(out)-8))

	path := filepath.Join(t.TempDir(), "test.wav")
	if err := os.WriteFile(path, out, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestAPlainMonoWavIsDecoded(t *testing.T) {
	path := wav(t, 1, Rate, []int16{0, 16384, -16384, 32767}, false)
	got, err := Decode(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 4 {
		t.Fatalf("got %d samples, want 4", len(got))
	}
	if math.Abs(float64(got[1])-0.5) > 0.001 {
		t.Fatalf("sample 1 is %v, want about 0.5", got[1])
	}
}

func TestAChunkBeforeTheDataDoesNotBreakIt(t *testing.T) {
	plain := wav(t, 1, Rate, []int16{1000, 2000, 3000}, false)
	padded := wav(t, 1, Rate, []int16{1000, 2000, 3000}, true)

	a, err := Decode(plain)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Decode(padded)
	if err != nil {
		t.Fatalf("a LIST chunk broke decoding: %v", err)
	}
	if len(a) != len(b) || a[0] != b[0] || a[2] != b[2] {
		t.Fatalf("the same audio decoded differently: %v vs %v", a, b)
	}
}

func TestStereoIsMixedDownToOne(t *testing.T) {
	path := wav(t, 2, Rate, []int16{16384, 0, 0, 16384}, false)
	got, err := Decode(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d frames from 4 stereo samples, want 2", len(got))
	}
	for i, v := range got {
		if math.Abs(float64(v)-0.25) > 0.001 {
			t.Fatalf("frame %d is %v, want the average 0.25", i, v)
		}
	}
}

func TestAHigherSampleRateIsBroughtDown(t *testing.T) {
	samples := make([]int16, 48000) // one second at 48 kHz
	for i := range samples {
		samples[i] = int16(i % 1000)
	}
	got, err := Decode(wav(t, 1, 48000, samples, false))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != Rate {
		t.Fatalf("48 kHz decoded to %d samples, want %d", len(got), Rate)
	}
}

func TestSomethingThatIsNotAWavIsRefusedClearly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "not.wav")
	os.WriteFile(path, []byte("this is not audio at all"), 0o644)

	if _, err := Decode(path); err == nil {
		t.Fatal("a text file decoded as audio")
	}
}

func TestATruncatedFileDoesNotPanic(t *testing.T) {
	full, err := os.ReadFile(wav(t, 1, Rate, []int16{1, 2, 3, 4, 5, 6, 7, 8}, false))
	if err != nil {
		t.Fatal(err)
	}
	for _, cut := range []int{0, 4, 11, 20, 40, len(full) - 3} {
		path := filepath.Join(t.TempDir(), "cut.wav")
		os.WriteFile(path, full[:cut], 0o644)
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("cut at %d panicked: %v", cut, r)
				}
			}()
			Decode(path) // an error is fine; a crash is not
		}()
	}
}

// TestEveryContainerTheAppClaimsToOpen verifies each advertised container path.
func TestEveryContainerTheAppClaimsToOpen(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "tone.wav")
	writeTone(t, source)

	for _, format := range []struct {
		ext  string
		with []string // afconvert arguments, or nil to go through ffmpeg
	}{
		{".m4a", []string{"-f", "m4af", "-d", "aac "}},
		{".aiff", []string{"-f", "AIFF", "-d", "BEI16"}},
		{".caf", []string{"-f", "caff", "-d", "LEI16"}},
		{".mp3", nil},
		{".ogg", nil},
		{".webm", nil},
	} {
		t.Run(format.ext, func(t *testing.T) {
			out := filepath.Join(dir, "tone"+format.ext)
			if format.with != nil {
				run(t, "/usr/bin/afconvert", append(format.with, source, out)...)
			} else {
				bin, err := find("ffmpeg")
				if err != nil {
					t.Skipf("no ffmpeg to make a %s with", format.ext)
				}
				run(t, bin, "-v", "error", "-y", "-i", source, out)
			}

			samples, err := Decode(out)
			if err != nil {
				t.Fatalf("%s: %v", format.ext, err)
			}
			if seconds := float64(len(samples)) / Rate; seconds < 0.8 || seconds > 1.3 {
				t.Fatalf("%s decoded to %.2fs, want about 1s", format.ext, seconds)
			}
		})
	}
}

func writeTone(t *testing.T, path string) {
	t.Helper()
	const n = Rate
	body := new(bytes.Buffer)
	for i := range n {
		v := int16(8000 * math.Sin(2*math.Pi*440*float64(i)/Rate))
		binary.Write(body, binary.LittleEndian, v)
	}
	head := new(bytes.Buffer)
	head.WriteString("RIFF")
	binary.Write(head, binary.LittleEndian, uint32(36+body.Len()))
	head.WriteString("WAVEfmt ")
	binary.Write(head, binary.LittleEndian, uint32(16))
	binary.Write(head, binary.LittleEndian, []uint16{1, 1})
	binary.Write(head, binary.LittleEndian, []uint32{Rate, Rate * 2})
	binary.Write(head, binary.LittleEndian, []uint16{2, 16})
	head.WriteString("data")
	binary.Write(head, binary.LittleEndian, uint32(body.Len()))
	if err := os.WriteFile(path, append(head.Bytes(), body.Bytes()...), 0o644); err != nil {
		t.Fatal(err)
	}
}

func run(t *testing.T, bin string, args ...string) {
	t.Helper()
	if out, err := exec.Command(bin, args...).CombinedOutput(); err != nil {
		t.Skipf("could not build the fixture (%s): %s", err, out)
	}
}

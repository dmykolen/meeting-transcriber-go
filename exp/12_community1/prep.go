// Experiment 12 writes the inputs experiment 11 measures.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/dmykolen/meeting-transcriber-go/internal/media"
)

func main() {
	id := "74"
	if len(os.Args) > 1 {
		id = os.Args[1]
	}
	home := os.Getenv("HOME") + "/MeetingTranscriber"
	out, err := exec.Command("sqlite3", filepath.Join(home, "meetings.db"),
		"SELECT audio FROM recordings WHERE id="+id).Output()
	if err != nil || len(out) == 0 {
		fmt.Println("no such recording:", id, err)
		os.Exit(1)
	}
	path := filepath.Join(home, "recordings", strings.TrimSpace(string(out)))

	mic, tap, ok := media.Sides(path)
	if !ok {
		fmt.Println("not one of ours")
		os.Exit(1)
	}
	dir := os.Args[len(os.Args)-1]
	if !strings.HasPrefix(dir, "/") {
		dir = os.TempDir()
	}
	for name, samples := range map[string][]float32{
		"mix": media.Fold(mic, tap),
		"tap": tap,
	} {
		at := filepath.Join(dir, fmt.Sprintf("%s-%s.wav", id, name))
		if err := media.Mono(at, samples); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		fmt.Printf("%s  %.1f s\n", at, float64(len(samples))/media.Rate)
	}
}

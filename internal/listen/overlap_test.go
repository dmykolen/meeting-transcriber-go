package listen

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Preroll must stop at the previous recording boundary.
func TestPrerollStopsWhereTheLastRecordingEnded(t *testing.T) {
	now := time.Now()
	preroll := 5 * time.Minute

	if got := reach(preroll, time.Time{}, now); got != preroll {
		t.Errorf("with nothing recorded yet the whole preroll is available, got %s", got)
	}
	if got := reach(preroll, now.Add(-90*time.Second), now); got != 90*time.Second {
		t.Errorf("a recording that ended 90s ago caps the preroll at 90s, got %s", got)
	}
	if got := reach(preroll, now.Add(-2*time.Hour), now); got != preroll {
		t.Errorf("an old recording does not cap anything, got %s", got)
	}
}

// Finished recordings must never reuse an existing filename.
func TestAFinishedRecordingNeverOverwritesAnother(t *testing.T) {
	dir := t.TempDir()
	taken := filepath.Join(dir, "meeting 2026-09-08 17-50.wav")
	if err := os.WriteFile(taken, []byte("first"), 0o644); err != nil {
		t.Fatal(err)
	}

	next := free(taken)
	if next == taken {
		t.Fatalf("free handed back a name that is already in use: %s", next)
	}
	if err := os.WriteFile(next, []byte("second"), 0o644); err != nil {
		t.Fatal(err)
	}
	if again := free(taken); again == taken || again == next {
		t.Errorf("free must skip every name already on disk, got %s", again)
	}

	kept, err := os.ReadFile(taken)
	if err != nil || string(kept) != "first" {
		t.Errorf("the first recording was overwritten: %q %v", kept, err)
	}
}

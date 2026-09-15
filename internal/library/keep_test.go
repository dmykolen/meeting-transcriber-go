package library

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dmykolen/meeting-transcriber-go/internal/store"
)

// recorded puts a file on disk and creates the matching row.
func recorded(t *testing.T, db *store.DB, dir, name string, age time.Duration, status store.Status) store.Recording {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), make([]byte, 4096), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := db.Add(store.Recording{
		Kind: store.Meeting, Title: name, Audio: name, Started: time.Now().Add(-age),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Progress(r.ID, status, 1); err != nil {
		t.Fatal(err)
	}
	return r
}

func swept(t *testing.T) (*store.DB, string) {
	t.Helper()
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "k.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db, dir
}

func TestOldAudioGoesAndTheTranscriptStays(t *testing.T) {
	db, dir := swept(t)
	old := recorded(t, db, dir, "old.wav", 40*24*time.Hour, store.Done)
	recent := recorded(t, db, dir, "recent.wav", 2*24*time.Hour, store.Done)

	gone, freed, err := Sweep(db, dir, 30)
	if err != nil || gone != 1 {
		t.Fatalf("deleted %d files (err %v), want 1", gone, err)
	}
	if freed != 4096 {
		t.Fatalf("freed %d bytes, want 4096", freed)
	}
	if _, err := os.Stat(filepath.Join(dir, "old.wav")); err == nil {
		t.Fatal("the old file is still there")
	}
	if _, err := os.Stat(filepath.Join(dir, "recent.wav")); err != nil {
		t.Fatal("a recent recording was deleted")
	}

	kept, err := db.Get(old.ID)
	if err != nil {
		t.Fatalf("the recording itself was deleted: %v", err)
	}
	if kept.Audio != "" {
		t.Fatalf("audio is %q; it should be empty once the file is gone", kept.Audio)
	}
	if still, _ := db.Get(recent.ID); still.Audio == "" {
		t.Fatal("a recording that still has its file was marked as having none")
	}
}

func TestNothingIsDeletedBeforeItHasBeenRead(t *testing.T) {
	db, dir := swept(t)
	recorded(t, db, dir, "queued.wav", 90*24*time.Hour, store.Queued)
	recorded(t, db, dir, "failed.wav", 90*24*time.Hour, store.Failed)

	if gone, _, err := Sweep(db, dir, 1); err != nil || gone != 0 {
		t.Fatalf("deleted %d unread recordings (err %v), want 0", gone, err)
	}
}

func TestZeroDaysKeepsEverything(t *testing.T) {
	db, dir := swept(t)
	recorded(t, db, dir, "ancient.wav", 5000*24*time.Hour, store.Done)

	if gone, _, err := Sweep(db, dir, 0); err != nil || gone != 0 {
		t.Fatalf("deleted %d with retention off (err %v), want 0", gone, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "ancient.wav")); err != nil {
		t.Fatal("a file was deleted although audio is set to be kept for ever")
	}
}

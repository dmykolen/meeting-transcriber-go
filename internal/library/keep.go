package library

import (
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/dmykolen/meeting-transcriber-go/internal/store"
)

// Sweep deletes audio older than the retention setting while keeping transcript
// data.
func Sweep(db *store.DB, dir string, days int) (int, int64, error) {
	if days <= 0 {
		return 0, 0, nil // 0 means keep everything, which is a real answer
	}
	recent, err := db.Recent(1000)
	if err != nil {
		return 0, 0, err
	}
	cutoff := time.Now().AddDate(0, 0, -days)

	gone, freed := 0, int64(0)
	for _, r := range recent {
		// Keep audio until processing has completed; otherwise the file is still
		// the source of truth.
		if r.Audio == "" || r.Status != store.Done || r.Started.After(cutoff) {
			continue
		}
		path := filepath.Join(dir, r.Audio)
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		if err := os.Remove(path); err != nil {
			slog.Warn("could not delete old audio", "file", r.Audio, "err", err)
			continue
		}
		// The cached folded copy is derived from the source audio.
		_ = os.Remove(filepath.Join(filepath.Dir(dir), "cache", r.Audio))
		if err := db.Dropped(r.ID); err != nil {
			slog.Warn("deleted the audio but could not record it", "id", r.ID, "err", err)
		}
		gone++
		freed += info.Size()
	}
	if gone > 0 {
		slog.Info("old audio deleted", "files", gone, "mb", freed/(1<<20), "older_than_days", days)
	}
	return gone, freed, nil
}

// Fortnight is how long something stays in the bin before final deletion.
const Fortnight = 14 * 24 * time.Hour

func (l *Library) Tidy(stop <-chan struct{}, days func() int) {
	for {
		if _, _, err := Sweep(l.db, l.dir, days()); err != nil {
			slog.Warn("could not tidy the recordings folder", "err", err)
		}
		if _, err := l.Empty(Fortnight); err != nil {
			slog.Warn("could not empty the bin", "err", err)
		}
		select {
		case <-stop:
			return
		case <-time.After(24 * time.Hour):
		}
	}
}

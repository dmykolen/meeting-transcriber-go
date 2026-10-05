package service

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/dmykolen/meeting-transcriber-go/internal/home"
	"github.com/dmykolen/meeting-transcriber-go/internal/listen"
	"github.com/dmykolen/meeting-transcriber-go/internal/update"
)

// UpdateState is what the interface shows about updates.
type UpdateState struct {
	Current   string          `json:"current"`
	Available *update.Release `json:"available"` // a newer release, or nil
	Checked   time.Time       `json:"checked"`   // the last successful check
	Stage     string          `json:"stage"`     // "", "downloading" or "ready"
	Fraction  float64         `json:"fraction"`
	Problem   string          `json:"problem"`
	// Installable says the app runs from an .app it can replace itself; when it
	// does not, the interface offers the release page instead.
	Installable bool `json:"installable"`
}

// updateEvery is how often GitHub is asked for a newer release.
const updateEvery = time.Hour

// Update is polled by the interface.
func (m *Meetings) Update() UpdateState {
	m.updMu.Lock()
	defer m.updMu.Unlock()
	state := m.upd
	state.Current = Version
	_, err := update.Bundle()
	state.Installable = err == nil
	return state
}

// WatchUpdates asks for a newer release now and then every hour, until ctx ends.
// Switched off in the settings, it asks nothing.
func (m *Meetings) WatchUpdates(ctx context.Context) {
	tick := time.NewTicker(updateEvery)
	defer tick.Stop()
	for {
		m.mu.Lock()
		off := m.config.NoUpdates
		m.mu.Unlock()
		if !off {
			if err := m.check(ctx); err != nil {
				slog.Warn("could not check for a newer version", "err", err)
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}

// CheckUpdate asks for a newer release now, for the button in the settings.
func (m *Meetings) CheckUpdate() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return m.check(ctx)
}

func (m *Meetings) check(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	url := os.Getenv("MT_UPDATE_URL")
	if url == "" {
		url = update.LatestURL
	}
	latest, err := update.Latest(ctx, url)
	m.updMu.Lock()
	if err != nil {
		m.upd.Problem = "Не вдалося перевірити оновлення: " + err.Error()
		m.updMu.Unlock()
		return err
	}
	m.upd.Problem, m.upd.Checked = "", time.Now()
	newer := update.Newer(Version, latest.Version)
	if !newer {
		m.upd.Available = nil
	} else if m.upd.Available == nil || m.upd.Available.Version != latest.Version {
		m.upd.Available, m.upd.Stage, m.upd.Fraction = &latest, "", 0
	}
	m.updMu.Unlock()
	slog.Info("checked for a newer version", "current", Version, "latest", latest.Version, "newer", newer)
	if newer && m.Notify != nil && m.db.Meta("update-notified") != latest.Version {
		if err := m.db.SetMeta("update-notified", latest.Version); err != nil {
			slog.Warn("could not remember which update was announced", "err", err)
		}
		// Asking macOS for permission waits for a person; the checks go on meanwhile.
		go m.Notify("Доступна нова версія "+latest.Version, "Відкрийте Meeting Transcriber, щоб оновитися.")
	}
	return nil
}

// InstallUpdate downloads and prepares the newer release behind the window;
// Update follows it, and RestartToUpdate finishes it.
func (m *Meetings) InstallUpdate() error {
	app, err := update.Bundle()
	if err != nil {
		return err
	}
	m.updMu.Lock()
	release := m.upd.Available
	if release == nil {
		m.updMu.Unlock()
		return errors.New("нової версії немає")
	}
	if m.upd.Stage != "" {
		m.updMu.Unlock()
		return nil
	}
	m.upd.Stage, m.upd.Fraction, m.upd.Problem = "downloading", 0, ""
	m.updMu.Unlock()

	slog.Info("downloading an update", "version", release.Version)
	go func() {
		staged, err := update.Stage(context.Background(), *release, app, func(f float64) {
			m.updMu.Lock()
			m.upd.Fraction = f
			m.updMu.Unlock()
		})
		m.updMu.Lock()
		defer m.updMu.Unlock()
		if err != nil {
			slog.Error("the update failed", "version", release.Version, "err", err)
			m.upd.Stage, m.upd.Problem = "", "Не вдалося оновити: "+err.Error()
			return
		}
		slog.Info("the update is ready", "version", release.Version, "staged", staged)
		m.staged, m.upd.Stage = staged, "ready"
	}()
	return nil
}

// RestartToUpdate quits the app and opens the new version in its place. It
// refuses while a recording is in progress.
func (m *Meetings) RestartToUpdate() error {
	m.updMu.Lock()
	staged := m.staged
	m.updMu.Unlock()
	if staged == "" {
		return errors.New("оновлення ще не завантажено")
	}
	if s := m.Listening(); s.Phase == listen.Recording || s.Phase == listen.WrappingUp || s.Phase == listen.Held {
		return errors.New("Триває запис: завершіть його, а тоді перезапустіть")
	}
	app, err := update.Bundle()
	if err != nil {
		return err
	}
	logs, _ := os.OpenFile(filepath.Join(home.Logs(m.dir), "update.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err := update.Relaunch(app, staged, logs); err != nil {
		return err
	}
	slog.Info("restarting into the new version", "app", app)
	application.Get().Quit()
	return nil
}

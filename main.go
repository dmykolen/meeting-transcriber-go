// Command mt runs the standalone Meeting Transcriber desktop app.
package main

import (
	"context"
	"embed"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/dmykolen/meeting-transcriber-go/internal/engine"
	"github.com/dmykolen/meeting-transcriber-go/internal/home"
	"github.com/dmykolen/meeting-transcriber-go/internal/insights"
	"github.com/dmykolen/meeting-transcriber-go/internal/library"
	"github.com/dmykolen/meeting-transcriber-go/internal/listen"
	"github.com/dmykolen/meeting-transcriber-go/internal/media"
	"github.com/dmykolen/meeting-transcriber-go/internal/models"
	"github.com/dmykolen/meeting-transcriber-go/internal/service"
	"github.com/dmykolen/meeting-transcriber-go/internal/store"
)

//go:embed all:frontend/dist
var assets embed.FS

// sound serves playback files under /audio/<file>.
func sound(dir, cache string) application.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// net/http has already decoded the path.
			name, is := strings.CutPrefix(r.URL.Path, "/audio/")
			if !is {
				next.ServeHTTP(w, r)
				return
			}
			// Playback filenames still come through the webview, so keep them to
			// a single basename.
			if name == "" || name != filepath.Base(name) {
				http.NotFound(w, r)
				return
			}
			// Serve the folded mono cache when available; raw stereo app files
			// replay the far side twice.
			listen, err := media.Listenable(filepath.Join(dir, name), cache)
			if err != nil {
				slog.Warn("playing the raw recording", "file", name, "err", err)
			}
			http.ServeFile(w, r, listen)
		})
	}
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--mcp-stdio" {
		if err := runMCPStdio(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	// Open the log before run(); startup failures still need somewhere to go.
	dir, err := home.Dir()
	if err == nil {
		if logs, err := os.OpenFile(filepath.Join(home.Logs(dir), "mt.log"),
			os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644); err == nil {
			defer logs.Close()
			slog.SetDefault(slog.New(slog.NewTextHandler(
				io.MultiWriter(logs, os.Stderr), &slog.HandlerOptions{Level: slog.LevelInfo})))
		}
	}
	if err := run(); err != nil {
		slog.Error("Meeting Transcriber stopped", "err", err)
		os.Exit(1)
	}
}

func runMCPStdio() error {
	dir, err := home.Dir()
	if err != nil {
		return err
	}
	cfg, err := home.Load(dir)
	if err != nil {
		return err
	}
	db, err := store.Open(home.Database(dir))
	if err != nil {
		return err
	}
	defer db.Close()
	lib := library.New(db, nil, insights.New(cfg.OpenAIKey, cfg.OpenAIModel, cfg.Language), home.Recordings(dir))
	return service.ServeMCPStdio(service.New(db, lib, dir, cfg))
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	dir, err := home.Dir()
	if err != nil {
		return err
	}
	slog.Info("starting", "folder", dir)
	// Use app-managed decoder tools, not whatever happens to be on PATH.
	media.Tools = models.Tools(home.Models(dir))

	cfg, err := home.Load(dir)
	if err != nil {
		return err
	}
	if cfg.OpenAIKey == "" {
		slog.Warn("no OpenAI key: summaries, the to-do list and Ask are off. " +
			"Add one in Settings — transcription and speakers run here either way")
	}
	db, err := store.Open(home.Database(dir))
	if err != nil {
		return err
	}
	defer db.Close()

	// Backfill provenance for older saved voiceprints once.
	if err := db.Trace(); err != nil {
		slog.Warn("could not trace where the saved voices came from", "err", err)
	}

	// Fetch and load models behind the window.
	setup := service.NewSetup(home.Models(dir))
	// Only fetch Parakeet when the setting asks for it.
	setup.Want(models.Optional(cfg.Transcriber))
	lib := library.New(db, nil, insights.New(cfg.OpenAIKey, cfg.OpenAIModel, cfg.Language), home.Recordings(dir))
	lib.Policy(store.When(cfg.Summarise))
	lib.Owner(cfg.Me)
	meetings := service.New(db, lib, dir, cfg)
	go func() {
		if err := service.RunMCP(ctx, meetings, os.Getenv("MT_MCP_ADDR")); err != nil {
			slog.Error("MCP server stopped", "err", err)
		}
	}()

	// Stale partial WAVs are not recoverable.
	listen.Recover(home.Recordings(dir))

	go func() {
		setup.Fetch(ctx)
		select {
		case <-setup.Wait():
		case <-ctx.Done():
			return
		}
		e, err := engine.Open(home.Models(dir), engine.Options{
			Language:    cfg.Spoken(),
			Threads:     runtime.NumCPU(),
			Transcriber: cfg.Transcriber,
		})
		setup.Loaded(err)
		if err != nil {
			slog.Error("the models would not load", "err", err)
			return
		}
		defer e.Close()
		slog.Info("models ready")
		lib.Use(e)

		// The listener shares loaded models and still runs beside the queue.
		ears := listen.New(home.Recordings(dir),
			models.Path(home.Models(dir), models.Speech), cfg.Listen,
			func(path string, kind store.Kind, started time.Time) {
				if _, err := lib.Add(kind, path, started, ""); err != nil {
					slog.Error("could not file a recording", "path", path, "err", err)
				}
			},
			func(seconds float64, why string) {
				if err := db.Skipped(seconds, why); err != nil {
					slog.Warn("could not record what was discarded", "err", err)
				}
			},
			// One live utterance at a time through the same ASR the queue uses.
			func(samples []float32) (string, error) {
				turns, err := e.Transcribe(samples)
				if err != nil {
					return "", err
				}
				said := make([]string, len(turns))
				for i, t := range turns {
					said[i] = t.Text
				}
				return strings.TrimSpace(strings.Join(said, " ")), nil
			},
			// Reuse the speaker embedder when mic-only company detection needs it.
			e.Print)
		meetings.Listener(ears)
		go ears.Run(ctx)

		// Let live capture win CPU over backlog transcription.
		lib.Wait(ears.Recording)

		// Retention only affects audio files.
		go lib.Tidy(ctx.Done(), func() int { return meetings.Settings().KeepAudioDays })

		lib.Run(ctx)
	}()

	app := application.New(application.Options{
		Name:        "Meeting Transcriber",
		Description: "Records meetings, writes them down, and tells you what was decided.",
		Services: []application.Service{
			application.NewService(meetings),
			application.NewService(setup.Bound()),
		},
		Assets: application.AssetOptions{
			Handler: application.BundledAssetFileServer(assets),
			// One route of our own, for playing a recording back. Everything
			// else falls through to the embedded interface.
			Middleware: sound(home.Recordings(dir), home.Cache(dir)),
		},
		Mac: application.MacOptions{ApplicationShouldTerminateAfterLastWindowClosed: true},
	})

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:  "Meeting Transcriber",
		Width:  1180,
		Height: 800,
		// Small enough to sit beside a call, which is where it will live.
		MinWidth:         900,
		MinHeight:        600,
		BackgroundColour: application.NewRGB(11, 11, 14),
		Mac: application.MacWindow{
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
			InvisibleTitleBarHeight: 42,
		},
	})

	go func() {
		<-ctx.Done()
		app.Quit()
	}()
	return app.Run()
}

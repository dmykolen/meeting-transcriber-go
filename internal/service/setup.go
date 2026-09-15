package service

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/dmykolen/meeting-transcriber-go/internal/models"
)

// Stage is the first-run setup stage.
type Stage string

const (
	Downloading Stage = "downloading" // fetching models on first run
	Loading     Stage = "loading"     // opening them, which takes seconds
	Ready       Stage = "ready"
	Broken      Stage = "broken"
)

// State is the first-run status snapshot.
type State struct {
	Stage    Stage   `json:"stage"`
	What     string  `json:"what"`     // which model, in words
	Fraction float64 `json:"fraction"` // 0..1 across the whole download
	Done     int64   `json:"done"`
	Total    int64   `json:"total"`
	Problem  string  `json:"problem,omitempty"`
}

// Setup drives model download and load state.
type Setup struct {
	dir   string
	extra models.Set

	mu    sync.Mutex
	state State
	ready chan struct{}
	once  sync.Once
}

func NewSetup(modelsDir string) *Setup {
	return &Setup{
		dir:   modelsDir,
		state: State{Stage: Downloading},
		ready: make(chan struct{}),
	}
}

// Want adds optional assets requested by the current config.
func (s *Setup) Want(extra models.Set) { s.extra = extra }

// Status is the narrow Wails-bound setup surface.
type Status struct{ setup *Setup }

// Bound returns the Wails-safe setup view.
func (s *Setup) Bound() *Status { return &Status{setup: s} }

// State is polled by the first-run screen.
func (s *Status) State() State { return s.setup.snapshot() }

func (s *Setup) snapshot() State {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state
}

// Wait closes once required assets are on disk.
func (s *Setup) Wait() <-chan struct{} { return s.ready }

// Fetch downloads whatever is missing.
func (s *Setup) Fetch(ctx context.Context) {
	missing := append(models.Required(), s.extra...).Missing(s.dir)
	if len(missing) == 0 {
		s.set(State{Stage: Loading, What: "Opening the models"})
		s.finish()
		return
	}

	total := missing.Size()
	slog.Info("first run: fetching models", "count", len(missing), "mb", total/1048576)
	s.set(State{Stage: Downloading, What: missing[0].Name, Total: total})

	report := make(chan models.Progress, 8)
	go models.Fetch(ctx, s.dir, missing, report)

	var finished int64
	for p := range report {
		if p.Err != nil {
			s.set(State{Stage: Broken, What: p.Model, Problem: p.Err.Error()})
			slog.Error("model download failed", "model", p.Model, "err", p.Err)
			return
		}
		if p.Finished {
			finished += p.Size
			continue
		}
		done := finished + p.Done
		s.set(State{
			Stage:    Downloading,
			What:     p.Model,
			Done:     done,
			Total:    total,
			Fraction: min(float64(done)/float64(total), 1),
		})
	}

	s.set(State{Stage: Loading, What: "Opening the models", Fraction: 1, Done: total, Total: total})
	s.finish()
}

// Loaded reports the result of engine initialization.
func (s *Setup) Loaded(err error) {
	if err != nil {
		s.set(State{Stage: Broken, What: "Loading the models", Problem: err.Error()})
		return
	}
	s.set(State{Stage: Ready, Fraction: 1})
}

func (s *Setup) set(state State) {
	s.mu.Lock()
	s.state = state
	s.mu.Unlock()
}

func (s *Setup) finish() { s.once.Do(func() { close(s.ready) }) }

// Human renders a byte count for the setup UI.
func Human(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGT"[exp])
}

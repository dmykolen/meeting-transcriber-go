package library

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/dmykolen/meeting-transcriber-go/internal/insights"
	"github.com/dmykolen/meeting-transcriber-go/internal/store"
)

// PreviewSummary renders a summary preview without mutating stored state.
func (l *Library) PreviewSummary(ctx context.Context, id int64) (*store.Summary, error) {
	if !l.AI().Ready() {
		return nil, errors.New("Підсумки вимкнено: оберіть AI у параметрах")
	}
	rows, err := l.db.Turns(id)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, errors.New("Ще немає розшифровки")
	}
	turns := make([]insights.Turn, len(rows))
	for i, r := range rows {
		turns[i] = insights.Turn{Start: r.Start, Speaker: r.Speaker, Text: r.Text}
	}
	slog.Info("summary draft started", "id", id, "turns", len(turns))
	began := time.Now()
	summary, err := l.AI().Summarise(ctx, turns, l.known())
	if err != nil {
		slog.Warn("summary draft finished", "id", id, "took", time.Since(began).Round(time.Millisecond),
			"status", "failed", "err", err)
		return nil, err
	}
	slog.Info("summary draft finished", "id", id, "took", time.Since(began).Round(time.Millisecond),
		"status", "ok", "title", summary.Title)
	return translate(summary), nil
}

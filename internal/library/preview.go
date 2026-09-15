package library

import (
	"context"
	"errors"
	"github.com/dmykolen/meeting-transcriber-go/internal/insights"
	"github.com/dmykolen/meeting-transcriber-go/internal/store"
)

// PreviewSummary renders a summary preview without mutating stored state.
func (l *Library) PreviewSummary(ctx context.Context, id int64) (*store.Summary, error) {
	if !l.llm.Ready() {
		return nil, errors.New("Додайте ключ AI в налаштуваннях")
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
	summary, err := l.llm.Summarise(ctx, turns)
	if err != nil {
		return nil, err
	}
	return translate(summary), nil
}

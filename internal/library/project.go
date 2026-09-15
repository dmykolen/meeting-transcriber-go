package library

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/dmykolen/meeting-transcriber-go/internal/insights"
	"github.com/dmykolen/meeting-transcriber-go/internal/store"
)

// Advance applies one summarised meeting to its project's living document.
func (l *Library) Advance(ctx context.Context, id int64) error {
	r, err := l.db.Get(id)
	if err != nil || r.Group == 0 || r.Summary == nil || !l.llm.Ready() {
		return err
	}
	kept, err := l.db.Held(r.Group)
	if err != nil {
		return err
	}
	if kept == nil {
		kept = &store.Kept{Work: []store.Item{}, Decisions: []store.Item{}, Questions: []store.Item{}}
	}
	for _, seen := range kept.Seen {
		if seen == id {
			return nil
		}
	}

	groups, err := l.db.Groups()
	if err != nil {
		return err
	}
	name := ""
	for _, g := range groups {
		if g.ID == r.Group {
			name = g.Name
		}
	}

	status, changes, err := l.llm.Advance(ctx, name, insights.Held(kept.Status, lines(kept)), meeting(r))
	if err != nil {
		slog.Warn("the project document did not move", "id", id, "group", r.Group, "err", err)
		return nil
	}
	words := make([]store.Word, len(changes))
	for i, c := range changes {
		words[i] = store.Word{Do: c.Do, Kind: c.Kind, ID: c.ID, Text: c.Text,
			Owner: c.Owner, Due: c.Due, State: c.State}
	}
	kept.Apply(words, status, id, r.Started)
	slog.Info("project document moved", "group", r.Group, "meeting", id, "changes", len(words))
	return l.db.Keep(r.Group, kept)
}

// Rebuild throws the document away and replays every meeting from the first.
func (l *Library) Rebuild(ctx context.Context, group int64) error {
	if err := l.db.Rebuild(group); err != nil {
		return err
	}
	rows, err := l.db.In(group, 1000)
	if err != nil {
		return err
	}
	// Oldest first so the document is rebuilt in chronological order.
	for i := len(rows) - 1; i >= 0; i-- {
		if err := l.Advance(ctx, rows[i].ID); err != nil {
			return err
		}
	}
	return nil
}

// lines renders the kept state for the prompt.
func lines(k *store.Kept) map[string][]insights.Line {
	out := map[string][]insights.Line{}
	for kind, items := range map[string][]store.Item{
		"work": k.Work, "decision": k.Decisions, "question": k.Questions,
	} {
		for _, it := range items {
			out[kind] = append(out[kind], insights.Line{
				ID: it.ID, Text: it.Text, Owner: it.Owner,
				Due: it.Due, State: it.State, Times: it.Times,
			})
		}
	}
	return out
}

// meeting renders one meeting summary for the prompt.
func meeting(r *store.Recording) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s, %s\n%s\n", r.Title, r.Started.Format("2 January 2006"), r.Summary.Overview)
	for label, texts := range map[string][]string{
		"decisions": r.Summary.Decisions, "open questions": r.Summary.OpenQuestions,
	} {
		for _, t := range texts {
			fmt.Fprintf(&b, "%s: %s\n", label, t)
		}
	}
	for _, a := range r.Summary.ActionItems {
		fmt.Fprintf(&b, "commitment: %s", a.Task)
		if a.Owner != "" {
			fmt.Fprintf(&b, " — %s", a.Owner)
		}
		if a.Due != "" {
			fmt.Fprintf(&b, ", by %s", a.Due)
		}
		b.WriteString("\n")
	}
	return b.String()
}

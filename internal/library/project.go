package library

import (
	"cmp"
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/dmykolen/meeting-transcriber-go/internal/insights"
	"github.com/dmykolen/meeting-transcriber-go/internal/store"
)

// tidyEvery is how many meetings are folded in between passes over the whole
// document that merge repeats and retire what is finished.
const tidyEvery = 8

// Advance applies one summarised meeting to its project's living document, and
// writes the project's picture again from the result.
func (l *Library) Advance(ctx context.Context, id int64) error { return l.advance(ctx, id, true) }

// advance folds one meeting in. The picture is written when brief is set: a
// rebuild writes it once, after the last meeting, not thirty times on the way.
func (l *Library) advance(ctx context.Context, id int64, brief bool) error {
	r, err := l.db.Get(id)
	if err != nil || r.Group == 0 || r.Summary == nil || !l.AI().Ready() {
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

	status, changes, err := l.AI().Advance(ctx, name, insights.Held(kept.Status, lines(kept, false)), meeting(r))
	if err != nil {
		slog.Warn("the project document did not move", "id", id, "group", r.Group, "err", err)
		return nil
	}
	words := make([]store.Word, len(changes))
	for i, c := range changes {
		words[i] = store.Word{Do: c.Do, Kind: c.Kind, ID: c.ID, Text: c.Text,
			Owner: c.Owner, Due: c.Due, State: c.State, Stream: c.Stream}
	}
	ignored := kept.Apply(words, status, id, r.Started)
	slog.Info("project document moved", "group", r.Group, "meeting", id, "changes", len(words), "ignored", ignored)
	if len(kept.Seen)%tidyEvery == 0 {
		l.tidy(ctx, r.Group, kept)
	}
	if brief {
		l.write(ctx, r.Group, kept)
	}
	return l.db.Keep(r.Group, kept)
}

// tidy runs a pass over the whole document. A failed pass leaves it as it was.
func (l *Library) tidy(ctx context.Context, group int64, kept *store.Kept) {
	pass, err := l.AI().Tidy(ctx, insights.Held("", lines(kept, true)), len(kept.Seen))
	if err != nil {
		slog.Warn("the project document was not tidied", "group", group, "err", err)
		return
	}
	var t store.Tidying
	for _, m := range pass.Merges {
		t.Merges = append(t.Merges, store.Merge{Keep: m.Keep, Drop: m.Drop, Text: m.Text})
	}
	for _, r := range pass.Retire {
		t.Retire = append(t.Retire, store.Retirement{ID: r.ID, State: r.State, Why: r.Why})
	}
	for _, m := range pass.Moves {
		t.Moves = append(t.Moves, store.Move{ID: m.ID, Stream: m.Stream})
	}
	for _, s := range pass.Streams {
		t.Renames = append(t.Renames, store.Rename{From: s.From, To: s.To})
	}
	slog.Info("project document tidied", "group", group, "changes", kept.Tidy(t))
}

// write makes the project's picture from its lines. A failure keeps the last one.
func (l *Library) write(ctx context.Context, group int64, kept *store.Kept) {
	made, err := l.AI().Brief(ctx, insights.Held(kept.Status, lines(kept, false)))
	if err != nil {
		slog.Warn("the project picture was not written", "group", group, "err", err)
		return
	}
	b := &store.Brief{Headline: made.Headline, Made: time.Now(), Attention: []store.Attention{}}
	for _, s := range made.Streams {
		b.Streams = append(b.Streams, store.StreamBrief{Name: s.Name, State: s.State, Lines: kept.Known(s.Lines)})
	}
	b.Decisions = kept.Known(made.Decisions)
	for _, a := range made.Attention {
		if len(kept.Known([]int{a.ID})) > 0 {
			b.Attention = append(b.Attention, store.Attention{ID: a.ID, Why: a.Why})
		}
	}
	kept.Brief = b
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
		if err := l.advance(ctx, rows[i].ID, i == 0); err != nil {
			return err
		}
	}
	return nil
}

// lines renders the kept state for the prompts. With last set each line says
// which meeting touched it last, which a pass over the whole document needs.
func lines(k *store.Kept, last bool) []insights.Line {
	seen := map[int64]int{}
	for i, id := range k.Seen {
		seen[id] = i + 1
	}
	var out []insights.Line
	for kind, items := range map[string][]store.Item{
		"work": k.Work, "decision": k.Decisions, "question": k.Questions,
	} {
		for _, it := range items {
			l := insights.Line{ID: it.ID, Kind: kind, Stream: cmp.Or(it.Stream, store.General), Text: it.Text,
				Owner: it.Owner, Due: it.Due, State: it.State, Times: it.Times}
			if last {
				l.Last = seen[it.From]
			}
			out = append(out, l)
		}
	}
	slices.SortFunc(out, func(a, b insights.Line) int { return a.ID - b.ID })
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

package store

import (
	"sort"
	"strings"
	"time"
)

// Standing is the mechanical project rollup built from stored meetings.
type Standing struct {
	Meetings  int       `json:"meetings"`
	Hours     float64   `json:"hours"`
	First     time.Time `json:"first"`
	Last      time.Time `json:"last"`
	Work      []Thread  `json:"work"`
	Decisions []Thread  `json:"decisions"`
	Questions []Thread  `json:"questions"`
	People    []Face    `json:"people"`
	// Whether a model-authored project document is available.
	Status  string `json:"status"`
	Written bool   `json:"written"`
	// How many meetings the model-authored document has folded in so far.
	Folded int `json:"folded"`
}

// Thread is one recurring line of work, one decision, or one open question.
type Thread struct {
	// The line's id in the kept document, or zero when this came from the fold.
	Item   int       `json:"item"`
	State  string    `json:"state"`
	By     string    `json:"by"`
	Pinned bool      `json:"pinned"`
	Text   string    `json:"text"`
	Owner  string    `json:"owner"`
	Due    string    `json:"due"`
	Done   bool      `json:"done"`
	Times  int       `json:"times"`
	From   int64     `json:"from"`  // the meeting that said it last
	Index  int       `json:"index"` // its place in that meeting's action items
	When   time.Time `json:"when"`
}

// Face is somebody heard in the project.
type Face struct {
	Name     string    `json:"name"`
	Seconds  float64   `json:"seconds"`
	Meetings int       `json:"meetings"`
	Last     time.Time `json:"last"`
}

// Standing gathers one project's current rollup.
func (d *DB) Standing(group int64) (*Standing, error) {
	rows, err := d.list(`WHERE r.folder = ? AND r.deleted IS NULL`, 1000, group)
	if err != nil {
		return nil, err
	}
	out := &Standing{Work: []Thread{}, Decisions: []Thread{}, Questions: []Thread{}, People: []Face{}}
	work, decided, asked := folder{}, folder{}, folder{}

	for _, r := range rows {
		out.Meetings++
		out.Hours += r.Duration / 3600
		if out.First.IsZero() || r.Started.Before(out.First) {
			out.First = r.Started
		}
		if r.Started.After(out.Last) {
			out.Last = r.Started
		}
		if r.Summary == nil {
			continue
		}
		for i, a := range r.Summary.ActionItems {
			work.add(a.Task, r, i, a)
		}
		for _, t := range r.Summary.Decisions {
			decided.add(t, r, -1, Action{})
		}
		for _, t := range r.Summary.OpenQuestions {
			asked.add(t, r, -1, Action{})
		}
	}

	// Open work first, then the most frequently repeated items.
	out.Work = work.sorted(func(a, b Thread) bool {
		if a.Done != b.Done {
			return !a.Done
		}
		if a.Times != b.Times {
			return a.Times > b.Times
		}
		return a.When.After(b.When)
	})
	out.Decisions = decided.sorted(func(a, b Thread) bool { return a.When.After(b.When) })
	out.Questions = asked.sorted(func(a, b Thread) bool {
		if a.Times != b.Times {
			return a.Times > b.Times
		}
		return a.When.After(b.When)
	})

	out.People, err = d.faces(group)
	if err != nil {
		return nil, err
	}

	// Prefer the model-maintained document when present; otherwise keep the
	// mechanical rollup.
	kept, err := d.Held(group)
	if err != nil || kept == nil {
		return out, nil
	}
	out.Status = kept.Status
	out.Written = true
	out.Folded = len(kept.Seen)
	out.Work = threads(kept.Work)
	out.Decisions = threads(kept.Decisions)
	out.Questions = threads(kept.Questions)
	return out, nil
}

// threads maps kept-document items into the page shape.
func threads(items []Item) []Thread {
	out := make([]Thread, 0, len(items))
	for _, it := range items {
		out = append(out, Thread{
			Item: it.ID, Text: it.Text, Owner: it.Owner, Due: it.Due,
			Done:  it.State == "done" || it.State == "answered" || it.State == "overturned",
			State: it.State, By: it.By, Pinned: it.Pinned,
			Times: it.Times, From: it.From, Index: -1, When: it.When,
		})
	}
	return out
}

// folder collapses repeats by a normalized text key.
type folder map[string]*Thread

func (f folder) add(text string, r Recording, index int, a Action) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	k := strings.Map(func(c rune) rune {
		if strings.ContainsRune(".,;:!?—-–'\"()", c) {
			return -1
		}
		return c
	}, strings.ToLower(strings.Join(strings.Fields(text), " ")))

	held, seen := f[k]
	if !seen {
		held = &Thread{Text: text}
		f[k] = held
	}
	held.Times++
	// The latest mention wins for owner/due metadata, and any completed mention
	// marks the thread done.
	if r.Started.After(held.When) {
		held.Text, held.When, held.From, held.Index = text, r.Started, r.ID, index
		held.Owner, held.Due = a.Owner, a.Due
	}
	held.Done = held.Done || a.Done
}

func (f folder) sorted(less func(a, b Thread) bool) []Thread {
	out := make([]Thread, 0, len(f))
	for _, s := range f {
		out = append(out, *s)
	}
	sort.Slice(out, func(i, j int) bool { return less(out[i], out[j]) })
	return out
}

// faces lists everybody heard in a project, longest-heard first.
func (d *DB) faces(group int64) ([]Face, error) {
	rows, err := d.sql.Query(`
		SELECT t.speaker, SUM(t.finish - t.start), COUNT(DISTINCT t.recording), MAX(r.started)
		FROM turns t JOIN recordings r ON r.id = t.recording
		WHERE r.folder = ? AND r.deleted IS NULL AND t.speaker <> ''
		GROUP BY t.speaker ORDER BY SUM(t.finish - t.start) DESC`, group)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Face{}
	for rows.Next() {
		var f Face
		var last int64
		if err := rows.Scan(&f.Name, &f.Seconds, &f.Meetings, &last); err != nil {
			return nil, err
		}
		f.Last = time.Unix(last, 0)
		out = append(out, f)
	}
	return out, rows.Err()
}

// Moment finds roughly where in a recording a project line was said.
func (d *DB) Moment(recording int64, text string) float64 {
	words := strings.Fields(strings.Map(func(r rune) rune {
		if strings.ContainsRune(`"'*()[]{}^:-`, r) {
			return ' ' // strip FTS5 operators from summary text
		}
		return r
	}, text))
	if len(words) == 0 {
		return 0
	}
	if len(words) > 12 {
		words = words[:12]
	}

	var at float64
	// Use OR rather than a phrase because summaries often reword the transcript.
	_ = d.sql.QueryRow(`
		SELECT start FROM transcript
		WHERE recording = ? AND transcript MATCH ?
		ORDER BY rank LIMIT 1`, recording, strings.Join(words, " OR ")).Scan(&at)
	return at
}

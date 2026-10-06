package store

import (
	"cmp"
	"encoding/json"
	"slices"
	"strings"
	"time"
)

// Kept is the model-maintained project document. The model mutates it by
// operations against stable ids instead of rewriting it from scratch.
type Kept struct {
	Status    string    `json:"status"`
	Work      []Item    `json:"work"`
	Decisions []Item    `json:"decisions"`
	Questions []Item    `json:"questions"`
	Brief     *Brief    `json:"brief"` // the picture written from the lines; nil until one is made
	Seen      []int64   `json:"seen"`  // recordings already folded in
	Next      int       `json:"next"`  // the next id to hand out; never reused
	Updated   time.Time `json:"updated"`
}

// Brief is the project as one reads it in a minute, written from the lines.
// It points at lines by id, so what it says can always be traced back.
type Brief struct {
	Headline  string        `json:"headline"`
	Streams   []StreamBrief `json:"streams"`
	Decisions []int         `json:"decisions"` // the standing decisions that shape the work most
	Attention []Attention   `json:"attention"` // open lines that are late, blocked or at risk
	Made      time.Time     `json:"made"`
}

// StreamBrief is where one line of work stands.
type StreamBrief struct {
	Name  string `json:"name"`
	State string `json:"state"`
	Lines []int  `json:"lines"` // the open lines that matter most
}

// Attention is a line that needs somebody, and why.
type Attention struct {
	ID  int    `json:"id"`
	Why string `json:"why"`
}

// An Item is one line of the project document plus its provenance.
type Item struct {
	ID    int    `json:"id"`
	Text  string `json:"text"`
	Owner string `json:"owner"`
	Due   string `json:"due"`
	// open, done, dropped for work; standing, overturned for decisions;
	// open, answered for questions.
	State string `json:"state"`
	// The line of work it belongs to; empty in a document written before streams.
	Stream string `json:"stream"`
	// How many meetings have said it.
	Times int       `json:"times"`
	From  int64     `json:"from"` // the meeting that last touched it
	When  time.Time `json:"when"`
	// Once pinned, the model may close or restate an item but may not rewrite it.
	Pinned bool `json:"pinned"`
	// For an overturned decision, what replaced it.
	By string `json:"by"`
}

// Held reads a project's document, or nil when none exists yet.
func (d *DB) Held(group int64) (*Kept, error) {
	var raw string
	if err := d.sql.QueryRow(`SELECT state FROM groups WHERE id = ?`, group).Scan(&raw); err != nil {
		return nil, err
	}
	if raw == "" {
		return nil, nil
	}
	var kept Kept
	if err := json.Unmarshal([]byte(raw), &kept); err != nil {
		return nil, nil // a corrupt document is rebuilt rather than shown as fatal
	}
	return &kept, nil
}

// Keep writes a kept-state document back to the group row.
func (d *DB) Keep(group int64, kept *Kept) error {
	kept.Updated = time.Now()
	blob, err := json.Marshal(kept)
	if err != nil {
		return err
	}
	_, err = d.sql.Exec(`UPDATE groups SET state = ? WHERE id = ?`, string(blob), group)
	return err
}

// Rebuild clears the kept document so it can be replayed from the start.
func (d *DB) Rebuild(group int64) error {
	_, err := d.sql.Exec(`UPDATE groups SET state = '' WHERE id = ?`, group)
	return err
}

// Word is one model-requested change against the kept document.
type Word struct {
	Do    string `json:"do"`   // add, update, close, restate, answer, overturn
	Kind  string `json:"kind"` // work, decision, question
	ID    int    `json:"id"`
	Text  string `json:"text"`
	Owner string `json:"owner"`
	Due   string `json:"due"`
	State string `json:"state"`
	// The stream an added line belongs to.
	Stream string `json:"stream"`
}

// General is the stream of a line that belongs to the whole project.
const General = "General"

// Apply folds one meeting's worth of model operations into the document and
// returns how many it could not: an unknown kind or verb, an id that is not in
// the document, or an add with no words.
func (k *Kept) Apply(words []Word, status string, from int64, when time.Time) (ignored int) {
	if status != "" {
		k.Status = status
	}
	for _, w := range words {
		list := k.list(w.Kind)
		if list == nil {
			ignored++
			continue
		}
		if w.Do == "add" {
			if w.Text == "" {
				ignored++
				continue
			}
			k.Next++
			*list = append(*list, Item{
				ID: k.Next, Text: w.Text, Owner: owner(w.Owner), Due: w.Due, Stream: stream(w.Stream),
				State: firstState(w.Kind), Times: 1, From: from, When: when,
			})
			continue
		}
		at := -1
		for i := range *list {
			if (*list)[i].ID == w.ID {
				at = i
			}
		}
		if at < 0 {
			ignored++ // an invented id
			continue
		}
		it := &(*list)[at]
		it.From, it.When = from, when
		switch w.Do {
		case "restate":
			it.Times++
		case "update":
			it.Times++
			if !it.Pinned {
				if w.Text != "" {
					it.Text = w.Text
				}
				it.Owner, it.Due = owner(w.Owner), w.Due
			}
		case "close":
			it.State = cmp.Or(w.State, "done")
		case "answer":
			it.State = "answered"
		case "overturn":
			it.State, it.By = "overturned", w.Text
		default:
			ignored++
		}
	}
	k.Seen = append(k.Seen, from)
	return ignored
}

// owner drops a speaker label: "SPEAKER_05" names nobody a reader could ask.
func owner(who string) string {
	if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(who)), "SPEAKER") {
		return ""
	}
	return strings.TrimSpace(who)
}

func stream(name string) string { return cmp.Or(strings.Join(strings.Fields(name), " "), General) }

// Tidying is what a pass over the whole document decided.
type Tidying struct {
	Merges  []Merge
	Retire  []Retirement
	Moves   []Move
	Renames []Rename
}

// Merge folds lines that say the same thing into one.
type Merge struct {
	Keep int
	Drop []int
	Text string // replacement wording; empty keeps the line's own
}

// Retirement closes a line that is finished in effect.
type Retirement struct {
	ID         int
	State, Why string
}

// Move puts a line in another stream.
type Move struct {
	ID     int
	Stream string
}

// Rename turns one stream's name into another's, merging them if both exist.
type Rename struct{ From, To string }

// Tidy applies a pass over the document and returns how many changes took. A
// pinned line keeps its words and is never merged away; a decision is never
// retired, only merged or moved.
func (k *Kept) Tidy(t Tidying) (changed int) {
	for _, m := range t.Merges {
		keep, kind := k.find(m.Keep)
		if keep == nil {
			continue
		}
		for _, id := range m.Drop {
			gone, gkind := k.find(id)
			if gone == nil || gone.ID == keep.ID || gkind != kind || gone.Pinned {
				continue
			}
			keep.Times += gone.Times
			if gone.When.After(keep.When) {
				keep.When, keep.From = gone.When, gone.From
			}
			list := k.list(kind)
			*list = slices.DeleteFunc(*list, func(it Item) bool { return it.ID == id })
			keep, _ = k.find(m.Keep) // the slice moved
			changed++
		}
		if m.Text != "" && keep != nil && !keep.Pinned {
			keep.Text = m.Text
		}
	}
	for _, r := range t.Retire {
		if it, kind := k.find(r.ID); it != nil && kind != "decision" && (it.State == "open") {
			it.State = cmp.Or(r.State, "dropped")
			changed++
		}
	}
	for _, mv := range t.Moves {
		if it, _ := k.find(mv.ID); it != nil && strings.TrimSpace(mv.Stream) != "" {
			it.Stream = stream(mv.Stream)
			changed++
		}
	}
	for _, rn := range t.Renames {
		for _, kind := range []string{"work", "decision", "question"} {
			for i, it := range *k.list(kind) {
				if it.Stream == rn.From && strings.TrimSpace(rn.To) != "" {
					(*k.list(kind))[i].Stream = stream(rn.To)
					changed++
				}
			}
		}
	}
	return changed
}

// find returns a line by id and which list it is in.
func (k *Kept) find(id int) (*Item, string) {
	for _, kind := range []string{"work", "decision", "question"} {
		list := *k.list(kind)
		for i := range list {
			if list[i].ID == id {
				return &list[i], kind
			}
		}
	}
	return nil, ""
}

func (k *Kept) list(kind string) *[]Item {
	switch kind {
	case "work":
		return &k.Work
	case "decision":
		return &k.Decisions
	case "question":
		return &k.Questions
	}
	return nil
}

func firstState(kind string) string {
	if kind == "decision" {
		return "standing"
	}
	return "open"
}

// Pin marks a line as user-edited and therefore not rewordable by the model.
func (d *DB) Pin(group int64, id int, text, owner, due string) error {
	kept, err := d.Held(group)
	if err != nil || kept == nil {
		return err
	}
	for _, list := range []*[]Item{&kept.Work, &kept.Decisions, &kept.Questions} {
		for i := range *list {
			if (*list)[i].ID != id {
				continue
			}
			it := &(*list)[i]
			it.Pinned = true
			if text != "" {
				it.Text = text
			}
			it.Owner, it.Due = owner, due
			return d.Keep(group, kept)
		}
	}
	return nil
}

// Tick marks a work item done or open.
func (d *DB) Tick(group int64, id int, done bool) error {
	kept, err := d.Held(group)
	if err != nil || kept == nil {
		return err
	}
	for i := range kept.Work {
		if kept.Work[i].ID == id {
			kept.Work[i].State = map[bool]string{true: "done", false: "open"}[done]
			return d.Keep(group, kept)
		}
	}
	return nil
}

// Known keeps the ids that name a line of the document, in order, once each.
func (k *Kept) Known(ids []int) []int {
	out := []int{}
	for _, id := range ids {
		if it, _ := k.find(id); it != nil && !slices.Contains(out, id) {
			out = append(out, id)
		}
	}
	return out
}

package store

import (
	"cmp"
	"encoding/json"
	"time"
)

// Kept is the model-maintained project document. The model mutates it by
// operations against stable ids instead of rewriting it from scratch.
type Kept struct {
	Status    string    `json:"status"`
	Work      []Item    `json:"work"`
	Decisions []Item    `json:"decisions"`
	Questions []Item    `json:"questions"`
	Seen      []int64   `json:"seen"` // recordings already folded in
	Next      int       `json:"next"` // the next id to hand out; never reused
	Updated   time.Time `json:"updated"`
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
}

// Apply folds one meeting's worth of model operations into the document.
func (k *Kept) Apply(words []Word, status string, from int64, when time.Time) {
	if status != "" {
		k.Status = status
	}
	for _, w := range words {
		list := k.list(w.Kind)
		if list == nil {
			continue
		}
		if w.Do == "add" {
			if w.Text == "" {
				continue
			}
			k.Next++
			*list = append(*list, Item{
				ID: k.Next, Text: w.Text, Owner: w.Owner, Due: w.Due,
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
			continue // ignore invented ids
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
				it.Owner, it.Due = w.Owner, w.Due
			}
		case "close":
			it.State = cmp.Or(w.State, "done")
		case "answer":
			it.State = "answered"
		case "overturn":
			it.State, it.By = "overturned", w.Text
		}
	}
	k.Seen = append(k.Seen, from)
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

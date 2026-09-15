package store

import (
	"testing"
	"time"
)

func fresh() *Kept {
	return &Kept{Work: []Item{}, Decisions: []Item{}, Questions: []Item{}}
}

// Repeated mentions must stay one line.
func TestOneCommitmentAcrossThreeMeetingsStaysOneLine(t *testing.T) {
	k := fresh()
	now := time.Now()

	k.Apply([]Word{{Do: "add", Kind: "work", Text: "Узгодити ролі", Owner: "Marta"}}, "перша", 1, now)
	k.Apply([]Word{{Do: "restate", Kind: "work", ID: 1}}, "друга", 2, now)
	k.Apply([]Word{{Do: "update", Kind: "work", ID: 1, Text: "Узгодити ролі з безпекою", Owner: "Serhii"}}, "третя", 3, now)

	if len(k.Work) != 1 {
		t.Fatalf("three mentions became %d lines: %+v", len(k.Work), k.Work)
	}
	w := k.Work[0]
	if w.Times != 3 {
		t.Fatalf("counted %d mentions, not 3", w.Times)
	}
	if w.Owner != "Serhii" || w.Text != "Узгодити ролі з безпекою" {
		t.Fatalf("the latest mention did not win: %+v", w)
	}
	if k.Status != "третя" {
		t.Fatalf("the status is %q; each pass replaces it", k.Status)
	}
	if len(k.Seen) != 3 {
		t.Fatalf("recorded %d meetings as folded in, not 3", len(k.Seen))
	}
}

// User-pinned wording may not be rewritten.
func TestTheModelCannotRewordWhatAPersonEdited(t *testing.T) {
	k := fresh()
	k.Apply([]Word{{Do: "add", Kind: "work", Text: "модельне формулювання"}}, "", 1, time.Now())
	k.Work[0].Pinned = true
	k.Work[0].Text = "як я це називаю"

	k.Apply([]Word{{Do: "update", Kind: "work", ID: 1, Text: "щось інше", Owner: "Хтось"}}, "", 2, time.Now())

	if k.Work[0].Text != "як я це називаю" {
		t.Fatalf("a pinned line was reworded to %q", k.Work[0].Text)
	}
	if k.Work[0].Times != 2 {
		t.Fatal("a pinned line should still count the mention")
	}
	k.Apply([]Word{{Do: "close", Kind: "work", ID: 1, State: "done"}}, "", 3, time.Now())
	if k.Work[0].State != "done" {
		t.Fatal("a pinned line could not be closed")
	}
}

// An overturned decision stays as history rather than being deleted.
func TestAnOverturnedDecisionSurvives(t *testing.T) {
	k := fresh()
	k.Apply([]Word{{Do: "add", Kind: "decision", Text: "Відкрити назовні"}}, "", 1, time.Now())
	k.Apply([]Word{{Do: "overturn", Kind: "decision", ID: 1, Text: "Доступ лишається через VPN"}}, "", 2, time.Now())

	if len(k.Decisions) != 1 {
		t.Fatalf("expected the old decision to remain, got %+v", k.Decisions)
	}
	d := k.Decisions[0]
	if d.State != "overturned" || d.By != "Доступ лишається через VPN" {
		t.Fatalf("the reversal was not recorded: %+v", d)
	}
	if d.Text != "Відкрити назовні" {
		t.Fatal("the original decision was overwritten instead of being kept")
	}
}

// Invented ids are ignored.
func TestAnInventedIdIsIgnored(t *testing.T) {
	k := fresh()
	k.Apply([]Word{{Do: "add", Kind: "work", Text: "справжня"}}, "", 1, time.Now())
	k.Apply([]Word{{Do: "close", Kind: "work", ID: 99, State: "done"}}, "", 2, time.Now())

	if k.Work[0].State != "open" {
		t.Fatal("an id the model made up changed a real line")
	}
	if len(k.Work) != 1 {
		t.Fatal("an invented id created a line")
	}
}

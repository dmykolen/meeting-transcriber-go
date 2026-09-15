package store

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestNotesKeepTheirOwnerAndSurviveRestore(t *testing.T) {
	db := open(t)
	meeting := add(t, db, Meeting)
	other := add(t, db, Meeting)
	group, err := db.NewGroup("Архітектура")
	if err != nil {
		t.Fatal(err)
	}
	note, err := db.PutNote(Sticky{Recording: meeting.ID, Text: "Власна думка", Colour: "blue"})
	if err != nil {
		t.Fatal(err)
	}
	note.Text = "Оновлена думка"
	if _, err = db.PutNote(note); err != nil {
		t.Fatal(err)
	}
	moved := note
	moved.Recording = other.ID
	if _, err = db.PutNote(moved); err == nil {
		t.Fatal("note changed owner")
	}
	if _, err = db.PutNote(Sticky{Recording: meeting.ID, Project: group.ID}); err == nil {
		t.Fatal("ambiguous owner accepted")
	}
	if _, err = db.PutNote(Sticky{Project: group.ID, Text: "Проєктна думка"}); err != nil {
		t.Fatal(err)
	}
	notes, err := db.Notes(meeting.ID, 0)
	if err != nil || len(notes) != 1 || notes[0].Text != note.Text {
		t.Fatalf("notes: %+v %v", notes, err)
	}
	if _, err = db.Delete(meeting.ID); err != nil {
		t.Fatal(err)
	}
	notes, err = db.Notes(meeting.ID, 0)
	if err != nil || len(notes) != 0 {
		t.Fatalf("orphan note: %+v %v", notes, err)
	}
	notes, err = db.Notes(0, group.ID)
	if err != nil || len(notes) != 1 {
		t.Fatalf("project note lost: %+v %v", notes, err)
	}
}

func TestActionEditsDoNotOverwriteOtherItemsOrRetitleMeeting(t *testing.T) {
	db := open(t)
	r := add(t, db, Meeting)
	if err := db.SaveSummary(r.ID, &Summary{Title: "AI title", Overview: "Огляд", ActionItems: []Action{{Task: "Перше"}, {Task: "Друге"}}}); err != nil {
		t.Fatal(err)
	}
	if err := db.Retitle(r.ID, "Моя назва"); err != nil {
		t.Fatal(err)
	}
	if err := db.TickAction(r.ID, 0, true); err != nil {
		t.Fatal(err)
	}
	if err := db.EditAction(r.ID, 1, Action{Task: "Оновлене друге", Owner: "Марта", Due: "2026-09-12"}); err != nil {
		t.Fatal(err)
	}
	if err := db.EditAction(r.ID, -1, Action{Task: "Третє"}); err != nil {
		t.Fatal(err)
	}
	got, err := db.Get(r.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "Моя назва" || !got.Summary.ActionItems[0].Done || len(got.Summary.ActionItems) != 3 || got.Summary.ActionItems[1].Owner != "Марта" {
		t.Fatalf("lost edits: %+v %+v", got, got.Summary)
	}
	if err := db.EditAction(r.ID, 20, Action{Task: "Missing"}); err == nil {
		t.Fatal("accepted missing item")
	}
	if err := db.EditAction(r.ID, -1, Action{Task: "   "}); err == nil {
		t.Fatal("accepted blank task")
	}
}

func TestSummaryAcceptanceComparesContentAndRejectsStaleDraft(t *testing.T) {
	db := open(t)
	r := add(t, db, Meeting)
	// Manual creation stores a partial JSON document, unlike a generated summary.
	if err := db.EditAction(r.ID, -1, Action{Task: "Ручна домовленість"}); err != nil {
		t.Fatal(err)
	}
	before, err := db.Get(r.ID)
	if err != nil {
		t.Fatal(err)
	}
	after := *before.Summary
	after.Overview = "Новий огляд"
	if err := db.AcceptSummary(r.ID, before.Summary, &after); err != nil {
		t.Fatal("equivalent partial JSON rejected", err)
	}
	if err := db.TickAction(r.ID, 0, true); err != nil {
		t.Fatal(err)
	}
	if err := db.AcceptSummary(r.ID, &after, before.Summary); err == nil {
		t.Fatal("stale draft overwrote checked commitment")
	}
	got, _ := db.Get(r.ID)
	if !got.Summary.ActionItems[0].Done {
		t.Fatal("check lost")
	}
}

func TestKnowledgeIncludesEveryOwnedKindAndExcludesDeletedContent(t *testing.T) {
	db := open(t)
	r := add(t, db, Meeting)
	g, err := db.NewGroup("Проєкт")
	if err != nil {
		t.Fatal(err)
	}
	if err = db.SaveTranscript(r.ID, "uk", 60, []Turn{{Start: 12, End: 25, Text: "Сказаний контекст", Speaker: "Марта"}}); err != nil {
		t.Fatal(err)
	}
	if err = db.SaveSummary(r.ID, &Summary{Overview: "Огляд", Decisions: []string{"Рішення"}, ActionItems: []Action{{Task: "Домовленість"}}, OpenQuestions: []string{"Питання"}}); err != nil {
		t.Fatal(err)
	}
	note, err := db.PutNote(Sticky{Recording: r.ID, Text: strings.Repeat("Ї", 1250)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.PutNote(Sticky{Project: g.ID, Text: "Контекст проєкту"}); err != nil {
		t.Fatal(err)
	}
	if err = db.Keep(g.ID, &Kept{Status: "Поточний стан"}); err != nil {
		t.Fatal(err)
	}
	corpus, err := db.Knowledge()
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]bool{}
	chunks := 0
	for _, h := range corpus {
		kinds[h.Kind] = true
		if !utf8.ValidString(h.Text) {
			t.Fatal("split Unicode")
		}
		if h.Note == note.ID && h.Kind == "note" {
			chunks++
		}
	}
	for _, kind := range []string{"transcript", "summary", "action", "question", "note", "project"} {
		if !kinds[kind] {
			t.Fatal("missing", kind)
		}
	}
	if chunks != 2 {
		t.Fatal("unexpected chunks", chunks)
	}
	h := corpus[0]
	old := KnowledgeKey(h)
	h.Text += " edit"
	if old == KnowledgeKey(h) {
		t.Fatal("cache failed to invalidate edited text")
	}
	if _, err = db.sql.Exec(`UPDATE recordings SET deleted=1 WHERE id=?`, r.ID); err != nil {
		t.Fatal(err)
	}
	corpus, err = db.Knowledge()
	if err != nil {
		t.Fatal(err)
	}
	for _, h := range corpus {
		if h.Recording == r.ID {
			t.Fatal("deleted source retrieved", h.Kind)
		}
	}
	if len(corpus) != 2 {
		t.Fatalf("project sources lost: %+v", corpus)
	}
}

func TestBinCountAndRestoreIncludeMoreThanTwoHundredRecordings(t *testing.T) {
	db := open(t)
	var last int64
	for i := 0; i < 205; i++ {
		r := add(t, db, Meeting)
		last = r.ID
		if err := db.Bury(r.ID); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := db.Bin()
	if err != nil || len(rows) != 205 {
		t.Fatalf("bin truncated: %d %v", len(rows), err)
	}
	n, err := db.PutNote(Sticky{Recording: last, Text: "Зберегти разом із зустріччю"})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.Restore(last); err != nil {
		t.Fatal(err)
	}
	notes, err := db.Notes(last, 0)
	if err != nil || len(notes) != 1 || notes[0].ID != n.ID {
		t.Fatal("restore lost note", err)
	}
}

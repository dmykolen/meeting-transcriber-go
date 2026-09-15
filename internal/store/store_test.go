package store

import (
	"path/filepath"
	"testing"
	"time"
)

func open(t *testing.T) *DB {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func add(t *testing.T, db *DB, kind Kind) Recording {
	t.Helper()
	r, err := db.Add(Recording{Kind: kind, Title: "meeting.wav", Audio: "a.wav", Started: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestARecordingStartsQueued(t *testing.T) {
	db := open(t)
	r := add(t, db, Meeting)
	if r.ID == 0 {
		t.Fatal("no id was assigned")
	}
	got, err := db.Get(r.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != Queued {
		t.Fatalf("status is %q on arrival", got.Status)
	}
}

func TestATranscriptIsStoredAndReadBackInOrder(t *testing.T) {
	db := open(t)
	r := add(t, db, Meeting)
	turns := []Turn{
		{Start: 0, End: 2, Speaker: "SPEAKER_00", Text: "Привіт, привіт."},
		{Start: 2, End: 5, Speaker: "SPEAKER_01", Text: "Давайте глянемо, що у нас."},
		{Start: 5, End: 9, Speaker: "SPEAKER_00", Text: "Що у нас по задачках?"},
	}
	if err := db.SaveTranscript(r.ID, "uk", 9, turns); err != nil {
		t.Fatal(err)
	}

	got, err := db.Turns(r.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[0].Text != turns[0].Text || got[2].Text != turns[2].Text {
		t.Fatalf("read back %d turns, first %q", len(got), got[0].Text)
	}
	rec, err := db.Get(r.ID)
	if err != nil {
		t.Fatal(err)
	}
	if rec.Language != "uk" || rec.Duration != 9 || rec.Turns != 3 {
		t.Fatalf("language=%q duration=%v turns=%d", rec.Language, rec.Duration, rec.Turns)
	}
	if len(rec.Speakers) != 2 {
		t.Fatalf("speakers=%v, want the two that spoke", rec.Speakers)
	}
}

func TestSavingATranscriptTwiceDoesNotDuplicateIt(t *testing.T) {
	db := open(t)
	r := add(t, db, Meeting)
	turns := []Turn{{Start: 0, End: 1, Text: "один"}}

	for range 3 {
		if err := db.SaveTranscript(r.ID, "uk", 1, turns); err != nil {
			t.Fatal(err)
		}
	}
	got, _ := db.Turns(r.ID)
	if len(got) != 1 {
		t.Fatalf("%d turns after three saves", len(got))
	}
	hits, _ := db.Search("один", 10)
	if len(hits) != 1 {
		t.Fatalf("%d search hits; the index kept the old copies", len(hits))
	}
}

func TestTheSummaryBecomesTheTitle(t *testing.T) {
	db := open(t)
	r := add(t, db, Meeting)

	err := db.SaveSummary(r.ID, &Summary{
		Title:       "Скрипти, безпека і Northwind",
		Overview:    "Пройшлися по задачах.",
		ActionItems: []Action{{Task: "Зібрати матеріали", Owner: "Marta"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	got, _ := db.Get(r.ID)
	if got.Title != "Скрипти, безпека і Northwind" {
		t.Fatalf("title is %q", got.Title)
	}
	if got.Summary == nil || len(got.Summary.ActionItems) != 1 {
		t.Fatal("the summary did not survive the round trip")
	}
	if got.Summary.ActionItems[0].Owner != "Marta" {
		t.Fatalf("owner is %q", got.Summary.ActionItems[0].Owner)
	}
}

func TestAnEmptyTitleLeavesTheOldOneAlone(t *testing.T) {
	db := open(t)
	r := add(t, db, Meeting)
	if err := db.SaveSummary(r.ID, &Summary{Overview: "no title"}); err != nil {
		t.Fatal(err)
	}
	if got, _ := db.Get(r.ID); got.Title != "meeting.wav" {
		t.Fatalf("title became %q; a summary with no title must not blank it", got.Title)
	}
}

func TestSearchFindsPassagesAcrossRecordings(t *testing.T) {
	db := open(t)
	first := add(t, db, Meeting)
	second := add(t, db, Meeting)
	db.SaveSummary(first.ID, &Summary{Title: "Планування"})
	db.SaveSummary(second.ID, &Summary{Title: "Дейлі"})

	db.SaveTranscript(first.ID, "uk", 5, []Turn{{Start: 1, Speaker: "Marta", Text: "документи для Northwind треба зібрати"}})
	db.SaveTranscript(second.ID, "uk", 5, []Turn{{Start: 2, Speaker: "Sofia", Text: "скрипти для витягування даних"}})

	hits, err := db.Search("northwind", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0].Recording != first.ID {
		t.Fatalf("hits=%+v", hits)
	}
	if hits[0].Title != "Планування" || hits[0].Speaker != "Marta" {
		t.Fatalf("a hit must carry where it came from: %+v", hits[0])
	}
}

func TestSearchMatchesAPartialLastWord(t *testing.T) {
	db := open(t)
	r := add(t, db, Meeting)
	db.SaveTranscript(r.ID, "uk", 5, []Turn{{Text: "документи для клієнта"}})

	if hits, _ := db.Search("доку", 10); len(hits) != 1 {
		t.Fatalf("a prefix found %d hits", len(hits))
	}
}

func TestSearchSurvivesWhatPeopleActuallyType(t *testing.T) {
	db := open(t)
	r := add(t, db, Meeting)
	db.SaveTranscript(r.ID, "uk", 5, []Turn{{Text: "нормальний текст"}})

	for _, query := range []string{`"`, `a" OR "b`, `NEAR(`, `*`, `--`, `'`} {
		if _, err := db.Search(query, 10); err != nil {
			t.Fatalf("%q broke the search: %v", query, err)
		}
	}
}

func TestRenamingASpeakerChangesTheTranscriptAndTheIndex(t *testing.T) {
	db := open(t)
	r := add(t, db, Meeting)
	db.SaveTranscript(r.ID, "uk", 5, []Turn{
		{Start: 0, Speaker: "SPEAKER_00", Text: "перше"},
		{Start: 1, Speaker: "SPEAKER_01", Text: "друге"},
	})

	if err := db.Rename(r.ID, "SPEAKER_00", "Marta"); err != nil {
		t.Fatal(err)
	}
	turns, _ := db.Turns(r.ID)
	if turns[0].Speaker != "Marta" || turns[1].Speaker != "SPEAKER_01" {
		t.Fatalf("speakers are %q and %q", turns[0].Speaker, turns[1].Speaker)
	}
	hits, _ := db.Search("перше", 10)
	if len(hits) != 1 || hits[0].Speaker != "Marta" {
		t.Fatalf("the search index still says %+v", hits)
	}
}

func TestRecentIsNewestFirst(t *testing.T) {
	db := open(t)
	base := time.Now()
	for i, when := range []time.Time{base.Add(-2 * time.Hour), base, base.Add(-time.Hour)} {
		db.Add(Recording{Kind: Meeting, Title: string(rune('a' + i)), Audio: "x.wav", Started: when})
	}
	got, err := db.Recent(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[0].Title != "b" || got[2].Title != "a" {
		t.Fatalf("order is %q %q %q", got[0].Title, got[1].Title, got[2].Title)
	}
}

func TestDeletingForgetsEverythingAndNamesTheAudio(t *testing.T) {
	db := open(t)
	r := add(t, db, Meeting)
	db.SaveTranscript(r.ID, "uk", 5, []Turn{{Text: "щось"}})

	audio, err := db.Delete(r.ID)
	if err != nil {
		t.Fatal(err)
	}
	if audio != "a.wav" {
		t.Fatalf("Delete returned %q; the caller needs the path to remove the file", audio)
	}
	if _, err := db.Get(r.ID); err == nil {
		t.Fatal("the recording is still there")
	}
	if hits, _ := db.Search("щось", 10); len(hits) != 0 {
		t.Fatal("the search index still holds a deleted transcript")
	}
}

func TestAFailureIsRecordedWithItsReason(t *testing.T) {
	db := open(t)
	r := add(t, db, Meeting)
	if err := db.Fail(r.ID, errNoModel); err != nil {
		t.Fatal(err)
	}
	got, _ := db.Get(r.ID)
	if got.Status != Failed || got.Problem == "" {
		t.Fatalf("status=%q problem=%q", got.Status, got.Problem)
	}
}

func TestANoteIsKeptAgainstTheRecording(t *testing.T) {
	db := open(t)
	r := add(t, db, Meeting)
	if err := db.SaveNote(r.ID, "спитати Тараса про доступи"); err != nil {
		t.Fatal(err)
	}
	if got, _ := db.Get(r.ID); got.Note != "спитати Тараса про доступи" {
		t.Fatalf("note is %q", got.Note)
	}
}

var errNoModel = &simpleError{"the speaker model would not load"}

type simpleError struct{ s string }

func (e *simpleError) Error() string { return e.s }

// Re-summarising must not overwrite a user title.
func TestAChosenTitleSurvivesBeingSummarisedAgain(t *testing.T) {
	db := open(t)
	r := add(t, db, Meeting)
	if err := db.SaveSummary(r.ID, &Summary{Title: "Скрипти і безпека"}); err != nil {
		t.Fatal(err)
	}
	if err := db.Retitle(r.ID, "Northwind: доступ"); err != nil {
		t.Fatal(err)
	}
	if err := db.SaveSummary(r.ID, &Summary{Title: "Щось інше", Overview: "kept"}); err != nil {
		t.Fatal(err)
	}

	got, _ := db.Get(r.ID)
	if got.Title != "Northwind: доступ" {
		t.Fatalf("the model overwrote a chosen title with %q", got.Title)
	}
	if got.Summary == nil || got.Summary.Overview != "kept" {
		t.Fatal("the new summary was not stored")
	}
}

func TestAnEmptyTitleIsRefused(t *testing.T) {
	db := open(t)
	r := add(t, db, Meeting)
	if err := db.Retitle(r.ID, "   "); err == nil {
		t.Fatal("a meeting was allowed to have no title")
	}
}

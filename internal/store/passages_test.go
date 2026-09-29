package store

import (
	"path/filepath"
	"testing"
)

func TestAPassageIsLongEnoughToMeanSomething(t *testing.T) {
	var turns []Turn
	for i := range 8 {
		at := float64(i) * 20
		who := "Marta"
		if i%2 == 1 {
			who = "Sofia"
		}
		turns = append(turns, Turn{Start: at, End: at + 20, Speaker: who, Text: "слово"})
	}
	pieces := Cut(1, turns)

	if len(pieces) < 2 || len(pieces) > 4 {
		t.Fatalf("160 s became %d passages; expected roughly one per %.0f s", len(pieces), Passage)
	}
	for _, p := range pieces {
		if p.Text == "" {
			t.Fatal("an empty passage was indexed")
		}
	}
}

func TestSearchByMeaningFindsTheNearestPassageAndStopsThere(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "s.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	r, err := db.Add(Recording{Kind: Meeting, Title: "Доступ назовні", Audio: "a.wav"})
	if err != nil {
		t.Fatal(err)
	}
	pieces := []Piece{
		{Recording: r.ID, Start: 0, Text: "про доступ ззовні"},
		{Recording: r.ID, Start: 45, Text: "про обід"},
	}
	if err := db.Index(r.ID, pieces, [][]float32{{1, 0, 0}, {0, 0, 1}}); err != nil {
		t.Fatal(err)
	}

	hits, err := db.Closest([]float32{1, 0, 0}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 {
		t.Fatalf("got %d hits, want only the passage that is actually about it", len(hits))
	}
	if hits[0].Text != "про доступ ззовні" {
		t.Fatalf("the wrong passage came first: %q", hits[0].Text)
	}

	if hits, _ := db.Closest([]float32{0, 1, 0}, 10); len(hits) != 0 {
		t.Fatalf("an unrelated question returned %d hits", len(hits))
	}
}

func TestReindexingReplacesRatherThanAccumulates(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "s.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	r, _ := db.Add(Recording{Kind: Meeting, Title: "t", Audio: "a.wav"})
	one := []Piece{{Recording: r.ID, Start: 0, Text: "first"}}
	for range 3 {
		if err := db.Index(r.ID, one, [][]float32{{1, 0}}); err != nil {
			t.Fatal(err)
		}
	}
	if with, _ := db.Indexed(); with != 1 {
		t.Fatalf("indexing three times left %d passages, want 1", with)
	}
}

func TestPassagesWithoutAKeyAreStillStored(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "s.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	r, _ := db.Add(Recording{Kind: Meeting, Title: "t", Audio: "a.wav"})
	if err := db.Index(r.ID, []Piece{{Recording: r.ID, Text: "щось"}}, nil); err != nil {
		t.Fatal(err)
	}
	with, without := db.Indexed()
	if with != 0 || without != 1 {
		t.Fatalf("with=%d without=%d; a machine with no key should still have the passage", with, without)
	}
	stale, err := db.Stale(10)
	if err != nil || len(stale) != 1 || stale[0] != r.ID {
		t.Fatalf("stale=%v err=%v, want the unindexed recording", stale, err)
	}
}

func TestVectorsFromAnotherModelAreForgottenNotCompared(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "s.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	r, err := db.Add(Recording{Kind: Meeting, Title: "Доступ", Audio: "a.wav"})
	if err != nil {
		t.Fatal(err)
	}
	vector := make([]float32, 512)
	vector[0] = 1
	if err := db.Index(r.ID, []Piece{{Recording: r.ID, Text: "VPN"}}, [][]float32{vector}); err != nil {
		t.Fatal(err)
	}
	if err := db.CacheKnowledge("note", vector); err != nil {
		t.Fatal(err)
	}

	// Everything stored before the choice existed came from OpenAI.
	if changed, err := db.Vectors(oldVectors); err != nil || changed {
		t.Fatalf("changed=%v err=%v; OpenAI's own vectors were thrown away", changed, err)
	}
	if changed, err := db.Vectors("local/model/512"); err != nil || !changed {
		t.Fatalf("changed=%v err=%v; a new model was not noticed", changed, err)
	}
	if with, without := db.Indexed(); with != 0 || without != 1 {
		t.Fatalf("with=%d without=%d; old vectors survived beside the new model", with, without)
	}
	if cached, _ := db.KnowledgeVectors(); len(cached) != 0 {
		t.Fatal("old knowledge vectors survived beside the new model")
	}
	if changed, _ := db.Vectors("local/model/512"); changed {
		t.Fatal("the same model counted as a change")
	}
}

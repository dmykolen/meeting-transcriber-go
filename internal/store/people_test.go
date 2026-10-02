package store

import (
	"math"
	"math/rand/v2"
	"path/filepath"
	"testing"
)

// voice makes a repeatable test vector.
func voice(seed uint64, drift float64) []float32 {
	r := rand.New(rand.NewPCG(seed, 7))
	noise := rand.New(rand.NewPCG(seed+999, 11))
	out := make([]float32, 192)
	for i := range out {
		out[i] = float32(r.NormFloat64() + drift*noise.NormFloat64())
	}
	return out
}

func openDB(t *testing.T) *DB {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "v.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestARememberedVoiceIsRecognisedInTheNextMeeting(t *testing.T) {
	db := openDB(t)
	if err := db.Remember("Sofia", voice(1, 0), Source{Recording: 1, Speaker: "Sofia"}); err != nil {
		t.Fatal(err)
	}
	people, err := db.People()
	if err != nil || len(people) != 1 {
		t.Fatalf("people %+v err %v", people, err)
	}

	names := Recognise(map[string][]float32{
		"SPEAKER_00": voice(1, 0.25),
		"SPEAKER_01": voice(2, 0),
	}, people)

	if names["SPEAKER_00"] != "Sofia" {
		t.Fatalf("names %v, want SPEAKER_00 recognised as Sofia", names)
	}
	if _, wrong := names["SPEAKER_01"]; wrong {
		t.Fatalf("a stranger was named: %v", names)
	}
}

func TestOnePersonIsNotGivenTwoSeatsAtTheTable(t *testing.T) {
	db := openDB(t)
	if err := db.Remember("Marta", voice(3, 0), Source{Recording: 2, Speaker: "Marta"}); err != nil {
		t.Fatal(err)
	}
	people, _ := db.People()

	names := Recognise(map[string][]float32{
		"SPEAKER_00": voice(3, 0.2),
		"SPEAKER_01": voice(3, 0.4),
	}, people)

	if len(names) != 1 {
		t.Fatalf("names %v, want exactly one of them named", names)
	}
}

func TestVoiceprintsAreCappedAndTheRedundantOneGoes(t *testing.T) {
	db := openDB(t)
	for i := range Keep + 4 {
		if err := db.Remember("Taras", voice(uint64(100+i), 0), Source{Recording: int64(i + 1), Speaker: "Taras"}); err != nil {
			t.Fatal(err)
		}
	}
	people, _ := db.People()
	if len(people) != 1 {
		t.Fatalf("people %+v, want one", people)
	}
	if got := len(people[0].Voiceprints); got != Keep {
		t.Fatalf("kept %d voiceprints, want %d", got, Keep)
	}
}

func TestVoicesTravelWithARename(t *testing.T) {
	db := openDB(t)
	r, err := db.Add(Recording{Kind: Meeting, Title: "t", Audio: "a.wav"})
	if err != nil {
		t.Fatal(err)
	}
	print := voice(42, 0)
	if err := db.SaveVoices(r.ID, map[string][]float32{"SPEAKER_00": print}); err != nil {
		t.Fatal(err)
	}
	if got := db.VoiceIn(r.ID, "SPEAKER_00"); len(got) == 0 {
		t.Fatal("the voiceprint was not kept")
	}

	if err := db.Rename(r.ID, "SPEAKER_00", "Serhii"); err != nil {
		t.Fatal(err)
	}
	if got := db.VoiceIn(r.ID, "Serhii"); len(got) == 0 {
		t.Fatal("the voiceprint did not follow the rename")
	}
	if got := db.VoiceIn(r.ID, "SPEAKER_00"); len(got) != 0 {
		t.Fatal("the old label still has a voiceprint")
	}
}

func TestCosineSeparatesAVoiceFromAStranger(t *testing.T) {
	same := Cosine(voice(7, 0), voice(7, 0.2))
	other := Cosine(voice(7, 0), voice(8, 0))
	if same <= other {
		t.Fatalf("same voice scored %.3f, a stranger %.3f", same, other)
	}
	if math.Abs(Cosine(voice(7, 0), voice(7, 0))-1) > 1e-6 {
		t.Fatal("a voice is not identical to itself")
	}
}

// Prints and sources must be trimmed in lockstep.
func TestASampleKeepsTrackOfWhereItCameFrom(t *testing.T) {
	db := openDB(t)
	for i := range Keep + 5 {
		if err := db.Remember("Marta", voice(uint64(500+i), 0),
			Source{Recording: int64(i + 1), Speaker: "Marta"}); err != nil {
			t.Fatal(err)
		}
	}
	people, _ := db.People()
	if len(people) != 1 {
		t.Fatalf("expected one person, got %d", len(people))
	}
	p := people[0]
	if len(p.Sources) != len(p.Voiceprints) {
		t.Fatalf("%d prints against %d sources — they have drifted apart",
			len(p.Voiceprints), len(p.Sources))
	}
	for i, src := range p.Sources {
		if src.Recording == 0 {
			t.Fatalf("sample %d has no source", i)
		}
	}
	newest := int64(Keep + 5)
	found := false
	for _, src := range p.Sources {
		if src.Recording == newest {
			found = true
		}
	}
	if !found {
		t.Fatalf("the newest sample was thrown away instead of a redundant one")
	}
}

// Adding a source must not shift legacy source-free samples.
func TestAnOlderPersonWithoutSourcesIsPaddedNotShifted(t *testing.T) {
	db := openDB(t)
	if err := db.Remember("Serhii", voice(11, 0), Source{}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.sql.Exec(`UPDATE people SET sources = '[]' WHERE name = 'Serhii'`); err != nil {
		t.Fatal(err)
	}
	if err := db.Remember("Serhii", voice(12, 0), Source{Recording: 7, Speaker: "Serhii"}); err != nil {
		t.Fatal(err)
	}
	people, _ := db.People()
	p := people[0]
	if len(p.Sources) != len(p.Voiceprints) {
		t.Fatalf("%d prints against %d sources", len(p.Voiceprints), len(p.Sources))
	}
	if p.Sources[len(p.Sources)-1].Recording != 7 {
		t.Fatalf("the new source landed on the wrong print: %+v", p.Sources)
	}
}

// Split clusters of one enrolled person must be rejoined.
func TestTwoClustersOfOneKnownVoiceAreRejoined(t *testing.T) {
	db := openDB(t)
	if err := db.Remember("Marta", voice(7, 0), Source{Recording: 1, Speaker: "Marta"}); err != nil {
		t.Fatal(err)
	}
	people, _ := db.People()

	prints := map[string][]float32{
		"SPEAKER_00": voice(7, 0.04),
		"SPEAKER_02": voice(7, 0.09),
		"SPEAKER_01": voice(400, 0),
	}
	same := Same(prints, people)
	if len(same) != 1 {
		t.Fatalf("expected one label to be folded into another, got %v", same)
	}
	for from, to := range same {
		if from == "SPEAKER_01" || to == "SPEAKER_01" {
			t.Fatalf("a voice the app has never met was folded away: %v", same)
		}
		if from == to {
			t.Fatal("a label was told to become itself")
		}
	}
}

// Unknown voices must stay distinct.
func TestStrangersAreNotFoldedTogether(t *testing.T) {
	db := openDB(t)
	_ = db.Remember("Marta", voice(7, 0), Source{Recording: 1, Speaker: "Marta"})
	people, _ := db.People()

	prints := map[string][]float32{
		"SPEAKER_00": voice(500, 0),
		"SPEAKER_01": voice(900, 0),
	}
	if same := Same(prints, people); len(same) != 0 {
		t.Fatalf("two strangers were merged: %v", same)
	}
}

// Resembling the same enrolled person is not enough to merge clusters.
func TestTwoDifferentVoicesAreNotMergedJustForResemblingTheSamePerson(t *testing.T) {
	db := openDB(t)
	// Enrol somebody with a broad enough set of samples to resemble both.
	for _, seed := range []uint64{20, 21, 22} {
		if err := db.Remember("Marta", voice(seed, 0),
			Source{Recording: 1, Speaker: "Marta"}); err != nil {
			t.Fatal(err)
		}
	}
	people, _ := db.People()

	prints := map[string][]float32{"SPEAKER_00": voice(20, 0.02), "SPEAKER_01": voice(22, 0.02)}
	for from, to := range Same(prints, people) {
		if Cosine(prints[from], prints[to]) < Rejoin {
			t.Fatalf("%s was folded into %s at a similarity of %.2f, under %.2f",
				from, to, Cosine(prints[from], prints[to]), Rejoin)
		}
	}
}

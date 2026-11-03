package store

import (
	"math"
	"math/rand/v2"
	"path/filepath"
	"slices"
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

// heard files a sample of a voice as taken from a new recording: the owner's
// word when sure, the app's guess otherwise.
func heard(t *testing.T, db *DB, name string, print []float32, sure bool) int64 {
	t.Helper()
	r, err := db.Add(Recording{Kind: Meeting, Title: name, Audio: name + ".wav"})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Remember(name, print, Source{Recording: r.ID, Speaker: name, Confirmed: sure}); err != nil {
		t.Fatal(err)
	}
	return r.ID
}

// from lists where one person's samples came from, and which the owner confirmed.
func from(t *testing.T, db *DB, name string) (recordings []int64, confirmed []bool) {
	t.Helper()
	people, err := db.People()
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range people {
		if p.Name == name {
			for _, s := range p.Sources {
				recordings, confirmed = append(recordings, s.Recording), append(confirmed, s.Confirmed)
			}
		}
	}
	return recordings, confirmed
}

func TestARememberedVoiceIsRecognisedInTheNextMeeting(t *testing.T) {
	db := openDB(t)
	heard(t, db, "Sofia", voice(1, 0), true)
	people, err := db.People()
	if err != nil || len(people) != 1 {
		t.Fatalf("people %+v err %v", people, err)
	}

	names := Recognise(map[string][]float32{
		"SPEAKER_00": voice(1, 0.25),
		"SPEAKER_01": voice(2, 0),
	}, people)

	if names["SPEAKER_00"].Name != "Sofia" || names["SPEAKER_00"].Score < Match {
		t.Fatalf("names %v, want SPEAKER_00 recognised as Sofia", names)
	}
	if _, wrong := names["SPEAKER_01"]; wrong {
		t.Fatalf("a stranger was named: %v", names)
	}
}

func TestOnePersonIsNotGivenTwoSeatsAtTheTable(t *testing.T) {
	db := openDB(t)
	heard(t, db, "Marta", voice(3, 0), true)
	people, _ := db.People()

	names := Recognise(map[string][]float32{
		"SPEAKER_00": voice(3, 0.2),
		"SPEAKER_01": voice(3, 0.4),
	}, people)

	if len(names) != 1 {
		t.Fatalf("names %v, want exactly one of them named", names)
	}
}

// Recording 322: the app's own guesses had become samples, and one of them
// named a colleague after somebody who was not in the meeting.
func TestAGuessRecognisesNobodyUntilTheOwnerConfirmsIt(t *testing.T) {
	db := openDB(t)
	r := heard(t, db, "Sofia", voice(1, 0), false)
	people, _ := db.People()
	if names := Recognise(map[string][]float32{"SPEAKER_00": voice(1, 0.25)}, people); len(names) != 0 {
		t.Fatalf("an unconfirmed guess named a voice: %v", names)
	}
	if err := db.Confirm(r, "Sofia"); err != nil {
		t.Fatal(err)
	}
	people, _ = db.People()
	if names := Recognise(map[string][]float32{"SPEAKER_00": voice(1, 0.25)}, people); names["SPEAKER_00"].Name != "Sofia" {
		t.Fatalf("a confirmed sample did not recognise its voice: %v", names)
	}
}

func TestTheAppsGuessesOnlyTakeFreeSlots(t *testing.T) {
	db := openDB(t)
	var first []int64
	for i := range Keep + 2 {
		first = append(first, heard(t, db, "Taras", voice(uint64(100+i), 0), false))
	}
	if got, _ := from(t, db, "Taras"); !slices.Equal(got, first[:Keep]) {
		t.Fatalf("samples from %v, want the first %d guesses %v and nothing pushed out", got, Keep, first[:Keep])
	}
}

func TestTheOwnersSamplesAreNeverPushedOut(t *testing.T) {
	db := openDB(t)
	var mine []int64
	for i := range Keep - 2 {
		mine = append(mine, heard(t, db, "Taras", voice(uint64(200+i), 0), true))
	}
	oldest := heard(t, db, "Taras", voice(300, 0), false)
	newest := heard(t, db, "Taras", voice(301, 0), false)
	// Full: the owner's word takes the oldest guess's place.
	mine = append(mine, heard(t, db, "Taras", voice(302, 0), true))
	if got, _ := from(t, db, "Taras"); slices.Contains(got, oldest) || !slices.Contains(got, newest) || len(got) != Keep {
		t.Fatalf("samples from %v: the owner's should replace the oldest guess %d", got, oldest)
	}
	mine = append(mine, heard(t, db, "Taras", voice(303, 0), true))
	// Ten of the owner's: neither a guess nor another of the owner's goes in.
	heard(t, db, "Taras", voice(304, 0), false)
	heard(t, db, "Taras", voice(305, 0), true)
	got, sure := from(t, db, "Taras")
	slices.Sort(got)
	if !slices.Equal(got, mine) || slices.Contains(sure, false) {
		t.Fatalf("samples from %v (confirmed %v), want the owner's ten %v", got, sure, mine)
	}
}

// Recording 258: renamed by hand, the voice stayed among the first name's
// samples, and search kept the first name.
func TestARenameTakesTheVoiceBackFromTheOldName(t *testing.T) {
	db := openDB(t)
	r := heard(t, db, "Ostap", voice(9, 0), false)
	kept := heard(t, db, "Ostap", voice(10, 0), true)
	if err := db.SaveVoices(r, map[string][]float32{"Ostap": voice(9, 0)}, map[string]float64{"Ostap": 0.6}); err != nil {
		t.Fatal(err)
	}
	if err := db.Index(r, []Piece{{Recording: r, Speaker: "Ostap", Text: "закупівля"}}, nil); err != nil {
		t.Fatal(err)
	}
	if err := db.Rename(r, "Ostap", "Petro"); err != nil {
		t.Fatal(err)
	}
	if got, _ := from(t, db, "Ostap"); !slices.Equal(got, []int64{kept}) {
		t.Fatalf("Ostap's samples are from %v, want only %d", got, kept)
	}
	if got, _ := db.Get(r); len(got.Guesses) != 0 {
		t.Fatalf("the guess outlived the rename: %v", got.Guesses)
	}
	var who string
	if err := db.sql.QueryRow(`SELECT speaker FROM passages WHERE recording = ?`, r).Scan(&who); err != nil || who != "Petro" {
		t.Fatalf("search still names %q (err %v)", who, err)
	}
}

func TestANoUnnamesTheVoiceAndTakesItsSample(t *testing.T) {
	db := openDB(t)
	r := heard(t, db, "Ivan", voice(4, 0), false)
	if err := db.SaveTranscript(r, "uk", 60, []Turn{
		{Start: 0, End: 30, Speaker: "SPEAKER_00", Text: "добре"},
		{Start: 30, End: 60, Speaker: "Ivan", Text: "реліз у п’ятницю"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.SaveVoices(r, map[string][]float32{"SPEAKER_00": voice(8, 0), "Ivan": voice(4, 0)},
		map[string]float64{"Ivan": 0.72}); err != nil {
		t.Fatal(err)
	}
	if err := db.Reject(r, "Ivan"); err != nil {
		t.Fatal(err)
	}
	got, _ := db.Get(r)
	if !slices.Equal(got.Speakers, []string{"SPEAKER_00", "SPEAKER_01"}) || len(got.Guesses) != 0 {
		t.Fatalf("speakers %v guesses %v, want the voice unnamed as SPEAKER_01", got.Speakers, got.Guesses)
	}
	if Cosine(db.VoiceIn(r, "SPEAKER_01"), voice(4, 0)) < 0.999 {
		t.Fatal("the voice did not keep its print under the new label")
	}
	if left, _ := from(t, db, "Ivan"); len(left) != 0 {
		t.Fatalf("Ivan still holds samples from %v", left)
	}
}

func TestADeletedMeetingTakesItsSamplesWithIt(t *testing.T) {
	db := openDB(t)
	kept := heard(t, db, "Hanna", voice(5, 0), true)
	binned := heard(t, db, "Hanna", voice(6, 0), true)
	if err := db.Bury(binned); err != nil {
		t.Fatal(err)
	}
	if got, _ := from(t, db, "Hanna"); !slices.Equal(got, []int64{kept}) {
		t.Fatalf("samples from %v: one from a meeting in the bin still counts", got)
	}
	if err := db.Restore(binned); err != nil {
		t.Fatal(err)
	}
	if got, sure := from(t, db, "Hanna"); !slices.Equal(got, []int64{kept, binned}) || slices.Contains(sure, false) {
		t.Fatalf("samples from %v (confirmed %v) after restoring the meeting", got, sure)
	}
	_ = db.Bury(binned)
	if _, err := db.Delete(binned); err != nil {
		t.Fatal(err)
	}
	stored, _ := db.everyone()
	if len(stored[0].Sources) != 1 || stored[0].Sources[0].Recording != kept {
		t.Fatalf("the deleted meeting's sample is still stored: %+v", stored[0].Sources)
	}
}

func TestVoicesTravelWithARename(t *testing.T) {
	db := openDB(t)
	r, err := db.Add(Recording{Kind: Meeting, Title: "t", Audio: "a.wav"})
	if err != nil {
		t.Fatal(err)
	}
	print := voice(42, 0)
	if err := db.SaveVoices(r.ID, map[string][]float32{"SPEAKER_00": print}, nil); err != nil {
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
	for i := range Keep {
		heard(t, db, "Marta", voice(uint64(500+i), 0), false)
	}
	var mine []int64
	for i := range 5 {
		mine = append(mine, heard(t, db, "Marta", voice(uint64(600+i), 0), true))
	}
	people, _ := db.People()
	p := people[0]
	if len(p.Sources) != len(p.Voiceprints) {
		t.Fatalf("%d prints against %d sources — they have drifted apart",
			len(p.Voiceprints), len(p.Sources))
	}
	for i, src := range p.Sources {
		if src.Recording == 0 {
			t.Fatalf("sample %d has no source", i)
		}
		if src.Confirmed != slices.Contains(mine, src.Recording) {
			t.Fatalf("sample %d from %d lost its confirmation", i, src.Recording)
		}
	}
	if got, _ := from(t, db, "Marta"); !slices.Equal(got[Keep-5:], mine) {
		t.Fatalf("samples from %v: the owner's newest should have taken the oldest guesses' places", got)
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
	r := heard(t, db, "Serhii", voice(12, 0), true)
	people, _ := db.People()
	p := people[0]
	if len(p.Sources) != len(p.Voiceprints) || len(p.Sources) != 2 {
		t.Fatalf("%d prints against %d sources", len(p.Voiceprints), len(p.Sources))
	}
	if p.Sources[len(p.Sources)-1].Recording != r {
		t.Fatalf("the new source landed on the wrong print: %+v", p.Sources)
	}
}

// Split clusters of one enrolled person must be rejoined.
func TestTwoClustersOfOneKnownVoiceAreRejoined(t *testing.T) {
	db := openDB(t)
	heard(t, db, "Marta", voice(7, 0), true)
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
	heard(t, db, "Marta", voice(7, 0), true)
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
		heard(t, db, "Marta", voice(seed, 0), true)
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

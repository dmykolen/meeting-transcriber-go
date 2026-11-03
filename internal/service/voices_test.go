package service

import "testing"

// Recording 258: renamed by hand, the voice stayed among the first name's
// samples and kept naming the wrong person.
func TestARenameMovesTheVoiceToTheNewName(t *testing.T) {
	m, rec, _ := testMeetings(t)
	if err := m.db.SaveVoices(rec, map[string][]float32{"Alice": {1, 0}}, map[string]float64{"Alice": 0.7}); err != nil {
		t.Fatal(err)
	}
	if err := m.Rename(rec, "Alice", "Carol"); err != nil {
		t.Fatal(err)
	}
	people, err := m.People()
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range people {
		if p.Name == "Alice" && p.Samples != 0 {
			t.Fatalf("Alice still holds the voice: %+v", p.Sources)
		}
		if p.Name == "Carol" && (p.Samples != 1 || !p.Sources[0].Confirmed) {
			t.Fatalf("Carol was not taught the voice as the owner's word: %+v", p.Sources)
		}
	}
	if got, _ := m.Open(rec); len(got.Guesses) != 0 {
		t.Fatalf("the guess outlived the rename: %v", got.Guesses)
	}
}

func TestTheOwnerSaysYesToOneGuessAndNoToAnother(t *testing.T) {
	m, rec, _ := testMeetings(t)
	if err := m.db.SaveVoices(rec, map[string][]float32{"Alice": {1, 0}, "Bob": {0, 1}},
		map[string]float64{"Alice": 0.81, "Bob": 0.58}); err != nil {
		t.Fatal(err)
	}
	if err := m.Confirm(rec, "Alice"); err != nil {
		t.Fatal(err)
	}
	if err := m.Reject(rec, "Bob"); err != nil {
		t.Fatal(err)
	}
	got, _ := m.Open(rec)
	if len(got.Guesses) != 0 || len(got.Speakers) != 2 || got.Speakers[1] != "SPEAKER_00" {
		t.Fatalf("speakers %v guesses %v, want Alice kept and Bob's voice unnamed", got.Speakers, got.Guesses)
	}
	people, _ := m.People()
	for _, p := range people {
		if p.Name == "Alice" && (p.Samples != 1 || !p.Sources[0].Confirmed) {
			t.Fatalf("Alice's sample was not confirmed: %+v", p.Sources)
		}
	}
}

// Renaming to "-" or back to a label was the only way to take a name away;
// it must not enrol "-" as a person the app then suggests elsewhere.
func TestARenameThatTakesTheNameAwayLearnsNobody(t *testing.T) {
	m, rec, _ := testMeetings(t)
	if err := m.db.SaveVoices(rec, map[string][]float32{"Alice": {1, 0}, "Bob": {0, 1}}, nil); err != nil {
		t.Fatal(err)
	}
	if err := m.Rename(rec, "Alice", "-"); err != nil {
		t.Fatal(err)
	}
	if err := m.Rename(rec, "Bob", "SPEAKER_03"); err != nil {
		t.Fatal(err)
	}
	people, _ := m.People()
	for _, p := range people {
		if p.Name == "-" || p.Name == "SPEAKER_03" || (p.Name == "Alice" && p.Samples != 0) {
			t.Fatalf("%s holds %d samples after the name was taken away", p.Name, p.Samples)
		}
	}
}

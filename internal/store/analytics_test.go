package store

import (
	"math"
	"testing"
)

func TestTalkTimeCountsAnOverlapOnce(t *testing.T) {
	// Two people talking over each other is one second of meeting, and the
	// difference between the sum and the union is exactly how much they did it.
	a := Analyse([]Turn{
		{Start: 0, End: 10, Speaker: "Marta", Text: "one two three"},
		{Start: 8, End: 20, Speaker: "Sofia", Text: "four five"},
	}, 30)

	if a.Speech != 20 {
		t.Fatalf("speech %v, want 20 (0..20 covered once)", a.Speech)
	}
	if a.Silence != 10 {
		t.Fatalf("silence %v, want 10", a.Silence)
	}
	if a.Overlap != 2 {
		t.Fatalf("overlap %v, want 2 (8..10)", a.Overlap)
	}
}

func TestSharesAddUpAndTheLoudestIsFirst(t *testing.T) {
	a := Analyse([]Turn{
		{Start: 0, End: 30, Speaker: "Dmytro", Text: "a b c"},
		{Start: 30, End: 40, Speaker: "Marta", Text: "d"},
		{Start: 40, End: 50, Speaker: "Marta", Text: "e"},
	}, 60)

	if len(a.Speakers) != 2 || a.Speakers[0].Speaker != "Dmytro" {
		t.Fatalf("speakers %+v, want Dmytro first", a.Speakers)
	}
	if a.Speakers[1].Turns != 2 {
		t.Fatalf("Marta had %d turns, want 2", a.Speakers[1].Turns)
	}
	total := 0.0
	for _, v := range a.Speakers {
		total += v.Share
	}
	if math.Abs(total-1) > 1e-9 {
		t.Fatalf("shares add up to %v, want 1", total)
	}
	if a.Speakers[0].Longest != 30 {
		t.Fatalf("longest stretch %v, want 30", a.Speakers[0].Longest)
	}
}

func TestBalanceSeparatesADiscussionFromABroadcast(t *testing.T) {
	even := Analyse([]Turn{
		{Start: 0, End: 10, Speaker: "A", Text: "x"},
		{Start: 10, End: 20, Speaker: "B", Text: "y"},
	}, 20)
	lopsided := Analyse([]Turn{
		{Start: 0, End: 99, Speaker: "A", Text: "x"},
		{Start: 99, End: 100, Speaker: "B", Text: "y"},
	}, 100)

	if even.Balance < 0.99 {
		t.Fatalf("an even split scored %v, want ~1", even.Balance)
	}
	if lopsided.Balance > 0.2 {
		t.Fatalf("a monologue scored %v, want near 0", lopsided.Balance)
	}
}

func TestQuestionsAreCountedInBothLanguages(t *testing.T) {
	a := Analyse([]Turn{
		{Start: 0, End: 5, Speaker: "Marta", Text: "Коли ми це закриваємо"},   // no mark, still a question
		{Start: 5, End: 10, Speaker: "Marta", Text: "What did we agree on?"},  // marked
		{Start: 10, End: 15, Speaker: "Sofia", Text: "Я зроблю це сьогодні."}, // a statement
	}, 20)

	by := map[string]int{}
	for _, v := range a.Speakers {
		by[v.Speaker] = v.Questions
	}
	if by["Marta"] != 2 {
		t.Fatalf("Marta asked %d, want 2", by["Marta"])
	}
	if by["Sofia"] != 0 {
		t.Fatalf("Sofia asked %d, want 0", by["Sofia"])
	}
}

func TestAnEmptyRecordingIsAllSilence(t *testing.T) {
	a := Analyse(nil, 120)
	if a.Speech != 0 || a.Silence != 120 {
		t.Fatalf("speech %v silence %v, want 0 and 120", a.Speech, a.Silence)
	}
}

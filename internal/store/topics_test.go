package store

import (
	"slices"
	"testing"
	"time"
)

func TestTopicsAreCountedAcrossSpellings(t *testing.T) {
	db := open(t)
	for i, topics := range [][]string{{"Безпека", "Northwind"}, {"безпека", "Скрипти"}, {"Безпека ", "northwind"}} {
		r, err := db.Add(Recording{Kind: Meeting, Audio: "a.wav", Started: time.Now().Add(time.Duration(i) * time.Hour)})
		if err != nil {
			t.Fatal(err)
		}
		if err := db.SaveSummary(r.ID, &Summary{Topics: topics}); err != nil {
			t.Fatal(err)
		}
	}
	got, err := db.Topics(2)
	if err != nil {
		t.Fatal(err)
	}
	// "Безпека" is written three ways and "Northwind" twice; the most common
	// spelling stands for each.
	if want := []string{"Безпека", "Northwind"}; !slices.Equal(got, want) {
		t.Fatalf("Topics = %q, want %q", got, want)
	}
}

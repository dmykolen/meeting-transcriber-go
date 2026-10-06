package store

import (
	"testing"
	"time"
)

// Repeated commitments across meetings must fold into one line.
func TestOneCommitmentSaidThriceIsOneLine(t *testing.T) {
	db := open(t)
	g, err := db.NewGroup("Northwind")
	if err != nil {
		t.Fatal(err)
	}

	said := []struct {
		task  string
		owner string
		day   int
	}{
		{"Узгодити перелік ролей", "", 1},
		{"узгодити перелік ролей.", "Marta", 2},    // same thing, punctuated
		{"Узгодити  перелік   ролей", "Serhii", 3}, // same thing, spaced
		{"Закрити доступ ззовні", "Taras", 3},
	}
	for i, s := range said {
		r, err := db.Add(Recording{
			Kind: Meeting, Title: "нарада", Audio: "a.wav",
			Started: time.Now().AddDate(0, 0, -10+s.day), Duration: 600,
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := db.Assign(r.ID, g.ID); err != nil {
			t.Fatal(err)
		}
		if err := db.SaveSummary(r.ID, &Summary{
			Title:       "нарада",
			ActionItems: []Action{{Task: s.task, Owner: s.owner}},
		}); err != nil {
			t.Fatalf("summary %d: %v", i, err)
		}
	}

	got, err := db.Standing(g.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Work) != 2 {
		for _, w := range got.Work {
			t.Logf("  %q ×%d", w.Text, w.Times)
		}
		t.Fatalf("three wordings of one commitment became %d lines", len(got.Work))
	}
	roles := got.Work[0]
	if roles.Times != 3 {
		t.Fatalf("the repeated commitment counted %d mentions, not 3", roles.Times)
	}
	if roles.Owner != "Serhii" {
		t.Fatalf("owner is %q; the most recent mention should win", roles.Owner)
	}
	if got.Meetings != 4 {
		t.Fatalf("counted %d meetings, not 4", got.Meetings)
	}
}

// Completing a commitment in any meeting closes it for the project.
func TestDoneAnywhereIsDone(t *testing.T) {
	db := open(t)
	g, _ := db.NewGroup("Ops")
	for i, done := range []bool{false, true} {
		r, _ := db.Add(Recording{Kind: Meeting, Title: "н", Audio: "a.wav",
			Started: time.Now().AddDate(0, 0, i), Duration: 60})
		_ = db.Assign(r.ID, g.ID)
		_ = db.SaveSummary(r.ID, &Summary{Title: "н",
			ActionItems: []Action{{Task: "Полагодити", Done: done}}})
	}
	got, _ := db.Standing(g.ID)
	if len(got.Work) != 1 || !got.Work[0].Done {
		t.Fatalf("a commitment ticked off in one meeting is still open: %+v", got.Work)
	}
}

func TestTopicsOfAProjectCountMeetingsNotMentions(t *testing.T) {
	db := open(t)
	g, err := db.NewGroup("P")
	if err != nil {
		t.Fatal(err)
	}
	for i, topics := range [][]string{{"Безпека", "безпека", "Northwind"}, {"БЕЗПЕКА"}, {"Скрипти"}} {
		r, _ := db.Add(Recording{Kind: Meeting, Audio: "a.wav", Started: time.Now().Add(time.Duration(i) * time.Hour)})
		db.SaveSummary(r.ID, &Summary{Topics: topics})
		db.Assign(r.ID, g.ID)
	}
	st, err := db.Standing(g.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(st.Topics) != 3 || st.Topics[0].Topic != "Безпека" || st.Topics[0].Count != 2 {
		t.Fatalf("topics = %+v", st.Topics)
	}
	// Only the first meeting had two different topics: Безпека with Northwind.
	if len(st.Links) != 1 || st.Links[0].N != 1 || st.Links[0].A+st.Links[0].B != "БезпекаNorthwind" && st.Links[0].A+st.Links[0].B != "NorthwindБезпека" {
		t.Fatalf("links = %+v", st.Links)
	}
}

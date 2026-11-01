package store

import (
	"slices"
	"testing"
	"time"
)

// A voice note has its own screen. It is nobody's meeting: it stays out of the
// meeting list's timeline, the topic vocabulary and the time spent in meetings,
// while its to-dos are still to-dos.
func TestVoiceNotesStayOutOfTheMeetingViews(t *testing.T) {
	db := open(t)
	at := time.Now().Add(-2 * time.Hour)
	meeting, err := db.Add(Recording{Kind: Meeting, Audio: "m.wav", Started: at, Duration: 1800})
	if err != nil {
		t.Fatal(err)
	}
	note, err := db.Add(Recording{Kind: Note, Audio: "n.wav", Started: at.Add(time.Hour), Duration: 3600})
	if err != nil {
		t.Fatal(err)
	}
	binned, err := db.Add(Recording{Kind: Note, Audio: "b.wav", Started: at})
	if err != nil {
		t.Fatal(err)
	}
	db.Bury(binned.ID)
	twice := []string{"Чому впав деплой?", "Чому впав деплой?"}
	db.SaveSummary(meeting.ID, &Summary{Topics: []string{"Деплой"}, OpenQuestions: twice[:1]})
	// A note written up before notes had a prompt of their own carries topics
	// and questions; a new one has only its text and to-dos.
	db.SaveSummary(note.ID, &Summary{Topics: []string{"Батарейки"}, OpenQuestions: twice[1:],
		ActionItems: []Action{{Task: "Купити батарейки", Due: "2026-01-02"}}})

	notes, err := db.VoiceNotes()
	if err != nil || len(notes) != 1 || notes[0].ID != note.ID {
		t.Fatalf("VoiceNotes = %+v, %v; want the one live note", notes, err)
	}
	if topics, _ := db.Topics(10); !slices.Equal(topics, []string{"Деплой"}) {
		t.Fatalf("Topics = %q; a note's topics joined the meetings' vocabulary", topics)
	}
	if marks, _ := db.Span(at.Add(-time.Minute), time.Now()); len(marks) != 1 || marks[0].ID != meeting.ID {
		t.Fatalf("Span = %+v; the timeline shows only meetings", marks)
	}
	b, err := db.Brief(1)
	if err != nil {
		t.Fatal(err)
	}
	if len(b.Meetings) != 1 || b.Minutes != 30 {
		t.Fatalf("brief has %d meetings and %d minutes; the note's hour is not meeting time", len(b.Meetings), b.Minutes)
	}
	if len(b.Mine) != 1 || len(b.Overdue) != 1 || b.Mine[0].Recording != note.ID {
		t.Fatalf("mine %+v, overdue %+v; the note's to-do, due long ago, is lost", b.Mine, b.Overdue)
	}
	if len(b.Nagging) != 0 {
		t.Fatalf("nagging %+v; a question said in a note made it a meeting's repeat", b.Nagging)
	}
}

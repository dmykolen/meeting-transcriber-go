package store

import (
	"math"
	"testing"
	"time"
)

func TestRhythmMeasuresWeeksHoursAndVoices(t *testing.T) {
	db := open(t)
	// Wednesday 2026-10-07 14:30; its Monday is the 5th.
	now := time.Date(2026, 10, 7, 14, 30, 0, 0, time.Local)
	meet := func(at time.Time, minutes float64, kind Kind, decisions int, turns ...Turn) {
		t.Helper()
		r, err := db.Add(Recording{Kind: kind, Audio: "a.wav", Started: at, Duration: minutes * 60})
		if err != nil {
			t.Fatal(err)
		}
		if decisions > 0 {
			if err := db.SaveSummary(r.ID, &Summary{Decisions: make([]string, decisions), ActionItems: []Action{{Task: "x"}}}); err != nil {
				t.Fatal(err)
			}
		}
		if err := db.SaveTranscript(r.ID, "uk", minutes*60, turns); err != nil {
			t.Fatal(err)
		}
	}
	// 40 minutes from 09:40 on Monday this week: 20 in the 09 hour, 20 in the 10.
	meet(time.Date(2026, 10, 5, 9, 40, 0, 0, time.Local), 40, Meeting, 2,
		Turn{Start: 0, End: 60, Speaker: "Marta", Text: "a"}, Turn{Start: 60, End: 90, Speaker: "SPEAKER_01", Text: "b"})
	// An hour on Tuesday last week, and a voice note that is not a meeting.
	meet(time.Date(2026, 9, 29, 16, 0, 0, 0, time.Local), 60, Meeting, 0, Turn{Start: 0, End: 30, Speaker: "Marta", Text: "c"})
	meet(time.Date(2026, 10, 6, 8, 0, 0, 0, time.Local), 10, Note, 0)
	// Older than the twelve weeks.
	meet(now.AddDate(0, 0, -7*13), 30, Meeting, 0)

	r, err := db.Rhythm(now, 7)
	if err != nil {
		t.Fatal(err)
	}
	cur, before := r.Weeks[Weeks-1], r.Weeks[Weeks-2]
	if cur.Meetings != 1 || cur.Decisions != 2 || cur.Commitments != 1 || math.Abs(cur.Hours-40.0/60) > 1e-9 {
		t.Fatalf("this week = %+v", cur)
	}
	if before.Meetings != 1 || math.Abs(before.Hours-1) > 1e-9 || !before.Start.Equal(cur.Start.AddDate(0, 0, -7)) {
		t.Fatalf("last week = %+v", before)
	}
	if got := r.Clock[0][9] + r.Clock[0][10]; math.Abs(got-40) > 1e-9 || math.Abs(r.Clock[0][9]-20) > 1e-9 {
		t.Fatalf("Monday 09:40 for 40 minutes: 09h %v, 10h %v", r.Clock[0][9], r.Clock[0][10])
	}
	if r.Clock[1][16] != 60 || r.Clock[1][8] != 0 {
		t.Fatalf("Tuesday 16h = %v, and the voice note counted: %v", r.Clock[1][16], r.Clock[1][8])
	}
	// Speech over the last 7 days: Marta 60 s, an unnamed voice 30 s; last week's turn is outside.
	if len(r.Voices) != 2 || r.Voices[0].Speaker != "Marta" || r.Voices[0].Seconds != 60 || r.Voices[1].Speaker != "" || r.Voices[1].Seconds != 30 {
		t.Fatalf("voices = %+v", r.Voices)
	}
}

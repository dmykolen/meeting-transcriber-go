package store

import (
	"encoding/json"
	"sort"
	"strings"
	"time"
)

// Weeks is how many weeks of history the rhythm covers.
const Weeks = 12

// Week is one Monday-to-Sunday stretch of meetings.
type Week struct {
	Start       time.Time `json:"start"` // its Monday, local midnight
	Hours       float64   `json:"hours"` // recorded in meetings
	Meetings    int       `json:"meetings"`
	Decisions   int       `json:"decisions"`   // written in those meetings' summaries
	Commitments int       `json:"commitments"` // the same, for action items
}

// Share is how long one person spoke.
type Share struct {
	Speaker string  `json:"speaker"` // empty for voices nobody has named
	Seconds float64 `json:"seconds"`
}

// Rhythm is what the archive says about how the time goes: measured from the
// recordings and their turns, with nothing scored or estimated.
type Rhythm struct {
	Weeks []Week `json:"weeks"` // oldest first, the current week last
	// Clock is minutes of meetings by weekday (Monday first) and hour of the
	// day, over the same weeks.
	Clock [7][24]float64 `json:"clock"`
	// Voices is who spoke over the last days asked for, most speech first.
	Voices []Share `json:"voices"`
}

// Rhythm measures the meetings of the last Weeks weeks, and speech over the
// last days.
func (d *DB) Rhythm(now time.Time, days int) (*Rhythm, error) {
	y, m, day := now.Date()
	monday := time.Date(y, m, day-(int(now.Weekday())+6)%7, 0, 0, 0, 0, now.Location())
	from := monday.AddDate(0, 0, -7*(Weeks-1))
	r := &Rhythm{Weeks: make([]Week, Weeks), Voices: []Share{}}
	for i := range r.Weeks {
		r.Weeks[i].Start = from.AddDate(0, 0, 7*i)
	}

	rows, err := d.sql.Query(`SELECT started, duration, summary FROM recordings
		WHERE deleted IS NULL AND kind = ? AND started >= ?`, Meeting, from.Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var started int64
		var seconds float64
		var blob *string
		if err := rows.Scan(&started, &seconds, &blob); err != nil {
			return nil, err
		}
		at := time.Unix(started, 0).In(now.Location())
		week := &r.Weeks[min(int(at.Sub(from).Hours()/(24*7)), Weeks-1)]
		week.Hours += seconds / 3600
		week.Meetings++
		if s := summaryOf(blob); s != nil {
			week.Decisions += len(s.Decisions)
			week.Commitments += len(s.ActionItems)
		}
		// The meeting's minutes go to the hours it actually spanned.
		for end := at.Add(time.Duration(seconds * float64(time.Second))); at.Before(end); {
			hour := time.Date(at.Year(), at.Month(), at.Day(), at.Hour()+1, 0, 0, 0, at.Location())
			stop := hour
			if end.Before(hour) {
				stop = end
			}
			r.Clock[(int(at.Weekday())+6)%7][at.Hour()] += stop.Sub(at).Minutes()
			at = hour
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	speech, err := d.sql.Query(`SELECT t.speaker, SUM(t.finish - t.start) FROM turns t
		JOIN recordings r ON r.id = t.recording
		WHERE r.deleted IS NULL AND r.kind = ? AND r.started >= ? GROUP BY t.speaker`,
		Meeting, now.AddDate(0, 0, -max(days, 1)).Unix())
	if err != nil {
		return nil, err
	}
	defer speech.Close()
	named := map[string]float64{}
	for speech.Next() {
		var who string
		var seconds float64
		if err := speech.Scan(&who, &seconds); err != nil {
			return nil, err
		}
		if strings.HasPrefix(strings.ToUpper(who), "SPEAKER") {
			who = ""
		}
		named[who] += seconds
	}
	if err := speech.Err(); err != nil {
		return nil, err
	}
	for who, seconds := range named {
		r.Voices = append(r.Voices, Share{who, seconds})
	}
	sort.Slice(r.Voices, func(i, j int) bool { return r.Voices[i].Seconds > r.Voices[j].Seconds })
	return r, nil
}

// summaryOf reads a stored summary; nil when there is none or it is unreadable.
func summaryOf(blob *string) *Summary {
	var s Summary
	if blob == nil || json.Unmarshal([]byte(*blob), &s) != nil {
		return nil
	}
	return &s
}

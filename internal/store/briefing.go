package store

import (
	"sort"
	"strings"
	"time"
	"unicode"
)

// Briefing is the cross-meeting dashboard view.
type Briefing struct {
	Since    time.Time     `json:"since"`
	Meetings []Recording   `json:"meetings"` // in the window, newest first
	Minutes  int           `json:"minutes"`  // spent in them
	Decided  []Said        `json:"decided"`  // what was settled, with where
	Mine     []Outstanding `json:"mine"`     // still open, oldest first
	Overdue  []Outstanding `json:"overdue"`  // open, and their deadline has passed
	Nagging  []Nagging     `json:"nagging"`  // asked in more than one meeting and still open
	Voices   []string      `json:"voices"`   // who was in them
	Skipped  int           `json:"skipped"`  // recordings the listener threw away
	Spared   int           `json:"spared"`   // minutes it did not have to transcribe
}

// Said is one line from a summary, with its source meeting.
type Said struct {
	Recording int64     `json:"recording"`
	Title     string    `json:"title"`
	Started   time.Time `json:"started"`
	Text      string    `json:"text"`
}

// Nagging is a question that keeps coming back.
type Nagging struct {
	Text  string `json:"text"`
	Times int    `json:"times"`
	Said  []Said `json:"said"`
}

// Brief builds a briefing over a recent window.
func (d *DB) Brief(days int) (*Briefing, error) {
	if days <= 0 {
		days = 1
	}
	recent, err := d.Recent(500)
	if err != nil {
		return nil, err
	}
	since := time.Now().AddDate(0, 0, -days)
	b := &Briefing{
		Since: since, Meetings: []Recording{}, Decided: []Said{},
		Mine: []Outstanding{}, Overdue: []Outstanding{}, Nagging: []Nagging{}, Voices: []string{},
	}

	seconds, heard := 0.0, map[string]bool{}
	asked := map[string][]Said{}

	for _, r := range recent {
		inWindow := r.Started.After(since)
		if inWindow {
			b.Meetings = append(b.Meetings, r)
			seconds += r.Duration
			for _, who := range r.Speakers {
				if !strings.HasPrefix(who, "SPEAKER_") {
					heard[who] = true
				}
			}
		}
		if r.Summary == nil {
			continue
		}
		where := Said{Recording: r.ID, Title: r.Title, Started: r.Started}

		if inWindow {
			for _, decision := range r.Summary.Decisions {
				line := where
				line.Text = decision
				b.Decided = append(b.Decided, line)
			}
		}
		// Open questions come from the whole history so repeats outside the
		// window still surface.
		for _, q := range r.Summary.OpenQuestions {
			line := where
			line.Text = q
			key := fingerprint(q)
			asked[key] = append(asked[key], line)
		}
		// Older unresolved commitments can still be overdue.
		for i, a := range r.Summary.ActionItems {
			if a.Done {
				continue
			}
			item := Outstanding{Recording: r.ID, Title: r.Title, Started: r.Started, Index: i, Action: a}
			b.Mine = append(b.Mine, item)
			if overdue(a.Due, r.Started) {
				b.Overdue = append(b.Overdue, item)
			}
		}
	}

	b.Minutes = int(seconds / 60)
	skipped, spared := d.Saved(since)
	b.Skipped, b.Spared = skipped, int(spared/60)
	for who := range heard {
		b.Voices = append(b.Voices, who)
	}
	sort.Strings(b.Voices)

	for _, said := range asked {
		if len(said) < 2 {
			continue
		}
		sort.Slice(said, func(i, j int) bool { return said[i].Started.After(said[j].Started) })
		b.Nagging = append(b.Nagging, Nagging{Text: said[0].Text, Times: len(said), Said: said})
	}
	sort.Slice(b.Nagging, func(i, j int) bool { return b.Nagging[i].Times > b.Nagging[j].Times })

	sort.Slice(b.Mine, func(i, j int) bool { return b.Mine[i].Started.Before(b.Mine[j].Started) })
	sort.Slice(b.Overdue, func(i, j int) bool { return b.Overdue[i].Started.Before(b.Overdue[j].Started) })
	return b, nil
}

// fingerprint buckets similar open questions without embeddings.
func fingerprint(text string) string {
	words := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	kept := make([]string, 0, len(words))
	for _, w := range words {
		if len([]rune(w)) > 3 && !filler[w] {
			kept = append(kept, w)
		}
	}
	sort.Strings(kept)
	if len(kept) > 6 {
		kept = kept[:6]
	}
	return strings.Join(kept, " ")
}

// filler are low-information words ignored when bucketing questions.
var filler = map[string]bool{
	"який": true, "яка": true, "яке": true, "які": true, "треба": true, "потрібно": true,
	"можна": true, "буде": true, "було": true, "щодо": true, "цього": true, "цьому": true,
	"what": true, "which": true, "should": true, "would": true, "could": true, "need": true,
	"about": true, "this": true, "that": true, "there": true, "does": true, "with": true,
	"from": true, "have": true, "will": true, "when": true, "были": true,
}

// overdue reads a human-written deadline conservatively.
func overdue(due string, said time.Time) bool {
	d := strings.ToLower(strings.TrimSpace(due))
	if d == "" {
		return false
	}
	day := 24 * time.Hour
	for phrase, within := range map[string]time.Duration{
		"сьогодні": 0, "today": 0, "зараз": 0, "now": 0, "asap": 0,
		"завтра": day, "tomorrow": day,
		"цього тижня": 7 * day, "this week": 7 * day, "на тижні": 7 * day,
		"next week": 14 * day, "наступного тижня": 14 * day,
	} {
		if strings.Contains(d, phrase) {
			return time.Since(said.Add(within)) > day
		}
	}
	// A written date, if it is one we can read.
	for _, layout := range []string{"2006-01-02", "02.01.2006", "02.01.06", "01/02/2006"} {
		if when, err := time.Parse(layout, strings.TrimSpace(due)); err == nil {
			return time.Now().After(when.Add(day))
		}
	}
	return false
}

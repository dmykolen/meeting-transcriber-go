package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"math"
	"sort"
	"strings"
)

// Keep is the per-person cap on stored voiceprints.
const Keep = 10

// Match is the recognition threshold against enrolled people.
const Match = 0.55

// Rejoin is the stricter threshold for merging two clusters from one recording.
const Rejoin = 0.75

// Person is an enrolled speaker with saved voiceprints.
type Person struct {
	ID          int64       `json:"id"`
	Name        string      `json:"name"`
	Voiceprints [][]float32 `json:"-"`
	Samples     int         `json:"samples"`
	Meetings    int         `json:"meetings"`
	Colour      string      `json:"colour"` // empty means derived from the name
	Sources     []Source    `json:"sources"`
}

// Source is where one voiceprint was taken from.
type Source struct {
	Recording int64   `json:"recording"`
	Speaker   string  `json:"speaker"`
	Title     string  `json:"title"`
	Audio     string  `json:"audio"`
	Start     float64 `json:"start"`
	Finish    float64 `json:"finish"`
}

// People lists enrolled speakers.
func (d *DB) People() ([]Person, error) {
	rows, err := d.sql.Query(`
		SELECT p.id, p.name, p.voiceprints, p.colour, p.sources,
		       (SELECT COUNT(DISTINCT recording) FROM turns WHERE speaker = p.name)
		FROM people p ORDER BY p.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var people []Person
	for rows.Next() {
		var p Person
		var raw, sources string
		if err := rows.Scan(&p.ID, &p.Name, &raw, &p.Colour, &sources, &p.Meetings); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(raw), &p.Voiceprints); err != nil {
			// A corrupt row should cost one person's recognition, not the list.
			p.Voiceprints = nil
		}
		_ = json.Unmarshal([]byte(sources), &p.Sources)
		p.Samples = len(p.Voiceprints)
		people = append(people, p)
	}
	return people, rows.Err()
}

// Remember files another sample of a voice under a name.
func (d *DB) Remember(name string, print []float32, from Source) error {
	if name == "" || len(print) == 0 {
		return errors.New("nothing to remember")
	}
	var raw, sources string
	err := d.sql.QueryRow(`SELECT voiceprints, sources FROM people WHERE name = ?`, name).
		Scan(&raw, &sources)

	var kept [][]float32
	var came []Source
	switch {
	case errors.Is(err, sql.ErrNoRows):
	case err != nil:
		return err
	default:
		_ = json.Unmarshal([]byte(raw), &kept)
		_ = json.Unmarshal([]byte(sources), &came)
	}
	kept = append(kept, print)
	// An older row may hold fewer sources than prints; pad so the two arrays
	// stay index-for-index and an unknown source reads as an empty one.
	for len(came) < len(kept)-1 {
		came = append(came, Source{})
	}
	came = append(came, from)

	if drop := crowded(kept); drop >= 0 {
		kept = append(kept[:drop:drop], kept[drop+1:]...)
		came = append(came[:drop:drop], came[drop+1:]...)
	}

	prints, err := json.Marshal(kept)
	if err != nil {
		return err
	}
	where, err := json.Marshal(came)
	if err != nil {
		return err
	}
	_, err = d.sql.Exec(
		`INSERT INTO people (name, voiceprints, sources) VALUES (?, ?, ?)
		 ON CONFLICT(name) DO UPDATE SET voiceprints = excluded.voiceprints,
		                                 sources = excluded.sources`,
		name, string(prints), string(where))
	return err
}

// PaintPerson sets or clears a person's explicit colour.
func (d *DB) PaintPerson(name, colour string) error {
	_, err := d.sql.Exec(`UPDATE people SET colour = ? WHERE name = ?`,
		strings.TrimSpace(colour), name)
	return err
}

// Sample fills in the best playback span for a saved source.
func (d *DB) Sample(from Source) (Source, bool) {
	if from.Recording == 0 {
		return from, false
	}
	err := d.sql.QueryRow(`
		SELECT r.title, r.audio, t.start, t.finish
		FROM turns t JOIN recordings r ON r.id = t.recording
		WHERE t.recording = ? AND t.speaker = ?
		ORDER BY (t.finish - t.start) DESC LIMIT 1`,
		from.Recording, from.Speaker).
		Scan(&from.Title, &from.Audio, &from.Start, &from.Finish)
	if err != nil {
		// The speaker was renamed since, so the label no longer matches a turn.
		// The meeting is still worth naming even without a moment inside it.
		if e := d.sql.QueryRow(`SELECT title, audio FROM recordings WHERE id = ?`,
			from.Recording).Scan(&from.Title, &from.Audio); e != nil {
			return from, false
		}
	}
	return from, true
}

// SaveVoices keeps this recording's current speaker voiceprints.
func (d *DB) SaveVoices(recording int64, prints map[string][]float32) error {
	// Even an empty map must be written; retranscription can renumber clusters,
	// so stale labels would poison later renames.
	if prints == nil {
		prints = map[string][]float32{}
	}
	encoded, err := json.Marshal(prints)
	if err != nil {
		return err
	}
	_, err = d.sql.Exec(`UPDATE recordings SET voices = ? WHERE id = ?`, string(encoded), recording)
	return err
}

// VoiceIn returns one speaker's voiceprint in one recording.
func (d *DB) VoiceIn(recording int64, speaker string) []float32 {
	var raw sql.NullString
	if err := d.sql.QueryRow(`SELECT voices FROM recordings WHERE id = ?`, recording).Scan(&raw); err != nil {
		return nil
	}
	var prints map[string][]float32
	if json.Unmarshal([]byte(raw.String), &prints) != nil {
		return nil
	}
	return prints[speaker]
}

// Forget removes a person entirely.
func (d *DB) Forget(name string) error {
	_, err := d.sql.Exec(`DELETE FROM people WHERE name = ?`, name)
	return err
}

// crowded returns the least useful sample to drop, or -1 when there is room.
func crowded(samples [][]float32) int {
	if len(samples) <= Keep {
		return -1
	}
	worst, twin := -2.0, 0
	for i := range samples {
		for j := range samples {
			if i == j {
				continue
			}
			if c := Cosine(samples[i], samples[j]); c > worst {
				worst, twin = c, min(i, j)
			}
		}
	}
	return twin
}

// Recognise pairs one recording's voiceprints with enrolled people.
func Recognise(prints map[string][]float32, people []Person) map[string]string {
	type pair struct {
		label string
		who   int
		score float64
	}
	var pairs []pair
	for label, print := range prints {
		for i, person := range people {
			best := -1.0
			for _, sample := range person.Voiceprints {
				if c := Cosine(print, sample); c > best {
					best = c
				}
			}
			if best >= Match {
				pairs = append(pairs, pair{label, i, best})
			}
		}
	}
	sort.Slice(pairs, func(a, b int) bool { return pairs[a].score > pairs[b].score })

	names, takenLabel, takenPerson := map[string]string{}, map[string]bool{}, map[int]bool{}
	for _, p := range pairs {
		if takenLabel[p.label] || takenPerson[p.who] {
			continue
		}
		takenLabel[p.label], takenPerson[p.who] = true, true
		names[p.label] = people[p.who].Name
	}
	return names
}

// Cosine is the similarity of two voiceprints in -1..1.
func Cosine(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return -1
	}
	var dot, na, nb float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		na += float64(a[i]) * float64(a[i])
		nb += float64(b[i]) * float64(b[i])
	}
	if na == 0 || nb == 0 {
		return -1
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

// RawVoices returns a recording's stored voiceprints as written.
func (d *DB) RawVoices(recording int64) string {
	var raw sql.NullString
	_ = d.sql.QueryRow(`SELECT voices FROM recordings WHERE id = ?`, recording).Scan(&raw)
	return raw.String
}

// Trace backfills source metadata for older saved voiceprints.
func (d *DB) Trace() error {
	people, err := d.People()
	if err != nil {
		return err
	}
	missing := false
	for _, p := range people {
		for i := range p.Voiceprints {
			if i >= len(p.Sources) || p.Sources[i].Recording == 0 {
				missing = true
			}
		}
	}
	if !missing {
		return nil
	}

	rows, err := d.sql.Query(`SELECT id, voices FROM recordings WHERE voices IS NOT NULL`)
	if err != nil {
		return err
	}
	defer rows.Close()

	type heard struct {
		recording int64
		speaker   string
		print     []float32
	}
	var all []heard
	for rows.Next() {
		var id int64
		var raw sql.NullString
		if err := rows.Scan(&id, &raw); err != nil {
			return err
		}
		var prints map[string][]float32
		if json.Unmarshal([]byte(raw.String), &prints) != nil {
			continue
		}
		for speaker, print := range prints {
			all = append(all, heard{id, speaker, print})
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, p := range people {
		found := make([]Source, len(p.Voiceprints))
		copy(found, p.Sources)
		for i, print := range p.Voiceprints {
			if found[i].Recording != 0 {
				continue
			}
			for _, h := range all {
				if same(print, h.print) {
					found[i] = Source{Recording: h.recording, Speaker: h.speaker}
					break
				}
			}
		}
		encoded, err := json.Marshal(found)
		if err != nil {
			continue
		}
		if _, err := d.sql.Exec(`UPDATE people SET sources = ? WHERE id = ?`,
			string(encoded), p.ID); err != nil {
			return err
		}
	}
	return nil
}

// same is exact equality, not similarity.
func same(a, b []float32) bool {
	if len(a) != len(b) || len(a) == 0 {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Appearances lists the groups one person has been heard in.
func (d *DB) Appearances(name string) ([]Group, error) {
	rows, err := d.sql.Query(`
		SELECT COALESCE(g.id, 0), COALESCE(g.name, ''), COALESCE(g.colour, ''),
		       COUNT(DISTINCT r.id)
		FROM turns t
		JOIN recordings r ON r.id = t.recording AND r.deleted IS NULL
		LEFT JOIN groups g ON g.id = r.folder
		WHERE t.speaker = ?
		GROUP BY g.id ORDER BY COUNT(DISTINCT r.id) DESC, g.name`, name)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Group{}
	for rows.Next() {
		var g Group
		if err := rows.Scan(&g.ID, &g.Name, &g.Colour, &g.Count); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// Same rejoins labels from one recording that likely belong to one person.
func Same(prints map[string][]float32, people []Person) map[string]string {
	best := map[string]struct {
		who   string
		score float64
	}{}
	for label, print := range prints {
		for _, person := range people {
			for _, sample := range person.Voiceprints {
				c := Cosine(print, sample)
				if c >= Match && c > best[label].score {
					best[label] = struct {
						who   string
						score float64
					}{person.Name, c}
				}
			}
		}
	}
	// One label per person keeps the name: the rest are told to become it.
	keeper := map[string]string{}
	for label, m := range best {
		if m.who == "" {
			continue
		}
		if held, taken := keeper[m.who]; !taken || best[held].score < m.score {
			keeper[m.who] = label
		}
	}
	same := map[string]string{}
	for label, m := range best {
		to := keeper[m.who]
		if m.who == "" || to == label {
			continue
		}
		// Logged either way. When a meeting comes back with the wrong number of
		// people this is the line that says whether the clusterer or this is to
		// blame, and it is nearly impossible to work out afterwards without it.
		alike := Cosine(prints[label], prints[to])
		slog.Info("two clusters resemble one person",
			"label", label, "into", to, "who", m.who,
			"alike", alike, "joined", alike >= Rejoin)
		if alike >= Rejoin {
			same[label] = to
		}
	}
	return same
}

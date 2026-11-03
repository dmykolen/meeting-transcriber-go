package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"slices"
	"sort"
	"strings"
)

// Keep is the per-person cap on stored voiceprints. The app fills free slots
// with its guesses but never makes room for one; the owner's samples are only
// ever replaced by the owner.
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
	Recording int64  `json:"recording"`
	Speaker   string `json:"speaker"`
	// The owner named this voice or said yes to it. Only these samples
	// recognise anybody: the app's own guesses filed themselves over good
	// samples and named one colleague after another (recording 322).
	Confirmed bool    `json:"confirmed"`
	Title     string  `json:"title"`
	Audio     string  `json:"audio"`
	Start     float64 `json:"start"`
	Finish    float64 `json:"finish"`
}

// People lists enrolled speakers with the samples that still stand. One from a
// recording in the bin, or deleted since, neither recognises nor shows; it
// keeps its slot, so restoring the recording gives it back as it was.
func (d *DB) People() ([]Person, error) {
	people, err := d.everyone()
	if err != nil {
		return nil, err
	}
	rows, err := d.sql.Query(`SELECT id FROM recordings WHERE deleted IS NULL`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	live := map[int64]bool{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		live[id] = true
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i, p := range people {
		var prints [][]float32
		came := []Source{}
		for j, from := range p.Sources {
			// A sample older than sources says nothing of where it is from.
			if from.Recording == 0 || live[from.Recording] {
				prints, came = append(prints, p.Voiceprints[j]), append(came, from)
			}
		}
		people[i].Voiceprints, people[i].Sources, people[i].Samples = prints, came, len(prints)
	}
	return people, nil
}

// everyone reads every person with every stored sample.
func (d *DB) everyone() ([]Person, error) {
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
		p.Voiceprints, p.Sources = decode(raw, sources)
		p.Samples = len(p.Voiceprints)
		people = append(people, p)
	}
	return people, rows.Err()
}

// decode reads stored samples, sources index for index with prints. An older
// row may hold fewer sources than prints; an unknown source reads as an empty
// one.
func decode(raw, sources string) ([][]float32, []Source) {
	var prints [][]float32
	if json.Unmarshal([]byte(raw), &prints) != nil {
		// A corrupt row should cost one person's recognition, not the list.
		prints = nil
	}
	var came []Source
	_ = json.Unmarshal([]byte(sources), &came)
	for len(came) < len(prints) {
		came = append(came, Source{})
	}
	return prints, came[:len(prints)]
}

// Remember files a sample of a voice under a name, as the owner's word when
// from.Confirmed and as the app's guess otherwise.
func (d *DB) Remember(name string, print []float32, from Source) error {
	if name == "" || len(print) == 0 {
		return errors.New("nothing to remember")
	}
	tx, err := d.sql.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := remember(tx, name, print, from); err != nil {
		return err
	}
	return tx.Commit()
}

// remember keeps one sample per person per recording.
func remember(tx *sql.Tx, name string, print []float32, from Source) error {
	prints, came, err := samples(tx, name)
	if err != nil {
		return err
	}
	same := slices.IndexFunc(came, func(s Source) bool { return from.Recording != 0 && s.Recording == from.Recording })
	switch {
	case same >= 0 && came[same].Confirmed && !from.Confirmed:
		return nil // a guess never overrules what the owner said
	case same >= 0:
		prints[same], came[same] = print, from
	case len(prints) < Keep:
		prints, came = append(prints, print), append(came, from)
	case !from.Confirmed:
		return nil
	default:
		guess := slices.IndexFunc(came, func(s Source) bool { return !s.Confirmed })
		if guess < 0 {
			slog.Info("voice not filed: ten confirmed samples already", "name", name, "recording", from.Recording)
			return nil
		}
		prints = append(slices.Delete(prints, guess, guess+1), print)
		came = append(slices.Delete(came, guess, guess+1), from)
	}
	return put(tx, name, prints, came)
}

// samples reads one person's stored samples; nobody yet is none.
func samples(tx *sql.Tx, name string) ([][]float32, []Source, error) {
	var raw, sources string
	err := tx.QueryRow(`SELECT voiceprints, sources FROM people WHERE name = ?`, name).Scan(&raw, &sources)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	prints, came := decode(raw, sources)
	return prints, came, nil
}

// put writes one person's samples, enrolling the name if it is new.
func put(tx *sql.Tx, name string, prints [][]float32, came []Source) error {
	encoded, err := json.Marshal(append([][]float32{}, prints...))
	if err != nil {
		return err
	}
	where, err := json.Marshal(append([]Source{}, came...))
	if err != nil {
		return err
	}
	_, err = tx.Exec(
		`INSERT INTO people (name, voiceprints, sources) VALUES (?, ?, ?)
		 ON CONFLICT(name) DO UPDATE SET voiceprints = excluded.voiceprints,
		                                 sources = excluded.sources`,
		name, string(encoded), string(where))
	return err
}

// unlearn takes back the samples one recording gave: one person's, or
// everybody's when name is empty.
func unlearn(tx *sql.Tx, recording int64, name string) error {
	rows, err := tx.Query(`SELECT name, voiceprints, sources FROM people WHERE ? IN ('', name)`, name)
	if err != nil {
		return err
	}
	type person struct {
		name   string
		prints [][]float32
		came   []Source
	}
	var changed []person
	for rows.Next() {
		var who, raw, sources string
		if err := rows.Scan(&who, &raw, &sources); err != nil {
			rows.Close()
			return err
		}
		prints, came := decode(raw, sources)
		p := person{name: who}
		for i, from := range came {
			if from.Recording != recording {
				p.prints, p.came = append(p.prints, prints[i]), append(p.came, from)
			}
		}
		if len(p.came) < len(came) {
			changed = append(changed, p)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, p := range changed {
		if err := put(tx, p.name, p.prints, p.came); err != nil {
			return err
		}
	}
	return nil
}

// Confirm is the owner's yes to a name in one recording: the guess is settled
// and the voice's sample counts from now on.
func (d *DB) Confirm(id int64, name string) error {
	print := d.VoiceIn(id, name)
	tx, err := d.sql.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`UPDATE recordings SET guesses = json_remove(guesses, '$.' || ?) WHERE id = ? AND guesses IS NOT NULL`,
		name, id); err != nil {
		return err
	}
	prints, came, err := samples(tx, name)
	if err != nil {
		return err
	}
	switch at := slices.IndexFunc(came, func(s Source) bool { return s.Recording == id }); {
	case len(print) > 0:
		err = remember(tx, name, print, Source{Recording: id, Speaker: name, Confirmed: true})
	case at >= 0:
		// The label was renamed since, but the sample still came from here.
		came[at].Confirmed = true
		err = put(tx, name, prints, came)
	}
	if err != nil {
		return err
	}
	return tx.Commit()
}

// Reject is the owner's no to a name in one recording: the voice becomes an
// unnamed speaker again, and the name gives back the sample it took from here.
func (d *DB) Reject(id int64, name string) error {
	taken, err := d.speakers(id)
	if err != nil {
		return err
	}
	var voices map[string]json.RawMessage
	_ = json.Unmarshal([]byte(d.RawVoices(id)), &voices)
	for n := 0; ; n++ {
		label := fmt.Sprintf("SPEAKER_%02d", n)
		if _, held := voices[label]; !held && !slices.Contains(taken, label) {
			return d.Rename(id, name, label)
		}
	}
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

// SaveVoices keeps this recording's current speaker voiceprints and the names
// the app guessed among them.
func (d *DB) SaveVoices(recording int64, prints map[string][]float32, guesses map[string]float64) error {
	// Even empty maps must be written; retranscription can renumber clusters,
	// so stale labels would poison later renames.
	if prints == nil {
		prints = map[string][]float32{}
	}
	if guesses == nil {
		guesses = map[string]float64{}
	}
	encoded, err := json.Marshal(prints)
	if err != nil {
		return err
	}
	guessed, err := json.Marshal(guesses)
	if err != nil {
		return err
	}
	_, err = d.sql.Exec(`UPDATE recordings SET voices = ?, guesses = ? WHERE id = ?`,
		string(encoded), string(guessed), recording)
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

// Guess is a name the app gave a voice by itself, and how alike the voice is to
// that person's confirmed samples (cosine, -1..1).
type Guess struct {
	Name  string
	Score float64
}

// Recognise pairs one recording's voiceprints with enrolled people, against
// the samples the owner confirmed only.
func Recognise(prints map[string][]float32, people []Person) map[string]Guess {
	type pair struct {
		label string
		who   int
		score float64
	}
	var pairs []pair
	for label, print := range prints {
		for i, person := range people {
			best := -1.0
			for _, sample := range person.sure() {
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

	names, takenLabel, takenPerson := map[string]Guess{}, map[string]bool{}, map[int]bool{}
	for _, p := range pairs {
		if takenLabel[p.label] || takenPerson[p.who] {
			continue
		}
		takenLabel[p.label], takenPerson[p.who] = true, true
		names[p.label] = Guess{people[p.who].Name, p.score}
	}
	return names
}

// sure is a person's confirmed samples.
func (p Person) sure() [][]float32 {
	var out [][]float32
	for i, print := range p.Voiceprints {
		if i < len(p.Sources) && p.Sources[i].Confirmed {
			out = append(out, print)
		}
	}
	return out
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
	people, err := d.everyone()
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
			for _, sample := range person.sure() {
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

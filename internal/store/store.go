// Package store keeps everything the app remembers in one SQLite file.
package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	_ "modernc.org/sqlite" // pure Go: no second C toolchain for a database
	"strings"
	"time"
)

// When says which recordings are worth a model call.
type When string

const (
	Always   When = "always"   // every recording, notes included
	Meetings When = "meetings" // only the ones somebody else was in
	Never    When = "never"    // a transcriber and nothing more
)

// Kind is what a recording turned out to be.
type Kind string

const (
	Meeting Kind = "meeting" // somebody else was talking: diarized
	Note    Kind = "note"    // thinking aloud
)

// Status is where a recording is in the pipeline.
type Status string

const (
	Queued       Status = "queued"
	Transcribing Status = "transcribing"
	Summarising  Status = "summarising"
	Done         Status = "done"
	Failed       Status = "failed"
)

// Recording is one meeting or note.
type Recording struct {
	ID       int64     `json:"id"`
	Kind     Kind      `json:"kind"`
	Title    string    `json:"title"` // from the summary; the file name until then
	Audio    string    `json:"audio"` // file name under recordings/, empty once deleted
	Group    int64     `json:"group"` // which group it is filed under, 0 for none
	Started  time.Time `json:"started"`
	Duration float64   `json:"duration"`
	Language string    `json:"language"`
	Status   Status    `json:"status"`
	Progress float64   `json:"progress"`
	Problem  string    `json:"problem,omitempty"`
	Summary  *Summary  `json:"summary,omitempty"`
	Speakers []string  `json:"speakers,omitempty"`
	Turns    int       `json:"turns"`
	Note     string    `json:"note,omitempty"`
}

// Summary is the stored JSON summary payload.
type Summary struct {
	Title         string    `json:"title"`
	Overview      string    `json:"overview"`
	Chapters      []Chapter `json:"chapters"`
	Topics        []string  `json:"topics"`
	Decisions     []string  `json:"decisions"`
	ActionItems   []Action  `json:"action_items"`
	OpenQuestions []string  `json:"open_questions"`
}

type Chapter struct {
	Start   float64 `json:"start"`
	Title   string  `json:"title"`
	Summary string  `json:"summary"`
}

type Action struct {
	Task  string `json:"task"`
	Owner string `json:"owner"`
	Due   string `json:"due"`
	Done  bool   `json:"done"`
}

// Turn is one row of a transcript.
type Turn struct {
	Start   float64 `json:"start"`
	End     float64 `json:"end"`
	Speaker string  `json:"speaker,omitempty"`
	Text    string  `json:"text"`
}

// DB is the SQLite handle.
type DB struct{ sql *sql.DB }

// Open creates the file and schema if needed.
func Open(path string) (*DB, error) {
	// WAL so that the UI can read a transcript while a recording is being
	// written, which is the normal case and not an edge one.
	handle, err := sql.Open("sqlite", path+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)")
	if err != nil {
		return nil, err
	}
	if _, err := handle.Exec(schema); err != nil {
		handle.Close()
		return nil, fmt.Errorf("schema: %w", err)
	}
	// SQLite has no ADD COLUMN IF NOT EXISTS, and a database made before this
	// column existed will not get it from CREATE TABLE IF NOT EXISTS. Failing
	// here is the ordinary outcome — it means the column is already there.
	for _, column := range []string{
		`voices TEXT`, `deleted INTEGER`, `folder INTEGER`,
		`titled INTEGER NOT NULL DEFAULT 0`,
	} {
		_, _ = handle.Exec(`ALTER TABLE recordings ADD COLUMN ` + column)
	}
	for table, columns := range map[string][]string{
		"groups": {`colour TEXT NOT NULL DEFAULT ''`, `state TEXT NOT NULL DEFAULT ''`},
		"people": {`colour TEXT NOT NULL DEFAULT ''`, `sources TEXT NOT NULL DEFAULT '[]'`},
	} {
		for _, column := range columns {
			_, _ = handle.Exec(`ALTER TABLE ` + table + ` ADD COLUMN ` + column)
		}
	}
	// A recording whose file a later one overwrote must not offer to play it.
	// The preroll used to reach back into a meeting that had already been
	// filed, and since the file name carries the start time to the minute, the
	// second recording was written over the first. Both fixes are in
	// internal/listen; this is for the rows already in the database.
	if n, err := handle.Exec(`
		UPDATE recordings SET audio = ''
		WHERE audio <> '' AND EXISTS (
			SELECT 1 FROM recordings later
			WHERE later.audio = recordings.audio AND later.id > recordings.id)`); err == nil {
		if rows, _ := n.RowsAffected(); rows > 0 {
			slog.Warn("recordings whose audio a later one overwrote; the sound is gone, the transcript is not",
				"recordings", rows)
		}
	}

	return &DB{sql: handle}, nil
}

func (d *DB) Close() error { return d.sql.Close() }

const schema = `
CREATE TABLE IF NOT EXISTS knowledge_vectors(key TEXT PRIMARY KEY,vector BLOB NOT NULL);
CREATE TABLE IF NOT EXISTS meta (key TEXT PRIMARY KEY, value TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS notes (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 recording INTEGER REFERENCES recordings(id) ON DELETE CASCADE,
 project INTEGER REFERENCES groups(id) ON DELETE CASCADE,
 text TEXT NOT NULL DEFAULT '', colour TEXT NOT NULL DEFAULT 'lime', at REAL NOT NULL DEFAULT 0,
 CHECK ((recording IS NOT NULL) != (project IS NOT NULL))
);

CREATE TABLE IF NOT EXISTS recordings (
  id        INTEGER PRIMARY KEY AUTOINCREMENT,
  kind      TEXT    NOT NULL,
  title     TEXT    NOT NULL DEFAULT '',
  audio     TEXT    NOT NULL,
  started   INTEGER NOT NULL,
  duration  REAL    NOT NULL DEFAULT 0,
  language  TEXT    NOT NULL DEFAULT '',
  status    TEXT    NOT NULL DEFAULT 'queued',
  progress  REAL    NOT NULL DEFAULT 0,
  problem   TEXT    NOT NULL DEFAULT '',
  summary   TEXT,
  -- Set when a person typed the title. The model may propose one, but it never
  -- overwrites one somebody chose.
  titled    INTEGER NOT NULL DEFAULT 0,
  note      TEXT    NOT NULL DEFAULT '',
  -- One voiceprint per speaker label, so that naming somebody after the fact
  -- still teaches the app what they sound like.
  voices    TEXT,
  -- When it was moved to the bin, or null while it is live.
  deleted   INTEGER,
  -- Which group it is filed under, or null.
  folder    INTEGER REFERENCES groups(id) ON DELETE SET NULL
);
CREATE INDEX IF NOT EXISTS recordings_started ON recordings(started DESC);

CREATE TABLE IF NOT EXISTS turns (
  recording INTEGER NOT NULL REFERENCES recordings(id) ON DELETE CASCADE,
  seq       INTEGER NOT NULL,
  start     REAL    NOT NULL,
  finish    REAL    NOT NULL,
  speaker   TEXT    NOT NULL DEFAULT '',
  text      TEXT    NOT NULL,
  PRIMARY KEY (recording, seq)
);

-- Full text over the transcript. Populated alongside turns rather than by a
-- trigger, so that a failed insert cannot leave the index describing rows that
-- are not there.
CREATE VIRTUAL TABLE IF NOT EXISTS transcript USING fts5(
  text, recording UNINDEXED, seq UNINDEXED, start UNINDEXED, speaker UNINDEXED,
  tokenize = 'unicode61 remove_diacritics 2'
);

-- Passages: the transcript in stretches worth finding, with the vector that
-- makes "what did we decide about access" find a passage that never says
-- "access". Without a key the vector is null and the same rows still serve
-- keyword search.
CREATE TABLE IF NOT EXISTS passages (
  recording INTEGER NOT NULL REFERENCES recordings(id) ON DELETE CASCADE,
  seq       INTEGER NOT NULL,
  start     REAL    NOT NULL,
  speaker   TEXT    NOT NULL DEFAULT '',
  text      TEXT    NOT NULL,
  vector    BLOB,
  PRIMARY KEY (recording, seq)
);

-- What the listener threw away rather than transcribing. Kept as counts, not
-- recordings: the point is to be able to say the setting is working, not to
-- keep the thing it discarded.
CREATE TABLE IF NOT EXISTS discarded (
  at      INTEGER NOT NULL,
  seconds REAL    NOT NULL,
  why     TEXT    NOT NULL
);
CREATE INDEX IF NOT EXISTS discarded_at ON discarded(at DESC);

CREATE TABLE IF NOT EXISTS groups (
  id     INTEGER PRIMARY KEY AUTOINCREMENT,
  name   TEXT    NOT NULL UNIQUE,
  -- Empty means the colour is derived from the name, so every project has one
  -- without anybody having to choose. A value here is somebody overruling that.
  colour TEXT    NOT NULL DEFAULT '',
  -- The living document the model keeps for this project; empty until it has
  -- run. See kept.go.
  state  TEXT    NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS people (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  name        TEXT    NOT NULL UNIQUE,
  voiceprints TEXT    NOT NULL DEFAULT '[]',
  colour      TEXT    NOT NULL DEFAULT '',
  -- Where each voiceprint came from, parallel to voiceprints: one
  -- {recording, speaker} per sample, so a saved sample can be played back
  -- instead of being an unexaminable vector.
  sources     TEXT    NOT NULL DEFAULT '[]'
);
`

// Add records a new recording and returns it with its id.
func (d *DB) Add(r Recording) (Recording, error) {
	res, err := d.sql.Exec(
		`INSERT INTO recordings (kind, title, audio, started, duration, status) VALUES (?, ?, ?, ?, ?, ?)`,
		r.Kind, r.Title, r.Audio, r.Started.Unix(), r.Duration, Queued)
	if err != nil {
		return r, err
	}
	r.ID, err = res.LastInsertId()
	r.Status = Queued
	return r, err
}

// Progress updates one recording's pipeline state.
func (d *DB) Progress(id int64, status Status, fraction float64) error {
	_, err := d.sql.Exec(`UPDATE recordings SET status = ?, progress = ? WHERE id = ?`, status, fraction, id)
	return err
}

// Fail records why a recording stopped.
func (d *DB) Fail(id int64, cause error) error {
	_, err := d.sql.Exec(`UPDATE recordings SET status = ?, problem = ? WHERE id = ?`, Failed, cause.Error(), id)
	return err
}

// SaveTranscript writes turns and the FTS copy in one transaction.
func (d *DB) SaveTranscript(id int64, language string, duration float64, turns []Turn) error {
	tx, err := d.sql.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM turns WHERE recording = ?`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM transcript WHERE recording = ?`, id); err != nil {
		return err
	}
	rows, err := tx.Prepare(`INSERT INTO turns (recording, seq, start, finish, speaker, text) VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer rows.Close()
	index, err := tx.Prepare(`INSERT INTO transcript (text, recording, seq, start, speaker) VALUES (?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer index.Close()

	for i, t := range turns {
		if _, err := rows.Exec(id, i, t.Start, t.End, t.Speaker, t.Text); err != nil {
			return err
		}
		if _, err := index.Exec(t.Text, id, i, t.Start, t.Speaker); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(`UPDATE recordings SET language = ?, duration = ? WHERE id = ?`, language, duration, id); err != nil {
		return err
	}
	return tx.Commit()
}

// SaveSummary stores the summary and, when allowed, updates the title from it.
func (d *DB) SaveSummary(id int64, s *Summary) error {
	blob, err := json.Marshal(s)
	if err != nil {
		return err
	}
	title := strings.TrimSpace(s.Title)
	if title == "" {
		_, err = d.sql.Exec(`UPDATE recordings SET summary = ? WHERE id = ?`, string(blob), id)
		return err
	}
	// Summarising again is a thing the user can ask for at any time, and it
	// must not undo a title they typed.
	_, err = d.sql.Exec(
		`UPDATE recordings SET summary = ?, title = ? WHERE id = ? AND titled = 0`,
		string(blob), title, id)
	if err == nil {
		_, err = d.sql.Exec(`UPDATE recordings SET summary = ? WHERE id = ? AND titled = 1`, string(blob), id)
	}
	return err
}

// Retitle stores a user-chosen title.
func (d *DB) Retitle(id int64, title string) error {
	title = strings.TrimSpace(title)
	if title == "" {
		return errors.New("a title is needed")
	}
	_, err := d.sql.Exec(`UPDATE recordings SET title = ?, titled = 1 WHERE id = ?`, title, id)
	return err
}

// SaveNote stores a recording note.
func (d *DB) SaveNote(id int64, note string) error {
	_, err := d.sql.Exec(`UPDATE recordings SET note = ? WHERE id = ?`, note, id)
	return err
}

// Rename changes a speaker label everywhere in one recording.
func (d *DB) Rename(id int64, from, to string) error {
	tx, err := d.sql.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`UPDATE turns SET speaker = ? WHERE recording = ? AND speaker = ?`, to, id, from); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE transcript SET speaker = ? WHERE recording = ? AND speaker = ?`, to, id, from); err != nil {
		return err
	}
	// The voiceprint moves with the label, so a second rename in the same
	// meeting still finds it.
	if _, err := tx.Exec(
		`UPDATE recordings SET voices = json_remove(json_set(voices, '$.' || ?, json_extract(voices, '$.' || ?)), '$.' || ?)
		 WHERE id = ? AND voices IS NOT NULL AND json_extract(voices, '$.' || ?) IS NOT NULL`,
		to, from, from, id, from); err != nil {
		return err
	}
	return tx.Commit()
}

// Redate moves a recording in time.
func (d *DB) Redate(id int64, when time.Time) error {
	_, err := d.sql.Exec(`UPDATE recordings SET started = ? WHERE id = ?`, when, id)
	return err
}

// Dropped records that a recording's audio has been deleted.
func (d *DB) Dropped(id int64) error {
	_, err := d.sql.Exec(`UPDATE recordings SET audio = '' WHERE id = ?`, id)
	return err
}

// Audio is the file name of a recording, empty after deletion.
func (d *DB) Audio(id int64) string {
	var name string
	_ = d.sql.QueryRow(`SELECT audio FROM recordings WHERE id = ?`, id).Scan(&name)
	return name
}

// Recent lists recordings, newest first, without transcripts.
func (d *DB) Recent(limit int) ([]Recording, error) {
	return d.list(`WHERE r.deleted IS NULL`, limit)
}

// In lists one group's recordings.
func (d *DB) In(group int64, limit int) ([]Recording, error) {
	return d.list(`WHERE r.deleted IS NULL AND r.folder = ?`, limit, group)
}

// list is the common listing query shape.
func (d *DB) list(where string, limit int, args ...any) ([]Recording, error) {
	// A negative limit explicitly asks SQLite for all rows. Zero keeps the default.
	if limit == 0 {
		limit = 100
	}
	rows, err := d.sql.Query(`
		SELECT r.id, r.kind, r.title, r.audio, r.started, r.duration, r.language,
		       r.status, r.progress, r.problem, r.summary, r.note, r.folder,
		       (SELECT COUNT(*) FROM turns t WHERE t.recording = r.id)
		FROM recordings r `+where+` ORDER BY r.started DESC LIMIT ?`,
		append(args, limit)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scan(rows)
}

// Get returns one recording with its speakers filled in.
func (d *DB) Get(id int64) (*Recording, error) {
	rows, err := d.sql.Query(`
		SELECT r.id, r.kind, r.title, r.audio, r.started, r.duration, r.language,
		       r.status, r.progress, r.problem, r.summary, r.note, r.folder,
		       (SELECT COUNT(*) FROM turns t WHERE t.recording = r.id)
		FROM recordings r WHERE r.id = ?`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	found, err := scan(rows)
	if err != nil {
		return nil, err
	}
	if len(found) == 0 {
		return nil, sql.ErrNoRows
	}
	r := found[0]
	if r.Speakers, err = d.speakers(id); err != nil {
		return nil, err
	}
	return &r, nil
}

// Turns returns one recording's transcript in order.
func (d *DB) Turns(id int64) ([]Turn, error) {
	rows, err := d.sql.Query(`SELECT start, finish, speaker, text FROM turns WHERE recording = ? ORDER BY seq`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var turns []Turn
	for rows.Next() {
		var t Turn
		if err := rows.Scan(&t.Start, &t.End, &t.Speaker, &t.Text); err != nil {
			return nil, err
		}
		turns = append(turns, t)
	}
	return turns, rows.Err()
}

func (d *DB) speakers(id int64) ([]string, error) {
	rows, err := d.sql.Query(`SELECT DISTINCT speaker FROM turns WHERE recording = ? AND speaker <> '' ORDER BY speaker`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var who []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		who = append(who, s)
	}
	return who, rows.Err()
}

// Delete forgets a recording and everything attached to it.
func (d *DB) Delete(id int64) (string, error) {
	var audio string
	if err := d.sql.QueryRow(`SELECT audio FROM recordings WHERE id = ?`, id).Scan(&audio); err != nil {
		return "", err
	}
	tx, err := d.sql.Begin()
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM transcript WHERE recording = ?`, id); err != nil {
		return "", err
	}
	if _, err := tx.Exec(`DELETE FROM recordings WHERE id = ?`, id); err != nil {
		return "", err
	}
	return audio, tx.Commit()
}

func scan(rows *sql.Rows) ([]Recording, error) {
	var out []Recording
	for rows.Next() {
		var (
			r       Recording
			started int64
			summary sql.NullString
			folder  sql.NullInt64
		)
		if err := rows.Scan(&r.ID, &r.Kind, &r.Title, &r.Audio, &started, &r.Duration,
			&r.Language, &r.Status, &r.Progress, &r.Problem, &summary, &r.Note,
			&folder, &r.Turns); err != nil {
			return nil, err
		}
		r.Started = time.Unix(started, 0)
		r.Group = folder.Int64
		if summary.Valid && summary.String != "" {
			var s Summary
			if err := json.Unmarshal([]byte(summary.String), &s); err == nil {
				r.Summary = &s
			}
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ErrNotFound is returned for a missing recording.
var ErrNotFound = errors.New("no such recording")

// TickAction marks one action item done or undone.
func (d *DB) TickAction(id int64, index int, done bool) error {
	path := fmt.Sprintf("$.action_items[%d]", index)
	if index < 0 {
		return errors.New("invalid action index")
	}
	result, err := d.sql.Exec(`UPDATE recordings SET summary=json_set(summary,?,json(?)) WHERE id=? AND deleted IS NULL AND json_type(summary,?)='object'`, path+".done", fmt.Sprint(done), id, path)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n != 1 {
		return fmt.Errorf("recording %d has no action item %d", id, index)
	}
	return nil
}

// Outstanding is one action item from one meeting.
type Outstanding struct {
	Recording int64     `json:"recording"`
	Title     string    `json:"title"`
	Started   time.Time `json:"started"`
	Index     int       `json:"index"`
	Action
}

func (d *DB) Actions(includeDone bool) ([]Outstanding, error) {
	recent, err := d.Recent(500)
	if err != nil {
		return nil, err
	}
	out := []Outstanding{}
	for _, r := range recent {
		if r.Summary == nil {
			continue
		}
		for i, a := range r.Summary.ActionItems {
			if a.Done && !includeDone {
				continue
			}
			out = append(out, Outstanding{
				Recording: r.ID, Title: r.Title, Started: r.Started, Index: i, Action: a,
			})
		}
	}
	return out, nil
}

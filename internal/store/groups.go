package store

import (
	"database/sql"
	"errors"
	"strings"
	"time"
)

// Group is a user-managed folder for recordings.
type Group struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Count  int    `json:"count"`
	Colour string `json:"colour"` // empty means derived from the name
}

// Groups lists groups with recording counts, busiest first.
func (d *DB) Groups() ([]Group, error) {
	rows, err := d.sql.Query(`
		SELECT g.id, g.name, g.colour, (
			SELECT COUNT(*) FROM recordings r
			WHERE r.folder = g.id AND r.deleted IS NULL
		) AS held
		FROM groups g ORDER BY held DESC, g.name`)
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

// Loose is how many recordings are unfiled.
func (d *DB) Loose() (int, error) {
	var n int
	err := d.sql.QueryRow(
		`SELECT COUNT(*) FROM recordings WHERE folder IS NULL AND deleted IS NULL`).Scan(&n)
	return n, err
}

// NewGroup creates a group or returns the existing one with that name.
func (d *DB) NewGroup(name string) (Group, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Group{}, errors.New("a group needs a name")
	}
	if _, err := d.sql.Exec(
		`INSERT INTO groups (name) VALUES (?) ON CONFLICT(name) DO NOTHING`, name); err != nil {
		return Group{}, err
	}
	var g Group
	g.Name = name
	err := d.sql.QueryRow(`SELECT id FROM groups WHERE name = ?`, name).Scan(&g.ID)
	return g, err
}

// Paint sets or clears a group's explicit colour.
func (d *DB) Paint(id int64, colour string) error {
	_, err := d.sql.Exec(`UPDATE groups SET colour = ? WHERE id = ?`, strings.TrimSpace(colour), id)
	return err
}

// RenameGroup changes a group's name.
func (d *DB) RenameGroup(id int64, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("a project needs a name")
	}
	_, err := d.sql.Exec(`UPDATE groups SET name = ? WHERE id = ?`, name, id)
	return err
}

// Assign files a recording under a group, or under none when group is zero.
func (d *DB) Assign(recording, group int64) error {
	if group == 0 {
		_, err := d.sql.Exec(`UPDATE recordings SET folder = NULL WHERE id = ?`, recording)
		return err
	}
	_, err := d.sql.Exec(`UPDATE recordings SET folder = ? WHERE id = ?`, group, recording)
	return err
}

// DropGroup removes a group while keeping its recordings.
func (d *DB) DropGroup(id int64) error {
	tx, err := d.sql.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`UPDATE recordings SET folder = NULL WHERE folder = ?`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM groups WHERE id = ?`, id); err != nil {
		return err
	}
	return tx.Commit()
}

// Bin lists recordings that were deleted but not yet purged.
func (d *DB) Bin() ([]Recording, error) { return d.list(`WHERE deleted IS NOT NULL`, -1) }

// Bury moves a recording to the bin.
func (d *DB) Bury(id int64) error {
	_, err := d.sql.Exec(`UPDATE recordings SET deleted = ? WHERE id = ?`, time.Now().Unix(), id)
	return err
}

// Restore takes a recording back out of the bin.
func (d *DB) Restore(id int64) error {
	_, err := d.sql.Exec(`UPDATE recordings SET deleted = NULL WHERE id = ?`, id)
	return err
}

// Buried lists bin entries older than the given age.
func (d *DB) Buried(olderThan time.Duration) ([]Recording, error) {
	cutoff := time.Now().Add(-olderThan).Unix()
	rows, err := d.sql.Query(
		`SELECT id, audio FROM recordings WHERE deleted IS NOT NULL AND deleted <= ?`, cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Recording
	for rows.Next() {
		var r Recording
		var audio sql.NullString
		if err := rows.Scan(&r.ID, &audio); err != nil {
			return nil, err
		}
		r.Audio = audio.String
		out = append(out, r)
	}
	return out, rows.Err()
}

// Skipped records that the listener threw a recording away.
func (d *DB) Skipped(seconds float64, why string) error {
	_, err := d.sql.Exec(`INSERT INTO discarded (at, seconds, why) VALUES (?, ?, ?)`,
		time.Now().Unix(), seconds, why)
	return err
}

// Saved reports how much listening work was discarded since a moment.
func (d *DB) Saved(since time.Time) (count int, seconds float64) {
	_ = d.sql.QueryRow(
		`SELECT COUNT(*), COALESCE(SUM(seconds), 0) FROM discarded WHERE at >= ?`,
		since.Unix()).Scan(&count, &seconds)
	return count, seconds
}

// Mark is the lightweight recording shape the timeline draws.
type Mark struct {
	ID       int64     `json:"id"`
	Kind     Kind      `json:"kind"`
	Started  time.Time `json:"started"`
	Duration float64   `json:"duration"`
	Folder   int64     `json:"folder"`
}

// Span lists recordings between two moments, oldest first.
func (d *DB) Span(from, to time.Time) ([]Mark, error) {
	rows, err := d.sql.Query(`
		SELECT id, kind, started, duration, COALESCE(folder, 0)
		FROM recordings
		WHERE deleted IS NULL AND started >= ? AND started <= ?
		ORDER BY started`, from.Unix(), to.Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Mark{}
	for rows.Next() {
		var m Mark
		var started int64
		if err := rows.Scan(&m.ID, &m.Kind, &started, &m.Duration, &m.Folder); err != nil {
			return nil, err
		}
		m.Started = time.Unix(started, 0)
		out = append(out, m)
	}
	return out, rows.Err()
}

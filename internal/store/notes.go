package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
)

// Sticky is a handwritten note owned by exactly one meeting or project.
type Sticky struct {
	ID        int64   `json:"id"`
	Recording int64   `json:"recording"`
	Project   int64   `json:"project"`
	Text      string  `json:"text"`
	Colour    string  `json:"colour"`
	At        float64 `json:"at"`
}

func (d *DB) Notes(recording, project int64) ([]Sticky, error) {
	rows, err := d.sql.Query(`SELECT id, COALESCE(recording,0), COALESCE(project,0), text, colour, at FROM notes WHERE (recording=? AND ?>0) OR (project=? AND ?>0) ORDER BY id`, recording, recording, project, project)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Sticky{}
	for rows.Next() {
		var n Sticky
		if err := rows.Scan(&n.ID, &n.Recording, &n.Project, &n.Text, &n.Colour, &n.At); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (d *DB) PutNote(n Sticky) (Sticky, error) {
	if (n.Recording > 0) == (n.Project > 0) {
		return n, errors.New("note needs one meeting or project")
	}
	if n.At < 0 {
		n.At = 0
	}
	if len(n.Text) > 100000 {
		return n, errors.New("note is too long")
	}
	if n.Colour != "lime" && n.Colour != "blue" && n.Colour != "pink" && n.Colour != "orange" {
		n.Colour = "lime"
	}
	var r, p any
	if n.Recording > 0 {
		r = n.Recording
	}
	if n.Project > 0 {
		p = n.Project
	}
	if n.ID == 0 {
		result, err := d.sql.Exec(`INSERT INTO notes(recording,project,text,colour,at) VALUES(?,?,?,?,?)`, r, p, n.Text, n.Colour, n.At)
		if err != nil {
			return n, err
		}
		n.ID, err = result.LastInsertId()
		return n, err
	}
	result, err := d.sql.Exec(`UPDATE notes SET text=?,colour=?,at=? WHERE id=? AND recording IS ? AND project IS ?`, n.Text, n.Colour, n.At, n.ID, r, p)
	if err != nil {
		return n, err
	}
	count, _ := result.RowsAffected()
	if count != 1 {
		return n, sql.ErrNoRows
	}
	return n, nil
}

func (d *DB) RemoveNote(id int64) error {
	_, err := d.sql.Exec(`DELETE FROM notes WHERE id=?`, id)
	return err
}

func (d *DB) AcceptSummary(id int64, before, after *Summary) error {
	if after == nil {
		return errors.New("summary is required")
	}
	var raw sql.NullString
	if err := d.sql.QueryRow(`SELECT summary FROM recordings WHERE id=? AND deleted IS NULL`, id).Scan(&raw); err != nil {
		return err
	}
	var current *Summary
	if raw.Valid && raw.String != "" {
		if err := json.Unmarshal([]byte(raw.String), &current); err != nil {
			return err
		}
	}
	old, err := json.Marshal(before)
	if err != nil {
		return err
	}
	actual, err := json.Marshal(current)
	if err != nil {
		return err
	}
	if string(actual) != string(old) {
		return errors.New("Підсумок змінився. Оновіть документ перед прийняттям редакції")
	}
	next, err := json.Marshal(after)
	if err != nil {
		return err
	}
	result, err := d.sql.Exec(`UPDATE recordings SET summary=? WHERE id=? AND deleted IS NULL AND summary IS ?`, string(next), id, raw)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n != 1 {
		return errors.New("Підсумок змінився. Оновіть документ перед прийняттям редакції")
	}
	// A first summary names a recording nobody has titled, as SaveSummary does
	// when it comes on its own.
	if title := strings.TrimSpace(after.Title); current == nil && title != "" {
		_, err = d.sql.Exec(`UPDATE recordings SET title = ? WHERE id = ? AND titled = 0`, title, id)
	}
	return err
}

// EditAction changes one stored action item, or appends one when index < 0.
func (d *DB) EditAction(id int64, index int, a Action) error {
	a.Task = strings.TrimSpace(a.Task)
	if a.Task == "" {
		return errors.New("a task is needed")
	}
	blob, err := json.Marshal(a)
	if err != nil {
		return err
	}
	if index < 0 {
		result, err := d.sql.Exec(`UPDATE recordings SET summary=json_insert(json_set(COALESCE(summary,'{}'),'$.action_items',json(CASE WHEN json_type(summary,'$.action_items')='array' THEN json_extract(summary,'$.action_items') ELSE '[]' END)),'$.action_items[#]',json(?)) WHERE id=? AND deleted IS NULL`, string(blob), id)
		if err != nil {
			return err
		}
		n, _ := result.RowsAffected()
		if n != 1 {
			return sql.ErrNoRows
		}
		return nil
	}
	path := "$.action_items[" + strconv.Itoa(index) + "]"
	result, err := d.sql.Exec(`UPDATE recordings SET summary=json_set(summary,?,json(?)) WHERE id=? AND deleted IS NULL AND json_type(summary,?)='object'`, path, string(blob), id, path)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n != 1 {
		return sql.ErrNoRows
	}
	return nil
}

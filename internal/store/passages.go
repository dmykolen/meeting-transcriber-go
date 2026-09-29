package store

import (
	"database/sql"
	"encoding/binary"
	"errors"
	"math"
	"sort"
	"strings"
)

// Passage is the target duration of one indexed transcript stretch.
const Passage = 45.0

// Piece is one indexed stretch of a meeting.
type Piece struct {
	Recording int64
	Start     float64
	Speaker   string
	Text      string
}

// Cut folds a transcript into passages.
func Cut(recording int64, turns []Turn) []Piece {
	var out []Piece
	var current Piece
	var open bool

	for _, t := range turns {
		if text := strings.TrimSpace(t.Text); text != "" {
			if !open {
				current = Piece{Recording: recording, Start: t.Start, Speaker: t.Speaker}
				open = true
			}
			current.Text += text + " "
		}
		if open && t.End-current.Start >= Passage {
			current.Text = strings.TrimSpace(current.Text)
			out, open = append(out, current), false
		}
	}
	if open {
		current.Text = strings.TrimSpace(current.Text)
		if current.Text != "" {
			out = append(out, current)
		}
	}
	return out
}

// Index replaces one recording's passages.
func (d *DB) Index(recording int64, pieces []Piece, vectors [][]float32) error {
	tx, err := d.sql.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM passages WHERE recording = ?`, recording); err != nil {
		return err
	}
	for i, p := range pieces {
		var vector []byte
		if i < len(vectors) && len(vectors[i]) > 0 {
			vector = pack(vectors[i])
		}
		if _, err := tx.Exec(
			`INSERT INTO passages (recording, seq, start, speaker, text, vector) VALUES (?, ?, ?, ?, ?, ?)`,
			recording, i, p.Start, p.Speaker, p.Text, vector); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Closest is semantic search over stored passage vectors.
func (d *DB) Closest(vector []float32, limit int) ([]Hit, error) {
	rows, err := d.sql.Query(`
		SELECT p.recording, r.title, p.start, p.speaker, p.text, p.vector
		FROM passages p JOIN recordings r ON r.id = p.recording
		WHERE p.vector IS NOT NULL`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type scored struct {
		hit   Hit
		score float64
	}
	var found []scored
	for rows.Next() {
		var h Hit
		var raw []byte
		if err := rows.Scan(&h.Recording, &h.Title, &h.Start, &h.Speaker, &h.Text, &raw); err != nil {
			return nil, err
		}
		found = append(found, scored{h, Cosine(vector, unpack(raw))})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.Slice(found, func(i, j int) bool { return found[i].score > found[j].score })

	// A floor, because cosine always returns something: without it a question
	// about nothing in the database comes back with the least unrelated
	// passage in it, presented as an answer.
	out := []Hit{}
	for _, s := range found {
		if len(out) >= limit || s.score < 0.25 {
			break
		}
		out = append(out, s.hit)
	}
	return out, nil
}

// Indexed reports how many passages carry vectors.
func (d *DB) Indexed() (with, without int) {
	_ = d.sql.QueryRow(`SELECT
		COUNT(vector), COUNT(*) - COUNT(vector) FROM passages`).Scan(&with, &without)
	return with, without
}

// oldVectors made every vector stored before the embedder could be chosen
// (insights.OpenAIVectors).
const oldVectors = "openai/text-embedding-3-small/512"

// Vectors records which model makes vectors from now on. When that is not the
// model that made the vectors already stored, they are forgotten: vectors from
// two models do not compare, and keyword search finds every passage until they
// are made again.
func (d *DB) Vectors(model string) (changed bool, err error) {
	was := oldVectors
	if err := d.sql.QueryRow(`SELECT value FROM meta WHERE key = 'vectors'`).Scan(&was); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return false, err
	}
	if was == model {
		return false, nil
	}
	tx, err := d.sql.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`UPDATE passages SET vector = NULL`); err != nil {
		return false, err
	}
	if _, err := tx.Exec(`DELETE FROM knowledge_vectors`); err != nil {
		return false, err
	}
	if _, err := tx.Exec(`INSERT OR REPLACE INTO meta (key, value) VALUES ('vectors', ?)`, model); err != nil {
		return false, err
	}
	return true, tx.Commit()
}

// Stale lists recordings whose passages still lack vectors.
func (d *DB) Stale(limit int) ([]int64, error) {
	rows, err := d.sql.Query(`
		SELECT DISTINCT recording FROM passages WHERE vector IS NULL
		UNION
		SELECT id FROM recordings WHERE status = 'done'
		  AND id NOT IN (SELECT recording FROM passages)
		LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// pack and unpack keep vectors as raw little-endian float32 rather than JSON.
func pack(v []float32) []byte {
	out := make([]byte, 4*len(v))
	for i, x := range v {
		binary.LittleEndian.PutUint32(out[4*i:], math.Float32bits(x))
	}
	return out
}

func unpack(raw []byte) []float32 {
	out := make([]float32, len(raw)/4)
	for i := range out {
		out[i] = math.Float32frombits(binary.LittleEndian.Uint32(raw[4*i:]))
	}
	return out
}

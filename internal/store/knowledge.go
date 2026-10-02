package store

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
)

// KnowledgeHit preserves the source kind and owner.
type KnowledgeHit struct {
	Key       string  `json:"key"`
	Kind      string  `json:"kind"`
	Recording int64   `json:"recording"`
	Project   int64   `json:"project"`
	Note      int64   `json:"note"`
	Title     string  `json:"title"`
	Start     float64 `json:"start"`
	Text      string  `json:"text"`
}

// Knowledge builds the corpus used for local knowledge search.
func (d *DB) Knowledge() ([]KnowledgeHit, error) {
	out := []KnowledgeHit{}
	add := func(h KnowledgeHit) {
		runes := []rune(strings.TrimSpace(h.Text))
		for start := 0; start < len(runes); start += 1050 {
			end := min(start+1200, len(runes))
			part := h
			part.Text = string(runes[start:end])
			part.Key = fmt.Sprintf("%s/%d/%d/%d/%g/%d", h.Kind, h.Recording, h.Project, h.Note, h.Start, start)
			out = append(out, part)
			if end == len(runes) {
				break
			}
		}
	}
	// Use passages when available, otherwise turn rows: unindexed recordings
	// remain discoverable. Soft-deleted content never enters this corpus.
	rows, err := d.sql.Query(`SELECT r.id,r.title,t.start,t.text FROM turns t JOIN recordings r ON r.id=t.recording WHERE r.deleted IS NULL AND NOT EXISTS(SELECT 1 FROM passages p WHERE p.recording=r.id)
 UNION ALL SELECT r.id,r.title,p.start,p.text FROM passages p JOIN recordings r ON r.id=p.recording WHERE r.deleted IS NULL`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		h := KnowledgeHit{Kind: "transcript"}
		if err = rows.Scan(&h.Recording, &h.Title, &h.Start, &h.Text); err != nil {
			rows.Close()
			return nil, err
		}
		add(h)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	rows, err = d.sql.Query(`SELECT id,title,COALESCE(summary,''),note FROM recordings WHERE deleted IS NULL`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id int64
		var title, raw, note string
		if err = rows.Scan(&id, &title, &raw, &note); err != nil {
			rows.Close()
			return nil, err
		}
		var s Summary
		_ = json.Unmarshal([]byte(raw), &s)
		add(KnowledgeHit{Kind: "summary", Recording: id, Title: title, Text: strings.Join(append([]string{s.Overview}, s.Decisions...), "\n")})
		for i, a := range s.ActionItems {
			// The same line a project item makes, so it reads the same in either
			// interface language and to the model.
			state := "open"
			if a.Done {
				state = "done"
			}
			add(KnowledgeHit{Kind: "action", Recording: id, Title: title, Note: int64(i), Text: fmt.Sprintf("%s · %s · %s · %s", a.Task, a.Owner, a.Due, state)})
		}
		add(KnowledgeHit{Kind: "question", Recording: id, Title: title, Text: strings.Join(s.OpenQuestions, "\n")})
		add(KnowledgeHit{Kind: "note", Recording: id, Title: title, Text: note})
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	rows, err = d.sql.Query(`SELECT n.id,COALESCE(n.recording,0),COALESCE(n.project,0),COALESCE(r.title,g.name,''),n.text FROM notes n LEFT JOIN recordings r ON r.id=n.recording LEFT JOIN groups g ON g.id=n.project WHERE n.recording IS NULL OR (r.id IS NOT NULL AND r.deleted IS NULL)`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		h := KnowledgeHit{Kind: "note"}
		if err = rows.Scan(&h.Note, &h.Recording, &h.Project, &h.Title, &h.Text); err != nil {
			rows.Close()
			return nil, err
		}
		add(h)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	rows, err = d.sql.Query(`SELECT id,name,state FROM groups`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var title, raw string
		if err = rows.Scan(&id, &title, &raw); err != nil {
			return nil, err
		}
		var k Kept
		_ = json.Unmarshal([]byte(raw), &k)
		lines := []string{k.Status}
		for _, list := range [][]Item{k.Work, k.Decisions, k.Questions} {
			for _, i := range list {
				lines = append(lines, fmt.Sprintf("%s · %s · %s · %s", i.Text, i.Owner, i.Due, i.State))
			}
		}
		add(KnowledgeHit{Kind: "project", Project: id, Title: title, Text: strings.Join(lines, "\n")})
	}
	return out, rows.Err()
}

// KnowledgeKey derives the cache key for one knowledge chunk.
func KnowledgeKey(h KnowledgeHit) string {
	return fmt.Sprintf("text-embedding-3-small/512/%s/%x", h.Key, sha256.Sum256([]byte(h.Title+"\n"+h.Text)))
}
// KnowledgeVectors returns cached embeddings for knowledge chunks.
func (d *DB) KnowledgeVectors() (map[string][]float32, error) {
	rows, err := d.sql.Query(`SELECT key,vector FROM knowledge_vectors`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string][]float32{}
	for rows.Next() {
		var key string
		var blob []byte
		if err = rows.Scan(&key, &blob); err != nil {
			return nil, err
		}
		out[key] = unpack(blob)
	}
	return out, rows.Err()
}
// CacheKnowledge stores one knowledge embedding.
func (d *DB) CacheKnowledge(key string, vector []float32) error {
	_, err := d.sql.Exec(`INSERT OR REPLACE INTO knowledge_vectors(key,vector) VALUES(?,?)`, key, pack(vector))
	return err
}
// DropKnowledgeVector deletes one cached knowledge embedding.
func (d *DB) DropKnowledgeVector(key string) error {
	_, err := d.sql.Exec(`DELETE FROM knowledge_vectors WHERE key=?`, key)
	return err
}

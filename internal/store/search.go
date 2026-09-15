package store

import (
	"fmt"
	"strings"

	"github.com/dmykolen/meeting-transcriber-go/internal/media"
)

// Hit is one passage that matched a search.
type Hit struct {
	Recording int64   `json:"recording"`
	Title     string  `json:"title"`
	Start     float64 `json:"start"`
	Speaker   string  `json:"speaker,omitempty"`
	Text      string  `json:"text"`
}

// Search finds passages across every transcript with SQLite FTS.
func (d *DB) Search(query string, limit int) ([]Hit, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 30
	}
	rows, err := d.sql.Query(`
		SELECT t.recording, r.title, t.start, t.speaker, t.text
		FROM transcript t JOIN recordings r ON r.id = t.recording
		WHERE transcript MATCH ?
		ORDER BY rank
		LIMIT ?`, fts(query), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var hits []Hit
	for rows.Next() {
		var h Hit
		if err := rows.Scan(&h.Recording, &h.Title, &h.Start, &h.Speaker, &h.Text); err != nil {
			return nil, err
		}
		hits = append(hits, h)
	}
	return hits, rows.Err()
}

// fts turns user text into a safe FTS5 query.
func fts(query string) string {
	words := strings.Fields(query)
	quoted := make([]string, 0, len(words))
	for i, w := range words {
		w = strings.ReplaceAll(w, `"`, `""`)
		if i == len(words)-1 {
			quoted = append(quoted, fmt.Sprintf(`"%s"*`, w))
			continue
		}
		quoted = append(quoted, fmt.Sprintf(`"%s"`, w))
	}
	return strings.Join(quoted, " ")
}

// Passages renders hits the way the model reads them.
func Passages(hits []Hit) []string {
	out := make([]string, 0, len(hits))
	for _, h := range hits {
		who := h.Speaker
		if who == "" {
			who = "Unknown"
		}
		out = append(out, fmt.Sprintf("%s [%s] %s: %s", h.Title, media.Clock(h.Start), who, h.Text))
	}
	return out
}

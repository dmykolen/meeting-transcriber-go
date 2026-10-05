package store

import (
	"sort"
	"strings"
)

// Topics lists the topics summaries already use, most used first, each spelled
// the way it was most often written. Grouped in Go: SQLite's lower() leaves
// Cyrillic alone.
func (d *DB) Topics(limit int) ([]string, error) {
	rows, err := d.sql.Query(`
		SELECT j.value FROM recordings r, json_each(r.summary, '$.topics') j
		WHERE r.deleted IS NULL AND r.summary IS NOT NULL ORDER BY r.started DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type seen struct {
		count     int
		spellings map[string]int
	}
	byKey := map[string]*seen{}
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return nil, err
		}
		value = strings.Join(strings.Fields(value), " ")
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if byKey[key] == nil {
			byKey[key] = &seen{spellings: map[string]int{}}
		}
		byKey[key].count++
		byKey[key].spellings[value]++
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(byKey))
	for k := range byKey {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		a, b := byKey[keys[i]], byKey[keys[j]]
		return a.count > b.count || a.count == b.count && keys[i] < keys[j]
	})
	if limit > 0 && len(keys) > limit {
		keys = keys[:limit]
	}
	out := make([]string, len(keys))
	for i, k := range keys {
		best := ""
		for spelling, n := range byKey[k].spellings {
			if best == "" || n > byKey[k].spellings[best] || n == byKey[k].spellings[best] && spelling < best {
				best = spelling
			}
		}
		out[i] = best
	}
	return out, nil
}

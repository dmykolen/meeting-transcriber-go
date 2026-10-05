package store

import "time"

// Call is one request to a model, kept for the usage report.
type Call struct {
	At                    time.Time
	Task, Provider, Model string
	Input, Output         int64
	Credits               float64 // GitHub AI Credits, as Copilot reports them
	Took                  time.Duration
	Failed                string
}

// Spent is what one model used on one day.
type Spent struct {
	Day      string  `json:"day"` // local, 2026-10-05
	Provider string  `json:"provider"`
	Model    string  `json:"model"`
	Calls    int64   `json:"calls"`
	Failed   int64   `json:"failed"`
	Input    int64   `json:"input"`
	Output   int64   `json:"output"`
	Credits  float64 `json:"credits"`
	Seconds  float64 `json:"seconds"`
}

// Called remembers a model call.
func (d *DB) Called(c Call) error {
	_, err := d.sql.Exec(`INSERT INTO usage (at, task, provider, model, input, output, credits, seconds, failed)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.At.Unix(), c.Task, c.Provider, c.Model, c.Input, c.Output, c.Credits, c.Took.Seconds(), c.Failed)
	return err
}

// Usage sums the calls since a moment by local day and model.
func (d *DB) Usage(since time.Time) ([]Spent, error) {
	rows, err := d.sql.Query(`
		SELECT date(at, 'unixepoch', 'localtime'), provider, model, count(*), count(nullif(failed, '')),
		       sum(input), sum(output), sum(credits), sum(seconds)
		FROM usage WHERE at >= ? GROUP BY 1, 2, 3 ORDER BY 1`, since.Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Spent
	for rows.Next() {
		var s Spent
		if err := rows.Scan(&s.Day, &s.Provider, &s.Model, &s.Calls, &s.Failed, &s.Input, &s.Output, &s.Credits, &s.Seconds); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

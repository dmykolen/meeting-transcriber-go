package service

import (
	"cmp"
	"log/slog"
	"slices"
	"time"

	"github.com/dmykolen/meeting-transcriber-go/internal/insights"
	"github.com/dmykolen/meeting-transcriber-go/internal/store"
)

// called keeps a finished model call for the usage report.
func (m *Meetings) called(u insights.Use) {
	err := m.db.Called(store.Call{At: time.Now(), Task: u.Task, Provider: u.Provider, Model: u.Model,
		Input: u.Input, Output: u.Output, Credits: u.Credits, Took: u.Took, Failed: u.Failed})
	if err != nil {
		slog.Warn("could not record a model call", "err", err)
	}
}

// UsageRow is what one model used over the period.
type UsageRow struct {
	store.Spent
	// Cost is in US dollars; Priced says whether it is known for this model.
	Cost   float64 `json:"cost"`
	Priced bool    `json:"priced"`
}

// UsageDay is one day of the chart.
type UsageDay struct {
	Day     string  `json:"day"`
	Calls   int64   `json:"calls"`
	Tokens  int64   `json:"tokens"`
	Cost    float64 `json:"cost"`
	Credits float64 `json:"credits"`
}

// UsageReport is the model usage over the last days, for the settings screen.
type UsageReport struct {
	Days    int        `json:"days"`
	Models  []UsageRow `json:"models"`  // most used first, a row per model
	Daily   []UsageDay `json:"daily"`   // every day of the period, oldest first
	Cost    float64    `json:"cost"`    // priced models only
	Credits float64    `json:"credits"` // GitHub AI Credits
}

// Usage reports what the models were asked and what it cost over the last days.
func (m *Meetings) Usage(days int) (UsageReport, error) {
	days = min(max(days, 1), 366)
	today := time.Now()
	y, mo, d := today.Date()
	spent, err := m.db.Usage(time.Date(y, mo, d-(days-1), 0, 0, 0, 0, today.Location()))
	if err != nil {
		return UsageReport{}, err
	}
	r := UsageReport{Days: days, Models: []UsageRow{}, Daily: make([]UsageDay, days)}
	for i := range r.Daily {
		r.Daily[i].Day = today.AddDate(0, 0, i+1-days).Format(time.DateOnly)
	}
	rows := map[[2]string]*UsageRow{}
	for _, s := range spent {
		cost, priced := insights.Cost(s.Provider, s.Model, s.Input, s.Output)
		key := [2]string{s.Provider, s.Model}
		row := rows[key]
		if row == nil {
			row = &UsageRow{Spent: store.Spent{Provider: s.Provider, Model: s.Model}}
			rows[key] = row
		}
		row.Calls, row.Failed, row.Input, row.Output = row.Calls+s.Calls, row.Failed+s.Failed, row.Input+s.Input, row.Output+s.Output
		row.Credits, row.Seconds, row.Cost, row.Priced = row.Credits+s.Credits, row.Seconds+s.Seconds, row.Cost+cost, row.Priced || priced
		if i := slices.IndexFunc(r.Daily, func(d UsageDay) bool { return d.Day == s.Day }); i >= 0 {
			d := &r.Daily[i]
			d.Calls, d.Tokens, d.Cost, d.Credits = d.Calls+s.Calls, d.Tokens+s.Input+s.Output, d.Cost+cost, d.Credits+s.Credits
		}
	}
	for _, row := range rows {
		r.Models = append(r.Models, *row)
		r.Cost, r.Credits = r.Cost+row.Cost, r.Credits+row.Credits
	}
	slices.SortFunc(r.Models, func(a, b UsageRow) int { return cmp.Compare(b.Input+b.Output, a.Input+a.Output) })
	return r, nil
}

package main

import (
	"regexp"
	"strings"
)

// metrics are what can be counted about a document without a judge: how big it
// is, how much of it is still open, how many open lines say the same thing, and
// how much has not been touched for a long time.
type metrics struct {
	Meetings      int            `json:"meetings"`
	Lines         int            `json:"lines"`
	PerMeeting    float64        `json:"lines_per_meeting"`
	Open          map[string]int `json:"open_by_kind"`
	Closed        int            `json:"closed_or_replaced"`
	OpenShare     float64        `json:"open_share"`
	DuplicatePair int            `json:"open_duplicate_pairs"`
	StaleOpen     int            `json:"open_untouched_for_10_meetings"`
	NoOwner       int            `json:"open_work_without_owner"`
	Streams       int            `json:"streams"`
	BiggestStream int            `json:"biggest_stream"`
	Ignored       int            `json:"ignored_operations"`
}

var word = regexp.MustCompile(`[\p{L}\p{N}]{4,}`)

func tokens(s string) map[string]bool {
	out := map[string]bool{}
	for _, w := range word.FindAllString(strings.ToLower(s), -1) {
		out[w] = true
	}
	return out
}

func jaccard(a, b map[string]bool) float64 {
	n := 0
	for w := range a {
		if b[w] {
			n++
		}
	}
	if len(a)+len(b)-n == 0 {
		return 0
	}
	return float64(n) / float64(len(a)+len(b)-n)
}

func measure(d *doc, meetings int) metrics {
	m := metrics{Meetings: meetings, Lines: len(d.Items), Open: map[string]int{}, Ignored: d.Ignored}
	per := map[string]int{}
	var open []item
	for _, it := range d.Items {
		per[it.Stream]++
		if !it.open() {
			m.Closed++
			continue
		}
		open = append(open, it)
		m.Open[it.Kind]++
		if it.Kind != "decision" && meetings-1-it.Last >= 10 {
			m.StaleOpen++
		}
		if it.Kind == "work" && it.Owner == "" {
			m.NoOwner++
		}
	}
	for i := range open {
		for j := i + 1; j < len(open); j++ {
			if open[i].Kind == open[j].Kind && jaccard(tokens(open[i].Text), tokens(open[j].Text)) >= 0.5 {
				m.DuplicatePair++
			}
		}
	}
	for s, n := range per {
		if s != "" {
			m.Streams++
		}
		m.BiggestStream = max(m.BiggestStream, n)
	}
	if meetings > 0 {
		m.PerMeeting = float64(m.Lines) / float64(meetings)
	}
	if m.Lines > 0 {
		m.OpenShare = float64(len(open)) / float64(m.Lines)
	}
	return m
}

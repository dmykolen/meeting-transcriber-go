package insights

import (
	"context"
	"strings"
	"testing"
)

func TestHeldShowsOpenLinesByStreamAndOnlyCountsTheRest(t *testing.T) {
	got := Held("Де ми", []Line{
		{ID: 1, Kind: "work", Stream: "Ролі", Text: "Зробити схему", Owner: "Marta", State: "open", Times: 2},
		{ID: 2, Kind: "work", Stream: "Ролі", Text: "Вже зроблено", State: "done"},
		{ID: 3, Kind: "decision", Stream: "Безпека", Text: "Лише VPN", State: "standing", Last: 4},
		{ID: 4, Kind: "decision", Stream: "Безпека", Text: "Скасовано", State: "overturned"},
	})
	for _, want := range []string{`streams: Безпека; Ролі`, `stream "Ролі" (1 open)`, "#1 work [open] Зробити схему — Marta (mentioned 2 times)",
		"#3 decision [standing] Лише VPN (last touched meeting 4)", "(2 finished or replaced lines are not shown)", "status: Де ми"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, "Вже зроблено") || strings.Contains(got, "Скасовано") {
		t.Fatalf("a finished line was shown:\n%s", got)
	}
	if Held("x", []Line{{ID: 1, State: "done"}}) != "" {
		t.Fatal("a document with nothing open should render as empty, so the first meeting says so")
	}
}

func TestNotesAndMeetingsAreSummarisedByTheirOwnPrompts(t *testing.T) {
	var seen string
	c := &Client{language: "Ukrainian", ask: func(_ context.Context, p prompt) (string, spent, error) {
		seen = p.instructions
		return `{"title":"x","overview":"","chapters":[],"topics":[],"decisions":[],"action_items":[],"open_questions":[]}`, spent{}, nil
	}}
	turns := []Turn{{Text: "x"}}
	if _, err := c.Summarise(context.Background(), turns, nil, true); err != nil || !strings.Contains(seen, "one person's microphone") {
		t.Fatalf("a note was not given the note prompt (%v):\n%.200s", err, seen)
	}
	if _, err := c.Summarise(context.Background(), turns, nil, false); err != nil || !strings.Contains(seen, "one meeting") || strings.Contains(seen, "one person's microphone") {
		t.Fatalf("a meeting was not given the meeting prompt (%v):\n%.200s", err, seen)
	}
}

func TestTidyAndBriefReadWhatTheModelSends(t *testing.T) {
	c := &Client{language: "Ukrainian", ask: func(_ context.Context, p prompt) (string, spent, error) {
		if p.name == "project_tidy" {
			return `{"merges":[{"keep":1,"drop":[2],"text":""}],"retire":[{"id":3,"state":"dropped","why":"x"}],"moves":[{"id":1,"stream":"Ролі"}],"streams":[{"from":"A","to":"B"}]}`, spent{}, nil
		}
		return `{"headline":"h","streams":[{"name":"Ролі","state":"s","lines":[1]}],"decisions":[3],"attention":[{"id":1,"why":"late"}]}`, spent{}, nil
	}}
	pass, err := c.Tidy(context.Background(), "lines", 8)
	if err != nil || len(pass.Merges) != 1 || pass.Merges[0].Drop[0] != 2 || pass.Retire[0].State != "dropped" || pass.Moves[0].Stream != "Ролі" || pass.Streams[0].To != "B" {
		t.Fatalf("tidy = %+v, %v", pass, err)
	}
	b, err := c.Brief(context.Background(), "lines")
	if err != nil || b.Headline != "h" || b.Streams[0].Lines[0] != 1 || b.Decisions[0] != 3 || b.Attention[0].Why != "late" {
		t.Fatalf("brief = %+v, %v", b, err)
	}
}

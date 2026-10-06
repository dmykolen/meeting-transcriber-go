package library

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dmykolen/meeting-transcriber-go/internal/insights"
	"github.com/dmykolen/meeting-transcriber-go/internal/store"
)

// A project is folded meeting by meeting through the real library and the real
// client; only the model's answers are faked. The document ends up in streams,
// repeats are merged by the pass every eighth meeting, and the picture points
// only at lines that exist.
func TestProjectIsFoldedIntoStreamsTidiedAndDescribed(t *testing.T) {
	var mu sync.Mutex
	var asked []string
	pictures := map[string]int{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Input string `json:"input"`
			Text  struct {
				Format struct{ Name string } `json:"format"`
			} `json:"text"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
		}
		mu.Lock()
		name := req.Text.Format.Name
		asked = append(asked, name)
		pictures[name]++
		first := pictures["project_changes"] == 1 && name == "project_changes"
		mu.Unlock()
		var answer string
		switch name {
		case "project_changes":
			if first {
				answer = `{"status":"перший","changes":[
				 {"do":"add","kind":"work","id":0,"text":"Зробити схему","owner":"SPEAKER_03","due":"","state":"","stream":"Ролі"},
				 {"do":"add","kind":"work","id":0,"text":"Зробити архітектурну схему","owner":"","due":"","state":"","stream":"Ролі"},
				 {"do":"add","kind":"decision","id":0,"text":"Лише VPN","owner":"","due":"","state":"","stream":"Доступ"}]}`
			} else {
				// Later meetings only see the picture the first one made.
				if !strings.Contains(req.Input, `stream "Ролі" (2 open)`) {
					t.Errorf("the second fold was not shown the picture so far:\n%s", req.Input)
				}
				answer = `{"status":"далі","changes":[{"do":"restate","kind":"work","id":1,"text":"","owner":"","due":"","state":"","stream":""}]}`
			}
		case "project_tidy":
			answer = `{"merges":[{"keep":1,"drop":[2],"text":""}],"retire":[],"moves":[],"streams":[]}`
		case "project_brief":
			answer = `{"headline":"Схему готують","streams":[{"name":"Ролі","state":"йде","lines":[1,99]}],"decisions":[3,98],"attention":[{"id":1,"why":"давно"},{"id":97,"why":"немає такого"}]}`
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"id": "r", "object": "response", "status": "completed", "output": []any{map[string]any{
			"id": "m", "type": "message", "role": "assistant", "status": "completed",
			"content": []any{map[string]any{"type": "output_text", "text": answer, "annotations": []any{}}}}}})
	}))
	defer server.Close()
	t.Setenv("OPENAI_BASE_URL", server.URL+"/v1")

	lib, db, _ := setup(t, &fakeEngine{})
	lib.Brain(insights.New(insights.Setup{Language: "uk", Provider: "openai", OpenAIKey: "test-key-not-real", OpenAIModel: "test-model"}))
	g, err := db.NewGroup("P")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < tidyEvery; i++ {
		r, err := db.Add(store.Recording{Kind: store.Meeting, Title: "m", Audio: "a.wav", Started: time.Date(2026, 10, 1+i, 10, 0, 0, 0, time.UTC)})
		if err != nil {
			t.Fatal(err)
		}
		if err := db.SaveSummary(r.ID, &store.Summary{Title: "m", Overview: "o"}); err != nil {
			t.Fatal(err)
		}
		if err := db.Assign(r.ID, g.ID); err != nil {
			t.Fatal(err)
		}
	}

	if err := lib.Rebuild(context.Background(), g.ID); err != nil {
		t.Fatal(err)
	}
	kept, err := db.Held(g.ID)
	if err != nil || kept == nil {
		t.Fatalf("no document: %v", err)
	}
	if len(kept.Work) != 1 || kept.Work[0].Stream != "Ролі" || kept.Work[0].Owner != "" || kept.Work[0].Times != tidyEvery+1 {
		t.Fatalf("the work was not merged into one line of a stream with no speaker label for an owner: %+v", kept.Work)
	}
	if len(kept.Decisions) != 1 || kept.Decisions[0].Stream != "Доступ" || len(kept.Seen) != tidyEvery {
		t.Fatalf("decisions or the meetings folded in: %+v seen %d", kept.Decisions, len(kept.Seen))
	}
	if b := kept.Brief; b == nil || b.Headline != "Схему готують" || len(b.Streams) != 1 || len(b.Streams[0].Lines) != 1 ||
		len(b.Decisions) != 1 || len(b.Attention) != 1 {
		t.Fatalf("the picture points at lines that do not exist, or was not kept: %+v", b)
	}
	// A rebuild writes the picture once, not once per meeting; the pass ran once.
	if pictures["project_brief"] != 1 || pictures["project_tidy"] != 1 || pictures["project_changes"] != tidyEvery {
		t.Fatalf("model calls: %v", pictures)
	}
	st, err := db.Standing(g.ID)
	if err != nil || st.Brief == nil || st.Work[0].Stream != "Ролі" {
		t.Fatalf("the page does not see the picture or the stream: %+v %v", st, err)
	}
}

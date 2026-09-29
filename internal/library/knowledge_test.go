package library

import (
	"context"
	"encoding/json"
	"github.com/dmykolen/meeting-transcriber-go/internal/insights"
	"github.com/dmykolen/meeting-transcriber-go/internal/store"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestExactArchiveSearchNeedsNoProviderAndSemanticNeverSilentlyFallsBack(t *testing.T) {
	lib, db, _ := setup(t, &fakeEngine{})
	r, err := db.Add(store.Recording{Kind: store.Meeting, Title: "Безпека", Audio: "a.wav"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.PutNote(store.Sticky{Recording: r.ID, Text: "Власний контекст VPN доступу"}); err != nil {
		t.Fatal(err)
	}
	hits, err := lib.SearchKnowledge(context.Background(), "VPN контекст", false)
	if err != nil || len(hits) != 1 || hits[0].Kind != "note" {
		t.Fatalf("exact search: %+v %v", hits, err)
	}
	if _, err = lib.SearchKnowledge(context.Background(), "VPN", true); err == nil {
		t.Fatal("semantic silently fell back without a key")
	}
	if _, _, err = lib.AskKnowledge(context.Background(), "VPN"); err == nil {
		t.Fatal("Q&A pretended to have a model")
	}
	if _, err = lib.PreviewSummary(context.Background(), r.ID); err == nil {
		t.Fatal("preview pretended to have a model")
	}
	if err = db.Bury(r.ID); err != nil {
		t.Fatal(err)
	}
	hits, err = lib.SearchKnowledge(context.Background(), "VPN", false)
	if err != nil || len(hits) != 0 {
		t.Fatal("deleted note retrieved", err)
	}
}

func TestKnowledgeEmbeddingsCacheEditsAndAnswerUseOwnedSources(t *testing.T) {
	var embedded atomic.Int64
	var supplied atomic.Value
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/embeddings") {
			input := req["input"].([]any)
			data := []map[string]any{}
			for i, raw := range input {
				if strings.Contains(raw.(string), "\n") {
					embedded.Add(1)
				}
				v := make([]float64, 512)
				v[0] = 1
				data = append(data, map[string]any{"index": i, "object": "embedding", "embedding": v})
			}
			json.NewEncoder(w).Encode(map[string]any{"object": "list", "data": data, "model": "text-embedding-3-small"})
			return
		}
		supplied.Store(req["input"].(string))
		json.NewEncoder(w).Encode(map[string]any{"id": "resp_test", "object": "response", "status": "completed", "output": []any{map[string]any{"id": "msg_test", "type": "message", "role": "assistant", "status": "completed", "content": []any{map[string]any{"type": "output_text", "text": "Контекст підтверджено [1].", "annotations": []any{}}}}}})
	}))
	defer server.Close()
	t.Setenv("OPENAI_BASE_URL", server.URL+"/v1")
	lib, db, _ := setup(t, &fakeEngine{})
	lib.Brain(insights.New(insights.Setup{Language: "uk", Provider: "openai", Embeddings: "openai", OpenAIKey: "test-key-not-real", OpenAIModel: "test-model"}))
	r, err := db.Add(store.Recording{Kind: store.Meeting, Title: "Безпека", Audio: "a.wav"})
	if err != nil {
		t.Fatal(err)
	}
	n, err := db.PutNote(store.Sticky{Recording: r.ID, Text: "Особистий контекст"})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		hits, err := lib.SearchKnowledge(context.Background(), "Контекст", true)
		if err != nil || len(hits) != 1 {
			t.Fatalf("semantic: %+v %v", hits, err)
		}
	}
	if embedded.Load() != 1 {
		t.Fatal("unchanged content re-embedded", embedded.Load())
	}
	n.Text = "Оновлений особистий контекст"
	if _, err = db.PutNote(n); err != nil {
		t.Fatal(err)
	}
	answer, hits, err := lib.AskKnowledge(context.Background(), "Що змінилося?")
	if err != nil || len(hits) != 1 || answer != "Контекст підтверджено [1]." {
		t.Fatalf("answer: %q %+v %v", answer, hits, err)
	}
	if embedded.Load() != 2 {
		t.Fatal("edited note did not invalidate cache")
	}
	if text := supplied.Load().(string); !strings.Contains(text, "[1] note · Безпека") || !strings.Contains(text, n.Text) {
		t.Fatal("answer lost source metadata", text)
	}
	vectors, err := db.KnowledgeVectors()
	if err != nil || len(vectors) != 1 {
		t.Fatal("stale cache not removed", len(vectors), err)
	}
}

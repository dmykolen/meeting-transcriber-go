package insights

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

func TestJSONIsFoundInsideACodeFence(t *testing.T) {
	got, err := unfence("Ось підсумок:\n```json\n{\"title\": \"VPN\"}\n```")
	if err != nil || got != `{"title": "VPN"}` {
		t.Fatalf("got %q, %v", got, err)
	}
	if _, err := unfence("Не можу скласти підсумок."); err == nil {
		t.Fatal("prose was accepted as JSON")
	}
}

// Qwen3-Embedding returns 1024 dimensions; the archive keeps 512, unit length,
// in the order the passages were sent.
func TestLocalVectorsAreCutTo512AndUnitLength(t *testing.T) {
	long := make([]float64, 1024)
	for i := range long {
		long[i] = float64(i%7) + 1
	}
	out := shorten([]openai.Embedding{{Index: 1, Embedding: long}, {Index: 5, Embedding: long}}, 2)
	if out[0] != nil {
		t.Fatal("a missing passage got somebody else's vector")
	}
	if len(out[1]) != Dimensions {
		t.Fatalf("%d dimensions, want %d", len(out[1]), Dimensions)
	}
	var norm float64
	for _, x := range out[1] {
		norm += float64(x) * float64(x)
	}
	if math.Abs(norm-1) > 1e-5 {
		t.Fatalf("norm² = %f, want 1", norm)
	}
}

func TestNoProviderMeansAIIsOffAndSaysSo(t *testing.T) {
	c := New(Setup{Language: "uk", Provider: "copilot", Embeddings: "local"}) // nothing on disk yet
	if c.Ready() || c.Searchable() {
		t.Fatal("a provider with nothing on disk claims to be ready")
	}
	if _, err := c.Summarise(context.Background(), []Turn{{Text: "Привіт"}}, nil); !errors.Is(err, ErrNoKey) {
		t.Fatalf("err = %v, want ErrNoKey", err)
	}
	if _, err := c.Query(context.Background(), "VPN"); !errors.Is(err, ErrNoKey) {
		t.Fatalf("err = %v, want ErrNoKey", err)
	}
}

// The SDK runs the CLI with the keychain off. A sign-in that kept its token
// there signed nobody in for an app launched from the Dock, and every summary
// failed.
func TestTheCopilotSignInLeavesItsTokenWhereTheSDKLooks(t *testing.T) {
	home, bin := t.TempDir(), filepath.Join(t.TempDir(), "copilot")
	// As copilot 1.0.85 does: the keychain when it may, else the config file if
	// the settings allow plain text, else a question only a terminal can answer.
	fake := `#!/bin/sh
[ "$*" = "--no-auto-update login" ] || { echo "unknown command: $*"; exit 2; }
echo "Opening your browser to authenticate..."
[ "$COPILOT_DISABLE_KEYTAR" = 1 ] || { echo "Signed in successfully as octocat."; exit 0; }
grep -Eq '"storeTokenPlaintext": *true' "$COPILOT_HOME/settings.json" ||
	{ echo "Login succeeded, but the token was not saved."; exit 1; }
echo '{"authTokens": {"https://github.com:octocat": {"token": "gho_x"}}}' > "$COPILOT_HOME/config.json"
echo "Signed in successfully as octocat."
`
	if err := os.WriteFile(bin, []byte(fake), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "settings.json"), []byte(`{"theme": "dark"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	var said []string
	if err := CopilotLogin(context.Background(), bin, home, func(s string) { said = append(said, s) }); err != nil {
		t.Fatalf("%v; the CLI said %q", err, said)
	}
	if _, err := os.Stat(filepath.Join(home, "config.json")); err != nil {
		t.Fatal("the token is not in the Copilot home")
	}
	if settings, _ := os.ReadFile(filepath.Join(home, "settings.json")); !strings.Contains(string(settings), `"theme":"dark"`) {
		t.Fatalf("settings.json lost what was in it: %s", settings)
	}
	if len(said) != 2 || said[1] != "Signed in successfully as octocat." {
		t.Fatalf("said %q", said)
	}
}

// llama-server's chat endpoint is the one that honours the schema; the fake
// refuses a structured request that arrives without it.
func TestALocalSummaryAsksForTheSchema(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			http.Error(w, "only chat completions carry the schema", http.StatusNotFound)
			return
		}
		var req struct {
			ResponseFormat struct {
				Type       string `json:"type"`
				JSONSchema struct {
					Name   string         `json:"name"`
					Schema map[string]any `json:"schema"`
				} `json:"json_schema"`
			} `json:"response_format"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		if req.ResponseFormat.Type != "json_schema" || req.ResponseFormat.JSONSchema.Name != "meeting_summary" {
			http.Error(w, "no schema", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"id": "x", "object": "chat.completion", "model": "local",
			"choices": []any{map[string]any{"index": 0, "finish_reason": "stop",
				"message": map[string]any{"role": "assistant", "content": `{"title":"Доступ через VPN","overview":"","chapters":[],"topics":[],"decisions":["Лише VPN"],"action_items":[],"open_questions":[]}`}}},
			"usage": map[string]any{"prompt_tokens": 120, "completion_tokens": 30, "total_tokens": 150},
		})
	}))
	defer server.Close()

	h := &helper{cmd: &exec.Cmd{}, done: make(chan struct{}),
		api: openai.NewClient(option.WithBaseURL(server.URL+"/v1/"), option.WithAPIKey("local"))}
	var used []Use
	c := &Client{language: "Ukrainian", ask: h.ask, provider: "local", model: "gemma",
		usage: func(u Use) { used = append(used, u) }}
	summary, err := c.Summarise(context.Background(), []Turn{{Speaker: "Marta", Text: "Домовились: доступ лише через VPN."}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Title != "Доступ через VPN" || len(summary.Decisions) != 1 {
		t.Fatalf("summary = %+v", summary)
	}
	if len(used) != 1 || used[0].Task != "meeting_summary" || used[0].Provider != "local" ||
		used[0].Input != 120 || used[0].Output != 30 || used[0].Failed != "" {
		t.Fatalf("the call was recorded as %+v", used)
	}
}

func TestTopicsKeepTheArchivesSpelling(t *testing.T) {
	got := Canon([]string{"  безпека ", "Північний вітер", "БЕЗПЕКА", "доступ  через VPN", ""},
		[]string{"Безпека", "Доступ через VPN"})
	want := []string{"Безпека", "Північний вітер", "Доступ через VPN"}
	if !slices.Equal(got, want) {
		t.Fatalf("Canon = %q, want %q", got, want)
	}
}

func TestSummaryPromptListsTheKnownTopics(t *testing.T) {
	var seen string
	c := &Client{language: "Ukrainian", ask: func(_ context.Context, p prompt) (string, spent, error) {
		seen = p.instructions
		return `{"title":"x","overview":"","chapters":[],"topics":["безпека","Нове"],"decisions":[],"action_items":[],"open_questions":[]}`, spent{}, nil
	}}
	got, err := c.Summarise(context.Background(), []Turn{{Text: "x"}}, []string{"Безпека", "Northwind"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(seen, "- Безпека\n- Northwind") || !strings.Contains(seen, "reuse one of these") {
		t.Fatalf("the prompt does not offer the topics:\n%s", seen)
	}
	if !slices.Equal(got.Topics, []string{"Безпека", "Нове"}) {
		t.Fatalf("topics = %q", got.Topics)
	}
	if _, err := c.Summarise(context.Background(), []Turn{{Text: "x"}}, nil); err != nil || strings.Contains(seen, "reuse one of these") {
		t.Fatalf("an archive without topics still got the topics paragraph: %v", err)
	}
}

func TestOnlyKnownOpenAIModelsHaveACost(t *testing.T) {
	if cost, ok := Cost("openai", "gpt-5.4-mini", 1_000_000, 1_000_000); !ok || cost != 5.25 {
		t.Fatalf("a million tokens each way cost %v, %v", cost, ok)
	}
	for _, c := range [][2]string{{"openai", "gpt-unknown"}, {"local", "gpt-5.4-mini"}, {"copilot", "auto"}} {
		if _, ok := Cost(c[0], c[1], 1, 1); ok {
			t.Fatalf("%v was given a price", c)
		}
	}
}

// The real llama-server with the real models, when they are on this machine:
// MT_TEST_LLAMA=1 go test ./internal/insights -run Local -v
func TestLocalModelsSummariseAndFindOnThisMac(t *testing.T) {
	if os.Getenv("MT_TEST_LLAMA") == "" {
		t.Skip("set MT_TEST_LLAMA=1 to run llama-server with the downloaded models")
	}
	root, _ := filepath.Abs("../..")
	home, _ := os.UserHomeDir()
	models := filepath.Join(home, "MeetingTranscriber", "models")
	c := New(Setup{Language: "uk", Provider: "local", Embeddings: "local",
		Server: filepath.Join(root, "build", "llama-server"),
		Chat:   filepath.Join(models, "chat.gguf"), Vectors: filepath.Join(models, "vectors.gguf")})
	defer c.Close()

	summary, err := c.Summarise(context.Background(), []Turn{
		{Start: 0, Speaker: "Marta", Text: "Треба вирішити доступ до нового кабінету і пакет для Northwind."},
		{Start: 12, Speaker: "Taras", Text: "Пропоную залишити доступ тільки через VPN. Публічний доступ не відкриваємо."},
		{Start: 41, Speaker: "Marta", Text: "Домовились: доступ лише через VPN."},
		{Start: 63, Speaker: "Marta", Text: "Я підготую пакет документів для Northwind до п'ятниці."},
		{Start: 124, Speaker: "Taras", Text: "Хто погоджує фінальний перелік IP-діапазонів?"},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("summary: %+v", summary)
	if summary.Title == "" || !strings.Contains(strings.ToLower(summary.Overview+strings.Join(summary.Decisions, " ")), "vpn") {
		t.Fatalf("the summary missed the one decision: %+v", summary)
	}

	passages, err := c.Embed(context.Background(), []string{
		"Marta: Домовились, доступ лише через VPN.",
		"Taras: Я перевірю перелік IP-діапазонів.",
	})
	if err != nil {
		t.Fatal(err)
	}
	question, err := c.Query(context.Background(), "Яке рішення щодо доступу?")
	if err != nil {
		t.Fatal(err)
	}
	near, far := cosine(question, passages[0]), cosine(question, passages[1])
	t.Logf("access passage %.3f, IP passage %.3f", near, far)
	if near <= far {
		t.Fatal("the question about access did not find the passage about access")
	}
}

func cosine(a, b []float32) float64 {
	var dot float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
	}
	return dot
}

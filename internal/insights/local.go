package insights

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/shared"
)

// Local models are served by llama-server, one helper process per model file.
// The generation context holds a long meeting with room for the answer; the
// embedding batch holds a passage whole. Thinking is off, so the reply is the
// answer alone.
var (
	chatArgs   = []string{"-c", "32768", "--reasoning", "off"}
	vectorArgs = []string{"--embeddings", "--pooling", "last", "-c", "8192", "-b", "8192", "-ub", "8192"}
)

// helper runs llama-server for one model, started on first use. The server
// puts the model to sleep after five idle minutes, so a quiet day holds no
// memory for it.
type helper struct {
	bin, model string
	args       []string

	mu     sync.Mutex
	cmd    *exec.Cmd
	done   chan struct{}
	api    openai.Client
	output tail
}

// client returns an API client for a running server, starting one if needed.
func (h *helper) client(ctx context.Context) (openai.Client, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.cmd != nil {
		select {
		case <-h.done: // it died; start another
		default:
			return h.api, nil
		}
	}
	// A helper left behind by a crash of this app still holds its port and
	// model; nothing else runs this binary on this file.
	_ = exec.Command("pkill", "-f", regexp.QuoteMeta(h.bin+" -m "+h.model)).Run()
	port, err := freePort()
	if err != nil {
		return openai.Client{}, err
	}
	key := rand.Text()
	cmd := exec.Command(h.bin, append([]string{"-m", h.model, "--host", "127.0.0.1", "--port", port,
		"--no-webui", "-np", "1", "--sleep-idle-seconds", "300"}, h.args...)...)
	// The key goes in the environment, where the process list does not show it.
	cmd.Env = append(os.Environ(), "LLAMA_API_KEY="+key)
	cmd.Stdout, cmd.Stderr = &h.output, &h.output
	if err := cmd.Start(); err != nil {
		return openai.Client{}, fmt.Errorf("the local model could not start: %w", err)
	}
	done := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(done)
	}()
	base := "http://127.0.0.1:" + port
	slog.Info("starting a local model", "model", filepath.Base(h.model), "port", port)
	if err := healthy(ctx, base, done); err != nil {
		_ = cmd.Process.Kill()
		return openai.Client{}, fmt.Errorf("the local model did not start: %w. %s", err, h.output.last())
	}
	h.cmd, h.done = cmd, done
	h.api = openai.NewClient(option.WithBaseURL(base+"/v1/"), option.WithAPIKey(key))
	return h.api, nil
}

func (h *helper) close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.cmd != nil {
		_ = h.cmd.Process.Kill()
		<-h.done
		h.cmd = nil
	}
}

// ask uses Chat Completions: llama-server's Responses endpoint ignores the
// JSON schema, its chat endpoint turns the schema into a grammar.
func (h *helper) ask(ctx context.Context, p prompt) (string, error) {
	api, err := h.client(ctx)
	if err != nil {
		return "", err
	}
	params := openai.ChatCompletionNewParams{
		Model:    "local",
		Messages: []openai.ChatCompletionMessageParamUnion{openai.SystemMessage(p.instructions), openai.UserMessage(p.input)},
	}
	if p.schema != nil {
		params.ResponseFormat = openai.ChatCompletionNewParamsResponseFormatUnion{
			OfJSONSchema: &shared.ResponseFormatJSONSchemaParam{
				JSONSchema: shared.ResponseFormatJSONSchemaJSONSchemaParam{Name: p.name, Schema: p.schema, Strict: openai.Bool(true)},
			},
		}
	}
	resp, err := api.Chat.Completions.New(ctx, params)
	if err != nil {
		return "", err
	}
	if len(resp.Choices) == 0 {
		return "", errors.New("the local model returned no answer")
	}
	return resp.Choices[0].Message.Content, nil
}

func (h *helper) vectors(ctx context.Context, texts []string) ([][]float32, error) {
	api, err := h.client(ctx)
	if err != nil {
		return nil, err
	}
	resp, err := api.Embeddings.New(ctx, openai.EmbeddingNewParams{
		Model: "local",
		Input: openai.EmbeddingNewParamsInputUnion{OfArrayOfStrings: texts},
	})
	if err != nil {
		return nil, err
	}
	return shorten(resp.Data, len(texts)), nil
}

// healthy waits until the server has loaded its model.
func healthy(ctx context.Context, base string, exited <-chan struct{}) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	tick := time.NewTicker(200 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-exited:
			return errors.New("it stopped while loading the model")
		case <-tick.C:
			req, _ := http.NewRequestWithContext(ctx, http.MethodGet, base+"/health", nil)
			if resp, err := http.DefaultClient.Do(req); err == nil {
				resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					return nil
				}
			}
		}
	}
}

func freePort() (string, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	defer l.Close()
	_, port, err := net.SplitHostPort(l.Addr().String())
	return port, err
}

// tail keeps the end of a helper's output, for an error worth reading.
type tail struct {
	mu  sync.Mutex
	buf []byte
}

func (t *tail) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.buf = append(t.buf, p...)
	if len(t.buf) > 4096 {
		t.buf = t.buf[len(t.buf)-4096:]
	}
	return len(p), nil
}

// last is the final few lines written.
func (t *tail) last() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	lines := strings.Split(strings.TrimSpace(string(t.buf)), "\n")
	return strings.Join(lines[max(0, len(lines)-3):], " ")
}

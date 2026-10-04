package insights

import (
	"bufio"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	copilot "github.com/github/copilot-sdk/go"
	"github.com/github/copilot-sdk/go/rpc"
)

// Model is one Copilot model the signed-in account may use.
type Model struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// pilot asks GitHub Copilot through the Copilot SDK, which drives the pinned
// copilot CLI. The CLI starts on first use and lives while Copilot is chosen.
type pilot struct {
	bin, home, model string

	mu     sync.Mutex
	client *copilot.Client
}

// sdk is a Copilot client with no tools and no repository context: it only
// ever answers the prompt it is given. Its sessions live in home, apart from
// the person's own Copilot CLI history.
func sdk(bin, home string) *copilot.Client {
	return copilot.NewClient(&copilot.ClientOptions{
		Connection:    copilot.StdioConnection{Path: bin},
		BaseDirectory: home,
		LogLevel:      "error",
		Mode:          copilot.ModeEmpty,
	})
}

func (p *pilot) start(ctx context.Context) (*copilot.Client, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.client == nil {
		c := sdk(p.bin, p.home)
		if err := c.Start(ctx); err != nil {
			return nil, fmt.Errorf("GitHub Copilot did not start: %w", err)
		}
		p.client = c
	}
	return p.client, nil
}

func (p *pilot) close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.client != nil {
		_ = p.client.Stop()
		p.client = nil
	}
}

func (p *pilot) ask(ctx context.Context, q prompt) (string, error) {
	c, err := p.start(ctx)
	if err != nil {
		return "", err
	}
	// Signed out, the CLI would only say that it could not resolve a model.
	if status, err := c.GetAuthStatus(ctx); err == nil && !status.IsAuthenticated {
		return "", errors.New("GitHub Copilot не підключено. Підключіть його в параметрах, у розділі «AI та архів»")
	}
	// The SDK gives up after a minute unless told otherwise; a long meeting
	// on a slow model takes longer.
	if _, set := ctx.Deadline(); !set {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 5*time.Minute)
		defer cancel()
	}
	instructions := q.instructions
	if q.schema != nil {
		shape, _ := json.Marshal(q.schema)
		instructions += "\n\nReply with one JSON object that matches this JSON Schema and nothing else, no code fence:\n" + string(shape)
	}
	session, err := c.CreateSession(ctx, &copilot.SessionConfig{
		ClientName: "Meeting Transcriber",
		// Left empty, the CLI would use its own default model, not Auto.
		Model:            cmp.Or(p.model, "auto"),
		SystemMessage:    &copilot.SystemMessageConfig{Mode: "replace", Content: instructions},
		AvailableTools:   []string{},
		WorkingDirectory: p.home,
		OnPermissionRequest: func(copilot.PermissionRequest, copilot.PermissionInvocation) (rpc.PermissionDecision, error) {
			return &rpc.PermissionDecisionReject{}, nil
		},
	})
	if err != nil {
		p.close() // the next call starts a fresh CLI
		return "", fmt.Errorf("GitHub Copilot: %w", err)
	}
	// A transcript is not left behind in Copilot's session history.
	defer func() {
		done, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancel()
		_ = session.Disconnect()
		_ = c.DeleteSession(done, session.SessionID)
	}()
	// SendAndWait keeps only the English text of a failure; its kind comes with
	// the event, to this handler first because it is registered first.
	var quota atomic.Bool
	defer session.On(func(e copilot.SessionEvent) {
		if d, ok := e.Data.(*copilot.SessionErrorData); ok && d.ErrorType == "quota" {
			quota.Store(true)
		}
	})()
	reply, err := session.SendAndWait(ctx, copilot.MessageOptions{Prompt: q.input})
	if quota.Load() {
		return "", errors.New("Квоту GitHub Copilot вичерпано. Оберіть інший AI у параметрах або дочекайтеся, поки квота оновиться")
	}
	if err != nil {
		return "", fmt.Errorf("GitHub Copilot: %w", err)
	}
	var said *copilot.AssistantMessageData
	if reply != nil {
		said, _ = reply.Data.(*copilot.AssistantMessageData)
	}
	if said == nil {
		return "", errors.New("GitHub Copilot returned no answer")
	}
	if q.schema == nil {
		return said.Content, nil
	}
	return unfence(said.Content)
}

// unfence finds the JSON object in a reply: asked for JSON alone, models still
// wrap it in a code fence now and then.
func unfence(reply string) (string, error) {
	start, end := strings.Index(reply, "{"), strings.LastIndex(reply, "}")
	if start < 0 || end < start || !json.Valid([]byte(reply[start:end+1])) {
		return "", fmt.Errorf("the model did not answer with JSON: %.120q", reply)
	}
	return reply[start : end+1], nil
}

// CopilotAccount says who is signed in to GitHub Copilot and which models they
// may use. An empty login means nobody is. Auto is left out: it is what no
// choice at all means.
func CopilotAccount(ctx context.Context, bin, home string) (string, []Model, error) {
	c := sdk(bin, home)
	if err := c.Start(ctx); err != nil {
		return "", nil, fmt.Errorf("GitHub Copilot did not start: %w", err)
	}
	defer c.Stop()
	status, err := c.GetAuthStatus(ctx)
	if err != nil || !status.IsAuthenticated || status.Login == nil {
		return "", nil, err
	}
	found, err := c.ListModels(ctx)
	if err != nil {
		return *status.Login, nil, err
	}
	models := []Model{}
	for _, m := range found {
		if m.ID != "auto" && (m.Policy == nil || m.Policy.State == "enabled") {
			models = append(models, Model{ID: m.ID, Name: m.Name})
		}
	}
	return *status.Login, models, nil
}

// CopilotLogin runs the copilot CLI's own sign-in: it opens the browser and
// the person approves access on GitHub. Each line the CLI prints goes to say,
// since it may carry a link or a code to type in.
func CopilotLogin(ctx context.Context, bin, home string, say func(string)) error {
	// The SDK runs the CLI without the macOS keychain (ModeEmpty), so the token
	// must be kept in the CLI's config under home. Stored in the keychain, it
	// was only ever found through the gh fallback, which a Dock-launched app has
	// no PATH to. Without a keychain the CLI asks before writing the token to
	// its config, at a terminal only; storeTokenPlaintext is that answer.
	path := filepath.Join(home, "settings.json")
	settings := map[string]any{}
	if data, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(data, &settings); err != nil {
			return fmt.Errorf("the Copilot settings could not be read: %w", err)
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	settings["storeTokenPlaintext"] = true
	data, _ := json.Marshal(settings)
	if err := os.MkdirAll(home, 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, bin, "--no-auto-update", "login")
	cmd.Env = append(os.Environ(), "COPILOT_HOME="+home, "COPILOT_DISABLE_KEYTAR=1")
	out, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = cmd.Stdout
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("the GitHub sign-in could not start: %w", err)
	}
	for lines := bufio.NewScanner(out); lines.Scan(); {
		if line := strings.TrimSpace(lines.Text()); line != "" {
			say(line)
		}
	}
	return cmd.Wait()
}

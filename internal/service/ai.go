package service

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/dmykolen/meeting-transcriber-go/internal/home"
	"github.com/dmykolen/meeting-transcriber-go/internal/insights"
	"github.com/dmykolen/meeting-transcriber-go/internal/models"
)

// AIState is what the settings screen shows about the AI in use.
type AIState struct {
	Ready      bool     `json:"ready"`      // summaries and answers work
	Searchable bool     `json:"searchable"` // search by meaning works
	Fetching   string   `json:"fetching"`   // what is downloading, in words
	Fraction   float64  `json:"fraction"`
	Indexing   string   `json:"indexing"`  // which search vectors are being made, in words
	Indexed    float64  `json:"indexed"`   // how far, 0..1
	SigningIn  bool     `json:"signingIn"` // the GitHub sign-in is waiting in the browser
	Said       []string `json:"said"`      // its last lines, which may hold a link or a code
	Problem    string   `json:"problem"`
}

// CopilotAccount is who is signed in to GitHub Copilot and what they may use.
type CopilotAccount struct {
	Login  string           `json:"login"`
	Models []insights.Model `json:"models"`
}

// ApplyAI builds the model client the settings describe and puts it to work.
// Whatever it still needs is fetched in the background, after which it is
// applied again.
func (m *Meetings) ApplyAI() {
	m.mu.Lock()
	ai, setup := m.config.AI, m.setup()
	m.mu.Unlock()

	if missing := wanted(ai).Missing(home.Models(m.dir)); len(missing) > 0 {
		go m.fetch(missing)
	}
	m.aiMu.Lock()
	m.noServer = (ai.Provider == "local" || ai.Embeddings == "local") && setup.Server == ""
	m.aiMu.Unlock()
	client := insights.New(setup)
	m.lib.Brain(client).Close()
	slog.Info("AI", "provider", ai.Provider, "ready", client.Ready(),
		"embeddings", ai.Embeddings, "searchable", client.Searchable())
	if client.Vectors == "" {
		return
	}
	switch changed, err := m.db.Vectors(client.Vectors); {
	case err != nil:
		slog.Error("could not check which model made the search vectors", "err", err)
	case changed:
		// The old vectors were forgotten. Remake the ones Search and Ask read
		// behind the window, so that the first question does not wait for them.
		go func() {
			err := m.reindex(context.Background(), false)
			slog.Info("search vectors remade for the new model", "err", err)
		}()
	}
}

// reindex makes the missing search vectors — transcript passages when asked,
// then the documents Search and Ask read — and shows how far it has got. One
// runs at a time.
func (m *Meetings) reindex(ctx context.Context, passages bool) error {
	m.aiMu.Lock()
	if m.ai.Indexing != "" {
		m.aiMu.Unlock()
		return errors.New("Пошук за змістом уже оновлюється")
	}
	m.aiMu.Unlock()
	defer m.indexing("", 0, 1)
	if passages {
		n, err := m.lib.Reindex(ctx, func(done, total int) { m.indexing("розшифровки", done, total) })
		slog.Info("transcript passages indexed", "recordings", n, "err", err)
		if err != nil {
			return err
		}
	}
	return m.lib.Learn(ctx, func(done, total int) { m.indexing("зустрічі, нотатки й проєкти", done, total) })
}

func (m *Meetings) indexing(what string, done, total int) {
	m.aiMu.Lock()
	m.ai.Indexing, m.ai.Indexed = what, float64(done)/float64(max(total, 1))
	m.aiMu.Unlock()
}

// setup resolves the settings to files on disk. Callers hold m.mu.
func (m *Meetings) setup() insights.Setup {
	dir := home.Models(m.dir)
	present := func(model models.Model) string {
		if models.Have(dir, model) {
			return models.Path(dir, model)
		}
		return ""
	}
	return insights.Setup{
		Language:     m.config.Language,
		Provider:     m.config.AI.Provider,
		Embeddings:   m.config.AI.Embeddings,
		OpenAIKey:    m.config.OpenAIKey,
		OpenAIModel:  m.config.OpenAIModel,
		CopilotModel: m.config.AI.CopilotModel,
		Copilot:      present(models.Copilot),
		CopilotHome:  home.Copilot(m.dir),
		Server:       llamaServer(),
		Chat:         present(chat(m.config.AI)),
		Vectors:      present(models.Vectors),
		Usage:        m.called,
	}
}

// wanted is what the AI settings need on disk.
func wanted(ai home.AI) models.Set {
	var set models.Set
	switch ai.Provider {
	case "copilot":
		set = append(set, models.Copilot)
	case "local":
		set = append(set, chat(ai))
	}
	if ai.Embeddings == "local" {
		set = append(set, models.Vectors)
	}
	return set
}

// chat is the local model: the one somebody linked, or the built-in one.
func chat(ai home.AI) models.Model {
	if custom, err := models.Custom(ai.LocalModel); err == nil {
		return custom
	}
	return models.Chat
}

// fetch downloads AI assets one at a time and applies the settings again. A
// second call while one runs does nothing: the first applies the settings when
// it finishes, which starts whatever is still missing.
func (m *Meetings) fetch(set models.Set) {
	m.aiMu.Lock()
	if m.fetching {
		m.aiMu.Unlock()
		return
	}
	m.fetching, m.ai.Problem = true, ""
	m.aiMu.Unlock()

	report := make(chan models.Progress, 8)
	go models.Fetch(context.Background(), home.Models(m.dir), set, report)
	var failed error
	for p := range report {
		m.aiMu.Lock()
		switch {
		case p.Err != nil:
			failed = p.Err
			m.ai.Problem = "Не вдалося завантажити: " + p.Err.Error()
		case !p.Finished:
			m.ai.Fetching, m.ai.Fraction = p.Model, p.Fraction()
		}
		m.aiMu.Unlock()
	}
	m.aiMu.Lock()
	m.fetching, m.ai.Fetching, m.ai.Fraction = false, "", 0
	m.aiMu.Unlock()
	if failed != nil {
		slog.Error("AI download failed", "err", failed)
		return
	}
	m.ApplyAI()
}

// AIStatus is polled by the settings screen.
func (m *Meetings) AIStatus() AIState {
	client := m.lib.AI()
	m.aiMu.Lock()
	defer m.aiMu.Unlock()
	state := m.ai
	state.Said = append([]string{}, m.ai.Said...)
	state.Ready, state.Searchable = client.Ready(), client.Searchable()
	switch {
	case state.Problem != "":
	case m.noServer:
		state.Problem = "Локальний AI недоступний: у цій збірці немає llama-server"
	default:
		state.Problem = client.Problem()
	}
	return state
}

// ConnectCopilot signs in to GitHub Copilot in the browser. It returns at once;
// AIStatus follows the sign-in.
func (m *Meetings) ConnectCopilot() error {
	dir := home.Models(m.dir)
	if !models.Have(dir, models.Copilot) {
		return errors.New("GitHub Copilot ще завантажується")
	}
	m.aiMu.Lock()
	if m.ai.SigningIn {
		m.aiMu.Unlock()
		return nil
	}
	m.ai.SigningIn, m.ai.Said, m.ai.Problem = true, nil, ""
	m.aiMu.Unlock()

	slog.Info("GitHub Copilot sign-in started")
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		err := insights.CopilotLogin(ctx, models.Path(dir, models.Copilot), home.Copilot(m.dir), func(line string) {
			m.aiMu.Lock()
			said := append(m.ai.Said, line)
			m.ai.Said = said[max(0, len(said)-4):]
			m.aiMu.Unlock()
		})
		m.aiMu.Lock()
		m.ai.SigningIn = false
		if err != nil {
			m.ai.Problem = "Вхід у GitHub не завершено: " + err.Error()
		}
		m.aiMu.Unlock()
		slog.Info("GitHub Copilot sign-in finished", "err", err)
		// A new account means a new CLI session.
		m.ApplyAI()
	}()
	return nil
}

// Copilot reports the signed-in GitHub account and its models. It starts the
// Copilot CLI for a moment, so the settings screen asks once when it opens.
func (m *Meetings) Copilot() (CopilotAccount, error) {
	dir := home.Models(m.dir)
	if !models.Have(dir, models.Copilot) {
		return CopilotAccount{Models: []insights.Model{}}, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	login, found, err := insights.CopilotAccount(ctx, models.Path(dir, models.Copilot), home.Copilot(m.dir))
	if found == nil {
		found = []insights.Model{}
	}
	return CopilotAccount{Login: login, Models: found}, err
}

// llamaServer finds llama-server where audiotee is found: beside the executable, as
// in the .app bundle and in build/, or where MT_LLAMA_SERVER says.
func llamaServer() string {
	if bin := os.Getenv("MT_LLAMA_SERVER"); bin != "" {
		return bin
	}
	self, err := os.Executable()
	if err != nil {
		return ""
	}
	for _, candidate := range []string{
		filepath.Join(filepath.Dir(self), "llama-server"),
		filepath.Join(filepath.Dir(self), "..", "Resources", "llama-server"),
	} {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return ""
}

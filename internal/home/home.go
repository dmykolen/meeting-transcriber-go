// Package home owns the app-managed filesystem layout under
// ~/MeetingTranscriber or MT_HOME.
package home

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

const Folder = "MeetingTranscriber"

func Dir() (string, error) {
	if override := os.Getenv("MT_HOME"); override != "" {
		return override, os.MkdirAll(override, 0o755)
	}
	base, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, Folder), os.MkdirAll(filepath.Join(base, Folder), 0o755)
}

func Recordings(dir string) string { return sub(dir, "recordings") }
func Models(dir string) string     { return sub(dir, "models") }
func Logs(dir string) string       { return sub(dir, "logs") }
func Cache(dir string) string    { return sub(dir, "cache") }
func Copilot(dir string) string  { return sub(dir, "copilot") }
func Database(dir string) string { return filepath.Join(dir, "meetings.db") }

func sub(dir, name string) string {
	path := filepath.Join(dir, name)
	_ = os.MkdirAll(path, 0o755)
	return path
}

// Config is the app's persisted settings.
type Config struct {
	Language string `toml:"language"`
	OpenAIKey   string `toml:"openai_key"`
	OpenAIModel string `toml:"openai_model"`
	Summarise Choice `toml:"summarise"`
	Density string `toml:"density"`
	Transcriber string `toml:"transcriber"`
	UILanguage  string `toml:"ui_language"` // "uk" or "en", the interface's own language
	Me string `toml:"me"`
	Listen Listen `toml:"listen"`
	Keep   Keep   `toml:"keep"`
	AI     AI     `toml:"ai"`
	Queue  Queue  `toml:"queue"`
}

// Queue says when recordings are transcribed: "after" each one, "at" a time of
// day, or when the Mac is "idle".
type Queue struct {
	When string `toml:"when"`
	At   string `toml:"at"` // "19:00"
}

// Whens are the values Queue.When accepts.
var Whens = []string{"after", "at", "idle"}

// AI says where summaries, answers and search vectors are made.
type AI struct {
	Provider     string `toml:"provider"`      // "openai", "copilot" or "local"
	CopilotModel string `toml:"copilot_model"` // empty lets Copilot choose
	LocalModel   string `toml:"local_model"`   // a .gguf link; empty is the built-in small model
	Embeddings   string `toml:"embeddings"`    // "openai" or "local"
}

// Providers and Embedders are the values the AI settings accept.
var (
	Providers = []string{"openai", "copilot", "local"}
	Embedders = []string{"openai", "local"}
)

// Listen configures always-on capture.
type Listen struct {
	Enabled bool `toml:"enabled"`
	System bool `toml:"system"`
	KeepNotes bool `toml:"keep_notes"`
	StartSpeech Duration `toml:"start_speech"`
	QuietEnds   Duration `toml:"quiet_ends"`
	Preroll     Duration `toml:"preroll"`
	Ring        Duration `toml:"ring"`
}

// Keep configures retention.
type Keep struct {
	AudioDays int `toml:"audio_days"`
}

// Choice is a string setting that used to be a bool.
type Choice string

func (c *Choice) UnmarshalTOML(v any) error {
	switch value := v.(type) {
	case string:
		*c = Choice(value)
	case bool:
		*c = "never"
		if value {
			*c = "always"
		}
	default:
		return fmt.Errorf("%v is not a setting this understands", v)
	}
	return nil
}

func (c Choice) MarshalTOML() ([]byte, error) { return []byte(`"` + string(c) + `"`), nil }

// Duration uses Go duration strings like "20s" or "3m".
type Duration struct{ time.Duration }

func (d *Duration) UnmarshalText(text []byte) error {
	parsed, err := time.ParseDuration(string(text))
	if err != nil {
		return fmt.Errorf("%q is not a duration like \"3m\": %w", text, err)
	}
	d.Duration = parsed
	return nil
}

func (d Duration) MarshalText() ([]byte, error) { return []byte(d.String()), nil }

// Auto means detect the spoken language from audio.
const Auto = "auto"

// Spoken returns the model-facing language value.
func (c Config) Spoken() string {
	if strings.EqualFold(c.Language, Auto) {
		return ""
	}
	return c.Language
}

// Defaults are the fresh-install settings.
func Defaults() Config {
	return Config{
		Language:    "uk",
		Transcriber: "whisper",
		UILanguage:  "uk",
		OpenAIModel: "gpt-5.4-mini",
		Summarise:   "meetings",
		Density:     "compact",
		Listen: Listen{
			Enabled:     true,
			System:      true,
			KeepNotes:   false,
			StartSpeech: Duration{20 * time.Second},
			QuietEnds:   Duration{3 * time.Minute},
			Preroll:     Duration{5 * time.Minute},
			Ring:        Duration{10 * time.Minute},
		},
		Keep:  Keep{AudioDays: 30},
		AI:    AI{Provider: "openai", Embeddings: "openai"},
		Queue: Queue{When: "after", At: "19:00"},
	}
}

// Load reads the config and writes a commented default on first run.
func Load(dir string) (Config, error) {
	path := filepath.Join(dir, "config.toml")
	cfg := Defaults()

	switch _, err := os.Stat(path); {
	case os.IsNotExist(err):
		// A first run speaks the language of the Mac; an older settings file
		// without the key keeps the Ukrainian it always had.
		cfg.UILanguage = system()
		return cfg, write(path, cfg)
	case err != nil:
		return cfg, err
	}
	// Keep decoded values and fall back to defaults for the rest.
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		slog.Warn("some settings could not be read and are back at their defaults",
			"file", path, "err", err)
	}
	// Empty in the file still means "not set".
	if cfg.OpenAIKey == "" {
		cfg.OpenAIKey = os.Getenv("OPENAI_API_KEY")
	}
	// Empty language means "use the default", not "detect it".
	if cfg.Language == "" {
		cfg.Language = Defaults().Language
	}
	if cfg.Transcriber == "" {
		cfg.Transcriber = Defaults().Transcriber
	}
	if cfg.Summarise == "" {
		cfg.Summarise = Defaults().Summarise
	}
	// A value this build does not know must not switch AI off or stop startup.
	if !slices.Contains(Providers, cfg.AI.Provider) {
		cfg.AI.Provider = Defaults().AI.Provider
	}
	if !slices.Contains(Embedders, cfg.AI.Embeddings) {
		cfg.AI.Embeddings = Defaults().AI.Embeddings
	}
	if !slices.Contains(Whens, cfg.Queue.When) {
		cfg.Queue.When = Defaults().Queue.When
	}
	if _, err := time.Parse("15:04", cfg.Queue.At); err != nil {
		cfg.Queue.At = Defaults().Queue.At
	}
	if !slices.Contains(UILanguages, cfg.UILanguage) {
		cfg.UILanguage = Defaults().UILanguage
	}
	return cfg, nil
}

// UILanguages are the languages the interface is written in.
var UILanguages = []string{"uk", "en"}

// system is "uk" when Ukrainian is among the Mac's preferred languages, and
// "en" otherwise.
func system() string {
	listed, _ := exec.Command("defaults", "read", "-g", "AppleLanguages").Output()
	if strings.Contains(string(listed), `"uk`) {
		return "uk"
	}
	return "en"
}

// Save writes the config back to disk.
func Save(dir string, cfg Config) error {
	return write(filepath.Join(dir, "config.toml"), cfg)
}

func write(path string, cfg Config) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	const header = `# Meeting Transcriber
#
# Everything this app owns lives in this folder. Delete it and the app is gone.
#
# openai_key    only for summaries, questions and semantic search. Transcription
#               and speaker detection run on this machine and never send audio
#               anywhere. Leave it empty and those features simply stay off.
# language      "uk", "en", … or "auto" to work it out per meeting. Naming it is
#               more accurate than detecting it, and it is what stops a
#               Ukrainian meeting being written down as Russian.
# summarise     "always", "meetings" or "never". A note nobody else was in is
#               usually a phone call or a thought said out loud.
# me            your name. Everything the microphone hears is you, so this is
#               applied directly rather than recognised.
# ui_language   "uk" or "en": the language of the interface itself.
# transcriber   "whisper" or "parakeet". Parakeet is faster and, on clean read
#               speech, more accurate on Ukrainian — but it picks the language
#               itself and cannot be told, and on a meeting recorded through a
#               room it found half the words Whisper did.
# [listen]      when a recording starts and stops on its own.
# [keep]        audio_days = 0 keeps recordings for ever.
# [ai]          provider "openai", "copilot" (your GitHub Copilot) or "local"
#               (a model on this Mac); embeddings "openai" or "local" make the
#               vectors behind search by meaning.
# [queue]       when recordings are transcribed: "after" each one, "at" a time
#               of day (at = "19:00"), or "idle" — once nobody has used the Mac
#               for five minutes and nothing else keeps it busy.

`
	if _, err := file.WriteString(header); err != nil {
		return err
	}
	return toml.NewEncoder(file).Encode(cfg)
}

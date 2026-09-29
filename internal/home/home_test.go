package home

import (
	"os"
	"path/filepath"
	"testing"
)

// Older settings files must not prevent startup.
func TestSettingsFromAnOlderVersionStillOpen(t *testing.T) {
	dir := t.TempDir()
	old := `
language = "uk"
openai_key = "sk-test"
summarise = true

[listen]
  enabled = true
  start_speech = "20s"
`
	if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("an old settings file stopped the app: %v", err)
	}
	if cfg.Summarise != "always" {
		t.Fatalf("summarise = %q; the old true meant every recording", cfg.Summarise)
	}
	if cfg.OpenAIKey != "sk-test" || cfg.Language != "uk" {
		t.Fatalf("the rest of the settings did not survive: %+v", cfg)
	}
	if !cfg.Listen.Enabled || cfg.Listen.StartSpeech.Seconds() != 20 {
		t.Fatalf("the listening settings did not survive: %+v", cfg.Listen)
	}
}

func TestAnUnreadableSettingCostsTheSettingAndNotTheApp(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config.toml"),
		[]byte("language = 42\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("a broken settings file stopped the app: %v", err)
	}
	if cfg.Language != Defaults().Language {
		t.Fatalf("language = %q, want the default", cfg.Language)
	}
}

func TestAScheduleThisBuildCannotReadTranscribesAfterEachRecording(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config.toml"),
		[]byte("[queue]\nwhen = \"sometimes\"\nat = \"25:61\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("a broken schedule stopped the app: %v", err)
	}
	if cfg.Queue != Defaults().Queue {
		t.Fatalf("queue = %+v, want the defaults", cfg.Queue)
	}
}

func TestAutoIsTheOnlyWayToAskForDetection(t *testing.T) {
	dir := t.TempDir()
	cfg, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Language != "uk" || cfg.Spoken() != "uk" {
		t.Fatalf("a fresh install speaks %q", cfg.Language)
	}
	cfg.Language = Auto
	if cfg.Spoken() != "" {
		t.Fatalf("auto gave the model %q instead of letting it detect", cfg.Spoken())
	}
}

func TestSettingsSurviveBeingWrittenAndReadBack(t *testing.T) {
	dir := t.TempDir()
	cfg := Defaults()
	cfg.Summarise = "never"
	cfg.Listen.KeepNotes = true
	if err := Save(dir, cfg); err != nil {
		t.Fatal(err)
	}
	back, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if back.Summarise != "never" || !back.Listen.KeepNotes {
		t.Fatalf("read back as %q / keep_notes %v", back.Summarise, back.Listen.KeepNotes)
	}
}

// A settings file from before the AI choice keeps OpenAI, as it always was.
func TestSettingsWithoutAnAIChoiceKeepOpenAI(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte(`openai_key = "sk-test"`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AI.Provider != "openai" || cfg.AI.Embeddings != "openai" {
		t.Fatalf("AI = %+v; an old file must keep working the way it did", cfg.AI)
	}
}

func TestAnUnknownAIChoiceFallsBackAndKeepsTheRest(t *testing.T) {
	dir := t.TempDir()
	written := `
[ai]
provider = "gemini"
local_model = "owner/repo/model.gguf"
embeddings = "local"
`
	if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte(written), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AI.Provider != "openai" {
		t.Fatalf("provider = %q; an unknown one must fall back rather than switch AI off", cfg.AI.Provider)
	}
	if cfg.AI.Embeddings != "local" || cfg.AI.LocalModel != "owner/repo/model.gguf" {
		t.Fatalf("the valid AI settings were lost with the bad one: %+v", cfg.AI)
	}
}

func TestAISettingsSurviveASave(t *testing.T) {
	dir := t.TempDir()
	cfg := Defaults()
	cfg.AI = AI{Provider: "copilot", CopilotModel: "gpt-5-mini", Embeddings: "local"}
	if err := Save(dir, cfg); err != nil {
		t.Fatal(err)
	}
	back, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if back.AI != cfg.AI {
		t.Fatalf("saved %+v, read back %+v", cfg.AI, back.AI)
	}
}

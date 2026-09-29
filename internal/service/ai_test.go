package service

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/dmykolen/meeting-transcriber-go/internal/home"
	"github.com/dmykolen/meeting-transcriber-go/internal/library"
	"github.com/dmykolen/meeting-transcriber-go/internal/store"
)

func settled(t *testing.T) *Meetings {
	t.Helper()
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "meetings.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	m := New(db, library.New(db, nil, nil, home.Recordings(dir)), dir, home.Defaults())
	m.ApplyAI()
	return m
}

// A key typed into the settings used to wait for a restart while the screens
// already said AI was on.
func TestAKeySavedInSettingsWorksWithoutARestart(t *testing.T) {
	m := settled(t)
	if m.Summaries() {
		t.Fatal("AI claims to be on without a key")
	}
	s := m.Settings()
	s.OpenAIKey = "sk-test"
	if err := m.SaveSettings(s); err != nil {
		t.Fatal(err)
	}
	for deadline := time.Now().Add(2 * time.Second); !m.Summaries() && time.Now().Before(deadline); {
		time.Sleep(10 * time.Millisecond)
	}
	if !m.Summaries() || !m.AIStatus().Searchable {
		t.Fatal("the saved key did not reach summaries and search")
	}
}

func TestAnUnknownAIChoiceIsRefused(t *testing.T) {
	m := settled(t)
	for _, change := range []func(*Settings){
		func(s *Settings) { s.AIProvider = "gemini" },
		func(s *Settings) { s.Embeddings = "somewhere" },
		func(s *Settings) { s.AIProvider, s.LocalModel = "local", "https://example.com/model.bin" },
	} {
		s := m.Settings()
		change(&s)
		if err := m.SaveSettings(s); err == nil {
			t.Fatalf("%+v was saved", s)
		}
	}
	if got := m.Settings(); got.AIProvider != "openai" || got.LocalModel != "" {
		t.Fatalf("a refused choice still changed the settings: %+v", got)
	}
}

func TestAScheduleNeedsAKnownChoiceAndARealTime(t *testing.T) {
	m := settled(t)
	for _, change := range []func(*Settings){
		func(s *Settings) { s.Transcribe = "sometimes" },
		func(s *Settings) { s.Transcribe, s.TranscribeAt = "at", "7pm" },
	} {
		s := m.Settings()
		change(&s)
		if err := m.SaveSettings(s); err == nil {
			t.Fatalf("%+v was saved", s)
		}
	}
	s := m.Settings()
	s.Transcribe, s.TranscribeAt = "at", "20:30"
	if err := m.SaveSettings(s); err != nil {
		t.Fatal(err)
	}
	if back, err := home.Load(m.dir); err != nil || back.Queue != (home.Queue{When: "at", At: "20:30"}) {
		t.Fatalf("the schedule on disk is %+v, %v", back.Queue, err)
	}
}

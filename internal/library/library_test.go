package library

import (
	"context"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dmykolen/meeting-transcriber-go/internal/engine"
	"github.com/dmykolen/meeting-transcriber-go/internal/insights"
	"github.com/dmykolen/meeting-transcriber-go/internal/store"
)

// fakeEngine stands in for the models while the rest of the pipeline stays real.
type fakeEngine struct {
	turns  []engine.Turn
	voices map[string][]float32
	err    error
	calls  int
	solo   bool
}

func (f *fakeEngine) Run(_, _ []float32) (engine.Result, error) {
	f.calls++
	return engine.Result{Turns: f.turns, Voices: f.voices}, f.err
}

func (f *fakeEngine) Solo(_ []float32, who string) (engine.Result, error) {
	f.calls++
	f.solo = true
	turns := append([]engine.Turn(nil), f.turns...)
	for i := range turns {
		turns[i].Speaker = who
	}
	return engine.Result{Turns: turns}, f.err
}

func setup(t *testing.T, e Engine) (*Library, *store.DB, string) {
	t.Helper()
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	recordings := filepath.Join(dir, "recordings")
	os.MkdirAll(recordings, 0o755)
	return New(db, e, insights.New("", "", "uk"), recordings), db, recordings
}

// silence writes a real, decodable WAV.
func silence(t *testing.T, dir, name string, seconds int) string {
	t.Helper()
	samples := 16000 * seconds
	out := make([]byte, 44+samples*2)
	copy(out[0:], "RIFF")
	binary.LittleEndian.PutUint32(out[4:], uint32(len(out)-8))
	copy(out[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(out[16:], 16)
	binary.LittleEndian.PutUint16(out[20:], 1)
	binary.LittleEndian.PutUint16(out[22:], 1)
	binary.LittleEndian.PutUint32(out[24:], 16000)
	binary.LittleEndian.PutUint32(out[28:], 32000)
	binary.LittleEndian.PutUint16(out[32:], 2)
	binary.LittleEndian.PutUint16(out[34:], 16)
	copy(out[36:], "data")
	binary.LittleEndian.PutUint32(out[40:], uint32(samples*2))

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, out, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestARecordingBecomesASearchableTranscript(t *testing.T) {
	fake := &fakeEngine{turns: []engine.Turn{
		{Start: 0, End: 2, Speaker: "SPEAKER_00", Text: "документи для Northwind"},
		{Start: 2, End: 4, Speaker: "SPEAKER_01", Text: "скрипти для витягування даних"},
	}}
	lib, db, dir := setup(t, fake)
	silence(t, dir, "a.wav", 4)

	r, err := lib.Add(store.Meeting, "a.wav", time.Now(), "")
	if err != nil {
		t.Fatal(err)
	}
	if err := lib.process(context.Background(), r.ID); err != nil {
		t.Fatal(err)
	}

	got, err := db.Get(r.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != store.Done {
		t.Fatalf("status is %q, want done", got.Status)
	}
	if got.Turns != 2 || len(got.Speakers) != 2 {
		t.Fatalf("turns=%d speakers=%v", got.Turns, got.Speakers)
	}
	if hits, _ := db.Search("northwind", 5); len(hits) != 1 {
		t.Fatalf("the transcript is not searchable: %d hits", len(hits))
	}
}

func TestAnUnreadableFileFailsThatRecordingAndNothingElse(t *testing.T) {
	fake := &fakeEngine{turns: []engine.Turn{{Text: "fine"}}}
	lib, db, dir := setup(t, fake)

	os.WriteFile(filepath.Join(dir, "broken.wav"), []byte("not audio"), 0o644)
	silence(t, dir, "good.wav", 2)

	bad, _ := lib.Add(store.Meeting, "broken.wav", time.Now().Add(-time.Hour), "")
	good, _ := lib.Add(store.Meeting, "good.wav", time.Now(), "")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	done := make(chan struct{})
	go func() { defer close(done); lib.Run(ctx) }()

	deadline := time.After(5 * time.Second)
	for {
		g, _ := db.Get(good.ID)
		b, _ := db.Get(bad.ID)
		if g.Status == store.Done && b.Status == store.Failed {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("good=%q bad=%q — the queue stalled on the bad one", g.Status, b.Status)
		case <-time.After(50 * time.Millisecond):
		}
	}
	cancel()
	<-done

	b, _ := db.Get(bad.ID)
	if b.Problem == "" {
		t.Fatal("a failed recording must say why")
	}
}

func TestSilenceFinishesRatherThanFailing(t *testing.T) {
	lib, db, dir := setup(t, &fakeEngine{turns: nil})
	silence(t, dir, "quiet.wav", 3)

	r, _ := lib.Add(store.Meeting, "quiet.wav", time.Now(), "")
	if err := lib.process(context.Background(), r.ID); err != nil {
		t.Fatal(err)
	}
	got, _ := db.Get(r.ID)
	if got.Status != store.Done {
		t.Fatalf("status is %q; an empty room is not a failure", got.Status)
	}
	if got.Turns != 0 {
		t.Fatalf("%d turns from silence", got.Turns)
	}
}

func TestATranscriptSurvivesTheSpeakerModelFailing(t *testing.T) {
	fake := &fakeEngine{
		turns: []engine.Turn{{Start: 0, End: 1, Text: "щось сказали"}},
		err:   errors.New("speaker model unavailable"),
	}
	lib, db, dir := setup(t, fake)
	silence(t, dir, "a.wav", 2)

	r, _ := lib.Add(store.Meeting, "a.wav", time.Now(), "")
	if err := lib.process(context.Background(), r.ID); err != nil {
		t.Fatalf("the whole recording failed for want of speakers: %v", err)
	}
	got, _ := db.Get(r.ID)
	if got.Status != store.Done || got.Turns != 1 {
		t.Fatalf("status=%q turns=%d", got.Status, got.Turns)
	}
}

func TestAnInterruptedRecordingIsPickedUpAgain(t *testing.T) {
	fake := &fakeEngine{turns: []engine.Turn{{Text: "продовжили"}}}
	lib, db, dir := setup(t, fake)
	silence(t, dir, "a.wav", 2)

	r, _ := lib.Add(store.Meeting, "a.wav", time.Now(), "")
	db.Progress(r.ID, store.Transcribing, 0.4) // as a crash would leave it

	id, ok := lib.next()
	if !ok || id != r.ID {
		t.Fatalf("next() returned %d, %v; the half-done recording was skipped", id, ok)
	}
	if err := lib.process(context.Background(), r.ID); err != nil {
		t.Fatal(err)
	}
	if got, _ := db.Get(r.ID); got.Status != store.Done {
		t.Fatalf("status is %q", got.Status)
	}
}

func TestTheQueueRunsOldestFirst(t *testing.T) {
	lib, _, dir := setup(t, &fakeEngine{})
	silence(t, dir, "old.wav", 1)
	silence(t, dir, "new.wav", 1)

	older, _ := lib.Add(store.Meeting, "old.wav", time.Now().Add(-time.Hour), "")
	lib.Add(store.Meeting, "new.wav", time.Now(), "")

	if id, _ := lib.next(); id != older.ID {
		t.Fatalf("next() chose %d, want the older %d", id, older.ID)
	}
}

func TestDeletingIsRecoverableAndEmptyingIsNot(t *testing.T) {
	lib, db, dir := setup(t, &fakeEngine{})
	path := silence(t, dir, "a.wav", 1)
	r, _ := lib.Add(store.Meeting, "a.wav", time.Now(), "")

	if err := lib.Delete(r.ID); err != nil {
		t.Fatal(err)
	}
	if recent, _ := db.Recent(10); len(recent) != 0 {
		t.Fatal("a deleted recording is still in the Library")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("the audio was destroyed before the bin was emptied")
	}
	if binned, _ := db.Bin(); len(binned) != 1 {
		t.Fatalf("the bin holds %d recordings, want 1", len(binned))
	}

	if err := lib.Restore(r.ID); err != nil {
		t.Fatal(err)
	}
	if recent, _ := db.Recent(10); len(recent) != 1 {
		t.Fatal("restoring did not bring it back to the Library")
	}

	_ = lib.Delete(r.ID)
	if gone, err := lib.Empty(0); err != nil || gone != 1 {
		t.Fatalf("emptied %d (err %v), want 1", gone, err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("the audio file survived the bin being emptied")
	}
	if _, err := db.Get(r.ID); err == nil {
		t.Fatal("the recording is still in the database")
	}
}

func TestOnlyMeetingsAreWorthAModelCall(t *testing.T) {
	lib, _, dir := setup(t, &fakeEngine{})
	silence(t, dir, "a.wav", 1)
	note, _ := lib.Add(store.Note, "a.wav", time.Now(), "")
	meeting, _ := lib.Add(store.Meeting, "a.wav", time.Now(), "")
	turns := []engine.Turn{{Text: "щось"}}

	lib.Policy(store.Meetings)
	if lib.worth(note.ID, turns) {
		t.Fatal("a note nobody else was in was sent to the model")
	}
	if !lib.worth(meeting.ID, turns) {
		t.Fatal("a meeting was not summarised")
	}

	lib.Policy(store.Always)
	if !lib.worth(note.ID, turns) {
		t.Fatal("always should mean always")
	}
	lib.Policy(store.Never)
	if lib.worth(meeting.ID, turns) {
		t.Fatal("never should mean never")
	}
}

func TestAskingWithoutAKeySaysSoRatherThanFailingOddly(t *testing.T) {
	lib, _, dir := setup(t, &fakeEngine{turns: []engine.Turn{{Text: "Northwind документи"}}})
	silence(t, dir, "a.wav", 2)
	r, _ := lib.Add(store.Meeting, "a.wav", time.Now(), "")
	lib.process(context.Background(), r.ID)

	_, hits, err := lib.Ask(context.Background(), "Northwind")
	if err == nil {
		t.Fatal("an answer was produced with no key")
	}
	if !errors.Is(err, insights.ErrNoKey) {
		t.Fatalf("the error is %v, which does not explain itself", err)
	}
	if len(hits) == 0 {
		t.Fatal("the passages were found; only the answer needed a key")
	}
}

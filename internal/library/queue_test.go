package library

import (
	"testing"
	"time"

	"github.com/dmykolen/meeting-transcriber-go/internal/store"
)

var morning = time.Date(2026, 9, 28, 10, 0, 0, 0, time.Local)

func queued(t *testing.T, lib *Library, dir, name string, started time.Time) store.Recording {
	t.Helper()
	silence(t, dir, name, 1)
	r, err := lib.Add(store.Meeting, name, started, "")
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestAnHourInTheScheduleHoldsTheDayUntilThen(t *testing.T) {
	lib, db, dir := setup(t, &fakeEngine{})
	lib.Schedule("at", "19:00")
	day := queued(t, lib, dir, "day.wav", morning)
	evening := queued(t, lib, dir, "evening.wav", morning.Add(11*time.Hour)) // 21:00

	if id, ok := lib.next(morning.Add(8 * time.Hour)); ok {
		t.Fatalf("recording %d was processed at 18:00; the schedule says 19:00", id)
	}
	if why, until := lib.Waiting(day, morning.Add(8*time.Hour)); why != "time" || !until.Equal(morning.Add(9*time.Hour)) {
		t.Fatalf("waiting for %q until %v, want the time, 19:00", why, until)
	}
	if id, ok := lib.next(morning.Add(9 * time.Hour)); !ok || id != day.ID {
		t.Fatalf("at 19:00 next() = %d, %v; want the day's recording", id, ok)
	}

	// Recorded after the hour, it waits for the same hour tomorrow.
	db.Progress(day.ID, store.Done, 1)
	tomorrow := morning.AddDate(0, 0, 1).Add(9 * time.Hour)
	if id, ok := lib.next(morning.Add(12 * time.Hour)); ok {
		t.Fatalf("recording %d was processed at 22:00", id)
	}
	if _, until := lib.Waiting(evening, morning.Add(12*time.Hour)); !until.Equal(tomorrow) {
		t.Fatalf("the 21:00 recording waits until %v, want tomorrow 19:00", until)
	}
	if id, ok := lib.next(tomorrow); !ok || id != evening.ID {
		t.Fatalf("tomorrow at 19:00 next() = %d, %v", id, ok)
	}
}

func TestIdleWaitsUntilTheMacIsFree(t *testing.T) {
	lib, _, dir := setup(t, &fakeEngine{})
	lib.Schedule("idle", "19:00")
	r := queued(t, lib, dir, "a.wav", morning)

	for _, why := range []string{"user", "busy"} {
		lib.occupied = func() string { return why }
		if _, ok := lib.next(morning); ok {
			t.Fatalf("processed while the Mac was %s", why)
		}
		if got, _ := lib.Waiting(r, morning); got != why {
			t.Fatalf("waiting for %q, want %q", got, why)
		}
	}
	lib.occupied = func() string { return "" }
	if id, ok := lib.next(morning); !ok || id != r.ID {
		t.Fatalf("a free Mac did not start the queue: %d, %v", id, ok)
	}
}

// Right after a recording the load is the app's own. It must not stop the rest
// of the batch; another program's load must.
func TestABatchIsNotStoppedByItsOwnLoad(t *testing.T) {
	lib, _, dir := setup(t, &fakeEngine{})
	lib.Schedule("idle", "19:00")
	r := queued(t, lib, dir, "a.wav", morning)
	lib.occupied = func() string { return "busy" }

	lib.finished = time.Now()
	if id, ok := lib.next(time.Now()); !ok || id != r.ID {
		t.Fatalf("the batch stopped at its own load: %d, %v", id, ok)
	}
	lib.finished = time.Now().Add(-10 * time.Minute)
	if _, ok := lib.next(time.Now()); ok {
		t.Fatal("another program's load was ignored")
	}
}

func TestARecordingAskedForByHandSkipsTheScheduleAndTheLine(t *testing.T) {
	lib, db, dir := setup(t, &fakeEngine{})
	lib.Schedule("at", "19:00")
	older := queued(t, lib, dir, "older.wav", morning)
	wanted := queued(t, lib, dir, "wanted.wav", morning.Add(time.Hour))
	noon := morning.Add(2 * time.Hour)

	if err := lib.Rush(wanted.ID); err != nil {
		t.Fatal(err)
	}
	if id, ok := lib.next(noon); !ok || id != wanted.ID {
		t.Fatalf("next() = %d, %v; want the recording asked for", id, ok)
	}
	if why, _ := lib.Waiting(wanted, noon); why != "next" {
		t.Fatalf("the rushed recording waits for %q", why)
	}
	if why, _ := lib.Waiting(older, noon); why != "time" {
		t.Fatalf("the other recording waits for %q, want the time", why)
	}

	db.Progress(wanted.ID, store.Done, 1)
	if err := lib.Rush(wanted.ID); err == nil {
		t.Fatal("a finished recording was put back in the queue")
	}
	// Retrying by hand is asking by hand.
	db.Progress(older.ID, store.Failed, 0)
	if err := lib.Again(older.ID); err != nil {
		t.Fatal(err)
	}
	if id, ok := lib.next(noon); !ok || id != older.ID {
		t.Fatalf("a retry waited for the schedule: %d, %v", id, ok)
	}
}

func TestEveryWaitHasAName(t *testing.T) {
	lib, _, dir := setup(t, nil)
	r := queued(t, lib, dir, "a.wav", morning)
	if why, _ := lib.Waiting(r, morning); why != "models" {
		t.Fatalf("before the models loaded it waits for %q", why)
	}
	lib.Use(&fakeEngine{})
	lib.Wait(func() bool { return true })
	if why, _ := lib.Waiting(r, morning); why != "recording" {
		t.Fatalf("during a recording it waits for %q", why)
	}
	lib.Wait(func() bool { return false })
	if why, _ := lib.Waiting(r, morning); why != "turn" {
		t.Fatalf("with nothing in the way it waits for %q", why)
	}
}

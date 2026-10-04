package main

import (
	"bytes"
	"errors"
	"log/slog"
	"regexp"
	"testing"
	"time"
)

type probe struct{ log *slog.Logger }

func (p *probe) say() { p.log.Warn("from a method") }

func TestALogLineReadsTimeLevelSourceThenMessage(t *testing.T) {
	var out bytes.Buffer
	log := slog.New(newLine(&out, slog.LevelInfo)).With("id", 7)
	log.Debug("below the level")
	log.WithGroup("llm").Info("summary finished", "took", 1500*time.Millisecond, "err", errors.New("quota spent"))
	(&probe{slog.New(newLine(&out, slog.LevelInfo))}).say()

	want := regexp.MustCompile(`^\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d\.\d{3} INFO log_test\.go:\d+:TestALogLineReadsTimeLevelSourceThenMessage\(\) - summary finished id=7 llm\.took=1\.5s llm\.err="quota spent"
\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d\.\d{3} WARN log_test\.go:\d+:say\(\) - from a method
$`)
	if !want.Match(out.Bytes()) {
		t.Fatalf("got\n%s", out.String())
	}
}

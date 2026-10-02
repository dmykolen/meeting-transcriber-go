package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestIdealSummaryScoresTen(t *testing.T) {
	value := summary{
		Title:    "Доступ, документи та IP-діапазони",
		Overview: "Команда залишила зовнішній доступ через VPN. Marta готує пакет для Northwind.",
		Chapters: []chapter{{Start: 0, Title: "Доступ", Summary: "Обговорили доступ."}},
		Topics:   []string{"VPN", "Northwind"},
		Decisions: []string{
			"Зовнішній доступ залишається лише через VPN.",
		},
		ActionItems: []actionItem{
			{Task: "Підготувати пакет для Northwind", Owner: "Marta", Due: "до п'ятниці"},
			{Task: "Перевірити IP-діапазони", Owner: "Taras"},
		},
		OpenQuestions: []string{
			"Хто погоджує фінальний перелік IP-діапазонів?",
			"Чи переходити на новий proxy?",
		},
	}
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	score, checks := scoreSummary(string(data))
	if score != 10 {
		t.Fatalf("score = %.1f, checks = %#v", score, checks)
	}
}

func TestSummaryRejectsTrailingOutput(t *testing.T) {
	score, checks := scoreSummary(`{"title":"","overview":"","chapters":[],"topics":[],"decisions":[],"action_items":[],"open_questions":[]} trailing`)
	if score != 0 || checks["valid_strict_schema"] {
		t.Fatalf("score = %.1f, checks = %#v", score, checks)
	}
}

func TestSummaryRejectsEmptyOutput(t *testing.T) {
	score, checks := scoreSummary("")
	if score != 0 || checks["valid_strict_schema"] {
		t.Fatalf("score = %.1f, checks = %#v", score, checks)
	}
}

func TestSummaryRejectsMissingRequiredField(t *testing.T) {
	score, checks := scoreSummary(`{
		"title":"Доступ до нового кабінету",
		"overview":"VPN і Northwind",
		"chapters":[],
		"decisions":[],
		"action_items":[],
		"open_questions":[]
	}`)
	if score != 0 || checks["valid_strict_schema"] {
		t.Fatalf("score = %.1f, checks = %#v", score, checks)
	}
}

func TestSummaryRejectsMinuteValuesInSecondsField(t *testing.T) {
	value := summary{
		Title:     "Доступ, документи та IP-діапазони",
		Overview:  "Команда залишила доступ через VPN і підготувала Northwind.",
		Chapters:  []chapter{{Start: 0}, {Start: 1}, {Start: 2}},
		Decisions: []string{"VPN"},
		ActionItems: []actionItem{
			{Task: "Northwind", Owner: "Marta", Due: "до п'ятниці"},
			{Task: "IP", Owner: "Taras"},
		},
		OpenQuestions: []string{"Хто погоджує IP?"},
	}
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	score, checks := scoreSummary(string(data))
	if checks["chapter_timestamps_in_seconds"] || score >= 10 {
		t.Fatalf("score = %.1f, checks = %#v", score, checks)
	}
}

func TestIdealGroundedAnswerScoresTen(t *testing.T) {
	answer := `Остаточне рішення — залишити зовнішній доступ лише через VPN
(джерело 1; підтверджено джерелом 3). Ручна нотатка, джерело 2,
суперечить цьому.`
	score, checks := scoreQA(answer)
	if score != 10 {
		t.Fatalf("score = %.1f, checks = %#v", score, checks)
	}
}

func TestAnswerThatDeniesConflictLosesConflictPoints(t *testing.T) {
	answer := `Остаточне рішення — доступ лише через VPN (джерело 1).
	Ручна нотатка (джерело 2) каже про публічний доступ. Немає суперечності.`
	score, checks := scoreQA(answer)
	if checks["describes_conflict"] || score >= 10 {
		t.Fatalf("score = %.1f, checks = %#v", score, checks)
	}
}

func TestCitationNumbersAreExact(t *testing.T) {
	answer := `Остаточне рішення — доступ лише через VPN (Source 10).
	Ручна нотатка (Source 20) суперечить рішенню (Source 30).`
	_, checks := scoreQA(answer)
	if checks["cites_source_1"] || checks["cites_source_2"] || checks["cites_source_3"] {
		t.Fatalf("substring citation accepted: %#v", checks)
	}
	if checks["no_unknown_sources"] {
		t.Fatalf("unknown sources accepted: %#v", checks)
	}
}

func TestResumeUsesSuccessfulMatchingFingerprint(t *testing.T) {
	path := filepath.Join(t.TempDir(), "results.jsonl")
	meta := modelInfo{Name: "model", Digest: "current"}
	rows := []result{
		{Benchmark: benchmarkVersion, Runtime: "ollama/1", Model: "model", ModelDigest: "current", Task: "summary", Run: 1},
		{Benchmark: benchmarkVersion, Runtime: "ollama/1", Model: "model", ModelDigest: "current", Task: "qa", Run: 1, Error: "failed"},
		{Benchmark: "old", Runtime: "ollama/1", Model: "model", ModelDigest: "current", Task: "summary", Run: 2},
		{Benchmark: benchmarkVersion, Runtime: "ollama/1", Model: "model", ModelDigest: "old", Task: "qa", Run: 2},
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if err := appendResult(file, row); err != nil {
			t.Fatal(err)
		}
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	done, err := completed(path, "ollama/1")
	if err != nil {
		t.Fatal(err)
	}
	if !done[resultKey(meta, "ollama/1", "summary", 1)] {
		t.Fatal("successful matching result was not reusable")
	}
	for _, key := range []string{
		resultKey(meta, "ollama/1", "qa", 1),
		resultKey(meta, "ollama/1", "summary", 2),
		resultKey(meta, "ollama/1", "qa", 2),
	} {
		if done[key] {
			t.Fatalf("invalid result marked complete: %s", key)
		}
	}
}

// Command 24_notes measures the voice-note prompt. Two parts:
//
//  1. Made-up dictations (cases.json, nothing private): each is written out by
//     the shipping prompt and checked by code: the date every to-do should get
//     ("on Friday" said on a Sunday), self-corrections ("tomorrow, no, Friday"),
//     no to-dos invented from a thought, and words that must or must not stay.
//  2. The archive's own notes, from a COPY of meetings.db: written by the note
//     prompt that shipped in 1.6.0 ("previous") and by Polish ("shipping"),
//     then compared blind by another model: which one the person would rather
//     keep, every statement the transcript does not support, and whether each
//     dated to-do has the right day. Counted by code: length, to-dos, to-dos
//     with a date, and meeting words ("discussed", "speaker") left in the text.
//
// The notes themselves are written to -out for reading, never to the repo.
//
//	go run ./exp/24_notes -db copy.db -out /tmp/notes -model gpt-6.1-sol -judge gpt-5.6-terra
package main

import (
	"cmp"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/dmykolen/meeting-transcriber-go/internal/home"
	"github.com/dmykolen/meeting-transcriber-go/internal/insights"
	"github.com/dmykolen/meeting-transcriber-go/internal/models"
	"github.com/dmykolen/meeting-transcriber-go/internal/store"
)

var (
	dbPath = flag.String("db", "", "a COPY of meetings.db; empty runs the made-up cases only")
	cases  = flag.String("cases", "exp/24_notes/cases.json", "the made-up dictations")
	runs   = flag.Int("runs", 1, "how many times to write out each made-up dictation")
	model  = flag.String("model", "", "the Copilot model that writes the notes (default: the one in the settings)")
	judge  = flag.String("judge", "", `the Copilot model that compares them ("none" skips the comparison)`)
	least  = flag.Int("chars", 80, "only archive notes with at least this many characters of speech")
	reuse  = flag.Bool("reuse", false, "take the previous prompt's notes from -out instead of asking again")
	out    = flag.String("out", ".", "where the notes go")
)

// tongue is the language the notes are written in, from the settings.
var tongue string

func main() {
	flag.Parse()
	dir, err := home.Dir()
	check(err)
	cfg, err := home.Load(dir)
	check(err)
	copilot := func(model string) *insights.Client {
		return insights.New(insights.Setup{
			Language: cfg.Language, Provider: "copilot", CopilotModel: model,
			Copilot: models.Path(home.Models(dir), models.Copilot), CopilotHome: home.Copilot(dir),
		})
	}
	tongue = cmp.Or(insights.Tongue[strings.ToLower(cfg.Language)], "the language the note was said in")
	writer := cmp.Or(*model, cfg.AI.CopilotModel)
	c := copilot(writer)
	defer c.Close()
	log.Printf("written by %s", writer)
	check(os.MkdirAll(*out, 0o755))

	made(c)
	if *dbPath == "" {
		return
	}
	archive(c, copilot, writer)
}

// made writes out each made-up dictation and checks what code can check.
func made(c *insights.Client) {
	blob, err := os.ReadFile(*cases)
	check(err)
	var all []struct {
		Name, Recorded, Said string
		Due                  map[string]string // a word of the to-do → its date, or "words"
		Todos                *int
		Says, Never          []string
	}
	check(json.Unmarshal(blob, &all))
	iso := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	passed, total := 0, 0
	written := map[string][]*insights.Summary{}
	for run := range *runs {
		for _, k := range all {
			at, err := time.ParseInLocation("2006-01-02T15:04:05", k.Recorded, time.Local)
			check(err)
			note, err := c.Polish(context.Background(), []insights.Turn{{Speaker: "Me", Text: k.Said}}, at)
			if err != nil {
				log.Printf("%s: %v", k.Name, err)
				total++
				continue
			}
			written[k.Name] = append(written[k.Name], note)
			var fails []string
			for word, want := range k.Due {
				var got *insights.ActionItem
				for i, a := range note.ActionItems {
					if strings.Contains(strings.ToLower(a.Task), word) {
						got = &note.ActionItems[i]
					}
				}
				switch {
				case got == nil:
					fails = append(fails, fmt.Sprintf("no to-do about %q", word))
				case want == "words" && (got.Due == "" || iso.MatchString(got.Due)):
					fails = append(fails, fmt.Sprintf("%q due %q, want the words", word, got.Due))
				case want != "words" && got.Due != want:
					fails = append(fails, fmt.Sprintf("%q due %q, want %s", word, got.Due, want))
				}
			}
			if k.Todos != nil && len(note.ActionItems) != *k.Todos {
				fails = append(fails, fmt.Sprintf("%d to-dos, want %d", len(note.ActionItems), *k.Todos))
			}
			text := strings.ToLower(note.Overview)
			for _, w := range k.Says {
				if !strings.Contains(text, strings.ToLower(w)) {
					fails = append(fails, fmt.Sprintf("the text lost %q", w))
				}
			}
			for _, w := range k.Never {
				if strings.Contains(text, w) {
					fails = append(fails, fmt.Sprintf("the text kept the corrected %q", w))
				}
			}
			total++
			if len(fails) == 0 {
				passed++
			}
			fmt.Printf("case %-11s run %d: %s\n", k.Name, run+1, cmp.Or(strings.Join(fails, "; "), "ok"))
		}
	}
	fmt.Printf("made-up dictations: %d of %d passed\n", passed, total)
	save("cases.json", written)
}

// archive replays the archive's notes through both prompts and judges them.
func archive(c *insights.Client, copilot func(string) *insights.Client, writer string) {
	db, err := store.Open(*dbPath)
	check(err)
	all, err := db.Recent(10000)
	check(err)
	var picked []store.Recording
	transcripts, turns := map[int64]string{}, map[int64][]insights.Turn{}
	for _, r := range all {
		if r.Kind != store.Note {
			continue
		}
		rows, err := db.Turns(r.ID)
		check(err)
		said := 0
		for _, t := range rows {
			said += len([]rune(t.Text))
			turns[r.ID] = append(turns[r.ID], insights.Turn{Start: t.Start, Speaker: t.Speaker, Text: t.Text})
		}
		if said >= *least {
			picked = append(picked, r)
			transcripts[r.ID] = insights.Transcript(turns[r.ID])
		}
	}
	log.Printf("%d archive notes with %d characters of speech or more", len(picked), *least)

	results := map[string]map[int64]*insights.Summary{"previous": {}, "shipping": {}}
	if *reuse {
		blob, err := os.ReadFile(filepath.Join(*out, "previous.json"))
		check(err)
		check(json.Unmarshal(blob, ptr(results["previous"])))
	}
	took := map[string]time.Duration{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, 4)
	for _, r := range picked {
		for _, v := range []string{"previous", "shipping"} {
			if results[v][r.ID] != nil {
				continue
			}
			wg.Go(func() {
				sem <- struct{}{}
				defer func() { <-sem }()
				began := time.Now()
				var s *insights.Summary
				var err error
				if v == "shipping" {
					s, err = c.Polish(context.Background(), turns[r.ID], r.Started)
				} else {
					s, err = before(c, turns[r.ID])
				}
				if err != nil {
					log.Printf("%s %d: %v", v, r.ID, err)
					return
				}
				mu.Lock()
				results[v][r.ID], took[v] = s, took[v]+time.Since(began)
				mu.Unlock()
				log.Printf("%s %d done in %s", v, r.ID, time.Since(began).Round(time.Second))
			})
		}
	}
	wg.Wait()
	for v, got := range results {
		save(v+".json", got)
		fmt.Println(report(v, picked, got, took[v]))
	}

	if *judge == "none" {
		return
	}
	j := copilot(cmp.Or(*judge, writer))
	defer j.Close()
	// Blind: the shipping note is A on even rows and B on odd.
	verdicts := map[int64]verdict{}
	for i, r := range picked {
		p, s := results["previous"][r.ID], results["shipping"][r.ID]
		if p == nil || s == nil {
			continue
		}
		a, b := shown(p), shown(s)
		swap := i%2 == 0
		if swap {
			a, b = b, a
		}
		wg.Go(func() {
			sem <- struct{}{}
			defer func() { <-sem }()
			input := fmt.Sprintf("Recorded on %s.\n\nTranscript:\n%s\nVersion A:\n%s\n\nVersion B:\n%s",
				r.Started.Format("Monday 2006-01-02 at 15:04"), transcripts[r.ID], a, b)
			var v verdict
			if err := j.Structured(context.Background(), judgePrompt, input, "comparison", judgeSchema, &v); err != nil {
				log.Printf("judge %d: %v", r.ID, err)
				return
			}
			if swap { // A was the shipping one; name them by variant
				v.Better = map[string]string{"A": "shipping", "B": "previous"}[v.Better]
				v.UnsupportedA, v.UnsupportedB = v.UnsupportedB, v.UnsupportedA
				v.DatesA, v.DatesB = v.DatesB, v.DatesA
			} else {
				v.Better = map[string]string{"A": "previous", "B": "shipping"}[v.Better]
			}
			v.Better = cmp.Or(v.Better, "tie")
			mu.Lock()
			verdicts[r.ID] = v
			mu.Unlock()
		})
	}
	wg.Wait()
	save("verdicts.json", verdicts)
	wins, unsupported, dated, wrong := map[string]int{}, map[string]int{}, 0, 0
	for _, v := range verdicts {
		wins[v.Better]++
		unsupported["previous"] += len(v.UnsupportedA)
		unsupported["shipping"] += len(v.UnsupportedB)
		for _, d := range v.DatesB {
			dated++
			if !strings.EqualFold(strings.TrimSpace(d), "ok") {
				wrong++
			}
		}
	}
	fmt.Printf("judged %d by %s: shipping better %d, previous better %d, tie %d; unsupported statements previous %d, shipping %d; shipping dated to-dos %d, wrong %d\n",
		len(verdicts), cmp.Or(*judge, writer), wins["shipping"], wins["previous"], wins["tie"], unsupported["previous"], unsupported["shipping"], dated, wrong)
}

// shown is what the person reads of a note, as JSON for the judge.
func shown(s *insights.Summary) string {
	blob, _ := json.Marshal(map[string]any{
		"title": s.Title, "text": s.Overview, "parts": s.Chapters, "decisions": s.Decisions,
		"to_dos": s.ActionItems, "questions": s.OpenQuestions,
	})
	return string(blob)
}

// meeting are the words of a meeting write-up that a note in its speaker's
// own voice should not need.
var meeting = regexp.MustCompile(`(?i)обговор|учасник|спікер|speaker|йшлося|ішлося|розповіда|обмірков`)

// report counts what code can count about one variant.
func report(variant string, picked []store.Recording, got map[int64]*insights.Summary, took time.Duration) string {
	var n, chars, longest, todos, dated, words, labels int
	iso := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	for _, r := range picked {
		s := got[r.ID]
		if s == nil {
			continue
		}
		n++
		text := s.Overview
		for _, ch := range s.Chapters {
			text += "\n" + ch.Summary
		}
		chars += len([]rune(text))
		longest = max(longest, len([]rune(text)))
		words += len(meeting.FindAllString(text, -1))
		labels += strings.Count(strings.ToUpper(text), "SPEAKER_")
		todos += len(s.ActionItems)
		for _, a := range s.ActionItems {
			if iso.MatchString(a.Due) {
				dated++
			}
		}
	}
	if n == 0 {
		return fmt.Sprintf("%-8s no results", variant)
	}
	return fmt.Sprintf("%-8s n=%d text chars/note=%d longest=%d to-dos=%d (dated %d) meeting words=%d SPEAKER labels=%d time/note=%s",
		variant, n, chars/n, longest, todos, dated, words, labels, (took / time.Duration(n)).Round(time.Second))
}

// before is the note the prompt that shipped in 1.6.0 would have written.
func before(c *insights.Client, turns []insights.Turn) (*insights.Summary, error) {
	instructions := fmt.Sprintf(previousNote, tongue)
	var out insights.Summary
	err := c.Structured(context.Background(), instructions, insights.Transcript(turns), "meeting_summary", insights.Schema, &out)
	return &out, err
}

type verdict struct {
	Better       string   `json:"better"`
	Why          string   `json:"why"`
	UnsupportedA []string `json:"unsupported_a"`
	UnsupportedB []string `json:"unsupported_b"`
	DatesA       []string `json:"dates_a"`
	DatesB       []string `json:"dates_b"`
}

const judgePrompt = `You compare two versions, A and B, of the same voice note: what one person said into their microphone, written up for that person to read later instead of listening again. You are given the day it was recorded, the transcript and both versions as JSON.

- unsupported_a, unsupported_b: every statement in that version the transcript does not support: a wrong or invented fact, name, number, reason, conclusion or to-do. Quote each briefly. Empty when there are none.
- dates_a, dates_b: one entry per to-do in that version that has a due date written as YYYY-MM-DD, in order: "ok" when that is the day the speaker meant, given what they said and the day it was recorded, or a few words on what is wrong. Empty when no to-do has such a date.
- better: "A", "B" or "tie": which version the person would rather keep as their note: it says what they said and meant, in their own voice, clean and complete, with their to-dos. Accuracy counts first: longer is better only when what it adds is true and worth keeping.
- why: one sentence.`

var judgeSchema = map[string]any{
	"type": "object", "additionalProperties": false,
	"required": []string{"unsupported_a", "unsupported_b", "dates_a", "dates_b", "better", "why"},
	"properties": map[string]any{
		"unsupported_a": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
		"unsupported_b": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
		"dates_a":       map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
		"dates_b":       map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
		"better":        map[string]any{"type": "string", "enum": []string{"A", "B", "tie"}},
		"why":           map[string]any{"type": "string"},
	},
}

func save(name string, v any) {
	blob, _ := json.MarshalIndent(v, "", " ")
	check(os.WriteFile(filepath.Join(*out, name), blob, 0o644))
}

func ptr[T any](v T) *T { return &v }

func check(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

// previousNote is the note prompt that shipped in 1.6.0, kept to compare against.
const previousNote = `You are given the transcript of a recording made by one person's microphone, with timestamps. Nobody else was in the conversation. It is a voice note, thinking aloud, one side of a phone call, a dictated message, or a talk to nobody. Sometimes the microphone only picked up something playing in the room (a video, a call on speakers, a lecture); then it is not the person's own speech.

Write a short record for the person who made it, who will want to find the thought again.

- title: four to eight words naming the subject.
- overview: two to four plain sentences: what the person was thinking about, working out or doing, and where they got to, with the specifics they gave. If what you read is clearly other people's content (a video, a lecture, a call on the speakers), say so first in one sentence and keep everything else minimal.
- topics: one to four broad subjects, two to four words each. Not side talk.
- decisions: only what the speaker decided for themselves and said so.
- action_items: the speaker's own to-dos and intentions ("I need to", "remind me to", "tomorrow I will"), phrased as instructions that start with a verb. Leave owner empty: it is the speaker. due is the deadline in the words it was said, or empty.
- chapters: only when the recording runs past ten minutes and really changes subject; otherwise at most one. Each summary says what the person worked out in that part, with the specifics, in two to five sentences.
- open_questions: what the speaker is still unsure of or means to find out. Phrase each as the question it was.

Where a passage is garbled or is background noise, leave it out rather than guess. If the whole transcript is unusable, say so in one sentence in the overview and leave every list empty.

Write in %s. This holds even when the transcript contains other languages, borrowed words or whole sentences in another language.`

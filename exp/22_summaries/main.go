// Command 22_summaries replays recorded meetings and notes through the summary
// prompt before the current one ("previous") and the shipping one, and counts:
// topics (how many, how many reuse the archive's), owners that are only a
// SPEAKER_nn label, action items with no owner, decisions and questions per
// recording, how long the overview and the chapter write-ups are, labels left
// in that prose, and time. Then the same model compares the two write-ups of
// each meeting blind (the order alternates): which one tells a colleague who
// missed it more, and which statements the transcript does not support.
// The summaries themselves are written to -out for reading, never to the repo.
//
//	go run ./exp/22_summaries -db copy.db -meetings 6 -notes 2 -min 20 -out /tmp/out
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
	"strings"
	"sync"
	"time"

	"github.com/dmykolen/meeting-transcriber-go/internal/home"
	"github.com/dmykolen/meeting-transcriber-go/internal/insights"
	"github.com/dmykolen/meeting-transcriber-go/internal/models"
	"github.com/dmykolen/meeting-transcriber-go/internal/store"
)

var (
	dbPath   = flag.String("db", "", "a COPY of meetings.db")
	meetings = flag.Int("meetings", 10, "meetings to replay (newest first)")
	notes    = flag.Int("notes", 8, "notes to replay")
	least    = flag.Float64("min", 0, "only recordings at least this many minutes long")
	reuse    = flag.Bool("reuse", false, "take the previous prompt's summaries from -out instead of asking again")
	model    = flag.String("model", "", "the Copilot model that writes the summaries (default: the one in the settings)")
	judge    = flag.String("judge", "", `the Copilot model that compares them (default: -model; "none" skips the comparison)`)
	ids      = flag.String("ids", "", "comma-separated recording ids to replay instead of picking them")
	out      = flag.String("out", ".", "where the summaries go")
)

func main() {
	flag.Parse()
	dir, err := home.Dir()
	check(err)
	cfg, err := home.Load(dir)
	check(err)
	db, err := store.Open(*dbPath)
	check(err)
	copilot := func(model string) *insights.Client {
		return insights.New(insights.Setup{
			Language: cfg.Language, Provider: "copilot", CopilotModel: model,
			Copilot: models.Path(home.Models(dir), models.Copilot), CopilotHome: home.Copilot(dir),
		})
	}
	writer := cmp.Or(*model, cfg.AI.CopilotModel)
	c, j := copilot(writer), copilot(cmp.Or(*judge, writer))
	defer c.Close()
	defer j.Close()
	log.Printf("written by %s, judged by %s", writer, cmp.Or(*judge, writer))
	known, err := db.Topics(80)
	check(err)
	check(os.MkdirAll(*out, 0o755))

	all, err := db.Recent(10000)
	check(err)
	var picked []store.Recording
	count := map[store.Kind]int{}
	want := map[store.Kind]int{store.Meeting: *meetings, store.Note: *notes}
	asked := map[string]bool{}
	for _, id := range strings.Split(*ids, ",") {
		asked[strings.TrimSpace(id)] = id != ""
	}
	for _, r := range all {
		if *ids != "" {
			if asked[fmt.Sprint(r.ID)] {
				count[r.Kind]++
				picked = append(picked, r)
			}
			continue
		}
		// A transcript worth summarising: enough turns, and a summary exists, so
		// the recording passed the "worth a model call" test when it was filed.
		if r.Summary != nil && r.Turns >= 30 && r.Duration >= *least*60 && count[r.Kind] < want[r.Kind] {
			count[r.Kind]++
			picked = append(picked, r)
		}
	}
	log.Printf("%d recordings (%d meetings, %d notes)", len(picked), count[store.Meeting], count[store.Note])

	variants := []string{"previous", "shipping"}
	results := map[string]map[int64]*insights.Summary{"previous": {}, "shipping": {}}
	if *reuse {
		blob, err := os.ReadFile(filepath.Join(*out, "previous.json"))
		check(err)
		check(json.Unmarshal(blob, ptr(results["previous"])))
	}
	took := map[string]time.Duration{}
	transcripts := map[int64]string{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, 4)
	for _, r := range picked {
		rows, err := db.Turns(r.ID)
		check(err)
		turns := make([]insights.Turn, len(rows))
		for i, t := range rows {
			turns[i] = insights.Turn{Start: t.Start, Speaker: t.Speaker, Text: t.Text}
		}
		transcripts[r.ID] = insights.Transcript(turns)
		for _, v := range variants {
			if v == "previous" && results[v][r.ID] != nil {
				continue
			}
			wg.Add(1)
			go func() {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				began := time.Now()
				var s *insights.Summary
				var err error
				if v == "shipping" {
					s, err = c.Summarise(context.Background(), turns, known, r.Kind == store.Note)
				} else {
					s, err = before(c, cfg.Language, turns, known, r.Kind == store.Note)
				}
				if err != nil {
					log.Printf("%s %d: %v", v, r.ID, err)
					return
				}
				mu.Lock()
				results[v][r.ID] = s
				took[v] += time.Since(began)
				mu.Unlock()
				log.Printf("%s %d (%s) done in %s", v, r.ID, r.Kind, time.Since(began).Round(time.Second))
			}()
		}
	}
	wg.Wait()

	for _, v := range variants {
		blob, _ := json.MarshalIndent(results[v], "", " ")
		check(os.WriteFile(filepath.Join(*out, v+".json"), blob, 0o644))
	}
	for _, kind := range []store.Kind{store.Meeting, store.Note} {
		for _, v := range variants {
			fmt.Println(report(v, kind, picked, results[v], known, took[v]))
		}
	}

	if *judge == "none" {
		return
	}
	// Blind comparison: the shipping write-up is A on even rows and B on odd.
	verdicts := map[int64]verdict{}
	for i, r := range picked {
		a, b := results["previous"][r.ID], results["shipping"][r.ID]
		if a == nil || b == nil {
			continue
		}
		swap := i%2 == 0
		if swap {
			a, b = b, a
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			va, _ := json.Marshal(a)
			vb, _ := json.Marshal(b)
			input := fmt.Sprintf("Transcript:\n%s\n\nWrite-up A:\n%s\n\nWrite-up B:\n%s", transcripts[r.ID], va, vb)
			var v verdict
			if err := j.Structured(context.Background(), judgePrompt, input, "comparison", judgeSchema, &v); err != nil {
				log.Printf("judge %d: %v", r.ID, err)
				return
			}
			if swap { // A was the shipping one; name them by variant
				v.Better = map[string]string{"A": "shipping", "B": "previous"}[v.Better]
				v.UnsupportedA, v.UnsupportedB = v.UnsupportedB, v.UnsupportedA
			} else {
				v.Better = map[string]string{"A": "previous", "B": "shipping"}[v.Better]
			}
			if v.Better == "" {
				v.Better = "tie"
			}
			mu.Lock()
			verdicts[r.ID] = v
			mu.Unlock()
		}()
	}
	wg.Wait()
	blob, _ := json.MarshalIndent(verdicts, "", " ")
	check(os.WriteFile(filepath.Join(*out, "verdicts.json"), blob, 0o644))
	wins := map[string]int{}
	unsupported := map[string]int{}
	for _, v := range verdicts {
		wins[v.Better]++
		unsupported["previous"] += len(v.UnsupportedA)
		unsupported["shipping"] += len(v.UnsupportedB)
	}
	fmt.Printf("judged %d: shipping better %d, previous better %d, tie %d; unsupported statements previous %d, shipping %d\n",
		len(verdicts), wins["shipping"], wins["previous"], wins["tie"], unsupported["previous"], unsupported["shipping"])
}

// before is the summary the previous prompt would have written.
func before(c *insights.Client, language string, turns []insights.Turn, known []string, note bool) (*insights.Summary, error) {
	instructions := fmt.Sprintf(map[bool]string{false: previousMeeting, true: previousNote}[note],
		cmp.Or(insights.Tongue[strings.ToLower(language)], "the language the meeting was held in"))
	if len(known) > 0 {
		instructions += fmt.Sprintf(topicsPrompt, "- "+strings.Join(known, "\n- "))
	}
	var out insights.Summary
	if err := c.Structured(context.Background(), instructions, insights.Transcript(turns), "meeting_summary", previousSchema, &out); err != nil {
		return nil, err
	}
	out.Topics = insights.Canon(out.Topics, known)
	return &out, nil
}

// previousSchema is insights.Schema with the descriptions it had then.
var previousSchema = func() map[string]any {
	var s map[string]any
	blob, _ := json.Marshal(insights.Schema)
	_ = json.Unmarshal(blob, &s)
	props := s["properties"].(map[string]any)
	props["overview"].(map[string]any)["description"] = "Two or three sentences on what happened"
	chapter := props["chapters"].(map[string]any)["items"].(map[string]any)["properties"].(map[string]any)
	chapter["title"].(map[string]any)["description"] = "A few words naming the topic"
	chapter["summary"].(map[string]any)["description"] = "One sentence on what was covered"
	return s
}()

type verdict struct {
	Better       string   `json:"better"`
	Why          string   `json:"why"`
	UnsupportedA []string `json:"unsupported_a"`
	UnsupportedB []string `json:"unsupported_b"`
}

const judgePrompt = `You compare two write-ups of the same meeting, A and B, made for a colleague who missed it and will not read the transcript. You are given the transcript and both write-ups as JSON.

- unsupported_a, unsupported_b: every statement in that write-up the transcript does not support: a wrong or invented fact, name, number, reason, decision, owner or deadline. Quote each briefly. Empty when there are none.
- better: "A", "B" or "tie": which write-up lets the colleague understand what was talked about, who brought what, what was concluded and what is open. Accuracy counts first: a longer write-up is better only when what it adds is true and worth knowing.
- why: one sentence.`

var judgeSchema = map[string]any{
	"type": "object", "additionalProperties": false,
	"required": []string{"unsupported_a", "unsupported_b", "better", "why"},
	"properties": map[string]any{
		"unsupported_a": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
		"unsupported_b": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
		"better":        map[string]any{"type": "string", "enum": []string{"A", "B", "tie"}},
		"why":           map[string]any{"type": "string"},
	},
}

func ptr[T any](v T) *T { return &v }

func check(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

// report counts what can be counted about one variant on one kind.
func report(variant string, kind store.Kind, picked []store.Recording, got map[int64]*insights.Summary, known []string, took time.Duration) string {
	var n, topics, reused, decisions, actions, noOwner, labelOwner, questions, chapters, overview, written, labels int
	var minutes float64
	vocab := map[string]bool{}
	for _, k := range known {
		vocab[strings.ToLower(k)] = true
	}
	for _, r := range picked {
		s := got[r.ID]
		if r.Kind != kind || s == nil {
			continue
		}
		n++
		minutes += r.Duration / 60
		topics += len(s.Topics)
		for _, t := range s.Topics {
			if vocab[strings.ToLower(t)] {
				reused++
			}
		}
		decisions += len(s.Decisions)
		questions += len(s.OpenQuestions)
		chapters += len(s.Chapters)
		overview += len([]rune(s.Overview))
		prose := s.Overview
		for _, ch := range s.Chapters {
			written += len([]rune(ch.Summary))
			prose += " " + ch.Summary
		}
		labels += strings.Count(strings.ToUpper(prose), "SPEAKER_")
		for _, a := range s.ActionItems {
			actions++
			if a.Owner == "" {
				noOwner++
			}
			if strings.HasPrefix(strings.ToUpper(a.Owner), "SPEAKER") {
				labelOwner++
			}
		}
	}
	if n == 0 {
		return fmt.Sprintf("%-8s %-8s no results", kind, variant)
	}
	f := func(x int) float64 { return float64(x) / float64(n) }
	return fmt.Sprintf("%-8s %-8s n=%d (%.0f min) topics/rec=%.1f reused=%d/%d decisions/rec=%.1f actions/rec=%.1f (no owner %d, SPEAKER label %d) questions/rec=%.1f chapters/rec=%.1f overview chars=%.0f chapter chars/rec=%.0f per minute=%.0f labels in prose=%d time/rec=%s",
		kind, variant, n, minutes, f(topics), reused, topics, f(decisions), f(actions), noOwner, labelOwner, f(questions), f(chapters), f(overview), f(written), float64(written)/minutes, labels, (took / time.Duration(n)).Round(time.Second))
}

// The prompts and schema descriptions that shipped before the detailed
// write-up, kept to compare against.
const previousMeeting = `You are given the transcript of one meeting, with timestamps and speakers.

Write it up for somebody who was not there, will not read the transcript, and will later come looking for what was decided and who owes what.

- title: four to eight words naming what the meeting was actually about: the subject, not the format. "Team meeting" and "Sync" are wrong.
- overview: two or three plain sentences: what the meeting dealt with and what came of it. State facts; no "the team discussed".
- topics: two to five broad subjects, two to four words each, the labels you would file this meeting under. Not every sentence is a topic, and side talk (food, schedules, greetings) is not one.
- decisions: only what was settled and will bind later work: a choice between options, an agreed approach, a rule. A plan for this week, a priority for now, a suggestion nobody accepted and a task are not decisions. If nothing was decided, return nothing.
- action_items: only what somebody took on: an accepted request ("ok, I'll check"), or an assignment nobody objected to. Phrase each as an instruction that starts with a verb. Not action items: something done in the past, a wish, "we should think about", a question. owner is the person who took it on, by the name the transcript gives. When the transcript has only a label such as SPEAKER_05 but the person is addressed or introduced by name somewhere, use that name; otherwise keep the label. due is the deadline in the words it was said. Leave an owner or a deadline empty rather than guess.
- chapters: cover the meeting in order, starting at 00:00, coarse enough that each one is worth jumping to: a handful for an hour, not one per minute.
- open_questions: questions the meeting raised and left unanswered that somebody will have to answer. Phrase each as the question it was, so that the same question asked next week is recognisable. Not rhetorical questions, and not ones answered later in the same meeting.

Transcripts of real meetings are imperfect and words are sometimes misheard. Where a passage is garbled, or is background noise or somebody else's media, leave it out rather than guess. If the whole transcript is unusable, say so in one sentence in the overview and leave every list empty.

Write in %s. This holds even when the transcript itself contains other languages, borrowed words or whole sentences in another language: those are what people say, and they do not change what you answer in.`

const previousNote = `You are given the transcript of a recording made by one person's microphone, with timestamps. Nobody else was in the conversation. It is a voice note, thinking aloud, one side of a phone call, a dictated message, or a talk to nobody. Sometimes the microphone only picked up something playing in the room (a video, a call on speakers, a lecture); then it is not the person's own speech.

Write a short record for the person who made it, who will want to find the thought again.

- title: four to eight words naming the subject.
- overview: two or three plain sentences: what the person was thinking about, working out or doing, and where they got to. If what you read is clearly other people's content (a video, a lecture, a call on the speakers), say so first in one sentence and keep everything else minimal.
- topics: one to four broad subjects, two to four words each. Not side talk.
- decisions: only what the speaker decided for themselves.
- action_items: the speaker's own to-dos and intentions ("I need to", "remind me to", "tomorrow I will"), phrased as instructions that start with a verb. Leave owner empty: it is the speaker. due is the deadline in the words it was said, or empty.
- chapters: only when the recording runs past ten minutes and really changes subject; otherwise at most one.
- open_questions: what the speaker is still unsure of or means to find out. Phrase each as the question it was.

Where a passage is garbled or is background noise, leave it out rather than guess. If the whole transcript is unusable, say so in one sentence in the overview and leave every list empty.

Write in %s. This holds even when the transcript contains other languages, borrowed words or whole sentences in another language.`

const topicsPrompt = `

Topics already used in earlier recordings, most used first:
%s

For "topics", reuse one of these exactly as written whenever it genuinely
covers a subject here. Add a new topic only when none of them fits;
never reword, translate or pluralise an existing one into a variant of it.`

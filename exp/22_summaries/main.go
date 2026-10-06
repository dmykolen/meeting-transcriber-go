// Command 22_summaries replays recorded meetings and notes through the shipping
// summary prompts and counts: topics (how many, how
// many reuse the archive's), owners that are only a SPEAKER_nn label, how many
// action items have no owner, decisions and questions per recording, and time.
// The summaries themselves are written to -out for reading, never to the repo.
//
//	go run ./exp/22_summaries -db copy.db -meetings 10 -notes 8 -out /tmp/out
package main

import (
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
	meetings = flag.Int("meetings", 10, "meetings to replay (longest recent first)")
	notes    = flag.Int("notes", 8, "notes to replay")
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
	c := insights.New(insights.Setup{
		Language: cfg.Language, Provider: "copilot", CopilotModel: cfg.AI.CopilotModel,
		Copilot: models.Path(home.Models(dir), models.Copilot), CopilotHome: home.Copilot(dir),
	})
	defer c.Close()
	known, err := db.Topics(80)
	check(err)
	check(os.MkdirAll(*out, 0o755))

	all, err := db.Recent(10000)
	check(err)
	var picked []store.Recording
	count := map[store.Kind]int{}
	want := map[store.Kind]int{store.Meeting: *meetings, store.Note: *notes}
	for _, r := range all {
		// A transcript worth summarising: enough turns, and a summary exists, so
		// the recording passed the "worth a model call" test when it was filed.
		if r.Summary != nil && r.Turns >= 30 && count[r.Kind] < want[r.Kind] {
			count[r.Kind]++
			picked = append(picked, r)
		}
	}
	log.Printf("%d recordings (%d meetings, %d notes)", len(picked), count[store.Meeting], count[store.Note])

	variants := []string{"shipping"}
	results := map[string]map[int64]*insights.Summary{}
	took := map[string]time.Duration{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, 4)
	for _, v := range variants {
		results[v] = map[int64]*insights.Summary{}
		for _, r := range picked {
			wg.Add(1)
			go func() {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				rows, err := db.Turns(r.ID)
				if err != nil {
					log.Printf("%d: %v", r.ID, err)
					return
				}
				turns := make([]insights.Turn, len(rows))
				for i, t := range rows {
					turns[i] = insights.Turn{Start: t.Start, Speaker: t.Speaker, Text: t.Text}
				}
				began := time.Now()
				s, err := c.Summarise(context.Background(), turns, known, r.Kind == store.Note)
				if err != nil {
					log.Printf("%s %d: %v", v, r.ID, err)
					return
				}
				mu.Lock()
				results[v][r.ID] = s
				took[v] += time.Since(began)
				mu.Unlock()
				log.Printf("%s %d (%s) done", v, r.ID, r.Kind)
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
			fmt.Println(report(v, kind, picked, results[v], known))
		}
	}
}

func check(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

// report counts what can be counted about one variant on one kind.
func report(variant string, kind store.Kind, picked []store.Recording, got map[int64]*insights.Summary, known []string) string {
	var n, topics, reused, decisions, actions, noOwner, labelOwner, questions, chapters, overview int
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
	return fmt.Sprintf("%-8s %-8s n=%d topics/rec=%.1f reused=%d/%d decisions/rec=%.1f actions/rec=%.1f (no owner %d, SPEAKER label %d) questions/rec=%.1f chapters/rec=%.1f overview chars=%.0f",
		kind, variant, n, f(topics), reused, topics, f(decisions), f(actions), noOwner, labelOwner, f(questions), f(chapters), f(overview))
}

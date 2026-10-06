// Command 21_project replays one project's meeting summaries through the model
// and measures what kind of document comes out.
//
// The "streams" variant added workstreams, a stricter idea of what is worth
// keeping, closing of what the meeting finished, a consolidation pass that merges
// duplicates and retires stale lines, and a written picture of the project; it
// replaced the earlier fold, which is no longer in the tree. Nothing here needs
// the transcripts, only the stored summaries, so a replay of thirty meetings is
// thirty short calls.
//
//	go run ./exp/21_project -db copy.db -group 2 -variant streams -out /tmp/out
//
// The database is opened read-write by the store package, so pass a copy.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/dmykolen/meeting-transcriber-go/internal/home"
	"github.com/dmykolen/meeting-transcriber-go/internal/insights"
	"github.com/dmykolen/meeting-transcriber-go/internal/models"
	"github.com/dmykolen/meeting-transcriber-go/internal/store"
)

var (
	dbPath  = flag.String("db", "", "a COPY of meetings.db")
	group   = flag.Int64("group", 2, "project (group) id")
	name    = flag.String("name", "the project", "what to call the project in the prompts")
	variant = flag.String("variant", "streams", "streams (the baseline fold was replaced by it in 1.5; see .spec/decisions.md)")
	limit   = flag.Int("limit", 0, "fold only the first n meetings (0 = all)")
	out     = flag.String("out", ".", "directory for the document and its metrics")
	every   = flag.Int("tidy", 10, "streams: consolidate after this many meetings")
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
	rows, err := db.In(*group, 1000)
	check(err)
	slices.Reverse(rows) // oldest first
	rows = slices.DeleteFunc(rows, func(r store.Recording) bool { return r.Summary == nil })
	if *limit > 0 && len(rows) > *limit {
		rows = rows[:*limit]
	}
	check(os.MkdirAll(*out, 0o755))
	log.Printf("%s: %d meetings, model %s", *variant, len(rows), cfg.AI.CopilotModel)

	var d *doc
	switch *variant {
	case "streams":
		d = streams(c, rows)
	default:
		log.Fatalf("unknown variant %q", *variant)
	}
	blob, _ := json.MarshalIndent(d, "", " ")
	check(os.WriteFile(filepath.Join(*out, *variant+".json"), blob, 0o644))
	m := measure(d, len(rows))
	mb, _ := json.MarshalIndent(m, "", " ")
	check(os.WriteFile(filepath.Join(*out, *variant+".metrics.json"), mb, 0o644))
	fmt.Println(string(mb))
}

func check(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

// doc is a project document in the shape both variants end in.
type doc struct {
	Status   string            `json:"status"`
	Items    []item            `json:"items"`
	Next     int               `json:"next"`
	Streams  map[string]string `json:"streams,omitempty"` // stream -> its state, streams variant
	Headline string            `json:"headline,omitempty"`
	Brief    string            `json:"brief,omitempty"`
	Ignored  int               `json:"ignored"` // operations that named an id or a kind that does not exist
}

type item struct {
	ID     int    `json:"id"`
	Kind   string `json:"kind"` // work, decision, question
	Text   string `json:"text"`
	Owner  string `json:"owner"`
	Due    string `json:"due"`
	State  string `json:"state"`
	Stream string `json:"stream,omitempty"`
	Times  int    `json:"times"`
	Last   int    `json:"last"` // index of the meeting that last touched it
	Why    string `json:"why,omitempty"`
}

func (it item) open() bool {
	return it.State == "open" || it.State == "standing"
}

// meetingText renders one stored summary for the prompt.
func meetingText(r store.Recording) string {
	var b strings.Builder
	s := r.Summary
	fmt.Fprintf(&b, "%s, %s\n%s\n", r.Title, r.Started.Format("2 January 2006"), s.Overview)
	if len(s.Topics) > 0 {
		fmt.Fprintf(&b, "topics: %s\n", strings.Join(s.Topics, "; "))
	}
	for _, ch := range s.Chapters {
		fmt.Fprintf(&b, "chapter: %s — %s\n", ch.Title, ch.Summary)
	}
	for _, t := range s.Decisions {
		fmt.Fprintf(&b, "decision: %s\n", t)
	}
	for _, t := range s.OpenQuestions {
		fmt.Fprintf(&b, "open question: %s\n", t)
	}
	for _, a := range s.ActionItems {
		fmt.Fprintf(&b, "commitment: %s", a.Task)
		if a.Owner != "" {
			fmt.Fprintf(&b, " — %s", a.Owner)
		}
		if a.Due != "" {
			fmt.Fprintf(&b, ", by %s", a.Due)
		}
		b.WriteString("\n")
	}
	return b.String()
}

var _ = time.Now

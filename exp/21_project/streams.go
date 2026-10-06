package main

import (
	"cmp"
	"context"
	"fmt"
	"log"
	"slices"
	"strings"

	"github.com/dmykolen/meeting-transcriber-go/internal/insights"
	"github.com/dmykolen/meeting-transcriber-go/internal/store"
)

const foldPrompt = `You keep the living picture of one project, meeting by meeting. Somebody opens it to learn where the project stands: its owner, a manager, a new colleague. It is a picture of the project, not a log of everything that was said.

You are given the picture as it stands and the summary of the meeting that has just happened. Answer with the changes this meeting makes. Never restate the picture.

The picture is organised in streams. A stream is a line of work with its own goal (for example "Role model" or "Procurement agent"). Every line belongs to exactly one stream. Use an existing stream, spelled exactly as listed, whenever the line belongs to it. Create a stream only for a genuinely new line of work, named in one to three words. Reusing is better than inventing, but a stream is a handful of related lines, not a heap: when a stream already holds about fifteen open lines, a new line that is about a narrower part of it gets a new, narrower stream. "General" is for the rare line that belongs to the whole project; it is never the default.

What is worth keeping. Somebody reading this in a month must still want the line:
- work: a commitment that moves the project: a deliverable, a request to somebody outside the meeting, a task with a clear owner. Not a personal chore, a scheduling errand ("call", "remind", "set up a meeting"), something to "think about", or a restatement of what was discussed.
- decision: something settled that will bind later work. A plan for this week, a temporary priority or a to-do is not a decision.
- question: something the project must still answer. Not a remark, and not something this meeting answers itself.

Rules, in order of importance:
1. Look for the line in the picture first, however differently worded, and refer to its id: "restate" when it was only mentioned again, "update" when its wording, owner or deadline genuinely changed. Two lines for one thing is the worst mistake.
2. Go through the open work and questions of every stream the meeting touched. When the meeting finished one, abandoned one or answered one, say so: "close" with state "done" or "dropped", or "answer" with the answer in text. Lines the meeting does not mention stay as they are.
3. "add" only what the picture lacks and is worth keeping. Most meetings add between none and four lines; an empty list is a good answer.
4. "overturn" only a decision this meeting explicitly reverses, with the replacement in text. Never delete the old one.
5. Owners: a real name as the summary gives it. When only a label such as SPEAKER_05 is given, leave the owner empty.

Also write "status": two or three plain sentences saying where the project stands now: what is agreed, what is waiting, what is late. It replaces the previous one.

Write in the language the meeting was held in. State facts, never opinions about people. Never invent a commitment nobody made.`

const tidyPrompt = `You are tidying one project's living document. Every line has an id, a kind (work, decision or question), a stream, a state, how many meetings mentioned it, and the number of the meeting that last touched it (out of %d so far).

Answer with:
- merges: lines that say the same thing. Keep the clearest id, list the others in "drop". Give replacement text only when none of them says it well.
- retire: open work or questions that are finished in effect: a later line shows they were done or replaced, or they were a small errand that cannot matter any more. State "done" or "dropped", and the reason in a few words. Be conservative: a line merely not mentioned lately is not retired. Never retire a decision.
- moves: lines that sit in the wrong stream. A stream with more than about fifteen open lines hides several lines of work: split it by moving groups of related lines into new streams with narrower names (each stream three to fifteen open lines), and empty "General" the same way. A decision that a later decision replaces is a merge or a move, never left standing twice.
- streams: streams that are the same line of work under two names; "from" is renamed to "to".

Answer with nothing for a part when nothing needs doing.`

const briefPrompt = `You are given a project's living document: its streams with their open lines and standing decisions. Write the picture a manager would want to read in one minute.

- headline: what the project is and where it stands overall, in two or three plain sentences.
- streams: for each stream with open work, one or two sentences on where it stands, then the ids of the open lines that matter most (at most four).
- decisions: the ids of the standing decisions that most shape the work (at most eight).
- attention: the ids of open lines that are blocked, overdue or at risk, with the reason in a few words (at most six).

Use only what the document says. Write in the language of the document.`

func obj(props map[string]any) map[string]any {
	req := make([]string, 0, len(props))
	for k := range props {
		req = append(req, k)
	}
	slices.Sort(req)
	return map[string]any{"type": "object", "properties": props, "required": req, "additionalProperties": false}
}
func arr(items map[string]any) map[string]any { return map[string]any{"type": "array", "items": items} }
func str(d string) map[string]any             { return map[string]any{"type": "string", "description": d} }
func num(d string) map[string]any             { return map[string]any{"type": "number", "description": d} }

var foldSchema = obj(map[string]any{
	"status": str("Two or three sentences on where the project stands now"),
	"changes": arr(obj(map[string]any{
		"do":     str("add, update, restate, close, answer or overturn"),
		"kind":   str("work, decision or question"),
		"id":     num("The id being changed; 0 when adding"),
		"text":   str("The line, the answer, or the replacing decision; empty when only restating"),
		"owner":  str("Who owns it, or empty"),
		"due":    str("The deadline as said, or empty"),
		"state":  str("done or dropped when closing; otherwise empty"),
		"stream": str("The stream the line belongs to; needed when adding"),
	})),
})

var tidySchema = obj(map[string]any{
	"merges":  arr(obj(map[string]any{"keep": num("id to keep"), "drop": arr(num("id merged into it")), "text": str("replacement text or empty")})),
	"retire":  arr(obj(map[string]any{"id": num("id"), "state": str("done or dropped"), "why": str("a few words")})),
	"moves":   arr(obj(map[string]any{"id": num("id"), "stream": str("the stream it belongs to")})),
	"streams": arr(obj(map[string]any{"from": str("stream renamed"), "to": str("name it becomes")})),
})

var briefSchema = obj(map[string]any{
	"headline":  str("Two or three sentences"),
	"streams":   arr(obj(map[string]any{"name": str("stream"), "state": str("one or two sentences"), "lines": arr(num("ids of the open lines that matter"))})),
	"decisions": arr(num("ids")),
	"attention": arr(obj(map[string]any{"id": num("id"), "why": str("a few words")})),
})

type change struct {
	Do, Kind, Text, Owner, Due, State, Stream string
	ID                                        int
}

func streams(c *insights.Client, rows []store.Recording) *doc {
	d := &doc{Streams: map[string]string{}}
	for i, r := range rows {
		var out struct {
			Status  string
			Changes []change
		}
		body := fmt.Sprintf("PROJECT: %s\n\nTHE PICTURE AS IT STANDS:\n%s\n\nTHE MEETING THAT JUST HAPPENED:\n%s",
			*name, cmp.Or(d.render(), "(empty — this is the first meeting)"), meetingText(r))
		if err := c.Structured(context.Background(), foldPrompt, body, "project_changes", foldSchema, &out); err != nil {
			log.Printf("fold %d/%d: %v", i+1, len(rows), err)
			continue
		}
		d.apply(out.Changes, i)
		d.Status = cmp.Or(out.Status, d.Status)
		log.Printf("fold %d/%d: %d changes, %d lines", i+1, len(rows), len(out.Changes), len(d.Items))
		if (i+1)%*every == 0 && i+1 < len(rows) {
			d.tidy(c, i+1)
		}
	}
	d.tidy(c, len(rows))
	d.brief(c)
	return d
}

// render shows the document the way the model reads it: open lines by stream.
func (d *doc) render() string {
	var b strings.Builder
	names := d.names()
	if len(names) == 0 {
		return ""
	}
	fmt.Fprintf(&b, "streams: %s\n", strings.Join(names, "; "))
	if d.Status != "" {
		fmt.Fprintf(&b, "status: %s\n", d.Status)
	}
	closed := 0
	for _, s := range names {
		fmt.Fprintf(&b, "\nstream %q (%d open):\n", s, d.openIn(s))
		for _, it := range d.Items {
			if it.Stream != s {
				continue
			}
			if !it.open() {
				closed++
				continue
			}
			fmt.Fprintf(&b, "  #%d %s [%s]", it.ID, it.Kind, it.State)
			fmt.Fprintf(&b, " %s", it.Text)
			if it.Owner != "" {
				fmt.Fprintf(&b, " — %s", it.Owner)
			}
			if it.Due != "" {
				fmt.Fprintf(&b, ", by %s", it.Due)
			}
			if it.Times > 1 {
				fmt.Fprintf(&b, " (mentioned %d times)", it.Times)
			}
			b.WriteString("\n")
		}
	}
	if closed > 0 {
		fmt.Fprintf(&b, "\n(%d finished or replaced lines are not shown)\n", closed)
	}
	return b.String()
}

func (d *doc) openIn(stream string) int {
	n := 0
	for _, it := range d.Items {
		if it.Stream == stream && it.open() {
			n++
		}
	}
	return n
}

func (d *doc) names() []string {
	seen := map[string]bool{}
	var out []string
	for _, it := range d.Items {
		if it.Stream != "" && !seen[it.Stream] {
			seen[it.Stream] = true
			out = append(out, it.Stream)
		}
	}
	return out
}

func (d *doc) find(id int) *item {
	for i := range d.Items {
		if d.Items[i].ID == id {
			return &d.Items[i]
		}
	}
	return nil
}

func (d *doc) apply(changes []change, at int) {
	for _, ch := range changes {
		if ch.Do == "add" {
			if strings.TrimSpace(ch.Text) == "" || !slices.Contains([]string{"work", "decision", "question"}, ch.Kind) {
				d.Ignored++
				continue
			}
			d.Next++
			state := map[string]string{"work": "open", "decision": "standing", "question": "open"}[ch.Kind]
			d.Items = append(d.Items, item{ID: d.Next, Kind: ch.Kind, Text: ch.Text, Owner: cleanOwner(ch.Owner), Due: ch.Due,
				State: state, Stream: cmp.Or(strings.TrimSpace(ch.Stream), "General"), Times: 1, Last: at})
			continue
		}
		it := d.find(ch.ID)
		if it == nil {
			d.Ignored++
			continue
		}
		it.Last = at
		switch ch.Do {
		case "restate":
			it.Times++
		case "update":
			it.Times++
			it.Text, it.Owner, it.Due = cmp.Or(ch.Text, it.Text), cleanOwner(ch.Owner), ch.Due
		case "close":
			it.State = cmp.Or(ch.State, "done")
		case "answer":
			it.State, it.Why = "answered", ch.Text
		case "overturn":
			it.State, it.Why = "overturned", ch.Text
		default:
			d.Ignored++
		}
	}
}

func cleanOwner(o string) string {
	if strings.HasPrefix(strings.ToUpper(o), "SPEAKER") {
		return ""
	}
	return o
}

// tidy is the consolidation pass.
func (d *doc) tidy(c *insights.Client, seen int) {
	var all strings.Builder
	for _, it := range d.Items {
		fmt.Fprintf(&all, "#%d %s [%s] stream %q, mentioned %d times, last touched meeting %d: %s\n", it.ID, it.Kind, it.State, it.Stream, it.Times, it.Last+1, it.Text)
	}
	var out struct {
		Merges []struct {
			Keep int
			Drop []int
			Text string
		}
		Retire []struct {
			ID         int
			State, Why string
		}
		Moves []struct {
			ID     int
			Stream string
		}
		Streams []struct{ From, To string }
	}
	if err := c.Structured(context.Background(), fmt.Sprintf(tidyPrompt, seen), all.String(), "project_tidy", tidySchema, &out); err != nil {
		log.Printf("tidy: %v", err)
		return
	}
	for _, m := range out.Merges {
		keep := d.find(m.Keep)
		if keep == nil {
			continue
		}
		for _, id := range m.Drop {
			if gone := d.find(id); gone != nil && gone.ID != keep.ID && gone.Kind == keep.Kind {
				keep.Times += gone.Times
				keep.Last = max(keep.Last, gone.Last)
				d.Items = slices.DeleteFunc(d.Items, func(it item) bool { return it.ID == id })
				keep = d.find(m.Keep)
			}
		}
		if m.Text != "" && keep != nil {
			keep.Text = m.Text
		}
	}
	for _, r := range out.Retire {
		if it := d.find(r.ID); it != nil && it.Kind != "decision" && it.open() {
			it.State, it.Why = cmp.Or(r.State, "dropped"), r.Why
		}
	}
	for _, mv := range out.Moves {
		if it := d.find(mv.ID); it != nil && mv.Stream != "" {
			it.Stream = mv.Stream
		}
	}
	for _, s := range out.Streams {
		for i := range d.Items {
			if d.Items[i].Stream == s.From && s.To != "" {
				d.Items[i].Stream = s.To
			}
		}
	}
	log.Printf("tidy after %d: %d merges, %d retired, %d moved, %d renamed -> %d lines", seen, len(out.Merges), len(out.Retire), len(out.Moves), len(out.Streams), len(d.Items))
}

func (d *doc) brief(c *insights.Client) {
	var out struct {
		Headline string
		Streams  []struct {
			Name, State string
			Lines       []int
		}
		Decisions []int
		Attention []struct {
			ID  int
			Why string
		}
	}
	if err := c.Structured(context.Background(), briefPrompt, d.render(), "project_brief", briefSchema, &out); err != nil {
		log.Printf("brief: %v", err)
		return
	}
	d.Headline = out.Headline
	var md strings.Builder
	fmt.Fprintf(&md, "# %s\n\n%s\n", *name, out.Headline)
	for _, s := range out.Streams {
		fmt.Fprintf(&md, "\n## %s\n%s\n", s.Name, s.State)
		for _, id := range s.Lines {
			if it := d.find(id); it != nil {
				fmt.Fprintf(&md, "- %s%s%s\n", it.Text, tail(" — ", it.Owner), tail(", by ", it.Due))
			}
		}
	}
	md.WriteString("\n## Decisions that shape the work\n")
	for _, id := range out.Decisions {
		if it := d.find(id); it != nil {
			fmt.Fprintf(&md, "- %s\n", it.Text)
		}
	}
	md.WriteString("\n## Needs attention\n")
	for _, a := range out.Attention {
		if it := d.find(a.ID); it != nil {
			fmt.Fprintf(&md, "- %s (%s)%s\n", it.Text, a.Why, tail(" — ", it.Owner))
		}
	}
	d.Brief = md.String()
}

func tail(sep, s string) string {
	if s == "" {
		return ""
	}
	return sep + s
}

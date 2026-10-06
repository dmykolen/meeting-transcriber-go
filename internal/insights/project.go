package insights

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"
)

// Advance applies one meeting summary to a project's living document.
func (c *Client) Advance(ctx context.Context, project, held, meeting string) (string, []Change, error) {
	if !c.Ready() {
		return "", nil, ErrNoKey
	}
	var out struct {
		Status  string   `json:"status"`
		Changes []Change `json:"changes"`
	}
	body := fmt.Sprintf(
		"PROJECT: %s\n\nTHE PICTURE AS IT STANDS:\n%s\n\nTHE MEETING THAT JUST HAPPENED:\n%s",
		project, cmp.Or(held, "(empty — this is the first meeting)"), meeting)

	if err := c.Structured(ctx, advancePrompt, body, "project_changes", changes, &out); err != nil {
		return "", nil, err
	}
	return out.Status, out.Changes, nil
}

// Change is one operation against the document.
type Change struct {
	Do     string `json:"do"`
	Kind   string `json:"kind"`
	ID     int    `json:"id"`
	Text   string `json:"text"`
	Owner  string `json:"owner"`
	Due    string `json:"due"`
	State  string `json:"state"`
	Stream string `json:"stream"`
}

const advancePrompt = `You keep the living picture of one project, meeting by meeting. Somebody opens it to learn where the project stands: its owner, a manager, a new colleague. It is a picture of the project, not a log of everything that was said.

You are given the picture as it stands and the summary of the meeting that has just happened. Answer with the changes this meeting makes. Never restate the picture.

The picture is organised in streams. A stream is a line of work with its own goal (for example "Role model" or "Procurement agent"). Every line belongs to exactly one stream. Use an existing stream, spelled exactly as listed, whenever the line belongs to it. Create a stream only for a genuinely new line of work, named in one to three words. Reusing is better than inventing, but a stream is a handful of related lines, not a heap: when a stream already holds about fifteen open lines, a new line about a narrower part of it gets a new, narrower stream. "General" is for the rare line that belongs to the whole project; it is never the default.

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

func object(properties map[string]any) map[string]any {
	required := make([]string, 0, len(properties))
	for name := range properties {
		required = append(required, name)
	}
	slices.Sort(required)
	return map[string]any{
		"type":                 "object",
		"properties":           properties,
		"required":             required,
		"additionalProperties": false,
	}
}

// changes is the structured-output schema the model must fill.
var changes = object(map[string]any{
	"status": text("Two or three sentences on where the project stands now"),
	"changes": array(object(map[string]any{
		"do":     choice("What this meeting does to the line", "add", "update", "restate", "close", "answer", "overturn"),
		"kind":   choice("Which kind of line", "work", "decision", "question"),
		"id":     number("The id being changed; 0 when adding"),
		"text":   text("The line, the answer, or the replacing decision; empty when only restating"),
		"owner":  text("Who owns it, or empty"),
		"due":    text("The deadline as said, or empty"),
		"state":  text("done or dropped when closing; otherwise empty"),
		"stream": text("The stream the line belongs to; needed when adding"),
	})),
})

// choice is a string the model must pick from a list. A small local model that
// is free to write any verb writes the wrong one, and the line is lost.
func choice(description string, values ...string) map[string]any {
	return map[string]any{"type": "string", "description": description, "enum": values}
}

// Tidying is a model's pass over a whole project document.
type Tidying struct {
	Merges []struct {
		Keep int    `json:"keep"`
		Drop []int  `json:"drop"`
		Text string `json:"text"`
	} `json:"merges"`
	Retire []struct {
		ID    int    `json:"id"`
		State string `json:"state"`
		Why   string `json:"why"`
	} `json:"retire"`
	Moves []struct {
		ID     int    `json:"id"`
		Stream string `json:"stream"`
	} `json:"moves"`
	Streams []struct {
		From string `json:"from"`
		To   string `json:"to"`
	} `json:"streams"`
}

const tidyPrompt = `You are tidying one project's living document. Every line has an id, a kind (work, decision or question), a stream, a state, how many meetings mentioned it, and the number of the meeting that last touched it (out of %d so far).

Answer with:
- merges: lines that say the same thing. Keep the clearest id, list the others in "drop". Give replacement text only when none of them says it well.
- retire: open work or questions that are finished in effect: a later line shows they were done or replaced, or they were a small errand that cannot matter any more. State "done" or "dropped", and the reason in a few words. Be conservative: a line merely not mentioned lately is not retired. Never retire a decision.
- moves: lines that sit in the wrong stream. A stream with more than about fifteen open lines hides several lines of work: split it by moving groups of related lines into new streams with narrower names (each stream three to fifteen open lines), and empty "General" the same way. A decision that a later decision replaces is a merge or a move, never left standing twice.
- streams: streams that are the same line of work under two names; "from" is renamed to "to".

Answer with nothing for a part when nothing needs doing.`

var tidying = object(map[string]any{
	"merges":  array(object(map[string]any{"keep": number("id to keep"), "drop": array(number("id merged into it")), "text": text("replacement text or empty")})),
	"retire":  array(object(map[string]any{"id": number("id"), "state": text("done or dropped"), "why": text("a few words")})),
	"moves":   array(object(map[string]any{"id": number("id"), "stream": text("the stream it belongs to")})),
	"streams": array(object(map[string]any{"from": text("stream renamed"), "to": text("name it becomes")})),
})

// Tidy asks for a pass over the whole document, every line as Held renders it
// with its state and how lately it was touched.
func (c *Client) Tidy(ctx context.Context, lines string, meetings int) (*Tidying, error) {
	if !c.Ready() {
		return nil, ErrNoKey
	}
	var out Tidying
	if err := c.Structured(ctx, fmt.Sprintf(tidyPrompt, meetings), lines, "project_tidy", tidying, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Brief is the project as one reads it in a minute.
type Brief struct {
	Headline string `json:"headline"`
	Streams  []struct {
		Name  string `json:"name"`
		State string `json:"state"`
		Lines []int  `json:"lines"`
	} `json:"streams"`
	Decisions []int `json:"decisions"`
	Attention []struct {
		ID  int    `json:"id"`
		Why string `json:"why"`
	} `json:"attention"`
}

const briefPrompt = `You are given a project's living document: its streams with their open lines and standing decisions. Write the picture a manager would want to read in one minute.

- headline: what the project is and where it stands overall, in two or three plain sentences.
- streams: for each stream with open work, one or two sentences on where it stands, then the ids of the open lines that matter most (at most four).
- decisions: the ids of the standing decisions that most shape the work (at most eight).
- attention: the ids of open lines that are blocked, overdue or at risk, with the reason in a few words (at most six).

Use only what the document says. Write in the language of the document.`

var briefing = object(map[string]any{
	"headline": text("Two or three sentences"),
	"streams": array(object(map[string]any{
		"name": text("stream"), "state": text("one or two sentences"), "lines": array(number("ids of the open lines that matter")),
	})),
	"decisions": array(number("ids")),
	"attention": array(object(map[string]any{"id": number("id"), "why": text("a few words")})),
})

// Brief writes the picture from the document as Held renders it.
func (c *Client) Brief(ctx context.Context, held string) (*Brief, error) {
	if !c.Ready() {
		return nil, ErrNoKey
	}
	var out Brief
	if err := c.Structured(ctx, briefPrompt, held, "project_brief", briefing, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Line is one line of the project document as the prompts show it.
type Line struct {
	ID     int
	Kind   string // work, decision or question
	Stream string
	Text   string
	Owner  string
	Due    string
	State  string
	Times  int
	Last   int // the number of the meeting that last touched it; 0 when not shown
}

// Held renders the document the way the model reads it: the open lines, by
// stream. Finished lines are only counted, so the prompt stays the size of the
// project's present rather than of its whole history.
func Held(status string, lines []Line) string {
	open := func(l Line) bool { return l.State == "open" || l.State == "standing" }
	var names []string
	count := map[string]int{}
	closed := 0
	for _, l := range lines {
		if !open(l) {
			closed++
			continue
		}
		if count[l.Stream] == 0 {
			names = append(names, l.Stream)
		}
		count[l.Stream]++
	}
	if len(names) == 0 {
		return ""
	}
	slices.Sort(names)
	var b strings.Builder
	fmt.Fprintf(&b, "streams: %s\n", strings.Join(names, "; "))
	if status != "" {
		fmt.Fprintf(&b, "status: %s\n", status)
	}
	for _, name := range names {
		fmt.Fprintf(&b, "\nstream %q (%d open):\n", name, count[name])
		for _, l := range lines {
			if l.Stream != name || !open(l) {
				continue
			}
			fmt.Fprintf(&b, "  #%d %s [%s] %s", l.ID, l.Kind, l.State, l.Text)
			if l.Owner != "" {
				fmt.Fprintf(&b, " — %s", l.Owner)
			}
			if l.Due != "" {
				fmt.Fprintf(&b, ", by %s", l.Due)
			}
			if l.Times > 1 {
				fmt.Fprintf(&b, " (mentioned %d times)", l.Times)
			}
			if l.Last > 0 {
				fmt.Fprintf(&b, " (last touched meeting %d)", l.Last)
			}
			b.WriteString("\n")
		}
	}
	if closed > 0 {
		fmt.Fprintf(&b, "\n(%d finished or replaced lines are not shown)\n", closed)
	}
	return b.String()
}

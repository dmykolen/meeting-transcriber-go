package insights

import (
	"cmp"
	"context"
	"fmt"
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
		"PROJECT: %s\n\nTHE DOCUMENT AS IT STANDS:\n%s\n\nTHE MEETING THAT JUST HAPPENED:\n%s",
		project, cmp.Or(held, "(empty — this is the first meeting)"), meeting)

	if err := c.Structured(ctx, advancePrompt, body, "project_changes", changes, &out); err != nil {
		return "", nil, err
	}
	return out.Status, out.Changes, nil
}

// Change is one operation against the document.
type Change struct {
	Do    string `json:"do"`
	Kind  string `json:"kind"`
	ID    int    `json:"id"`
	Text  string `json:"text"`
	Owner string `json:"owner"`
	Due   string `json:"due"`
	State string `json:"state"`
}

const advancePrompt = `You keep one living document per project, updated meeting by meeting.

You are given the document as it stands — every line carries an id — and the
summary of the meeting that has just happened. Answer with the changes that
meeting makes to the document. Never restate the document.

Rules, in order of importance:

1. Before adding anything, look for it in the document. If this meeting is
   talking about something already there, however differently worded, refer to
   its id. "restate" when it was simply mentioned again; "update" when the
   wording, the owner or the deadline has genuinely changed. Adding a second
   line for one commitment is the worst mistake you can make here.
2. "add" only for something the document does not contain at all.
3. "close" a commitment the meeting says is finished or abandoned; put "done" or
   "dropped" in state.
4. "answer" a question the meeting answers, and put the answer in text.
5. "overturn" a decision this meeting reverses, and put the new decision in text.
   Never delete the old one — a project's history of reversals is the most
   expensive thing in it to reconstruct.
6. Leave everything else alone. Silence about a line means it has not changed.

Also write "status": one short paragraph, in the plainest language, saying where
the project stands right now — what is agreed, what is waiting, what is late.
Somebody should be able to read it aloud to their manager. Rewrite it fully each
time; it replaces the previous one.

Write in the language the meeting was held in. State facts, never opinions about
whether people are doing well. Never invent a commitment nobody made.`

// changes is the structured-output schema the model must fill.
var changes = object(map[string]any{
	"status": text("One short paragraph on where the project stands now"),
	"changes": array(object(map[string]any{
		"do":    text("add, update, restate, close, answer or overturn"),
		"kind":  text("work, decision or question"),
		"id":    number("The id being changed; 0 when adding"),
		"text":  text("The line, the answer, or the replacing decision; empty when only restating"),
		"owner": text("Who owns it, or empty"),
		"due":   text("The deadline exactly as said, or empty"),
		"state": text("done or dropped when closing; otherwise empty"),
	})),
})

// Held renders the document in the prompt shape the model consumes.
func Held(status string, sections map[string][]Line) string {
	var b strings.Builder
	if status != "" {
		fmt.Fprintf(&b, "status: %s\n\n", status)
	}
	for _, kind := range []string{"work", "decision", "question"} {
		lines := sections[kind]
		if len(lines) == 0 {
			continue
		}
		fmt.Fprintf(&b, "%s:\n", kind)
		for _, l := range lines {
			fmt.Fprintf(&b, "  #%d [%s] %s", l.ID, l.State, l.Text)
			if l.Owner != "" {
				fmt.Fprintf(&b, " — %s", l.Owner)
			}
			if l.Due != "" {
				fmt.Fprintf(&b, ", by %s", l.Due)
			}
			if l.Times > 1 {
				fmt.Fprintf(&b, " (said %d times)", l.Times)
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}
	return b.String()
}

// Line is the stored-item shape Held needs.
type Line struct {
	ID    int
	Text  string
	Owner string
	Due   string
	State string
	Times int
}

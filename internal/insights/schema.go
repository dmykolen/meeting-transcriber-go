package insights

// Schema is the strict JSON shape the summary model must return.
var Schema = object(map[string]any{
	"title":    text("Four to eight words naming what this meeting was about"),
	"overview": text("One paragraph: what it was for, the main subjects and what came of each, what is left open"),
	"chapters": array(object(map[string]any{
		"start":   number("Seconds from the start of the recording"),
		"title":   text("A few words naming the subject of this part"),
		"summary": text("What happened in this part: the question, who said what and why, facts and figures, where it ended"),
	})),
	"topics":    array(text("A topic discussed")),
	"decisions": array(text("Something actually settled")),
	"action_items": array(object(map[string]any{
		"task":  text("What was committed to, phrased as an instruction"),
		"owner": text("Who owns it, or empty if nobody was named"),
		"due":   text("The deadline exactly as said, or empty"),
	})),
	"open_questions": array(text("Raised and left unresolved")),
})

// NoteSchema is the shape of a voice note written out by Polish.
var NoteSchema = object(map[string]any{
	"title": text("Three to seven words naming what the note is about"),
	"text":  text("The note in the speaker's own voice, cleaned up; paragraphs, and \"- \" lines for a list"),
	"action_items": array(object(map[string]any{
		"task": text("What they mean to do, phrased as an instruction"),
		"due":  text("YYYY-MM-DD when the day follows from what was said, the words when vague, or empty"),
	})),
})

func array(items map[string]any) map[string]any {
	return map[string]any{"type": "array", "items": items}
}

func text(description string) map[string]any {
	return map[string]any{"type": "string", "description": description}
}

func number(description string) map[string]any {
	return map[string]any{"type": "number", "description": description}
}

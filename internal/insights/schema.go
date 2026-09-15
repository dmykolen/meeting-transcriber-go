package insights

// schema is the strict JSON shape the summary model must return.
var schema = object(map[string]any{
	"title":    text("Four to eight words naming what this meeting was about"),
	"overview": text("Two or three sentences on what happened"),
	"chapters": array(object(map[string]any{
		"start":   number("Seconds from the start of the recording"),
		"title":   text("A few words naming the topic"),
		"summary": text("One sentence on what was covered"),
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

func object(properties map[string]any) map[string]any {
	required := make([]string, 0, len(properties))
	for name := range properties {
		required = append(required, name)
	}
	return map[string]any{
		"type":                 "object",
		"properties":           properties,
		"required":             required,
		"additionalProperties": false,
	}
}

func array(items map[string]any) map[string]any {
	return map[string]any{"type": "array", "items": items}
}

func text(description string) map[string]any {
	return map[string]any{"type": "string", "description": description}
}

func number(description string) map[string]any {
	return map[string]any{"type": "number", "description": description}
}

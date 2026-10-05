package store

import (
	"testing"
	"time"
)

func TestUsageIsSummedByDayAndModel(t *testing.T) {
	db := open(t)
	now := time.Now()
	for _, c := range []Call{
		{At: now, Task: "meeting_summary", Provider: "openai", Model: "gpt-5.4-mini", Input: 100, Output: 10, Took: 2 * time.Second},
		{At: now, Task: "answer", Provider: "openai", Model: "gpt-5.4-mini", Input: 50, Output: 5, Took: time.Second, Failed: "quota"},
		{At: now, Task: "embed", Provider: "local", Model: "vectors", Input: 7},
		{At: now.AddDate(0, 0, -40), Task: "answer", Provider: "openai", Model: "gpt-5.4-mini", Input: 999},
	} {
		if err := db.Called(c); err != nil {
			t.Fatal(err)
		}
	}
	got, err := db.Usage(now.AddDate(0, 0, -30))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("rows = %+v", got)
	}
	// Ordered by day, then provider: local before openai.
	if m := got[1]; m.Model != "gpt-5.4-mini" || m.Calls != 2 || m.Failed != 1 || m.Input != 150 || m.Output != 15 || m.Seconds != 3 {
		t.Fatalf("the OpenAI row = %+v", m)
	}
}

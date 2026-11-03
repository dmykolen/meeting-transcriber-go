// Command 26_voice_repair brings an archive made before confirmed voice samples
// (2026-11-02, see "Voice samples" in .spec/decisions.md) up to the new rules.
// The database cannot tell the owner's names from the app's; the app log can, so
// this reads it. Run it on a copy first, and on the real file only while the app
// is closed:
//
//	go run ./exp/26_voice_repair -db <meetings.db> -log <logs/mt.log>
//
// In order: people that only took a name away ("-") are forgotten; the
// corrections in exp/truth/25-corrections.json are applied as renames and Nos;
// samples whose meeting is gone or no longer has that name are dropped; every
// name the owner gave by hand in a logged meeting is filed as a confirmed
// sample; search passages take the transcript's current speakers; and, with
// -guesses, each name the app gave by itself becomes a guess with its
// similarity to that person's confirmed samples, when there are any.
package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/dmykolen/meeting-transcriber-go/internal/store"
)

func main() {
	path := flag.String("db", "", "meetings.db to repair")
	logPath := flag.String("log", "", "the app log, logs/mt.log")
	fixPath := flag.String("fix", "exp/truth/25-corrections.json", `{"recording": {"label": "name" or null}}`)
	me := flag.String("me", "Me", "the owner's name in the settings")
	guess := flag.Bool("guesses", false, "also turn the names the app gave by itself into guesses")
	flag.Parse()

	db, err := store.Open(*path)
	check(err)
	raw, err := sql.Open("sqlite", *path)
	check(err)
	person := func(n string) bool {
		return n != *me && strings.ContainsFunc(n, unicode.IsLetter) && !strings.HasPrefix(strings.ToUpper(n), "SPEAKER_")
	}

	fmt.Println("== people that only took a name away")
	var names []string
	for _, r := range rows(raw, `SELECT name FROM people`) {
		switch {
		case person(r[0]):
			names = append(names, r[0])
		case r[0] != *me:
			fmt.Printf("forget %q\n", r[0])
			check(db.Forget(r[0]))
		}
	}

	// The log names what the recogniser gave in each transcription; the last one
	// of a recording is what its names came from.
	text, err := os.ReadFile(*logPath)
	check(err)
	transcribing := regexp.MustCompile(`transcribing id=(\d+)`)
	// Older lines quote the message: msg="voices recognised" id=123 who="[…]".
	recognised := regexp.MustCompile(`voices recognised"? id=(\d+) who="?\[(.*?)\]`)
	longest := slices.Clone(names)
	sort.Slice(longest, func(a, b int) bool { return len(longest[a]) > len(longest[b]) })
	auto, last := map[int64][]string{}, map[int64]int{}
	for i, line := range strings.Split(string(text), "\n") {
		if m := transcribing.FindStringSubmatch(line); m != nil {
			auto[id(m[1])], last[id(m[1])] = nil, i
		} else if m := recognised.FindStringSubmatch(line); m != nil {
			who, got := m[2], []string(nil)
			for _, n := range longest {
				if strings.Contains(who, n) {
					got, who = append(got, n), strings.ReplaceAll(who, n, "")
				}
			}
			auto[id(m[1])] = got
		}
	}
	logged := make([]int64, 0, len(last))
	for r := range last {
		logged = append(logged, r)
	}
	sort.Slice(logged, func(a, b int) bool { return last[logged[a]] < last[logged[b]] })

	fmt.Println("\n== corrections by hand")
	fixes := map[string]map[string]*string{}
	if b, err := os.ReadFile(*fixPath); err == nil {
		check(json.Unmarshal(b, &fixes))
	}
	touched := map[string]bool{}
	for rec, labels := range fixes {
		r := id(rec)
		// No first, so a right name never merges into a wrong one.
		for _, no := range []bool{true, false} {
			for label, to := range labels {
				if (to == nil) != no || !slices.Contains(speakers(db, r), label) {
					continue
				}
				if no {
					fmt.Printf("%d: %q is nobody enrolled\n", r, label)
					check(db.Reject(r, label))
					continue
				}
				fmt.Printf("%d: %q is %q\n", r, label, *to)
				print := db.VoiceIn(r, label)
				check(db.Rename(r, label, *to))
				if len(print) > 0 {
					check(db.Remember(*to, print, store.Source{Recording: r, Speaker: *to, Confirmed: true}))
				}
				touched[fmt.Sprint(r, *to)] = true
			}
		}
	}

	fmt.Println("\n== samples whose meeting is gone or no longer has the name")
	binned := map[int64]bool{}
	for _, r := range rows(raw, `SELECT id, deleted IS NOT NULL FROM recordings`) {
		binned[id(r[0])] = r[1] == "1"
	}
	for _, r := range rows(raw, `SELECT name, sources FROM people`) {
		var came []store.Source
		_ = json.Unmarshal([]byte(r[1]), &came)
		for _, from := range came {
			gone, kept := binned[from.Recording]
			if from.Recording == 0 || (kept && (gone || slices.Contains(speakers(db, from.Recording), r[0]) || voices(db, from.Recording)[r[0]] != nil)) {
				continue
			}
			fmt.Printf("drop %q sample from %d (meeting %s)\n", r[0], from.Recording, map[bool]string{true: "has another name there", false: "deleted"}[kept])
			check(db.Reject(from.Recording, r[0]))
		}
	}

	fmt.Println("\n== names given by hand become confirmed samples")
	for _, r := range logged {
		if gone, kept := binned[r]; !kept || gone {
			continue
		}
		v := voices(db, r)
		for _, label := range sortedKeys(v) {
			if person(label) && !slices.Contains(auto[r], label) && len(v[label]) == 192 && !touched[fmt.Sprint(r, label)] {
				fmt.Printf("confirm %q from %d\n", label, r)
				check(db.Remember(label, v[label], store.Source{Recording: r, Speaker: label, Confirmed: true}))
			}
		}
	}

	fmt.Println("\n== search passages take the transcript's speakers")
	turn := `(SELECT t.speaker FROM turns t WHERE t.recording = passages.recording AND t.start = passages.start ORDER BY t.seq LIMIT 1)`
	res, err := raw.Exec(`UPDATE passages SET speaker = ` + turn + ` WHERE ` + turn + ` IS NOT NULL AND speaker <> ` + turn)
	check(err)
	n, _ := res.RowsAffected()
	fmt.Println("passages renamed:", n)

	fmt.Println("\n== names the app gave become guesses")
	if !*guess {
		fmt.Println("skipped: pass -guesses")
	}
	people, err := db.People()
	check(err)
	type sample struct {
		from  int64
		print []float32
	}
	sure := map[string][]sample{}
	for _, p := range people {
		for i, s := range p.Sources {
			if s.Confirmed {
				sure[p.Name] = append(sure[p.Name], sample{s.Recording, p.Voiceprints[i]})
			}
		}
	}
	guessed, unknown := 0, map[string]int{}
	for _, r := range logged {
		if gone, kept := binned[r]; !kept || gone || !*guess {
			continue
		}
		v := voices(db, r)
		got, err := db.Get(r)
		check(err)
		g := got.Guesses
		if g == nil {
			g = map[string]float64{}
		}
		changed := false
		for _, label := range sortedKeys(v) {
			if !person(label) || !slices.Contains(auto[r], label) || len(v[label]) != 192 || touched[fmt.Sprint(r, label)] {
				continue
			}
			if _, already := g[label]; already {
				continue
			}
			best := -2.0
			for _, s := range sure[label] {
				if s.from != r {
					best = max(best, store.Cosine(v[label], s.print))
				}
			}
			if best == -2 {
				unknown[label]++
				continue
			}
			g[label], changed = best, true
			guessed++
			if best < 0.70 {
				fmt.Printf("%d: %q %.0f%%\n", r, label, 100*best)
			}
		}
		if changed {
			check(db.SaveVoices(r, v, g))
		}
	}
	fmt.Printf("guesses made: %d; left as they were, the person has no confirmed sample: %v\n", guessed, unknown)

	fmt.Println("\n== people now")
	people, err = db.People()
	check(err)
	for _, p := range people {
		c := 0
		for _, s := range p.Sources {
			if s.Confirmed {
				c++
			}
		}
		fmt.Printf("%-24s %2d samples, %d confirmed\n", p.Name, p.Samples, c)
	}
}

func rows(db *sql.DB, query string) [][]string {
	r, err := db.Query(query)
	check(err)
	defer r.Close()
	var out [][]string
	for r.Next() {
		var a, b sql.NullString
		cols, _ := r.Columns()
		if len(cols) == 1 {
			check(r.Scan(&a))
		} else {
			check(r.Scan(&a, &b))
		}
		out = append(out, []string{a.String, b.String})
	}
	check(r.Err())
	return out
}

func speakers(db *store.DB, r int64) []string {
	got, err := db.Get(r)
	if err != nil {
		return nil
	}
	return got.Speakers
}

func voices(db *store.DB, r int64) map[string][]float32 {
	var v map[string][]float32
	_ = json.Unmarshal([]byte(db.RawVoices(r)), &v)
	return v
}

func sortedKeys(v map[string][]float32) []string {
	keys := make([]string, 0, len(v))
	for k := range v {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func id(s string) int64 {
	n, err := strconv.ParseInt(s, 10, 64)
	check(err)
	return n
}

func check(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

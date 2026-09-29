package library

import (
	"context"
	"errors"
	"fmt"
	"github.com/dmykolen/meeting-transcriber-go/internal/insights"
	"github.com/dmykolen/meeting-transcriber-go/internal/store"
	"sort"
	"strings"
)

// SearchKnowledge uses the existing embedding provider and local SQLite cache.
func (l *Library) SearchKnowledge(ctx context.Context, query string, semantic bool) ([]store.KnowledgeHit, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return []store.KnowledgeHit{}, nil
	}
	docs, err := l.db.Knowledge()
	if err != nil {
		return nil, err
	}
	if len(docs) == 0 {
		return []store.KnowledgeHit{}, nil
	}
	type match struct {
		hit   store.KnowledgeHit
		score float64
	}
	matches := []match{}
	if !semantic {
		terms := strings.Fields(strings.ToLower(query))
		for _, h := range docs {
			body := strings.ToLower(h.Title + " " + h.Text)
			found := true
			for _, term := range terms {
				if !strings.Contains(body, term) {
					found = false
					break
				}
			}
			if found {
				matches = append(matches, match{h, 1})
			}
		}
	} else {
		ai := l.AI()
		if !ai.Searchable() {
			return nil, errSearchOff
		}
		cached, err := l.learn(ctx, ai, docs, func(int) {})
		if err != nil {
			return nil, err
		}
		asked, err := ai.Query(ctx, query)
		if err != nil {
			return nil, err
		}
		for _, h := range docs {
			score := store.Cosine(asked, cached[store.KnowledgeKey(h)])
			if score >= .25 {
				matches = append(matches, match{h, score})
			}
		}
	}
	sort.SliceStable(matches, func(i, j int) bool { return matches[i].score > matches[j].score })
	out := []store.KnowledgeHit{}
	for _, m := range matches[:min(60, len(matches))] {
		out = append(out, m.hit)
	}
	return out, nil
}

// learn gives every knowledge document a vector, forgets the vectors of
// documents that are gone, and reports how many documents are done.
func (l *Library) learn(ctx context.Context, ai *insights.Client, docs []store.KnowledgeHit, progress func(done int)) (map[string][]float32, error) {
	cached, err := l.db.KnowledgeVectors()
	if err != nil {
		return nil, err
	}
	current := map[string]bool{}
	missing := []store.KnowledgeHit{}
	for _, h := range docs {
		key := store.KnowledgeKey(h)
		current[key] = true
		if len(cached[key]) == 0 {
			missing = append(missing, h)
		}
	}
	for key := range cached {
		if !current[key] {
			if err = l.db.DropKnowledgeVector(key); err != nil {
				return nil, err
			}
		}
	}
	for i := 0; i < len(missing); i += 32 {
		if err = ctx.Err(); err != nil {
			return nil, err
		}
		batch := missing[i:min(i+32, len(missing))]
		texts := make([]string, len(batch))
		for j, h := range batch {
			texts[j] = h.Title + "\n" + h.Text
		}
		vectors, err := ai.Embed(ctx, texts)
		if err != nil {
			return nil, err
		}
		if len(vectors) != len(batch) {
			return nil, errors.New("Не вдалося індексувати всі фрагменти")
		}
		for j, h := range batch {
			if len(vectors[j]) != 512 {
				return nil, errors.New("Неповна відповідь під час індексації. Повторіть пошук")
			}
			key := store.KnowledgeKey(h)
			cached[key] = vectors[j]
			if err = l.db.CacheKnowledge(key, vectors[j]); err != nil {
				return nil, err
			}
		}
		progress(len(docs) - len(missing) + i + len(batch))
	}
	return cached, nil
}

func (l *Library) AskKnowledge(ctx context.Context, question string) (string, []store.KnowledgeHit, error) {
	hits, err := l.SearchKnowledge(ctx, question, true)
	if err != nil {
		return "", nil, err
	}
	if len(hits) == 0 {
		return "", hits, errors.New("У доступному архіві немає достатніх джерел для відповіді")
	}
	hits = hits[:min(16, len(hits))]
	passages := make([]string, len(hits))
	for i, h := range hits {
		stamp := ""
		if h.Kind == "transcript" {
			stamp = fmt.Sprintf(" · %d:%02d", int(h.Start)/60, int(h.Start)%60)
		}
		passages[i] = fmt.Sprintf("[%d] %s · %s%s\n%s", i+1, h.Kind, h.Title, stamp, h.Text)
	}
	answer, err := l.AI().Answer(ctx, question, passages)
	return answer, hits, err
}

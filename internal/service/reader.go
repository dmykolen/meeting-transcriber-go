package service

import (
	"context"
	"github.com/dmykolen/meeting-transcriber-go/internal/store"
	"time"
)

func (m *Meetings) Notes(recording, project int64) ([]store.Sticky, error) {
	return m.db.Notes(recording, project)
}
func (m *Meetings) PutNote(note store.Sticky) (store.Sticky, error) { return m.db.PutNote(note) }
func (m *Meetings) RemoveNote(id int64) error                       { return m.db.RemoveNote(id) }
func (m *Meetings) EditAction(id int64, index int, action store.Action) error {
	return m.db.EditAction(id, index, action)
}
func (m *Meetings) PreviewSummary(id int64) (*store.Summary, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	return m.lib.PreviewSummary(ctx, id)
}
func (m *Meetings) AcceptSummary(id int64, before, after *store.Summary) error {
	return m.db.AcceptSummary(id, before, after)
}

type KnowledgeAnswer struct {
	Text    string               `json:"text"`
	Sources []store.KnowledgeHit `json:"sources"`
}

func (m *Meetings) SearchKnowledge(query string, semantic bool) ([]store.KnowledgeHit, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	return m.lib.SearchKnowledge(ctx, query, semantic)
}
func (m *Meetings) AskKnowledge(question string) (*KnowledgeAnswer, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	text, hits, err := m.lib.AskKnowledge(ctx, question)
	if err != nil {
		return nil, err
	}
	return &KnowledgeAnswer{Text: text, Sources: hits}, nil
}

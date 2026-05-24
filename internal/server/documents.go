// SPDX-License-Identifier: GPL-3.0-only

package server

import (
	"sync"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// Document is the latest known content of an open buffer. Version numbering
// is owned by the client per LSP.
type Document struct {
	URI     uri.URI
	Lang    protocol.LanguageIdentifier
	Version int32
	Text    string
}

type DocumentStore struct {
	mu   sync.RWMutex
	docs map[uri.URI]*Document
}

func NewDocumentStore() *DocumentStore {
	return &DocumentStore{docs: make(map[uri.URI]*Document)}
}

// Open records a newly-opened document, replacing any prior entry for the
// same URI (the spec permits re-opening).
func (s *DocumentStore) Open(item protocol.TextDocumentItem) *Document {
	d := &Document{
		URI:     item.URI,
		Lang:    item.LanguageID,
		Version: item.Version,
		Text:    item.Text,
	}
	s.mu.Lock()
	s.docs[item.URI] = d
	s.mu.Unlock()
	return d
}

// Update replaces the full text. Only Full sync is supported for now.
func (s *DocumentStore) Update(u uri.URI, version int32, text string) (*Document, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, ok := s.docs[u]
	if !ok {
		return nil, false
	}
	d.Version = version
	d.Text = text
	return d, true
}

func (s *DocumentStore) Close(u uri.URI) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.docs[u]; !ok {
		return false
	}
	delete(s.docs, u)
	return true
}

func (s *DocumentStore) Get(u uri.URI) (*Document, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	d, ok := s.docs[u]
	return d, ok
}

// All returns a snapshot of currently-open documents. Callers may iterate
// the result without holding the store's lock.
func (s *DocumentStore) All() map[uri.URI]*Document {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[uri.URI]*Document, len(s.docs))
	for k, v := range s.docs {
		out[k] = v
	}
	return out
}

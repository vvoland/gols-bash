// SPDX-License-Identifier: GPL-3.0-only

package analyser

import (
	"sync"

	"go.lsp.dev/uri"
	"mvdan.cc/sh/v3/syntax"
)

// Index is a goroutine-safe URI → parsed-file map. Open documents and
// disk-scanned workspace files share the same store; the latest write
// wins. Lookup is read-mostly.
type Index struct {
	mu      sync.RWMutex
	entries map[uri.URI]*syntax.File
}

func NewIndex() *Index {
	return &Index{entries: make(map[uri.URI]*syntax.File)}
}

// AddOrReplace stores file under u. A nil file removes the entry — for
// parse failures with no recoverable tree, callers can pass nil to drop
// stale state instead of indexing a half-tree.
func (i *Index) AddOrReplace(u uri.URI, file *syntax.File) {
	i.mu.Lock()
	defer i.mu.Unlock()
	if file == nil {
		delete(i.entries, u)
		return
	}
	i.entries[u] = file
}

func (i *Index) Remove(u uri.URI) {
	i.mu.Lock()
	delete(i.entries, u)
	i.mu.Unlock()
}

func (i *Index) Get(u uri.URI) *syntax.File {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.entries[u]
}

func (i *Index) Len() int {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return len(i.entries)
}

// UsageHit pairs a usage with the file it came from.
type UsageHit struct {
	URI   uri.URI
	Usage Usage
}

type DeclarationHit struct {
	URI         uri.URI
	Declaration Declaration
}

// AllDeclarations walks every indexed file's AST for declarations.
func (i *Index) AllDeclarations() []DeclarationHit {
	i.mu.RLock()
	defer i.mu.RUnlock()
	var hits []DeclarationHit
	for u, file := range i.entries {
		for _, decl := range FindDeclarations(file) {
			hits = append(hits, DeclarationHit{URI: u, Declaration: decl})
		}
	}
	return hits
}

// AllUsages walks every indexed file's AST for occurrences of name.
// When includeDecl is false, write usages are skipped to match LSP's
// ReferenceContext.IncludeDeclaration semantics.
func (i *Index) AllUsages(name string, includeDecl bool) []UsageHit {
	i.mu.RLock()
	defer i.mu.RUnlock()
	var hits []UsageHit
	for u, file := range i.entries {
		for _, usage := range FindUsages(file, name) {
			if !includeDecl && usage.Kind == UsageWrite {
				continue
			}
			hits = append(hits, UsageHit{URI: u, Usage: usage})
		}
	}
	return hits
}

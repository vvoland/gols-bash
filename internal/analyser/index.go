// SPDX-License-Identifier: GPL-3.0-only

package analyser

import (
	"os"
	"path/filepath"
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
	return declarationHits(i.entries, nil)
}

// ReachableDeclarations walks declarations in files connected to from by
// literal source/. statements.
func (i *Index) ReachableDeclarations(from uri.URI) []DeclarationHit {
	i.mu.RLock()
	defer i.mu.RUnlock()
	allowed := reachableURIs(i.entries, from)
	return declarationHits(i.entries, allowed)
}

func declarationHits(entries map[uri.URI]*syntax.File, allowed map[uri.URI]bool) []DeclarationHit {
	var hits []DeclarationHit
	for u, file := range entries {
		if allowed != nil && !allowed[u] {
			continue
		}
		for _, decl := range FindDeclarations(file) {
			hits = append(hits, DeclarationHit{URI: u, Declaration: decl})
		}
	}
	return hits
}

func reachableURIs(entries map[uri.URI]*syntax.File, from uri.URI) map[uri.URI]bool {
	seen := map[uri.URI]bool{from: true}
	queue := []uri.URI{from}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for next := range sourceNeighbours(entries, cur) {
			if seen[next] {
				continue
			}
			seen[next] = true
			queue = append(queue, next)
		}
	}
	return seen
}

func sourceNeighbours(entries map[uri.URI]*syntax.File, from uri.URI) map[uri.URI]bool {
	out := make(map[uri.URI]bool)
	for _, target := range sourcedURIs(from, entries[from]) {
		if _, ok := entries[target]; ok {
			out[target] = true
		}
	}
	for candidate, file := range entries {
		if candidate == from {
			continue
		}
		for _, target := range sourcedURIs(candidate, file) {
			if target == from {
				out[candidate] = true
				break
			}
		}
	}
	return out
}

func sourcedURIs(base uri.URI, file *syntax.File) []uri.URI {
	if file == nil {
		return nil
	}
	var out []uri.URI
	syntax.Walk(file, func(n syntax.Node) bool {
		call, ok := n.(*syntax.CallExpr)
		if !ok || len(call.Args) < 2 {
			return true
		}
		cmd, ok := singleLit(call.Args[0])
		if !ok || cmd != "source" && cmd != "." {
			return true
		}
		target, ok := singleLit(call.Args[1])
		if !ok {
			return true
		}
		path := target
		if !filepath.IsAbs(path) {
			path = filepath.Join(filepath.Dir(base.Filename()), path)
		}
		if info, err := os.Stat(path); err != nil || info.IsDir() {
			return true
		}
		out = append(out, uri.File(path))
		return true
	})
	return out
}

func singleLit(word *syntax.Word) (string, bool) {
	if word == nil || len(word.Parts) != 1 {
		return "", false
	}
	lit, ok := word.Parts[0].(*syntax.Lit)
	if !ok {
		return "", false
	}
	return lit.Value, true
}

// AllUsages walks every indexed file's AST for occurrences of name.
// When includeDecl is false, write usages are skipped to match LSP's
// ReferenceContext.IncludeDeclaration semantics.
func (i *Index) AllUsages(name string, includeDecl bool) []UsageHit {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return usageHits(i.entries, nil, name, includeDecl)
}

// ReachableUsages walks occurrences of name in files connected to from by
// literal source/. statements.
//
// Source reachability is treated as an undirected component so a sourced file
// can find callers in scripts that source it, while unrelated scripts with the
// same symbol name are excluded.
func (i *Index) ReachableUsages(from uri.URI, name string, includeDecl bool) []UsageHit {
	i.mu.RLock()
	defer i.mu.RUnlock()
	allowed := reachableURIs(i.entries, from)
	return usageHits(i.entries, allowed, name, includeDecl)
}

func usageHits(entries map[uri.URI]*syntax.File, allowed map[uri.URI]bool, name string, includeDecl bool) []UsageHit {
	var hits []UsageHit
	for u, file := range entries {
		if allowed != nil && !allowed[u] {
			continue
		}
		for _, usage := range FindUsages(file, name) {
			if !includeDecl && usage.Kind == UsageWrite {
				continue
			}
			hits = append(hits, UsageHit{URI: u, Usage: usage})
		}
	}
	return hits
}

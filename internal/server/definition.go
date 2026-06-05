// SPDX-License-Identifier: GPL-3.0-only

package server

import (
	"os"
	"path/filepath"
	"sort"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
	"mvdan.cc/sh/v3/syntax"

	"grono.dev/gols-bash/internal/analyser"
	utillsp "grono.dev/gols-bash/internal/util/lsp"
)

// definition resolves the word under pos to an indexed function or variable
// declaration in the current sourced-file component.
// It prefers declarations in the current document.
func (s *bashServer) definition(d *Document, pos protocol.Position) []protocol.Location {
	if d == nil || d.AST == nil {
		return nil
	}
	off := s.offsetForPosition(d.Text, int(pos.Line), int(pos.Character))
	if off < 0 {
		return nil
	}
	if loc := s.sourceDefinition(d, off); loc != nil {
		return []protocol.Location{*loc}
	}
	word, _, _ := utillsp.WordAtOffset(d.Text, off)
	if word == "" {
		return nil
	}
	for _, decl := range analyser.FindDeclarations(d.AST) {
		if decl.Name != word {
			continue
		}
		return []protocol.Location{{
			URI:   d.URI,
			Range: s.rangeToLSP(d.Text, decl.Pos, decl.End),
		}}
	}
	for _, hit := range sortedDeclarationHits(s.index.ReachableDeclarations(d.URI)) {
		if hit.URI == d.URI || hit.Declaration.Name != word {
			continue
		}
		text := s.textForURI(hit.URI)
		return []protocol.Location{{
			URI:   hit.URI,
			Range: s.rangeToLSP(text, hit.Declaration.Pos, hit.Declaration.End),
		}}
	}
	return nil
}

func (s *bashServer) sourceDefinition(d *Document, off int) *protocol.Location {
	var target string
	syntax.Walk(d.AST, func(n syntax.Node) bool {
		if target != "" {
			return false
		}
		call, ok := n.(*syntax.CallExpr)
		if !ok || len(call.Args) < 2 {
			return true
		}
		cmd, ok := singleLit(call.Args[0])
		if !ok || cmd != "source" && cmd != "." {
			return true
		}
		arg, ok := singleLit(call.Args[1])
		if !ok || !posContainsOffset(call.Args[1].Pos(), call.Args[1].End(), off) {
			return true
		}
		target = arg
		return false
	})
	if target == "" {
		return nil
	}
	path := target
	if !filepath.IsAbs(path) {
		path = filepath.Join(filepath.Dir(d.URI.Filename()), path)
	}
	if info, err := os.Stat(path); err != nil || info.IsDir() {
		return nil
	}
	return &protocol.Location{
		URI:   uri.File(path),
		Range: protocol.Range{Start: protocol.Position{}, End: protocol.Position{}},
	}
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

func posContainsOffset(start, end syntax.Pos, off int) bool {
	return int(start.Offset()) <= off && off < int(end.Offset())
}

func sortedDeclarationHits(hits []analyser.DeclarationHit) []analyser.DeclarationHit {
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].URI != hits[j].URI {
			return hits[i].URI < hits[j].URI
		}
		return hits[i].Declaration.Pos.Offset() < hits[j].Declaration.Pos.Offset()
	})
	return hits
}

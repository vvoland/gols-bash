// SPDX-License-Identifier: GPL-3.0-only

package server

import (
	"sort"

	"go.lsp.dev/protocol"

	"grono.dev/gols-bash/internal/analyser"
	utillsp "grono.dev/gols-bash/internal/util/lsp"
)

// definition resolves the word under pos to an indexed function or variable
// declaration, preferring declarations in the current document.
func (s *bashServer) definition(d *Document, pos protocol.Position) []protocol.Location {
	if d == nil || d.AST == nil {
		return nil
	}
	off := s.offsetForPosition(d.Text, int(pos.Line), int(pos.Character))
	if off < 0 {
		return nil
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
	for _, hit := range sortedDeclarationHits(s.index.AllDeclarations()) {
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

func sortedDeclarationHits(hits []analyser.DeclarationHit) []analyser.DeclarationHit {
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].URI != hits[j].URI {
			return hits[i].URI < hits[j].URI
		}
		return hits[i].Declaration.Pos.Offset() < hits[j].Declaration.Pos.Offset()
	})
	return hits
}

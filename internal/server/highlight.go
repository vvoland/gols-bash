// SPDX-License-Identifier: GPL-3.0-only

package server

import (
	"go.lsp.dev/protocol"

	"grono.dev/gols-bash/internal/analyser"
	utillsp "grono.dev/gols-bash/internal/util/lsp"
)

// highlight returns same-document occurrences of the word under pos, with
// Read/Write kinds distinguishing expansions/calls from declarations.
func (s *bashServer) highlight(d *Document, pos protocol.Position) []protocol.DocumentHighlight {
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
	usages := analyser.FindUsages(d.AST, word)
	out := make([]protocol.DocumentHighlight, 0, len(usages))
	for _, u := range usages {
		kind := protocol.DocumentHighlightKindRead
		if u.Kind == analyser.UsageWrite {
			kind = protocol.DocumentHighlightKindWrite
		}
		out = append(out, protocol.DocumentHighlight{
			Range: s.rangeToLSP(d.Text, u.Pos, u.End),
			Kind:  kind,
		})
	}
	return out
}

// SPDX-License-Identifier: GPL-3.0-only

package server

import (
	"go.lsp.dev/protocol"
	"mvdan.cc/sh/v3/syntax"

	utillsp "grono.dev/gols-bash/internal/util/lsp"
)

// definition resolves the word under pos to a same-file function declaration.
// Variables, sourced files, and indexed workspace declarations are not yet used.
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
	for _, stmt := range d.AST.Stmts {
		fn, ok := stmt.Cmd.(*syntax.FuncDecl)
		if !ok || fn.Name == nil || fn.Name.Value != word {
			continue
		}
		return []protocol.Location{{
			URI:   d.URI,
			Range: s.rangeToLSP(d.Text, fn.Name.Pos(), fn.Name.End()),
		}}
	}
	return nil
}

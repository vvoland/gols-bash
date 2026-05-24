// SPDX-License-Identifier: GPL-3.0-only

package server

import (
	"go.lsp.dev/protocol"
	"mvdan.cc/sh/v3/syntax"
)

// documentSymbols emits one DocumentSymbol per top-level function decl.
func (s *bashServer) documentSymbols(d *Document) []protocol.DocumentSymbol {
	if d == nil || d.AST == nil {
		return nil
	}
	var out []protocol.DocumentSymbol
	for _, stmt := range d.AST.Stmts {
		fn, ok := stmt.Cmd.(*syntax.FuncDecl)
		if !ok || fn.Name == nil {
			continue
		}
		out = append(out, protocol.DocumentSymbol{
			Name:           fn.Name.Value,
			Kind:           protocol.SymbolKindFunction,
			Range:          s.rangeToLSP(d.Text, fn.Pos(), fn.End()),
			SelectionRange: s.rangeToLSP(d.Text, fn.Name.Pos(), fn.Name.End()),
		})
	}
	return out
}

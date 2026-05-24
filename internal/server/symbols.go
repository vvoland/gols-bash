// SPDX-License-Identifier: GPL-3.0-only

package server

import (
	"go.lsp.dev/protocol"
	"mvdan.cc/sh/v3/syntax"
)

// documentSymbols walks the file's top-level statements and emits one
// DocumentSymbol per function declaration.
func documentSymbols(file *syntax.File) []protocol.DocumentSymbol {
	if file == nil {
		return nil
	}
	var out []protocol.DocumentSymbol
	for _, stmt := range file.Stmts {
		fn, ok := stmt.Cmd.(*syntax.FuncDecl)
		if !ok || fn.Name == nil {
			continue
		}
		out = append(out, protocol.DocumentSymbol{
			Name:           fn.Name.Value,
			Kind:           protocol.SymbolKindFunction,
			Range:          lspRange(fn.Pos(), fn.End()),
			SelectionRange: lspRange(fn.Name.Pos(), fn.Name.End()),
		})
	}
	return out
}

// lspRange converts a mvdan/sh start/end position pair (1-based line/col)
// to an LSP Range (0-based).
func lspRange(start, end syntax.Pos) protocol.Range {
	return protocol.Range{Start: lspPos(start), End: lspPos(end)}
}

func lspPos(p syntax.Pos) protocol.Position {
	line := uint32(p.Line())
	col := uint32(p.Col())
	if line > 0 {
		line--
	}
	if col > 0 {
		col--
	}
	return protocol.Position{Line: line, Character: col}
}

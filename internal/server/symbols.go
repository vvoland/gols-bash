// SPDX-License-Identifier: GPL-3.0-only

package server

import (
	"sort"
	"strings"

	"go.lsp.dev/protocol"
	"mvdan.cc/sh/v3/syntax"

	"grono.dev/gols-bash/internal/analyser"
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

func (s *bashServer) workspaceSymbols(query string) []protocol.SymbolInformation {
	query = strings.ToLower(query)
	hits := s.index.AllDeclarations()
	out := make([]protocol.SymbolInformation, 0, len(hits))
	for _, h := range hits {
		if query != "" && !strings.Contains(strings.ToLower(h.Declaration.Name), query) {
			continue
		}
		text := s.textForURI(h.URI)
		out = append(out, protocol.SymbolInformation{
			Name: h.Declaration.Name,
			Kind: symbolKindForDeclaration(h.Declaration.Kind),
			Location: protocol.Location{
				URI:   h.URI,
				Range: s.rangeToLSP(text, h.Declaration.Pos, h.Declaration.End),
			},
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return out[i].Location.URI < out[j].Location.URI
	})
	return out
}

func symbolKindForDeclaration(kind analyser.DeclarationKind) protocol.SymbolKind {
	if kind == analyser.DeclarationVariable {
		return protocol.SymbolKindVariable
	}
	return protocol.SymbolKindFunction
}

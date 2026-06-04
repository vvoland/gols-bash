// SPDX-License-Identifier: GPL-3.0-only

package server

import (
	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"

	"grono.dev/gols-bash/internal/analyser"
	utillsp "grono.dev/gols-bash/internal/util/lsp"
)

// rename returns a best-effort workspace-wide edit for the word under pos.
// It does not yet model Bash local scope or sourced-file reachability.
//
// Returns (nil, error) when newName is missing or not a valid identifier.
// Returns (empty WorkspaceEdit, nil) when the cursor is on no word or the
// symbol is never declared in the workspace — refusing to rename builtins
// or stray references.
func (s *bashServer) rename(d *Document, pos protocol.Position, newName string) (*protocol.WorkspaceEdit, error) {
	if !isValidIdentifier(newName) {
		return nil, jsonrpc2.NewError(jsonrpc2.InvalidParams, "invalid new name: "+newName)
	}
	if d == nil || d.AST == nil {
		return &protocol.WorkspaceEdit{}, nil
	}
	off := s.offsetForPosition(d.Text, int(pos.Line), int(pos.Character))
	if off < 0 {
		return &protocol.WorkspaceEdit{}, nil
	}
	word, _, _ := utillsp.WordAtOffset(d.Text, off)
	if word == "" {
		return &protocol.WorkspaceEdit{}, nil
	}
	hits := s.index.AllUsages(word, true)
	if !hasDeclaration(hits) {
		return &protocol.WorkspaceEdit{}, nil
	}
	changes := make(map[protocol.DocumentURI][]protocol.TextEdit)
	for _, h := range hits {
		text := s.textForURI(h.URI)
		changes[protocol.DocumentURI(h.URI)] = append(changes[protocol.DocumentURI(h.URI)], protocol.TextEdit{
			Range:   s.rangeToLSP(text, h.Usage.Pos, h.Usage.End),
			NewText: newName,
		})
	}
	return &protocol.WorkspaceEdit{Changes: changes}, nil
}

func hasDeclaration(hits []analyser.UsageHit) bool {
	for _, h := range hits {
		if h.Usage.Kind == analyser.UsageWrite {
			return true
		}
	}
	return false
}

// isValidIdentifier reports whether s is a syntactically valid bash
// identifier ([A-Za-z_][A-Za-z0-9_]*). Function names may legally contain
// more characters, but variables can't — accept the stricter set so the
// rename is safe for either.
func isValidIdentifier(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		b := s[i]
		switch {
		case b >= 'a' && b <= 'z', b >= 'A' && b <= 'Z', b == '_':
			continue
		case b >= '0' && b <= '9':
			if i == 0 {
				return false
			}
		default:
			return false
		}
	}
	return true
}

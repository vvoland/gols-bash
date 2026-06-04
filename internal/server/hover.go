// SPDX-License-Identifier: GPL-3.0-only

package server

import (
	"fmt"
	"path/filepath"
	"sort"

	"go.lsp.dev/protocol"
	"mvdan.cc/sh/v3/syntax"

	"grono.dev/gols-bash/internal/analyser"
	"grono.dev/gols-bash/internal/data"
	utillsp "grono.dev/gols-bash/internal/util/lsp"
)

// hover returns the markup shown on hover.
func (s *bashServer) hover(d *Document, pos protocol.Position) *protocol.Hover {
	if d == nil {
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
	md := s.hoverMarkdownFor(word, d)
	if md == "" {
		return nil
	}
	return &protocol.Hover{
		Contents: protocol.MarkupContent{Kind: protocol.Markdown, Value: md},
	}
}

func (s *bashServer) hoverMarkdownFor(word string, d *Document) string {
	switch {
	case data.IsReservedWord(word):
		return fmt.Sprintf("**%s** — bash reserved word", word)
	case data.IsBuiltin(word):
		return fmt.Sprintf("**%s** — bash builtin\n\nRun `help %s` for full documentation.", word, word)
	}
	if fn := findLocalFunction(d.AST, word); fn != nil {
		return fmt.Sprintf("**%s** — local function (declared at line %d)", word, fn.Pos().Line())
	}
	if hit, ok := s.findWorkspaceDeclaration(word); ok {
		return fmt.Sprintf("**%s** — workspace %s (declared in `%s` at line %d)",
			word, hoverDeclarationKind(hit.Declaration.Kind), filepath.Base(hit.URI.Filename()), hit.Declaration.Pos.Line())
	}
	return ""
}

func (s *bashServer) findWorkspaceDeclaration(word string) (analyser.DeclarationHit, bool) {
	hits := s.index.AllDeclarations()
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].URI != hits[j].URI {
			return hits[i].URI < hits[j].URI
		}
		return hits[i].Declaration.Pos.Offset() < hits[j].Declaration.Pos.Offset()
	})
	for _, h := range hits {
		if h.Declaration.Name == word {
			return h, true
		}
	}
	return analyser.DeclarationHit{}, false
}

func hoverDeclarationKind(kind analyser.DeclarationKind) string {
	if kind == analyser.DeclarationVariable {
		return "variable"
	}
	return "function"
}

func findLocalFunction(file *syntax.File, name string) *syntax.FuncDecl {
	if file == nil {
		return nil
	}
	for _, stmt := range file.Stmts {
		fn, ok := stmt.Cmd.(*syntax.FuncDecl)
		if !ok || fn.Name == nil {
			continue
		}
		if fn.Name.Value == name {
			return fn
		}
	}
	return nil
}

// SPDX-License-Identifier: GPL-3.0-only

package server

import (
	"fmt"

	"go.lsp.dev/protocol"
	"mvdan.cc/sh/v3/syntax"

	"grono.dev/gols-bash/internal/data"
	utillsp "grono.dev/gols-bash/internal/util/lsp"
)

// hover returns the markup shown on hover.
// Lookup is limited to reserved words, builtins, and same-file functions.
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
	md := hoverMarkdownFor(word, d)
	if md == "" {
		return nil
	}
	return &protocol.Hover{
		Contents: protocol.MarkupContent{Kind: protocol.Markdown, Value: md},
	}
}

func hoverMarkdownFor(word string, d *Document) string {
	switch {
	case data.IsReservedWord(word):
		return fmt.Sprintf("**%s** — bash reserved word", word)
	case data.IsBuiltin(word):
		return fmt.Sprintf("**%s** — bash builtin\n\nRun `help %s` for full documentation.", word, word)
	}
	if fn := findLocalFunction(d.AST, word); fn != nil {
		return fmt.Sprintf("**%s** — local function (declared at line %d)", word, fn.Pos().Line())
	}
	return ""
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

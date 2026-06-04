// SPDX-License-Identifier: GPL-3.0-only

package server

import (
	"sort"
	"strings"

	"go.lsp.dev/protocol"
	"mvdan.cc/sh/v3/syntax"

	"grono.dev/gols-bash/internal/data"
)

type completionCandidate struct {
	label      string
	newText    string
	detail     string
	kind       protocol.CompletionItemKind
	textFormat protocol.InsertTextFormat
}

var snippetCompletions = []completionCandidate{
	{
		label:      "if statement",
		newText:    "if ${1:condition}; then\n\t$0\nfi",
		detail:     "bash snippet",
		kind:       protocol.CompletionItemKindSnippet,
		textFormat: protocol.InsertTextFormatSnippet,
	},
	{
		label:      "for loop",
		newText:    "for ${1:name} in ${2:items}; do\n\t$0\ndone",
		detail:     "bash snippet",
		kind:       protocol.CompletionItemKindSnippet,
		textFormat: protocol.InsertTextFormatSnippet,
	},
	{
		label:      "case statement",
		newText:    "case ${1:word} in\n\t${2:pattern})\n\t\t$0\n\t\t;;\nesac",
		detail:     "bash snippet",
		kind:       protocol.CompletionItemKindSnippet,
		textFormat: protocol.InsertTextFormatSnippet,
	},
}

func (s *bashServer) completion(d *Document, pos protocol.Position) []protocol.CompletionItem {
	if d == nil {
		return nil
	}
	off := s.offsetForPosition(d.Text, int(pos.Line), int(pos.Character))
	if off < 0 {
		return nil
	}
	prefix, start := completionPrefix(d.Text, off)
	if strings.HasPrefix(prefix, "#") {
		return nil
	}
	varsOnly := completionVariablesOnly(d.Text, start)
	candidates := completionCandidates(d.AST, varsOnly)
	items := make([]protocol.CompletionItem, 0, len(candidates))
	seen := make(map[string]struct{}, len(candidates))
	for _, c := range candidates {
		if _, ok := seen[c.label]; ok || !strings.HasPrefix(c.label, prefix) {
			continue
		}
		seen[c.label] = struct{}{}
		item := protocol.CompletionItem{
			Label:            c.label,
			Kind:             c.kind,
			Detail:           c.detail,
			InsertTextFormat: c.textFormat,
			TextEdit: &protocol.TextEdit{
				Range: protocol.Range{
					Start: s.posForOffset(d.Text, start),
					End:   pos,
				},
				NewText: completionNewText(c),
			},
		}
		items = append(items, item)
	}
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].Label < items[j].Label
	})
	return items
}

func completionNewText(c completionCandidate) string {
	if c.newText != "" {
		return c.newText
	}
	return c.label
}

func completionCandidates(file *syntax.File, varsOnly bool) []completionCandidate {
	var out []completionCandidate
	for _, name := range localVariables(file) {
		out = append(out, completionCandidate{
			label:      name,
			detail:     "variable",
			kind:       protocol.CompletionItemKindVariable,
			textFormat: protocol.InsertTextFormatPlainText,
		})
	}
	if varsOnly {
		return out
	}
	for _, name := range localFunctions(file) {
		out = append(out, completionCandidate{
			label:      name,
			detail:     "function",
			kind:       protocol.CompletionItemKindFunction,
			textFormat: protocol.InsertTextFormatPlainText,
		})
	}
	for _, name := range data.Builtins {
		out = append(out, completionCandidate{
			label:      name,
			detail:     "bash builtin",
			kind:       protocol.CompletionItemKindFunction,
			textFormat: protocol.InsertTextFormatPlainText,
		})
	}
	for _, name := range data.ReservedWords {
		out = append(out, completionCandidate{
			label:      name,
			detail:     "bash reserved word",
			kind:       protocol.CompletionItemKindKeyword,
			textFormat: protocol.InsertTextFormatPlainText,
		})
	}
	out = append(out, snippetCompletions...)
	return out
}

func completionPrefix(src string, off int) (string, int) {
	if off > len(src) {
		off = len(src)
	}
	start := off
	for start > 0 && isCompletionWordByte(src[start-1]) {
		start--
	}
	return src[start:off], start
}

func completionVariablesOnly(src string, start int) bool {
	if start > 0 && src[start-1] == '$' {
		return true
	}
	return start > 1 && src[start-1] == '{' && src[start-2] == '$'
}

func isCompletionWordByte(b byte) bool {
	return b >= 'a' && b <= 'z' ||
		b >= 'A' && b <= 'Z' ||
		b >= '0' && b <= '9' ||
		b == '_' || b == '-' || b == '.'
}

func localFunctions(file *syntax.File) []string {
	if file == nil {
		return nil
	}
	var names []string
	for _, stmt := range file.Stmts {
		fn, ok := stmt.Cmd.(*syntax.FuncDecl)
		if !ok || fn.Name == nil {
			continue
		}
		names = append(names, fn.Name.Value)
	}
	return names
}

func localVariables(file *syntax.File) []string {
	if file == nil {
		return nil
	}
	seen := map[string]struct{}{}
	syntax.Walk(file, func(n syntax.Node) bool {
		assign, ok := n.(*syntax.Assign)
		if !ok || assign.Name == nil {
			return true
		}
		seen[assign.Name.Value] = struct{}{}
		return true
	})
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

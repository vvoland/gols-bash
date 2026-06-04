// SPDX-License-Identifier: GPL-3.0-only

package server

import (
	"os"
	"path/filepath"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
	"gotest.tools/v3/assert"
)

func TestCompletionIncludesBuiltinReservedFunctionVariableAndSnippet(t *testing.T) {
	s, _ := newTestServer()
	u := uri.URI("file:///tmp/completion.sh")
	src := "my_var=1\ngreet() {\n  echo hi\n}\n\ne"
	dispatch(t, s, protocol.MethodTextDocumentDidOpen, protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{URI: u, Version: 1, Text: src},
	})

	list := call[*protocol.CompletionList](t, s, protocol.MethodTextDocumentCompletion, protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: u},
			Position:     protocol.Position{Line: 5, Character: 1},
		},
	})

	assert.Assert(t, completionHas(list.Items, "echo", protocol.CompletionItemKindFunction))
	assert.Assert(t, completionHas(list.Items, "elif", protocol.CompletionItemKindKeyword))

	list = call[*protocol.CompletionList](t, s, protocol.MethodTextDocumentCompletion, protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: u},
			Position:     protocol.Position{Line: 5, Character: 0},
		},
	})
	assert.Assert(t, completionHas(list.Items, "greet", protocol.CompletionItemKindFunction))
	assert.Assert(t, completionHas(list.Items, "my_var", protocol.CompletionItemKindVariable))
	assert.Assert(t, completionHas(list.Items, "if statement", protocol.CompletionItemKindSnippet))
}

func TestCompletionInParameterExpansionReturnsVariablesOnly(t *testing.T) {
	s, _ := newTestServer()
	u := uri.URI("file:///tmp/completion.sh")
	src := "my_var=1\necho $m"
	dispatch(t, s, protocol.MethodTextDocumentDidOpen, protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{URI: u, Version: 1, Text: src},
	})

	list := call[*protocol.CompletionList](t, s, protocol.MethodTextDocumentCompletion, protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: u},
			Position:     protocol.Position{Line: 1, Character: 7},
		},
	})

	assert.Assert(t, completionHas(list.Items, "my_var", protocol.CompletionItemKindVariable))
	assert.Assert(t, !completionHasLabel(list.Items, "mapfile"))
}

func TestCompletionIncludesPathExecutables(t *testing.T) {
	dir := t.TempDir()
	executable := filepath.Join(dir, "custom-tool")
	notExecutable := filepath.Join(dir, "custom-data")
	assert.NilError(t, os.WriteFile(executable, []byte("#!/bin/sh\n"), 0o755))
	assert.NilError(t, os.WriteFile(notExecutable, []byte("data\n"), 0o644))
	t.Setenv("PATH", dir)

	s, _ := newTestServer()
	u := uri.URI("file:///tmp/completion.sh")
	dispatch(t, s, protocol.MethodTextDocumentDidOpen, protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{URI: u, Version: 1, Text: "custom"},
	})

	list := call[*protocol.CompletionList](t, s, protocol.MethodTextDocumentCompletion, protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: u},
			Position:     protocol.Position{Line: 0, Character: 6},
		},
	})

	assert.Assert(t, completionHas(list.Items, "custom-tool", protocol.CompletionItemKindFile))
	assert.Assert(t, !completionHasLabel(list.Items, "custom-data"))
}

func TestCompletionResolveReturnsItem(t *testing.T) {
	s, _ := newTestServer()
	item := call[*protocol.CompletionItem](t, s, protocol.MethodCompletionItemResolve, protocol.CompletionItem{
		Label:  "echo",
		Kind:   protocol.CompletionItemKindFunction,
		Detail: "bash builtin",
	})
	assert.Equal(t, item.Label, "echo")
	assert.Equal(t, item.Detail, "bash builtin")
}

func completionHas(items []protocol.CompletionItem, label string, kind protocol.CompletionItemKind) bool {
	for _, item := range items {
		if item.Label == label && item.Kind == kind {
			return true
		}
	}
	return false
}

func completionHasLabel(items []protocol.CompletionItem, label string) bool {
	for _, item := range items {
		if item.Label == label {
			return true
		}
	}
	return false
}

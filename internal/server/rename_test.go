// SPDX-License-Identifier: GPL-3.0-only

package server

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
	"mvdan.cc/sh/v3/syntax"
)

func TestRenameAcrossFiles(t *testing.T) {
	root := t.TempDir()
	libPath := filepath.Join(root, "lib.sh")
	usePath := filepath.Join(root, "use.sh")
	assert.NilError(t, os.WriteFile(libPath, []byte("greet() { echo hi; }\n"), 0o644))
	assert.NilError(t, os.WriteFile(usePath, []byte("greet\ngreet\n"), 0o644))

	s, _ := newTestServer()
	parseBash := func(name, src string) *syntax.File {
		f, _ := syntax.NewParser().Parse(strings.NewReader(src), name)
		return f
	}
	for _, p := range []string{libPath, usePath} {
		b, _ := os.ReadFile(p)
		s.index.AddOrReplace(uri.File(p), parseBash(p, string(b)))
	}

	useURI := uri.File(usePath)
	dispatch(t, s, protocol.MethodTextDocumentDidOpen, protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{URI: useURI, Version: 1, Text: "greet\ngreet\n"},
	})

	edit := call[*protocol.WorkspaceEdit](t, s, protocol.MethodTextDocumentRename,
		protocol.RenameParams{
			TextDocumentPositionParams: protocol.TextDocumentPositionParams{
				TextDocument: protocol.TextDocumentIdentifier{URI: useURI},
				Position:     protocol.Position{Line: 0, Character: 0},
			},
			NewName: "salute",
		})

	assert.Assert(t, edit != nil)
	assert.Assert(t, cmp.Len(edit.Changes, 2)) // two URIs
	assert.Assert(t, cmp.Len(edit.Changes[protocol.DocumentURI(useURI)], 2))
	assert.Assert(t, cmp.Len(edit.Changes[protocol.DocumentURI(uri.File(libPath))], 1))
	for _, edits := range edit.Changes {
		for _, e := range edits {
			assert.Equal(t, e.NewText, "salute")
		}
	}
}

func TestRenameRejectsInvalidIdentifier(t *testing.T) {
	s, _ := newTestServer()
	u := uri.URI("file:///tmp/r.sh")
	dispatch(t, s, protocol.MethodTextDocumentDidOpen, protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{URI: u, Version: 1, Text: "greet() { :; }\ngreet\n"},
	})

	c, err := jsonrpc2.NewCall(jsonrpc2.NewNumberID(1), protocol.MethodTextDocumentRename, protocol.RenameParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: u},
			Position:     protocol.Position{Line: 1, Character: 0},
		},
		NewName: "9bad-name",
	})
	assert.NilError(t, err)

	var gotErr error
	rep := func(_ context.Context, _ interface{}, e error) error {
		gotErr = e
		return nil
	}
	assert.NilError(t, s.handle(context.Background(), rep, c))
	assert.Assert(t, gotErr != nil, "expected error for invalid name")
}

func TestPrepareRenameReturnsDeclaredSymbolRange(t *testing.T) {
	s, _ := newTestServer()
	u := uri.URI("file:///tmp/r.sh")
	dispatch(t, s, protocol.MethodTextDocumentDidOpen, protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{URI: u, Version: 1, Text: "name=world\necho $name\n"},
	})

	r := call[*protocol.Range](t, s, protocol.MethodTextDocumentPrepareRename,
		protocol.PrepareRenameParams{TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: u},
			Position:     protocol.Position{Line: 1, Character: 7},
		}})

	assert.Assert(t, r != nil)
	assert.Equal(t, r.Start.Line, uint32(1))
	assert.Equal(t, r.Start.Character, uint32(6))
	assert.Equal(t, r.End.Line, uint32(1))
	assert.Equal(t, r.End.Character, uint32(10))
}

func TestPrepareRenameRejectsUndeclaredAndInvalidSymbols(t *testing.T) {
	s, _ := newTestServer()
	u := uri.URI("file:///tmp/r.sh")
	dispatch(t, s, protocol.MethodTextDocumentDidOpen, protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{URI: u, Version: 1, Text: "_=ignored\necho hi\n"},
	})

	r := call[*protocol.Range](t, s, protocol.MethodTextDocumentPrepareRename,
		protocol.PrepareRenameParams{TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: u},
			Position:     protocol.Position{Line: 1, Character: 0},
		}})
	assert.Assert(t, r == nil)

	r = call[*protocol.Range](t, s, protocol.MethodTextDocumentPrepareRename,
		protocol.PrepareRenameParams{TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: u},
			Position:     protocol.Position{Line: 0, Character: 0},
		}})
	assert.Assert(t, r == nil)
}

func TestRenameSkippedForUndeclared(t *testing.T) {
	s, _ := newTestServer()
	u := uri.URI("file:///tmp/r.sh")
	// "echo" is a builtin, not declared in the workspace — rename is a no-op.
	dispatch(t, s, protocol.MethodTextDocumentDidOpen, protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{URI: u, Version: 1, Text: "echo hi\n"},
	})
	edit := call[*protocol.WorkspaceEdit](t, s, protocol.MethodTextDocumentRename,
		protocol.RenameParams{
			TextDocumentPositionParams: protocol.TextDocumentPositionParams{
				TextDocument: protocol.TextDocumentIdentifier{URI: u},
				Position:     protocol.Position{Line: 0, Character: 0},
			},
			NewName: "shout",
		})
	assert.Assert(t, edit != nil)
	assert.Assert(t, cmp.Len(edit.Changes, 0))
}

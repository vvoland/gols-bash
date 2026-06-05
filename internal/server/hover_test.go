// SPDX-License-Identifier: GPL-3.0-only

package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
	"gotest.tools/v3/assert"
)

func TestHoverBuiltin(t *testing.T) {
	s, _ := newTestServer()
	u := uri.URI("file:///tmp/h.sh")
	dispatch(t, s, protocol.MethodTextDocumentDidOpen, protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{URI: u, Version: 1, Text: "echo hi\n"},
	})

	h := call[*protocol.Hover](t, s, protocol.MethodTextDocumentHover, protocol.HoverParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: u},
			Position:     protocol.Position{Line: 0, Character: 1}, // inside "echo"
		},
	})
	assert.Assert(t, h != nil)
	assert.Assert(t, strings.Contains(h.Contents.Value, "builtin"), "got %q", h.Contents.Value)
	assert.Assert(t, strings.Contains(h.Contents.Value, "echo"))
}

func TestHoverReservedWord(t *testing.T) {
	s, _ := newTestServer()
	u := uri.URI("file:///tmp/h.sh")
	dispatch(t, s, protocol.MethodTextDocumentDidOpen, protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{URI: u, Version: 1, Text: "if true; then echo; fi\n"},
	})
	h := call[*protocol.Hover](t, s, protocol.MethodTextDocumentHover, protocol.HoverParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: u},
			Position:     protocol.Position{Line: 0, Character: 0}, // "if"
		},
	})
	assert.Assert(t, h != nil)
	assert.Assert(t, strings.Contains(h.Contents.Value, "reserved"))
}

func TestHoverLocalFunction(t *testing.T) {
	s, _ := newTestServer()
	u := uri.URI("file:///tmp/h.sh")
	src := "greet() {\n  echo hi\n}\n\ngreet\n"
	dispatch(t, s, protocol.MethodTextDocumentDidOpen, protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{URI: u, Version: 1, Text: src},
	})
	h := call[*protocol.Hover](t, s, protocol.MethodTextDocumentHover, protocol.HoverParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: u},
			Position:     protocol.Position{Line: 4, Character: 0}, // "greet" call
		},
	})
	assert.Assert(t, h != nil)
	assert.Assert(t, strings.Contains(h.Contents.Value, "local function"), "got %q", h.Contents.Value)
	assert.Assert(t, strings.Contains(h.Contents.Value, "line 1"))
}

func TestHoverWorkspaceFunction(t *testing.T) {
	root := t.TempDir()
	libPath := filepath.Join(root, "lib.sh")
	usePath := filepath.Join(root, "use.sh")
	unrelatedPath := filepath.Join(root, "unrelated.sh")
	assert.NilError(t, os.WriteFile(libPath, []byte("greet() { echo hi; }\n"), 0o644))
	assert.NilError(t, os.WriteFile(unrelatedPath, []byte("greet() { echo bye; }\n"), 0o644))

	s, _ := newTestServer()
	libDoc := s.docs.Open(protocol.TextDocumentItem{URI: uri.File(libPath), Version: 1, Text: "greet() { echo hi; }\n"})
	s.index.AddOrReplace(libDoc.URI, libDoc.AST)
	unrelatedDoc := s.docs.Open(protocol.TextDocumentItem{URI: uri.File(unrelatedPath), Version: 1, Text: "greet() { echo bye; }\n"})
	s.index.AddOrReplace(unrelatedDoc.URI, unrelatedDoc.AST)
	dispatch(t, s, protocol.MethodTextDocumentDidOpen, protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{URI: uri.File(usePath), Version: 1, Text: ". ./lib.sh\ngreet\n"},
	})

	h := call[*protocol.Hover](t, s, protocol.MethodTextDocumentHover, protocol.HoverParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(usePath)},
			Position:     protocol.Position{Line: 1, Character: 0},
		},
	})

	assert.Assert(t, h != nil)
	assert.Assert(t, strings.Contains(h.Contents.Value, "workspace function"), "got %q", h.Contents.Value)
	assert.Assert(t, strings.Contains(h.Contents.Value, "lib.sh"), "got %q", h.Contents.Value)
	assert.Assert(t, !strings.Contains(h.Contents.Value, "unrelated.sh"), "got %q", h.Contents.Value)
}

func TestHoverSkipsUnrelatedWorkspaceFunction(t *testing.T) {
	root := t.TempDir()
	libPath := filepath.Join(root, "lib.sh")
	usePath := filepath.Join(root, "use.sh")
	assert.NilError(t, os.WriteFile(libPath, []byte("greet() { echo hi; }\n"), 0o644))

	s, _ := newTestServer()
	libDoc := s.docs.Open(protocol.TextDocumentItem{URI: uri.File(libPath), Version: 1, Text: "greet() { echo hi; }\n"})
	s.index.AddOrReplace(libDoc.URI, libDoc.AST)
	dispatch(t, s, protocol.MethodTextDocumentDidOpen, protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{URI: uri.File(usePath), Version: 1, Text: "greet\n"},
	})

	h := call[*protocol.Hover](t, s, protocol.MethodTextDocumentHover, protocol.HoverParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(usePath)},
			Position:     protocol.Position{Line: 0, Character: 0},
		},
	})

	assert.Assert(t, h == nil)
}

func TestHoverWorkspaceVariable(t *testing.T) {
	s, _ := newTestServer()
	u := uri.URI("file:///tmp/h.sh")
	dispatch(t, s, protocol.MethodTextDocumentDidOpen, protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{URI: u, Version: 1, Text: "name=world\necho $name\n"},
	})

	h := call[*protocol.Hover](t, s, protocol.MethodTextDocumentHover, protocol.HoverParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: u},
			Position:     protocol.Position{Line: 1, Character: 7},
		},
	})

	assert.Assert(t, h != nil)
	assert.Assert(t, strings.Contains(h.Contents.Value, "workspace variable"), "got %q", h.Contents.Value)
	assert.Assert(t, strings.Contains(h.Contents.Value, "line 1"), "got %q", h.Contents.Value)
}

func TestHoverUnknownWord(t *testing.T) {
	s, _ := newTestServer()
	u := uri.URI("file:///tmp/h.sh")
	dispatch(t, s, protocol.MethodTextDocumentDidOpen, protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{URI: u, Version: 1, Text: "myfn arg\n"},
	})
	h := call[*protocol.Hover](t, s, protocol.MethodTextDocumentHover, protocol.HoverParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: u},
			Position:     protocol.Position{Line: 0, Character: 0},
		},
	})
	assert.Assert(t, h == nil)
}

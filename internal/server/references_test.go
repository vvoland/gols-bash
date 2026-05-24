// SPDX-License-Identifier: GPL-3.0-only

package server

import (
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
)

const refSrc = `greet() {
  echo hi
}

greet
greet
`

func TestReferencesExcludeDeclaration(t *testing.T) {
	s, _ := newTestServer()
	u := uri.URI("file:///tmp/r.sh")
	dispatch(t, s, protocol.MethodTextDocumentDidOpen, protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{URI: u, Version: 1, Text: refSrc},
	})

	locs := call[[]protocol.Location](t, s, protocol.MethodTextDocumentReferences,
		protocol.ReferenceParams{
			TextDocumentPositionParams: protocol.TextDocumentPositionParams{
				TextDocument: protocol.TextDocumentIdentifier{URI: u},
				Position:     protocol.Position{Line: 4, Character: 0}, // call on line 4
			},
			Context: protocol.ReferenceContext{IncludeDeclaration: false},
		})
	assert.Assert(t, cmp.Len(locs, 2))
	assert.Equal(t, locs[0].Range.Start.Line, uint32(4))
	assert.Equal(t, locs[1].Range.Start.Line, uint32(5))
}

func TestReferencesIncludeDeclaration(t *testing.T) {
	s, _ := newTestServer()
	u := uri.URI("file:///tmp/r.sh")
	dispatch(t, s, protocol.MethodTextDocumentDidOpen, protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{URI: u, Version: 1, Text: refSrc},
	})

	locs := call[[]protocol.Location](t, s, protocol.MethodTextDocumentReferences,
		protocol.ReferenceParams{
			TextDocumentPositionParams: protocol.TextDocumentPositionParams{
				TextDocument: protocol.TextDocumentIdentifier{URI: u},
				Position:     protocol.Position{Line: 0, Character: 0}, // on decl
			},
			Context: protocol.ReferenceContext{IncludeDeclaration: true},
		})
	assert.Assert(t, cmp.Len(locs, 3))
	assert.Equal(t, locs[0].Range.Start.Line, uint32(0)) // decl
}

func TestReferencesVariableExpansion(t *testing.T) {
	s, _ := newTestServer()
	u := uri.URI("file:///tmp/r.sh")
	src := "FOO=1\necho $FOO\necho \"${FOO}\"\n"
	dispatch(t, s, protocol.MethodTextDocumentDidOpen, protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{URI: u, Version: 1, Text: src},
	})
	locs := call[[]protocol.Location](t, s, protocol.MethodTextDocumentReferences,
		protocol.ReferenceParams{
			TextDocumentPositionParams: protocol.TextDocumentPositionParams{
				TextDocument: protocol.TextDocumentIdentifier{URI: u},
				Position:     protocol.Position{Line: 0, Character: 0}, // FOO assign
			},
			Context: protocol.ReferenceContext{IncludeDeclaration: true},
		})
	assert.Assert(t, cmp.Len(locs, 3))
}

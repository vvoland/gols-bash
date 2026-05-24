// SPDX-License-Identifier: GPL-3.0-only

package server

import (
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
)

const defSrc = `greet() {
  echo hi
}

farewell() {
  echo bye
}

greet
farewell
`

func TestDefinitionJumpsToFunction(t *testing.T) {
	s, _ := newTestServer()
	u := uri.URI("file:///tmp/def.sh")
	dispatch(t, s, protocol.MethodTextDocumentDidOpen, protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{URI: u, Version: 1, Text: defSrc},
	})

	// Cursor on the "greet" call (line 8, column 2).
	locs := call[[]protocol.Location](t, s, protocol.MethodTextDocumentDefinition,
		protocol.DefinitionParams{
			TextDocumentPositionParams: protocol.TextDocumentPositionParams{
				TextDocument: protocol.TextDocumentIdentifier{URI: u},
				Position:     protocol.Position{Line: 8, Character: 2},
			},
		})
	assert.Assert(t, cmp.Len(locs, 1))
	assert.Equal(t, locs[0].URI, u)
	assert.Equal(t, locs[0].Range.Start.Line, uint32(0))
	assert.Equal(t, locs[0].Range.Start.Character, uint32(0))
	assert.Equal(t, locs[0].Range.End.Character, uint32(5)) // len("greet")
}

func TestDefinitionUnknownWordReturnsEmpty(t *testing.T) {
	s, _ := newTestServer()
	u := uri.URI("file:///tmp/def.sh")
	dispatch(t, s, protocol.MethodTextDocumentDidOpen, protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{URI: u, Version: 1, Text: defSrc},
	})
	locs := call[[]protocol.Location](t, s, protocol.MethodTextDocumentDefinition,
		protocol.DefinitionParams{
			TextDocumentPositionParams: protocol.TextDocumentPositionParams{
				TextDocument: protocol.TextDocumentIdentifier{URI: u},
				Position:     protocol.Position{Line: 1, Character: 2}, // "echo"
			},
		})
	assert.Assert(t, cmp.Len(locs, 0))
}

func TestDefinitionUTF8Encoding(t *testing.T) {
	s, _ := newTestServer()
	// Negotiate utf-8 so character counts are bytes.
	s.initialize([]byte(`{"capabilities":{"general":{"positionEncodings":["utf-8"]}}}`))

	u := uri.URI("file:///tmp/u8.sh")
	// Bash identifiers are ASCII, but the file holds multi-byte content
	// (the café comment) that the byte-offset path must navigate.
	src := "greet() {\n  echo hi\n}\n# café — multibyte\ngreet\n"
	dispatch(t, s, protocol.MethodTextDocumentDidOpen, protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{URI: u, Version: 1, Text: src},
	})
	// Cursor on the "greet" call (line 4, byte char 2).
	locs := call[[]protocol.Location](t, s, protocol.MethodTextDocumentDefinition,
		protocol.DefinitionParams{
			TextDocumentPositionParams: protocol.TextDocumentPositionParams{
				TextDocument: protocol.TextDocumentIdentifier{URI: u},
				Position:     protocol.Position{Line: 4, Character: 2},
			},
		})
	assert.Assert(t, cmp.Len(locs, 1))
	assert.Equal(t, locs[0].Range.Start.Line, uint32(0))
}

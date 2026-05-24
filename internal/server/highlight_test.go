// SPDX-License-Identifier: GPL-3.0-only

package server

import (
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
)

func TestDocumentHighlightMarksReadVsWrite(t *testing.T) {
	s, _ := newTestServer()
	u := uri.URI("file:///tmp/hl.sh")
	dispatch(t, s, protocol.MethodTextDocumentDidOpen, protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{URI: u, Version: 1, Text: refSrc},
	})

	hits := call[[]protocol.DocumentHighlight](t, s, protocol.MethodTextDocumentDocumentHighlight,
		protocol.DocumentHighlightParams{
			TextDocumentPositionParams: protocol.TextDocumentPositionParams{
				TextDocument: protocol.TextDocumentIdentifier{URI: u},
				Position:     protocol.Position{Line: 0, Character: 0},
			},
		})

	assert.Assert(t, cmp.Len(hits, 3))
	// First hit is the decl on line 0 → Write.
	assert.Equal(t, hits[0].Range.Start.Line, uint32(0))
	assert.Equal(t, hits[0].Kind, protocol.DocumentHighlightKindWrite)
	assert.Equal(t, hits[1].Kind, protocol.DocumentHighlightKindRead)
	assert.Equal(t, hits[2].Kind, protocol.DocumentHighlightKindRead)
}

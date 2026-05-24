// SPDX-License-Identifier: GPL-3.0-only

package server

import (
	"strings"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
)

func TestFormattingRewritesFullBuffer(t *testing.T) {
	s, _ := newTestServer()
	u := uri.URI("file:///tmp/fmt.sh")

	dispatch(t, s, protocol.MethodTextDocumentDidOpen, protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI:     u,
			Version: 1,
			Text:    "if true; then    echo   hi;fi\n",
		},
	})

	edits := call[[]protocol.TextEdit](t, s, protocol.MethodTextDocumentFormatting,
		protocol.DocumentFormattingParams{TextDocument: protocol.TextDocumentIdentifier{URI: u}})

	assert.Assert(t, cmp.Len(edits, 1))
	assert.Equal(t, edits[0].Range.Start.Line, uint32(0))
	assert.Assert(t, edits[0].Range.End.Line > 1<<20, "End.Line should be effectively MAX")
	assert.Assert(t, !strings.Contains(edits[0].NewText, "    echo   hi"),
		"output should be re-spaced; got %q", edits[0].NewText)
	assert.Assert(t, strings.Contains(edits[0].NewText, "echo hi"))
}

func TestFormattingHonorsIndentOptions(t *testing.T) {
	s, _ := newTestServer()
	u := uri.URI("file:///tmp/indent.sh")
	dispatch(t, s, protocol.MethodTextDocumentDidOpen, protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI: u, Version: 1,
			Text: "if true; then\necho hi\nfi\n",
		},
	})

	tabbed := call[[]protocol.TextEdit](t, s, protocol.MethodTextDocumentFormatting,
		protocol.DocumentFormattingParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: u},
			Options:      protocol.FormattingOptions{InsertSpaces: false, TabSize: 4},
		})
	assert.Assert(t, cmp.Len(tabbed, 1))
	assert.Assert(t, strings.Contains(tabbed[0].NewText, "\techo hi"),
		"expected tab indent; got %q", tabbed[0].NewText)

	spaced := call[[]protocol.TextEdit](t, s, protocol.MethodTextDocumentFormatting,
		protocol.DocumentFormattingParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: u},
			Options:      protocol.FormattingOptions{InsertSpaces: true, TabSize: 2},
		})
	assert.Assert(t, cmp.Len(spaced, 1))
	assert.Assert(t, strings.Contains(spaced[0].NewText, "\n  echo hi"),
		"expected 2-space indent; got %q", spaced[0].NewText)
	assert.Assert(t, !strings.Contains(spaced[0].NewText, "\techo"),
		"should not contain tabs; got %q", spaced[0].NewText)
}

func TestFormattingSkipsBrokenBuffer(t *testing.T) {
	s, _ := newTestServer()
	u := uri.URI("file:///tmp/bad.sh")

	dispatch(t, s, protocol.MethodTextDocumentDidOpen, protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{URI: u, Version: 1, Text: "if then\n"},
	})

	edits := call[[]protocol.TextEdit](t, s, protocol.MethodTextDocumentFormatting,
		protocol.DocumentFormattingParams{TextDocument: protocol.TextDocumentIdentifier{URI: u}})
	assert.Assert(t, cmp.Len(edits, 0))
}

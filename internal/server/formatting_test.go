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

func TestFormattingUsesConfiguredIndent(t *testing.T) {
	s, _ := newTestServer()
	u := uri.URI("file:///tmp/configured-indent.sh")
	dispatch(t, s, protocol.MethodTextDocumentDidOpen, protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI: u, Version: 1,
			Text: "if true; then\necho hi\nfi\n",
		},
	})
	dispatch(t, s, protocol.MethodWorkspaceDidChangeConfiguration, protocol.DidChangeConfigurationParams{
		Settings: map[string]interface{}{"formatIndentSpaces": 3},
	})

	edits := call[[]protocol.TextEdit](t, s, protocol.MethodTextDocumentFormatting,
		protocol.DocumentFormattingParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: u},
			Options:      protocol.FormattingOptions{InsertSpaces: false, TabSize: 4},
		})

	assert.Assert(t, cmp.Len(edits, 1))
	assert.Assert(t, strings.Contains(edits[0].NewText, "\n   echo hi"),
		"expected configured 3-space indent; got %q", edits[0].NewText)
	assert.Assert(t, !strings.Contains(edits[0].NewText, "\techo hi"),
		"configured space indent should override LSP tab option; got %q", edits[0].NewText)
}

func TestFormattingUsesEditorConfigIndent(t *testing.T) {
	root := t.TempDir()
	assert.NilError(t, os.WriteFile(filepath.Join(root, ".editorconfig"), []byte(`root = true

[*.sh]
indent_style = space
indent_size = 3
`), 0o644))
	path := filepath.Join(root, "script.sh")
	s, _ := newTestServer()
	u := uri.File(path)
	dispatch(t, s, protocol.MethodTextDocumentDidOpen, protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI: u, Version: 1,
			Text: "if true; then\necho hi\nfi\n",
		},
	})

	edits := call[[]protocol.TextEdit](t, s, protocol.MethodTextDocumentFormatting,
		protocol.DocumentFormattingParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: u},
			Options:      protocol.FormattingOptions{InsertSpaces: false, TabSize: 4},
		})

	assert.Assert(t, cmp.Len(edits, 1))
	assert.Assert(t, strings.Contains(edits[0].NewText, "\n   echo hi"),
		"expected .editorconfig 3-space indent; got %q", edits[0].NewText)
}

func TestFormattingConfigOverridesEditorConfigIndent(t *testing.T) {
	root := t.TempDir()
	assert.NilError(t, os.WriteFile(filepath.Join(root, ".editorconfig"), []byte(`root = true

[*.sh]
indent_style = space
indent_size = 8
`), 0o644))
	path := filepath.Join(root, "script.sh")
	s, _ := newTestServer()
	u := uri.File(path)
	dispatch(t, s, protocol.MethodTextDocumentDidOpen, protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI: u, Version: 1,
			Text: "if true; then\necho hi\nfi\n",
		},
	})
	dispatch(t, s, protocol.MethodWorkspaceDidChangeConfiguration, protocol.DidChangeConfigurationParams{
		Settings: map[string]interface{}{"formatIndentSpaces": 2},
	})

	edits := call[[]protocol.TextEdit](t, s, protocol.MethodTextDocumentFormatting,
		protocol.DocumentFormattingParams{TextDocument: protocol.TextDocumentIdentifier{URI: u}})

	assert.Assert(t, cmp.Len(edits, 1))
	assert.Assert(t, strings.Contains(edits[0].NewText, "\n  echo hi"),
		"expected configured 2-space indent; got %q", edits[0].NewText)
	assert.Assert(t, !strings.Contains(edits[0].NewText, "\n        echo hi"),
		"server config should override .editorconfig; got %q", edits[0].NewText)
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

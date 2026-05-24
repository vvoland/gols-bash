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
	"mvdan.cc/sh/v3/syntax"
)

func TestReferencesCrossFile(t *testing.T) {
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
	// Pre-populate the index without going through initialize (the
	// initialized handler does this asynchronously in production).
	for _, p := range []string{libPath, usePath} {
		b, _ := os.ReadFile(p)
		s.index.AddOrReplace(uri.File(p), parseBash(p, string(b)))
	}

	// Open use.sh as the active document.
	useURI := uri.File(usePath)
	dispatch(t, s, protocol.MethodTextDocumentDidOpen, protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{URI: useURI, Version: 1, Text: "greet\ngreet\n"},
	})

	locs := call[[]protocol.Location](t, s, protocol.MethodTextDocumentReferences,
		protocol.ReferenceParams{
			TextDocumentPositionParams: protocol.TextDocumentPositionParams{
				TextDocument: protocol.TextDocumentIdentifier{URI: useURI},
				Position:     protocol.Position{Line: 0, Character: 0},
			},
			Context: protocol.ReferenceContext{IncludeDeclaration: true},
		})

	// 1 decl (lib.sh) + 2 calls (use.sh) = 3 hits across both files.
	assert.Assert(t, cmp.Len(locs, 3))
	var uris []string
	for _, l := range locs {
		uris = append(uris, string(l.URI))
	}
	assert.Assert(t, contains(uris, string(uri.File(libPath))))
	assert.Assert(t, contains(uris, string(useURI)))
}

func contains(s []string, x string) bool {
	for _, v := range s {
		if v == x {
			return true
		}
	}
	return false
}

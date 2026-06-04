// SPDX-License-Identifier: GPL-3.0-only

package server

import (
	"context"
	"encoding/json"
	"testing"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
)

// call dispatches a request and decodes the result.
func call[T any](t *testing.T, s *bashServer, method string, params interface{}) T {
	t.Helper()
	c, err := jsonrpc2.NewCall(jsonrpc2.NewNumberID(1), method, params)
	assert.NilError(t, err)

	var got T
	rep := func(_ context.Context, result interface{}, err error) error {
		assert.NilError(t, err)
		b, mErr := json.Marshal(result)
		assert.NilError(t, mErr)
		assert.NilError(t, json.Unmarshal(b, &got))
		return nil
	}
	assert.NilError(t, s.handle(context.Background(), rep, c))
	return got
}

func TestDocumentSymbolListsFunctions(t *testing.T) {
	s, _ := newTestServer()
	u := uri.URI("file:///tmp/syms.sh")

	dispatch(t, s, protocol.MethodTextDocumentDidOpen, protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI:     u,
			Version: 1,
			Text:    "greet() {\n  echo hi\n}\n\nfunction farewell {\n  echo bye\n}\n",
		},
	})

	syms := call[[]protocol.DocumentSymbol](t, s, protocol.MethodTextDocumentDocumentSymbol,
		protocol.DocumentSymbolParams{TextDocument: protocol.TextDocumentIdentifier{URI: u}})

	assert.Assert(t, cmp.Len(syms, 2))
	assert.Equal(t, syms[0].Name, "greet")
	assert.Equal(t, syms[0].Kind, protocol.SymbolKindFunction)
	assert.Equal(t, syms[0].SelectionRange.Start.Line, uint32(0))
	assert.Equal(t, syms[1].Name, "farewell")
	assert.Equal(t, syms[1].SelectionRange.Start.Line, uint32(4))
}

func TestDocumentSymbolUnknownDoc(t *testing.T) {
	s, _ := newTestServer()
	syms := call[[]protocol.DocumentSymbol](t, s, protocol.MethodTextDocumentDocumentSymbol,
		protocol.DocumentSymbolParams{TextDocument: protocol.TextDocumentIdentifier{URI: uri.URI("file:///nope")}})
	assert.Assert(t, cmp.Len(syms, 0))
}

func TestWorkspaceSymbolListsIndexedDeclarations(t *testing.T) {
	s, _ := newTestServer()
	u := uri.URI("file:///tmp/workspace.sh")

	dispatch(t, s, protocol.MethodTextDocumentDidOpen, protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI:     u,
			Version: 1,
			Text:    "target_var=1\ntarget_fn() {\n  echo hi\n}\nother=2\n",
		},
	})

	syms := call[[]protocol.SymbolInformation](t, s, protocol.MethodWorkspaceSymbol,
		protocol.WorkspaceSymbolParams{Query: "target"})

	assert.Assert(t, cmp.Len(syms, 2))
	assert.Equal(t, syms[0].Name, "target_fn")
	assert.Equal(t, syms[0].Kind, protocol.SymbolKindFunction)
	assert.Equal(t, syms[0].Location.URI, u)
	assert.Equal(t, syms[1].Name, "target_var")
	assert.Equal(t, syms[1].Kind, protocol.SymbolKindVariable)
}

func TestWorkspaceSymbolUnknownQueryReturnsEmptyList(t *testing.T) {
	s, _ := newTestServer()
	u := uri.URI("file:///tmp/workspace.sh")
	dispatch(t, s, protocol.MethodTextDocumentDidOpen, protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{URI: u, Version: 1, Text: "present=1\n"},
	})

	syms := call[[]protocol.SymbolInformation](t, s, protocol.MethodWorkspaceSymbol,
		protocol.WorkspaceSymbolParams{Query: "missing"})

	assert.Assert(t, cmp.Len(syms, 0))
}

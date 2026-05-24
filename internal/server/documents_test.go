// SPDX-License-Identifier: GPL-3.0-only

package server

import (
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
)

func TestDocumentStoreLifecycle(t *testing.T) {
	s := NewDocumentStore()
	u := uri.URI("file:///tmp/script.sh")

	assert.Assert(t, cmp.Len(s.All(), 0))

	s.Open(protocol.TextDocumentItem{
		URI:        u,
		LanguageID: "shellscript",
		Version:    1,
		Text:       "echo hello",
	})
	assert.Assert(t, cmp.Len(s.All(), 1))

	d, ok := s.Get(u)
	assert.Assert(t, ok)
	assert.Equal(t, d.Text, "echo hello")
	assert.Equal(t, d.Version, int32(1))

	d2, ok := s.Update(u, 2, "echo world")
	assert.Assert(t, ok)
	assert.Equal(t, d2.Text, "echo world")
	assert.Equal(t, d2.Version, int32(2))

	_, ok = s.Update(uri.URI("file:///nope"), 1, "")
	assert.Assert(t, cmp.Equal(ok, false))

	assert.Assert(t, s.Close(u))
	assert.Assert(t, cmp.Len(s.All(), 0))
	assert.Assert(t, cmp.Equal(s.Close(u), false))
}

func TestDocumentStoreParses(t *testing.T) {
	s := NewDocumentStore()
	u := uri.URI("file:///tmp/script.sh")

	d := s.Open(protocol.TextDocumentItem{
		URI:     u,
		Version: 1,
		Text:    "echo hello\n",
	})
	assert.NilError(t, d.ParseErr)
	assert.Assert(t, d.AST != nil)
	assert.Assert(t, cmp.Len(d.AST.Stmts, 1))

	d2, ok := s.Update(u, 2, "if then\n")
	assert.Assert(t, ok)
	assert.Assert(t, d2.ParseErr != nil, "malformed input should produce ParseErr")
}

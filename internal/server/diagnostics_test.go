// SPDX-License-Identifier: GPL-3.0-only

package server

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"sync"
	"testing"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"grono.dev/gols-bash/internal/analyser"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
)

type recordedNotification struct {
	method string
	params *protocol.PublishDiagnosticsParams
}

type notifyRecorder struct {
	mu   sync.Mutex
	sent []recordedNotification
}

func (r *notifyRecorder) notify(_ context.Context, method string, params interface{}) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	b, _ := json.Marshal(params)
	var p protocol.PublishDiagnosticsParams
	_ = json.Unmarshal(b, &p)
	r.sent = append(r.sent, recordedNotification{method: method, params: &p})
	return nil
}

func newTestServer() (*bashServer, *notifyRecorder) {
	rec := &notifyRecorder{}
	s := &bashServer{
		log:    slog.New(slog.NewTextHandler(io.Discard, nil)),
		docs:   NewDocumentStore(),
		notify: rec.notify,
		index:  analyser.NewIndex(),
	}
	return s, rec
}

func nopReplier(_ context.Context, _ interface{}, _ error) error { return nil }

func dispatch(t *testing.T, s *bashServer, method string, params interface{}) {
	t.Helper()
	n, err := jsonrpc2.NewNotification(method, params)
	assert.NilError(t, err)
	assert.NilError(t, s.handle(context.Background(), nopReplier, n))
}

func TestDidOpenPublishesParseError(t *testing.T) {
	s, rec := newTestServer()
	u := uri.URI("file:///tmp/bad.sh")

	dispatch(t, s, protocol.MethodTextDocumentDidOpen, protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI:     u,
			Version: 1,
			Text:    "if then\n",
		},
	})

	assert.Assert(t, cmp.Len(rec.sent, 1))
	got := rec.sent[0]
	assert.Equal(t, got.method, protocol.MethodTextDocumentPublishDiagnostics)
	assert.Equal(t, got.params.URI, u)
	assert.Assert(t, cmp.Len(got.params.Diagnostics, 1))
	d := got.params.Diagnostics[0]
	assert.Equal(t, d.Severity, protocol.DiagnosticSeverityError)
	assert.Equal(t, d.Source, "bash")
}

func TestDidOpenPublishesEmptyForValid(t *testing.T) {
	s, rec := newTestServer()
	dispatch(t, s, protocol.MethodTextDocumentDidOpen, protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI:     uri.URI("file:///tmp/ok.sh"),
			Version: 1,
			Text:    "echo hi\n",
		},
	})
	assert.Assert(t, cmp.Len(rec.sent, 1))
	assert.Assert(t, cmp.Len(rec.sent[0].params.Diagnostics, 0))
}

func TestDidCloseClearsDiagnostics(t *testing.T) {
	s, rec := newTestServer()
	u := uri.URI("file:///tmp/bad.sh")

	dispatch(t, s, protocol.MethodTextDocumentDidOpen, protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{URI: u, Version: 1, Text: "if then\n"},
	})
	dispatch(t, s, protocol.MethodTextDocumentDidClose, protocol.DidCloseTextDocumentParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: u},
	})

	assert.Assert(t, cmp.Len(rec.sent, 2))
	assert.Assert(t, cmp.Len(rec.sent[1].params.Diagnostics, 0))
}

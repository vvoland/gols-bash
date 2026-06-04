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

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
	"grono.dev/gols-bash/internal/analyser"
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
		log:         slog.New(slog.NewTextHandler(io.Discard, nil)),
		docs:        NewDocumentStore(),
		notify:      rec.notify,
		index:       analyser.NewIndex(),
		codeActions: make(map[uri.URI][]protocol.CodeAction),
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

func TestDidOpenPublishesShellCheckDiagnostics(t *testing.T) {
	s, rec := newTestServer()
	s.shellcheck = func(_ context.Context, _ *Document) (diagnosticResult, error) {
		return diagnosticResult{Diagnostics: []protocol.Diagnostic{{
			Range:    protocol.Range{Start: protocol.Position{Line: 0}, End: protocol.Position{Line: 0, Character: 4}},
			Severity: protocol.DiagnosticSeverityWarning,
			Code:     "SC2086",
			Source:   "shellcheck",
			Message:  "Double quote to prevent globbing and word splitting.",
		}}}, nil
	}

	dispatch(t, s, protocol.MethodTextDocumentDidOpen, protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI:     uri.URI("file:///tmp/lint.sh"),
			Version: 1,
			Text:    "echo $name\n",
		},
	})

	assert.Assert(t, cmp.Len(rec.sent, 1))
	assert.Assert(t, cmp.Len(rec.sent[0].params.Diagnostics, 1))
	d := rec.sent[0].params.Diagnostics[0]
	assert.Equal(t, d.Source, "shellcheck")
	assert.Equal(t, d.Code, "SC2086")
}

func TestShellCheckDiagnosticsParsesJSON(t *testing.T) {
	raw := []byte(`{
		"comments": [{
			"line": 2,
			"endLine": 2,
			"column": 6,
			"endColumn": 11,
			"level": "warning",
			"code": 2086,
			"message": "Double quote to prevent globbing and word splitting."
		}]
	}`)

	result, err := shellCheckDiagnostics(uri.URI("file:///tmp/lint.sh"), raw)
	assert.NilError(t, err)
	diags := result.Diagnostics
	assert.Assert(t, cmp.Len(diags, 1))
	d := diags[0]
	assert.Equal(t, d.Range.Start.Line, uint32(1))
	assert.Equal(t, d.Range.Start.Character, uint32(5))
	assert.Equal(t, d.Range.End.Character, uint32(10))
	assert.Equal(t, d.Severity, protocol.DiagnosticSeverityWarning)
	assert.Equal(t, d.Code, "SC2086")
	assert.Equal(t, d.CodeDescription.Href, protocol.URI("https://www.shellcheck.net/wiki/SC2086"))
	assert.Equal(t, d.Source, "shellcheck")
}

func TestShellCheckDiagnosticsCreatesCodeActions(t *testing.T) {
	raw := []byte(`{
		"comments": [{
			"line": 1,
			"endLine": 1,
			"column": 6,
			"endColumn": 11,
			"level": "warning",
			"code": 2086,
			"message": "Double quote to prevent globbing and word splitting.",
			"fix": {
				"replacements": [{
					"line": 1,
					"endLine": 1,
					"column": 6,
					"endColumn": 11,
					"replacement": "\"$name\""
				}]
			}
		}]
	}`)

	u := uri.URI("file:///tmp/lint.sh")
	result, err := shellCheckDiagnostics(u, raw)
	assert.NilError(t, err)
	assert.Assert(t, cmp.Len(result.CodeActions, 1))
	action := result.CodeActions[0]
	assert.Equal(t, action.Kind, protocol.QuickFix)
	assert.Equal(t, action.Title, "Apply fix for SC2086")
	assert.Assert(t, cmp.Len(action.Edit.Changes[u], 1))
	assert.Equal(t, action.Edit.Changes[u][0].NewText, "\"$name\"")
}

func TestCodeActionReturnsStoredQuickFixes(t *testing.T) {
	s, _ := newTestServer()
	u := uri.URI("file:///tmp/lint.sh")
	s.setCodeActions(u, []protocol.CodeAction{{Title: "Apply fix for SC2086", Kind: protocol.QuickFix}})

	actions := call[[]protocol.CodeAction](t, s, protocol.MethodTextDocumentCodeAction, protocol.CodeActionParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: u},
		Context:      protocol.CodeActionContext{Only: []protocol.CodeActionKind{protocol.QuickFix}},
	})

	assert.Assert(t, cmp.Len(actions, 1))
	assert.Equal(t, actions[0].Title, "Apply fix for SC2086")
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

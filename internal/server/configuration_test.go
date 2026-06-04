// SPDX-License-Identifier: GPL-3.0-only

package server

import (
	"context"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
)

func TestParseSettingsAcceptsTopLevelShellCheckConfig(t *testing.T) {
	settings := parseSettings([]byte(`{
		"shellcheckPath": "/custom/shellcheck",
		"shellcheckArguments": ["--external-sources"]
	}`), defaultSettings())

	assert.Equal(t, settings.ShellCheckPath, "/custom/shellcheck")
	assert.DeepEqual(t, settings.ShellCheckArguments, []string{"--external-sources"})
}

func TestParseSettingsAcceptsNestedBashIdeConfig(t *testing.T) {
	settings := parseSettings([]byte(`{
		"bashIde": {
			"shellcheckPath": "/nested/shellcheck",
			"shellcheckArguments": "--shell=bash"
		}
	}`), defaultSettings())

	assert.Equal(t, settings.ShellCheckPath, "/nested/shellcheck")
	assert.DeepEqual(t, settings.ShellCheckArguments, []string{"--shell=bash"})
}

func TestDidChangeConfigurationDisablesShellCheckAndRelintsOpenDocs(t *testing.T) {
	s, rec := newTestServer()
	u := uri.URI("file:///tmp/config.sh")
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
		TextDocument: protocol.TextDocumentItem{URI: u, Version: 1, Text: "echo $name\n"},
	})

	dispatch(t, s, protocol.MethodWorkspaceDidChangeConfiguration, protocol.DidChangeConfigurationParams{
		Settings: map[string]interface{}{"shellcheckPath": ""},
	})

	assert.Assert(t, cmp.Len(rec.sent, 2))
	assert.Assert(t, cmp.Len(rec.sent[0].params.Diagnostics, 1))
	assert.Assert(t, cmp.Len(rec.sent[1].params.Diagnostics, 0))
	assert.Assert(t, s.shellcheck == nil)
}

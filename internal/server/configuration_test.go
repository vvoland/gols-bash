// SPDX-License-Identifier: GPL-3.0-only

package server

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
)

func TestApplySettingsPassesNegotiatedEncodingToShellCheck(t *testing.T) {
	script := filepath.Join(t.TempDir(), "shellcheck")
	assert.NilError(t, os.WriteFile(script, []byte("#!/bin/sh\necho '{\"comments\":[{\"line\":1,\"endLine\":1,\"column\":7,\"endColumn\":12,\"level\":\"warning\",\"code\":2086,\"message\":\"quote it\"}]}'\n"), 0o755))
	assert.NilError(t, os.Chmod(script, 0o755))
	s, _ := newTestServer()
	s.initialize([]byte(`{"capabilities":{"general":{"positionEncodings":["utf-8"]}}}`))
	s.applySettings(serverSettings{ShellCheckPath: script})
	d := &Document{URI: uri.URI("file:///tmp/unicode.sh"), Text: "echo é$name\n"}
	result, err := s.shellcheck(context.Background(), d)
	assert.NilError(t, err)
	assert.Assert(t, cmp.Len(result.Diagnostics, 1))
	assert.Equal(t, result.Diagnostics[0].Range.Start.Character, uint32(7))
}

func TestParseSettingsAcceptsTopLevelShellCheckConfig(t *testing.T) {
	settings := parseSettings([]byte(`{
		"shellcheckPath": "/custom/shellcheck",
		"shellcheckArguments": ["--external-sources"],
		"formatIndentSpaces": 2,
		"workspaceScanEnabled": false
	}`), defaultSettings())

	assert.Equal(t, settings.ShellCheckPath, "/custom/shellcheck")
	assert.DeepEqual(t, settings.ShellCheckArguments, []string{"--external-sources"})
	assert.Assert(t, settings.FormatIndentSpaces != nil)
	assert.Equal(t, *settings.FormatIndentSpaces, uint(2))
	assert.Equal(t, settings.WorkspaceScanEnabled, false)
}

func TestParseSettingsAcceptsNestedBashIdeConfig(t *testing.T) {
	settings := parseSettings([]byte(`{
		"bashIde": {
			"shellcheckPath": "/nested/shellcheck",
			"shellcheckArguments": "--shell=bash",
			"formatIndentSpaces": 3,
			"workspaceScanEnabled": false
		}
	}`), defaultSettings())

	assert.Equal(t, settings.ShellCheckPath, "/nested/shellcheck")
	assert.DeepEqual(t, settings.ShellCheckArguments, []string{"--shell=bash"})
	assert.Assert(t, settings.FormatIndentSpaces != nil)
	assert.Equal(t, *settings.FormatIndentSpaces, uint(3))
	assert.Equal(t, settings.WorkspaceScanEnabled, false)
}

func TestScanWorkspacesHonorsWorkspaceScanEnabled(t *testing.T) {
	root := t.TempDir()
	assert.NilError(t, os.WriteFile(filepath.Join(root, "lib.sh"), []byte("greet() { :; }\n"), 0o644))

	s, _ := newTestServer()
	s.workspaceRoots = []string{root}
	s.applySettings(parseSettings([]byte(`{"workspaceScanEnabled": false}`), s.settings))
	s.scanWorkspaces(context.Background())

	assert.Equal(t, s.index.Len(), 0)
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
		Settings: map[string]any{"shellcheckPath": ""},
	})

	assert.Assert(t, cmp.Len(rec.sent, 2))
	assert.Assert(t, cmp.Len(rec.sent[0].params.Diagnostics, 1))
	assert.Assert(t, cmp.Len(rec.sent[1].params.Diagnostics, 0))
	assert.Assert(t, s.shellcheck == nil)
}

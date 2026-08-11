// SPDX-License-Identifier: GPL-3.0-only

package server

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"grono.dev/gols-bash/internal/analyser"
)

func TestWorkspaceScanPreservesOpenBuffer(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "script.sh")
	writeWorkspaceFile(t, path, "from_disk() { :; }\n")

	s, _ := newTestServer()
	s.workspaceRoots = []string{root}
	u := uri.File(path)
	dispatch(t, s, protocol.MethodTextDocumentDidOpen, protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{URI: u, Version: 1, Text: "from_buffer() { :; }\n"},
	})

	s.scanWorkspaces(context.Background())
	assert.DeepEqual(t, indexedDeclarationNames(s, u), []string{"from_buffer"})
}

func TestDidCloseRestoresDiskOrRemovesIndex(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "script.sh")
	writeWorkspaceFile(t, path, "from_disk() { :; }\n")

	s, _ := newTestServer()
	s.workspaceRoots = []string{root}
	u := uri.File(path)
	dispatch(t, s, protocol.MethodTextDocumentDidOpen, protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{URI: u, Version: 1, Text: "unsaved() { :; }\n"},
	})
	dispatch(t, s, protocol.MethodTextDocumentDidClose, protocol.DidCloseTextDocumentParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: u},
	})
	assert.DeepEqual(t, indexedDeclarationNames(s, u), []string{"from_disk"})

	outside := uri.File(filepath.Join(t.TempDir(), "outside.sh"))
	dispatch(t, s, protocol.MethodTextDocumentDidOpen, protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{URI: outside, Version: 1, Text: "discarded() { :; }\n"},
	})
	dispatch(t, s, protocol.MethodTextDocumentDidClose, protocol.DidCloseTextDocumentParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: outside},
	})
	assert.Assert(t, s.index.Get(outside) == nil)
}

func TestWatchedFilesRefreshAndPreserveOpenDocuments(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "watched.sh")
	u := uri.File(path)
	s, _ := newTestServer()
	s.workspaceRoots = []string{root}

	writeWorkspaceFile(t, path, "created() { :; }\n")
	dispatchWatchedFile(t, s, u, protocol.FileChangeTypeCreated)
	assert.DeepEqual(t, indexedDeclarationNames(s, u), []string{"created"})

	writeWorkspaceFile(t, path, "changed() { :; }\n")
	dispatchWatchedFile(t, s, u, protocol.FileChangeTypeChanged)
	assert.DeepEqual(t, indexedDeclarationNames(s, u), []string{"changed"})

	dispatch(t, s, protocol.MethodTextDocumentDidOpen, protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{URI: u, Version: 1, Text: "open_buffer() { :; }\n"},
	})
	writeWorkspaceFile(t, path, "newer_disk() { :; }\n")
	dispatchWatchedFile(t, s, u, protocol.FileChangeTypeChanged)
	assert.DeepEqual(t, indexedDeclarationNames(s, u), []string{"open_buffer"})

	assert.NilError(t, os.Remove(path))
	dispatchWatchedFile(t, s, u, protocol.FileChangeTypeDeleted)
	assert.DeepEqual(t, indexedDeclarationNames(s, u), []string{"open_buffer"})

	dispatch(t, s, protocol.MethodTextDocumentDidClose, protocol.DidCloseTextDocumentParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: u},
	})
	assert.Assert(t, s.index.Get(u) == nil)
}

func TestWatchedFileRemovesExtensionlessScriptAfterShebangRemoval(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "script")
	u := uri.File(path)
	s, _ := newTestServer()
	s.workspaceRoots = []string{root}
	writeWorkspaceFile(t, path, "#!/bin/sh\ncreated() { :; }\n")
	dispatchWatchedFile(t, s, u, protocol.FileChangeTypeCreated)
	assert.Assert(t, s.index.Get(u) != nil)

	writeWorkspaceFile(t, path, "created() { :; }\n")
	dispatchWatchedFile(t, s, u, protocol.FileChangeTypeChanged)
	assert.Assert(t, s.index.Get(u) == nil)
}

func TestDidSaveIndexesProvidedText(t *testing.T) {
	s, _ := newTestServer()
	u := uri.URI("file:///tmp/saved.sh")
	dispatch(t, s, protocol.MethodTextDocumentDidOpen, protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{URI: u, Version: 1, Text: "before_save() { :; }\n"},
	})
	dispatch(t, s, protocol.MethodTextDocumentDidSave, map[string]any{
		"textDocument": map[string]any{"uri": u},
		"text":         "saved_text() { :; }\n",
	})
	assert.DeepEqual(t, indexedDeclarationNames(s, u), []string{"saved_text"})
	d, ok := s.docs.Get(u)
	assert.Assert(t, ok)
	assert.Equal(t, d.Text, "saved_text() { :; }\n")
}

func TestInitializedRegistersWatchedFilesDynamically(t *testing.T) {
	s, _ := newTestServer()
	calls := make(chan *protocol.RegistrationParams, 1)
	s.call = func(_ context.Context, method string, params, _ any) (jsonrpc2.ID, error) {
		assert.Equal(t, method, protocol.MethodClientRegisterCapability)
		p, ok := params.(*protocol.RegistrationParams)
		assert.Assert(t, ok)
		calls <- p
		return jsonrpc2.NewNumberID(1), nil
	}
	s.initialize([]byte(`{"capabilities":{"workspace":{"didChangeWatchedFiles":{"dynamicRegistration":true}}}}`))
	dispatch(t, s, protocol.MethodInitialized, struct{}{})

	select {
	case p := <-calls:
		assert.Assert(t, cmp.Len(p.Registrations, 1))
		assert.Equal(t, p.Registrations[0].Method, protocol.MethodWorkspaceDidChangeWatchedFiles)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for dynamic registration")
	}
}

func dispatchWatchedFile(t *testing.T, s *bashServer, u uri.URI, changeType protocol.FileChangeType) {
	t.Helper()
	dispatch(t, s, protocol.MethodWorkspaceDidChangeWatchedFiles, protocol.DidChangeWatchedFilesParams{
		Changes: []*protocol.FileEvent{{URI: u, Type: changeType}},
	})
}

func indexedDeclarationNames(s *bashServer, u uri.URI) []string {
	file := s.index.Get(u)
	declarations := analyser.FindDeclarations(file)
	names := make([]string, 0, len(declarations))
	for _, declaration := range declarations {
		names = append(names, declaration.Name)
	}
	return names
}

func writeWorkspaceFile(t *testing.T, path, content string) {
	t.Helper()
	assert.NilError(t, os.WriteFile(path, []byte(content), 0o644))
}

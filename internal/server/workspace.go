// SPDX-License-Identifier: GPL-3.0-only

package server

import (
	"context"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"grono.dev/gols-bash/internal/analyser"
)

func (s *bashServer) openDocument(item protocol.TextDocumentItem) *Document {
	s.workspaceMu.Lock()
	defer s.workspaceMu.Unlock()
	d := s.docs.Open(item)
	s.index.AddOrReplace(d.URI, d.AST)
	return d
}

func (s *bashServer) updateDocument(u uri.URI, version int32, text string) (*Document, bool) {
	s.workspaceMu.Lock()
	defer s.workspaceMu.Unlock()
	d, ok := s.docs.Update(u, version, text)
	if ok {
		s.index.AddOrReplace(d.URI, d.AST)
	}
	return d, ok
}

func (s *bashServer) saveDocument(u uri.URI, text *string) (*Document, bool) {
	s.workspaceMu.Lock()
	defer s.workspaceMu.Unlock()
	d, ok := s.docs.Save(u, text)
	if ok {
		s.index.AddOrReplace(d.URI, d.AST)
	}
	return d, ok
}

func (s *bashServer) closeDocument(u uri.URI) {
	s.workspaceMu.Lock()
	defer s.workspaceMu.Unlock()
	if !s.docs.Close(u) {
		return
	}
	path, ok := s.workspaceFilePath(u)
	if !ok || !analyser.IsShellPath(path) {
		s.index.Remove(u)
		return
	}
	src, err := os.ReadFile(path)
	if err != nil {
		s.index.Remove(u)
		return
	}
	s.index.AddOrReplace(u, s.indexParse(path, string(src)))
}

// indexWorkspacePath coordinates the asynchronous scanner's disk read and
// index write with document changes. Whichever operation obtains workspaceMu
// last wins, except disk content can never replace an already-open buffer.
func (s *bashServer) indexWorkspacePath(path string) {
	s.workspaceMu.Lock()
	defer s.workspaceMu.Unlock()
	u := uri.File(path)
	if _, open := s.docs.Get(u); open {
		return
	}
	src, err := os.ReadFile(path)
	if err != nil {
		s.log.Debug("read workspace file failed", "uri", u, "error", err)
		return
	}
	s.index.AddOrReplace(u, s.indexParse(path, string(src)))
}

func (s *bashServer) refreshWorkspaceFile(u uri.URI) {
	s.workspaceMu.Lock()
	defer s.workspaceMu.Unlock()
	if _, open := s.docs.Get(u); open {
		return
	}
	path, ok := s.workspaceFilePath(u)
	if !ok || !analyser.IsShellPath(path) {
		s.index.Remove(u)
		return
	}
	src, err := os.ReadFile(path)
	if err != nil {
		s.log.Warn("read watched file failed", "uri", u, "error", err)
		s.index.Remove(u)
		return
	}
	s.index.AddOrReplace(u, s.indexParse(path, string(src)))
}

func (s *bashServer) removeWorkspaceFile(u uri.URI) {
	s.workspaceMu.Lock()
	defer s.workspaceMu.Unlock()
	if _, open := s.docs.Get(u); open {
		return
	}
	if _, ok := s.workspaceFilePath(u); ok {
		s.index.Remove(u)
	}
}

func (s *bashServer) handleDidChangeWatchedFiles(raw json.RawMessage) {
	var p protocol.DidChangeWatchedFilesParams
	if err := json.Unmarshal(raw, &p); err != nil {
		s.log.Warn("invalid watched file change", "error", err)
		return
	}
	for _, change := range p.Changes {
		if change == nil {
			continue
		}
		switch change.Type {
		case protocol.FileChangeTypeCreated, protocol.FileChangeTypeChanged:
			s.refreshWorkspaceFile(change.URI)
		case protocol.FileChangeTypeDeleted:
			s.removeWorkspaceFile(change.URI)
		}
	}
}

func (s *bashServer) workspaceFilePath(u uri.URI) (string, bool) {
	parsed, err := url.ParseRequestURI(string(u))
	if err != nil || parsed.Scheme != uri.FileScheme {
		return "", false
	}
	path := u.Filename()
	path, err = filepath.Abs(path)
	if err != nil {
		return "", false
	}
	for _, root := range s.workspaceRoots {
		absRoot, err := filepath.Abs(root)
		if err != nil {
			continue
		}
		rel, err := filepath.Rel(absRoot, path)
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return path, true
		}
	}
	return "", false
}

func (s *bashServer) registerFileWatchers(ctx context.Context) {
	if s.call == nil {
		return
	}
	options := &protocol.DidChangeWatchedFilesRegistrationOptions{
		Watchers: []protocol.FileSystemWatcher{{
			GlobPattern: "**/*",
			Kind:        protocol.WatchKindCreate + protocol.WatchKindChange + protocol.WatchKindDelete,
		}},
	}
	params := &protocol.RegistrationParams{Registrations: []protocol.Registration{{
		ID:              "gols-bash-watch-shell-files",
		Method:          protocol.MethodWorkspaceDidChangeWatchedFiles,
		RegisterOptions: options,
	}}}
	if _, err := s.call(ctx, protocol.MethodClientRegisterCapability, params, nil); err != nil {
		s.log.Warn("register watched files failed", "error", err)
	}
}

func (s *bashServer) goBackground(ctx context.Context, fn func(context.Context)) {
	s.background.Go(func() {
		fn(ctx)
	})
}

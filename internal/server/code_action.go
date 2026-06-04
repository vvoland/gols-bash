// SPDX-License-Identifier: GPL-3.0-only

package server

import (
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

func (s *bashServer) codeActionOptions() *protocol.CodeActionOptions {
	return &protocol.CodeActionOptions{CodeActionKinds: []protocol.CodeActionKind{protocol.QuickFix}}
}

func (s *bashServer) setCodeActions(u uri.URI, actions []protocol.CodeAction) {
	s.codeActionsMu.Lock()
	defer s.codeActionsMu.Unlock()
	if len(actions) == 0 {
		delete(s.codeActions, u)
		return
	}
	if s.codeActions == nil {
		s.codeActions = make(map[uri.URI][]protocol.CodeAction)
	}
	s.codeActions[u] = actions
}

func (s *bashServer) codeActionsFor(p protocol.CodeActionParams) []protocol.CodeAction {
	if !allowsQuickFix(p.Context.Only) {
		return []protocol.CodeAction{}
	}
	s.codeActionsMu.RLock()
	defer s.codeActionsMu.RUnlock()
	actions := s.codeActions[uri.URI(p.TextDocument.URI)]
	out := make([]protocol.CodeAction, len(actions))
	copy(out, actions)
	return out
}

func allowsQuickFix(only []protocol.CodeActionKind) bool {
	if len(only) == 0 {
		return true
	}
	for _, kind := range only {
		if kind == protocol.QuickFix {
			return true
		}
	}
	return false
}

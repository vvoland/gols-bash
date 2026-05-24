// SPDX-License-Identifier: GPL-3.0-only

package server

import (
	"errors"

	"go.lsp.dev/protocol"
	"mvdan.cc/sh/v3/syntax"
)

// parseDiagnostics converts a mvdan/sh parse error into LSP diagnostics.
// mvdan stops at the first hard error, so the slice has zero or one entry.
// text is needed to convert mvdan's byte column into an encoding-correct
// LSP position.
func (s *bashServer) parseDiagnostics(text string, err error) []protocol.Diagnostic {
	if err == nil {
		return nil
	}
	var pe syntax.ParseError
	if !errors.As(err, &pe) {
		return []protocol.Diagnostic{{
			Severity: protocol.DiagnosticSeverityError,
			Source:   "bash",
			Message:  err.Error(),
		}}
	}
	pos := s.posToLSP(text, pe.Pos)
	return []protocol.Diagnostic{{
		Range:    protocol.Range{Start: pos, End: pos},
		Severity: protocol.DiagnosticSeverityError,
		Source:   "bash",
		Message:  pe.Text,
	}}
}

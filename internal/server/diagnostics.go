// SPDX-License-Identifier: GPL-3.0-only

package server

import (
	"errors"

	"go.lsp.dev/protocol"
	"mvdan.cc/sh/v3/syntax"
)

// parseDiagnostics converts a mvdan/sh parse error into LSP diagnostics.
// mvdan/sh stops at the first hard error, so the slice has zero or one entry.
// Positions are translated from 1-based (mvdan/sh) to 0-based (LSP).
func parseDiagnostics(err error) []protocol.Diagnostic {
	if err == nil {
		return nil
	}
	var pe syntax.ParseError
	if !errors.As(err, &pe) {
		return []protocol.Diagnostic{{
			Range:    protocol.Range{},
			Severity: protocol.DiagnosticSeverityError,
			Source:   "bash",
			Message:  err.Error(),
		}}
	}
	pos := lspPos(pe.Pos)
	return []protocol.Diagnostic{{
		Range:    protocol.Range{Start: pos, End: pos},
		Severity: protocol.DiagnosticSeverityError,
		Source:   "bash",
		Message:  pe.Text,
	}}
}

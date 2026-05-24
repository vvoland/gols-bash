// SPDX-License-Identifier: GPL-3.0-only

package server

import (
	"bytes"
	"math"

	"go.lsp.dev/protocol"
	"mvdan.cc/sh/v3/syntax"
)

// formatDocument returns a single full-buffer TextEdit, or nil if the
// document either is unknown or did not parse cleanly (we refuse to format
// broken input to avoid silently dropping or rewriting syntax errors).
func (s *bashServer) formatDocument(d *Document, opts protocol.FormattingOptions) ([]protocol.TextEdit, error) {
	if d == nil || d.AST == nil || d.ParseErr != nil {
		return nil, nil
	}
	var indent uint
	if opts.InsertSpaces {
		indent = uint(opts.TabSize)
		if indent == 0 {
			indent = 4
		}
	}
	var buf bytes.Buffer
	if err := syntax.NewPrinter(syntax.Indent(indent)).Print(&buf, d.AST); err != nil {
		return nil, err
	}
	return []protocol.TextEdit{{
		Range: protocol.Range{
			Start: protocol.Position{Line: 0, Character: 0},
			End:   protocol.Position{Line: math.MaxUint32, Character: math.MaxUint32},
		},
		NewText: buf.String(),
	}}, nil
}

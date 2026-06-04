// SPDX-License-Identifier: GPL-3.0-only

package server

import (
	"go.lsp.dev/protocol"
	"mvdan.cc/sh/v3/syntax"

	utillsp "grono.dev/gols-bash/internal/util/lsp"
)

// offsetForPosition translates an LSP Position into a byte offset in text,
// honouring the negotiated PositionEncoding. Returns -1 if out of range.
func (s *bashServer) offsetForPosition(text string, line, character int) int {
	if s.encoding() == EncodingUTF8 {
		return utillsp.OffsetForPositionBytes(text, line, character)
	}
	return utillsp.OffsetForPosition(text, line, character)
}

// posToLSP converts a mvdan/sh Pos to an LSP Position. mvdan reports
// 1-based line + 1-based BYTE column; we shift to 0-based and, if needed,
// re-count the column in UTF-16 code units using text.
func (s *bashServer) posToLSP(text string, p syntax.Pos) protocol.Position {
	line := int(p.Line())
	if line > 0 {
		line--
	}
	col := int(p.Col())
	if col > 0 {
		col--
	}
	if s.encoding() == EncodingUTF8 {
		return protocol.Position{Line: uint32(line), Character: uint32(col)}
	}
	// Find the line-start byte offset (Pos.Offset is absolute) and count
	// UTF-16 units across the in-line prefix.
	off := int(p.Offset())
	start := off - col
	if start < 0 || off > len(text) {
		return protocol.Position{Line: uint32(line), Character: uint32(col)}
	}
	return protocol.Position{
		Line:      uint32(line),
		Character: uint32(utillsp.UTF16Len(text[start:off])),
	}
}

func (s *bashServer) rangeToLSP(text string, start, end syntax.Pos) protocol.Range {
	return protocol.Range{Start: s.posToLSP(text, start), End: s.posToLSP(text, end)}
}

func (s *bashServer) posForOffset(text string, off int) protocol.Position {
	if off < 0 {
		off = 0
	}
	if off > len(text) {
		off = len(text)
	}
	line, lineStart := 0, 0
	for i := 0; i < off; i++ {
		if text[i] == '\n' {
			line++
			lineStart = i + 1
		}
	}
	colText := text[lineStart:off]
	if s.encoding() == EncodingUTF8 {
		return protocol.Position{Line: uint32(line), Character: uint32(len(colText))}
	}
	return protocol.Position{Line: uint32(line), Character: uint32(utillsp.UTF16Len(colText))}
}

// SPDX-License-Identifier: GPL-3.0-only

package server

import (
	"os"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	utillsp "grono.dev/gols-bash/internal/util/lsp"
)

// references returns indexed occurrences of the word under pos.
// It is name-based and does not yet model Bash scope or sourced-file reachability.
func (s *bashServer) references(d *Document, pos protocol.Position, includeDecl bool) []protocol.Location {
	if d == nil || d.AST == nil {
		return nil
	}
	off := s.offsetForPosition(d.Text, int(pos.Line), int(pos.Character))
	if off < 0 {
		return nil
	}
	word, _, _ := utillsp.WordAtOffset(d.Text, off)
	if word == "" {
		return nil
	}
	hits := s.index.AllUsages(word, includeDecl)
	out := make([]protocol.Location, 0, len(hits))
	for _, h := range hits {
		text := s.textForURI(h.URI)
		out = append(out, protocol.Location{
			URI:   h.URI,
			Range: s.rangeToLSP(text, h.Usage.Pos, h.Usage.End),
		})
	}
	return out
}

// textForURI returns the text we should consult when converting positions
// for a file. Open documents win (their text matches the indexed AST);
// otherwise re-read from disk. Returns "" when nothing is available — the
// position will then be a best-effort byte-column rendering.
func (s *bashServer) textForURI(u uri.URI) string {
	if d, ok := s.docs.Get(u); ok {
		return d.Text
	}
	b, err := os.ReadFile(u.Filename())
	if err != nil {
		return ""
	}
	return string(b)
}

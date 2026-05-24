// SPDX-License-Identifier: GPL-3.0-only

package server

import (
	"encoding/json"

	"go.lsp.dev/protocol"
)

// PositionEncoding tags how the client wants Position.character counted.
// UTF-16 is the LSP default. UTF-8 (LSP 3.17) matches our internal byte
// representation 1:1. UTF-32 = full codepoints.
//
// UTF-16 is iota=0 so a zero-valued atomic load lands on the default.
type PositionEncoding uint32

const (
	EncodingUTF16 PositionEncoding = iota
	EncodingUTF8
	EncodingUTF32
)

func (e PositionEncoding) String() string {
	switch e {
	case EncodingUTF8:
		return "utf-8"
	case EncodingUTF32:
		return "utf-32"
	}
	return "utf-16"
}

// pickPositionEncoding inspects raw initialize params for
// GeneralClientCapabilities.PositionEncodings (LSP 3.17) and picks the
// most efficient encoding we support — UTF-8 first, then UTF-16.
//
// The vendored go.lsp.dev/protocol predates 3.17, so we probe raw JSON
// rather than relying on its types.
func pickPositionEncoding(raw json.RawMessage) PositionEncoding {
	var probe struct {
		Capabilities struct {
			General struct {
				PositionEncodings []string `json:"positionEncodings"`
			} `json:"general"`
		} `json:"capabilities"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return EncodingUTF16
	}
	for _, enc := range probe.Capabilities.General.PositionEncodings {
		if enc == "utf-8" {
			return EncodingUTF8
		}
	}
	return EncodingUTF16
}

// go.lsp.dev/protocol is LSP 3.16 and lacks positionEncoding on
// ServerCapabilities, so we use a local shape for the initialize reply.
type serverCapabilities struct {
	TextDocumentSync           any              `json:"textDocumentSync,omitempty"`
	DocumentSymbolProvider     bool             `json:"documentSymbolProvider,omitempty"`
	DocumentFormattingProvider bool             `json:"documentFormattingProvider,omitempty"`
	DefinitionProvider         bool             `json:"definitionProvider,omitempty"`
	PositionEncoding           PositionEncoding `json:"positionEncoding,omitempty"`
}

type initializeResult struct {
	Capabilities serverCapabilities   `json:"capabilities"`
	ServerInfo   *protocol.ServerInfo `json:"serverInfo,omitempty"`
}

func (s *bashServer) initialize(raw json.RawMessage) *initializeResult {
	enc := pickPositionEncoding(raw)
	s.posEncoding.Store(uint32(enc))

	return &initializeResult{
		Capabilities: serverCapabilities{
			TextDocumentSync: &protocol.TextDocumentSyncOptions{
				OpenClose: true,
				Change:    protocol.TextDocumentSyncKindFull,
			},
			DocumentSymbolProvider:     true,
			DocumentFormattingProvider: true,
			DefinitionProvider:         true,
			PositionEncoding:           enc,
		},
		ServerInfo: &protocol.ServerInfo{
			Name:    "gols-bash",
			Version: "0.0.0",
		},
	}
}

func (s *bashServer) encoding() PositionEncoding {
	return PositionEncoding(s.posEncoding.Load())
}

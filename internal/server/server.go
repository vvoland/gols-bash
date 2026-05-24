// SPDX-License-Identifier: GPL-3.0-only

package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// notifyFunc sends a notification to the client. Real connections wire this
// to jsonrpc2.Conn.Notify; tests substitute a recording stub.
type notifyFunc func(ctx context.Context, method string, params interface{}) error

// Config is the runtime configuration passed in from main.
type Config struct {
	In      io.ReadCloser
	Out     io.WriteCloser
	LogFile string
	Verbose bool
}

// Run blocks until the client disconnects or ctx is cancelled.
func Run(ctx context.Context, cfg Config) error {
	logger, closeLog, err := newLogger(cfg)
	if err != nil {
		return err
	}
	defer closeLog()

	stream := jsonrpc2.NewStream(stdio{r: cfg.In, w: cfg.Out})
	conn := jsonrpc2.NewConn(stream)

	srv := &bashServer{
		log:    logger,
		docs:   NewDocumentStore(),
		notify: conn.Notify,
	}
	conn.Go(ctx, srv.handle)

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-conn.Done():
		if err := conn.Err(); err != nil {
			return err
		}
		return nil
	}
}

type bashServer struct {
	log    *slog.Logger
	docs   *DocumentStore
	notify notifyFunc
}

func (s *bashServer) publishDiagnostics(ctx context.Context, u uri.URI, version int32, diags []protocol.Diagnostic) {
	if diags == nil {
		diags = []protocol.Diagnostic{}
	}
	err := s.notify(ctx, protocol.MethodTextDocumentPublishDiagnostics, &protocol.PublishDiagnosticsParams{
		URI:         u,
		Version:     uint32(version),
		Diagnostics: diags,
	})
	if err != nil {
		s.log.Warn("publishDiagnostics failed", "uri", u, "error", err)
	}
}

func (s *bashServer) handle(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	switch req.Method() {
	case protocol.MethodInitialize:
		var p protocol.InitializeParams
		if err := json.Unmarshal(req.Params(), &p); err != nil {
			return reply(ctx, nil, fmt.Errorf("unmarshal initialize: %w", err))
		}
		return reply(ctx, s.initialize(&p), nil)

	case protocol.MethodInitialized:
		s.log.Info("client initialized")
		return reply(ctx, nil, nil)

	case protocol.MethodShutdown:
		s.log.Info("shutdown requested")
		return reply(ctx, nil, nil)

	case protocol.MethodExit:
		s.log.Info("exit")
		return reply(ctx, nil, nil)

	case protocol.MethodTextDocumentDidOpen:
		var p protocol.DidOpenTextDocumentParams
		if err := json.Unmarshal(req.Params(), &p); err != nil {
			return reply(ctx, nil, fmt.Errorf("unmarshal didOpen: %w", err))
		}
		d := s.docs.Open(p.TextDocument)
		s.log.Debug("didOpen", "uri", d.URI, "lang", d.Lang, "len", len(d.Text))
		s.publishDiagnostics(ctx, d.URI, d.Version, parseDiagnostics(d.ParseErr))
		return reply(ctx, nil, nil)

	case protocol.MethodTextDocumentDidChange:
		var p protocol.DidChangeTextDocumentParams
		if err := json.Unmarshal(req.Params(), &p); err != nil {
			return reply(ctx, nil, fmt.Errorf("unmarshal didChange: %w", err))
		}
		if len(p.ContentChanges) == 0 {
			return reply(ctx, nil, nil)
		}
		last := p.ContentChanges[len(p.ContentChanges)-1]
		d, ok := s.docs.Update(p.TextDocument.URI, p.TextDocument.Version, last.Text)
		if !ok {
			s.log.Warn("didChange for unopened document", "uri", p.TextDocument.URI)
			return reply(ctx, nil, nil)
		}
		s.publishDiagnostics(ctx, d.URI, d.Version, parseDiagnostics(d.ParseErr))
		return reply(ctx, nil, nil)

	case protocol.MethodTextDocumentDidClose:
		var p protocol.DidCloseTextDocumentParams
		if err := json.Unmarshal(req.Params(), &p); err != nil {
			return reply(ctx, nil, fmt.Errorf("unmarshal didClose: %w", err))
		}
		s.docs.Close(p.TextDocument.URI)
		s.publishDiagnostics(ctx, p.TextDocument.URI, 0, nil)
		return reply(ctx, nil, nil)

	case protocol.MethodTextDocumentDocumentSymbol:
		var p protocol.DocumentSymbolParams
		if err := json.Unmarshal(req.Params(), &p); err != nil {
			return reply(ctx, nil, fmt.Errorf("unmarshal documentSymbol: %w", err))
		}
		d, ok := s.docs.Get(p.TextDocument.URI)
		if !ok {
			return reply(ctx, []protocol.DocumentSymbol{}, nil)
		}
		syms := documentSymbols(d.AST)
		if syms == nil {
			syms = []protocol.DocumentSymbol{}
		}
		return reply(ctx, syms, nil)

	default:
		return reply(ctx, nil, jsonrpc2.ErrMethodNotFound)
	}
}

func (s *bashServer) initialize(_ *protocol.InitializeParams) *protocol.InitializeResult {
	sync := protocol.TextDocumentSyncKindFull
	return &protocol.InitializeResult{
		Capabilities: protocol.ServerCapabilities{
			TextDocumentSync: &protocol.TextDocumentSyncOptions{
				OpenClose: true,
				Change:    sync,
			},
			DocumentSymbolProvider: true,
		},
		ServerInfo: &protocol.ServerInfo{
			Name:    "gols-bash",
			Version: "0.0.0",
		},
	}
}

func newLogger(cfg Config) (*slog.Logger, func(), error) {
	level := slog.LevelInfo
	if cfg.Verbose {
		level = slog.LevelDebug
	}

	var w io.Writer = os.Stderr
	closeFn := func() {}
	if cfg.LogFile != "" {
		f, err := os.OpenFile(cfg.LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return nil, nil, fmt.Errorf("open log file: %w", err)
		}
		w = f
		closeFn = func() { _ = f.Close() }
	}

	var h slog.Handler
	if cfg.LogFile != "" {
		h = slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level})
	} else {
		h = slog.NewTextHandler(w, &slog.HandlerOptions{Level: level})
	}
	return slog.New(h), closeFn, nil
}

// stdio adapts a separate reader and writer (typically os.Stdin and
// os.Stdout) into the io.ReadWriteCloser shape jsonrpc2 expects.
type stdio struct {
	r io.ReadCloser
	w io.WriteCloser
}

func (s stdio) Read(p []byte) (int, error)  { return s.r.Read(p) }
func (s stdio) Write(p []byte) (int, error) { return s.w.Write(p) }
func (s stdio) Close() error {
	rerr := s.r.Close()
	werr := s.w.Close()
	if rerr != nil {
		return rerr
	}
	return werr
}

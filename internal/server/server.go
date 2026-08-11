// SPDX-License-Identifier: GPL-3.0-only

package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
	"sync/atomic"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
	"mvdan.cc/sh/v3/syntax"

	"grono.dev/gols-bash/internal/analyser"
)

// notifyFunc sends a notification to the client. Real connections wire this
// to jsonrpc2.Conn.Notify; tests substitute a recording stub.
type notifyFunc func(ctx context.Context, method string, params any) error

// ErrExitWithoutShutdown reports the LSP-mandated abnormal exit status.
// The exit notification itself was handled cleanly and should not be logged as
// a server failure.
var ErrExitWithoutShutdown = errors.New("exit received before shutdown")

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

	runCtx, cancel := context.WithCancel(ctx)
	stream := jsonrpc2.NewStream(stdio{r: cfg.In, w: cfg.Out})
	conn := jsonrpc2.NewConn(stream)

	settings := defaultSettings()
	runner := newShellCheckRunner(settings.ShellCheckPath, settings.ShellCheckArguments, logger)
	srv := &bashServer{
		log:             logger,
		docs:            NewDocumentStore(),
		notify:          conn.Notify,
		index:           analyser.NewIndex(),
		codeActions:     make(map[uri.URI][]protocol.CodeAction),
		settings:        settings,
		shellcheck:      runner.lint,
		exit:            make(chan struct{}),
		cancel:          cancel,
		closeConnection: conn.Close,
	}
	runner.encoding = srv.encoding
	conn.Go(runCtx, srv.handle)
	defer func() {
		cancel()
		_ = conn.Close()
		<-conn.Done()
	}()

	exitResult := func() error {
		if srv.abnormalExit.Load() {
			return ErrExitWithoutShutdown
		}
		return nil
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-srv.exit:
		return exitResult()
	case <-conn.Done():
		select {
		case <-srv.exit:
			return exitResult()
		default:
		}
		if err := conn.Err(); err != nil {
			return err
		}
		return nil
	}
}

type bashServer struct {
	log             *slog.Logger
	docs            *DocumentStore
	notify          notifyFunc
	posEncoding     atomic.Uint32
	index           *analyser.Index
	codeActionsMu   sync.RWMutex
	codeActions     map[uri.URI][]protocol.CodeAction
	settingsMu      sync.RWMutex
	settings        serverSettings
	shellcheck      shellcheckFunc
	workspaceRoots  []string
	shutdown        atomic.Bool
	abnormalExit    atomic.Bool
	exitOnce        sync.Once
	exit            chan struct{}
	cancel          context.CancelFunc
	closeConnection func() error
}

func (s *bashServer) signalExit() {
	s.exitOnce.Do(func() {
		if s.exit != nil {
			close(s.exit)
		}
		if s.cancel != nil {
			s.cancel()
		}
		if s.closeConnection != nil {
			_ = s.closeConnection()
		}
	})
}

// indexParse parses src for the workspace scanner. Errors are swallowed
// to nil — the index ignores nil files, matching the "skip on error"
// scanner behaviour.
func (s *bashServer) indexParse(name, src string) *syntax.File {
	f, _ := newParser().Parse(strings.NewReader(src), name)
	return f
}

// scanWorkspaces populates the index from every configured workspace root.
// Errors on individual files have already been logged by the scanner; a
// root-level failure (root unreadable) is logged at warn and ignored so
// the rest of the server keeps working.
func (s *bashServer) scanWorkspaces(ctx context.Context) {
	s.settingsMu.RLock()
	enabled := s.settings.WorkspaceScanEnabled
	s.settingsMu.RUnlock()
	if !enabled {
		s.log.Info("workspace scan disabled")
		return
	}
	for _, root := range s.workspaceRoots {
		if err := analyser.ScanWorkspace(ctx, root, s.index, s.indexParse); err != nil {
			s.log.Warn("workspace scan failed", "root", root, "error", err)
		}
	}
	s.log.Info("workspace scan complete", "files", s.index.Len())
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
	if s.shutdown.Load() && req.Method() != protocol.MethodExit {
		return reply(ctx, nil, jsonrpc2.ErrInvalidRequest)
	}
	switch req.Method() {
	case protocol.MethodInitialize:
		return reply(ctx, s.initialize(req.Params()), nil)

	case protocol.MethodInitialized:
		s.log.Info("client initialized", "workspaceRoots", s.workspaceRoots)
		go s.scanWorkspaces(context.Background())
		return reply(ctx, nil, nil)

	case protocol.MethodWorkspaceDidChangeConfiguration:
		s.handleDidChangeConfiguration(ctx, req.Params())
		return reply(ctx, nil, nil)

	case protocol.MethodShutdown:
		s.log.Info("shutdown requested")
		err := reply(ctx, nil, nil)
		if err == nil {
			s.shutdown.Store(true)
		}
		return err

	case protocol.MethodExit:
		s.log.Info("exit")
		err := reply(ctx, nil, nil)
		s.abnormalExit.Store(!s.shutdown.Load())
		s.signalExit()
		return err

	case protocol.MethodTextDocumentDidOpen:
		var p protocol.DidOpenTextDocumentParams
		if err := json.Unmarshal(req.Params(), &p); err != nil {
			return reply(ctx, nil, fmt.Errorf("unmarshal didOpen: %w", err))
		}
		d := s.docs.Open(p.TextDocument)
		s.log.Debug("didOpen", "uri", d.URI, "lang", d.Lang, "len", len(d.Text))
		s.index.AddOrReplace(d.URI, d.AST)
		s.publishDiagnostics(ctx, d.URI, d.Version, s.documentDiagnostics(ctx, d))
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
		s.index.AddOrReplace(d.URI, d.AST)
		s.publishDiagnostics(ctx, d.URI, d.Version, s.documentDiagnostics(ctx, d))
		return reply(ctx, nil, nil)

	case protocol.MethodTextDocumentDidClose:
		var p protocol.DidCloseTextDocumentParams
		if err := json.Unmarshal(req.Params(), &p); err != nil {
			return reply(ctx, nil, fmt.Errorf("unmarshal didClose: %w", err))
		}
		s.docs.Close(p.TextDocument.URI)
		s.setCodeActions(p.TextDocument.URI, nil)
		s.publishDiagnostics(ctx, p.TextDocument.URI, 0, nil)
		return reply(ctx, nil, nil)

	case protocol.MethodTextDocumentCodeAction:
		var p protocol.CodeActionParams
		if err := json.Unmarshal(req.Params(), &p); err != nil {
			return reply(ctx, nil, fmt.Errorf("unmarshal codeAction: %w", err))
		}
		return reply(ctx, s.codeActionsFor(p), nil)

	case protocol.MethodTextDocumentCompletion:
		var p protocol.CompletionParams
		if err := json.Unmarshal(req.Params(), &p); err != nil {
			return reply(ctx, nil, fmt.Errorf("unmarshal completion: %w", err))
		}
		d, _ := s.docs.Get(p.TextDocument.URI)
		items := s.completion(d, p.Position)
		if items == nil {
			items = []protocol.CompletionItem{}
		}
		return reply(ctx, &protocol.CompletionList{Items: items}, nil)

	case protocol.MethodCompletionItemResolve:
		var p protocol.CompletionItem
		if err := json.Unmarshal(req.Params(), &p); err != nil {
			return reply(ctx, nil, fmt.Errorf("unmarshal completion resolve: %w", err))
		}
		return reply(ctx, &p, nil)

	case protocol.MethodTextDocumentFormatting:
		var p protocol.DocumentFormattingParams
		if err := json.Unmarshal(req.Params(), &p); err != nil {
			return reply(ctx, nil, fmt.Errorf("unmarshal formatting: %w", err))
		}
		d, _ := s.docs.Get(p.TextDocument.URI)
		edits, err := s.formatDocument(d, p.Options)
		if err != nil {
			return reply(ctx, nil, err)
		}
		if edits == nil {
			edits = []protocol.TextEdit{}
		}
		return reply(ctx, edits, nil)

	case protocol.MethodTextDocumentRename:
		var p protocol.RenameParams
		if err := json.Unmarshal(req.Params(), &p); err != nil {
			return reply(ctx, nil, fmt.Errorf("unmarshal rename: %w", err))
		}
		d, _ := s.docs.Get(p.TextDocument.URI)
		edit, err := s.rename(d, p.Position, p.NewName)
		return reply(ctx, edit, err)

	case protocol.MethodTextDocumentPrepareRename:
		var p protocol.PrepareRenameParams
		if err := json.Unmarshal(req.Params(), &p); err != nil {
			return reply(ctx, nil, fmt.Errorf("unmarshal prepareRename: %w", err))
		}
		d, _ := s.docs.Get(p.TextDocument.URI)
		return reply(ctx, s.prepareRename(d, p.Position), nil)

	case protocol.MethodTextDocumentDocumentHighlight:
		var p protocol.DocumentHighlightParams
		if err := json.Unmarshal(req.Params(), &p); err != nil {
			return reply(ctx, nil, fmt.Errorf("unmarshal highlight: %w", err))
		}
		d, _ := s.docs.Get(p.TextDocument.URI)
		hits := s.highlight(d, p.Position)
		if hits == nil {
			hits = []protocol.DocumentHighlight{}
		}
		return reply(ctx, hits, nil)

	case protocol.MethodTextDocumentReferences:
		var p protocol.ReferenceParams
		if err := json.Unmarshal(req.Params(), &p); err != nil {
			return reply(ctx, nil, fmt.Errorf("unmarshal references: %w", err))
		}
		d, _ := s.docs.Get(p.TextDocument.URI)
		locs := s.references(d, p.Position, p.Context.IncludeDeclaration)
		if locs == nil {
			locs = []protocol.Location{}
		}
		return reply(ctx, locs, nil)

	case protocol.MethodTextDocumentHover:
		var p protocol.HoverParams
		if err := json.Unmarshal(req.Params(), &p); err != nil {
			return reply(ctx, nil, fmt.Errorf("unmarshal hover: %w", err))
		}
		d, _ := s.docs.Get(p.TextDocument.URI)
		return reply(ctx, s.hover(d, p.Position), nil)

	case protocol.MethodTextDocumentDefinition:
		var p protocol.DefinitionParams
		if err := json.Unmarshal(req.Params(), &p); err != nil {
			return reply(ctx, nil, fmt.Errorf("unmarshal definition: %w", err))
		}
		d, _ := s.docs.Get(p.TextDocument.URI)
		locs := s.definition(d, p.Position)
		if locs == nil {
			locs = []protocol.Location{}
		}
		return reply(ctx, locs, nil)

	case protocol.MethodTextDocumentDocumentSymbol:
		var p protocol.DocumentSymbolParams
		if err := json.Unmarshal(req.Params(), &p); err != nil {
			return reply(ctx, nil, fmt.Errorf("unmarshal documentSymbol: %w", err))
		}
		d, ok := s.docs.Get(p.TextDocument.URI)
		if !ok {
			return reply(ctx, []protocol.DocumentSymbol{}, nil)
		}
		syms := s.documentSymbols(d)
		if syms == nil {
			syms = []protocol.DocumentSymbol{}
		}
		return reply(ctx, syms, nil)

	case protocol.MethodWorkspaceSymbol:
		var p protocol.WorkspaceSymbolParams
		if err := json.Unmarshal(req.Params(), &p); err != nil {
			return reply(ctx, nil, fmt.Errorf("unmarshal workspaceSymbol: %w", err))
		}
		syms := s.workspaceSymbols(p.Query)
		if syms == nil {
			syms = []protocol.SymbolInformation{}
		}
		return reply(ctx, syms, nil)

	default:
		return reply(ctx, nil, jsonrpc2.ErrMethodNotFound)
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

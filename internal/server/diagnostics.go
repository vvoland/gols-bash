// SPDX-License-Identifier: GPL-3.0-only

package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"strconv"
	"sync/atomic"

	"go.lsp.dev/protocol"
	"mvdan.cc/sh/v3/syntax"
)

type diagnosticResult struct {
	Diagnostics []protocol.Diagnostic
	CodeActions []protocol.CodeAction
}

type shellcheckFunc func(context.Context, *Document) (diagnosticResult, error)

func (s *bashServer) documentDiagnostics(ctx context.Context, d *Document) []protocol.Diagnostic {
	diags := s.parseDiagnostics(d.Text, d.ParseErr)
	s.settingsMu.RLock()
	shellcheck := s.shellcheck
	s.settingsMu.RUnlock()
	if len(diags) > 0 || shellcheck == nil {
		s.setCodeActions(d.URI, nil)
		return diags
	}
	lintResult, err := shellcheck(ctx, d)
	if err != nil {
		s.log.Warn("shellcheck failed", "uri", d.URI, "error", err)
		s.setCodeActions(d.URI, nil)
		return diags
	}
	s.setCodeActions(d.URI, lintResult.CodeActions)
	return append(diags, lintResult.Diagnostics...)
}

type shellCheckRunner struct {
	path     string
	args     []string
	log      *slog.Logger
	disabled atomic.Bool
}

func newShellCheckRunner(path string, args []string, log *slog.Logger) *shellCheckRunner {
	return &shellCheckRunner{path: path, args: args, log: log}
}

func (r *shellCheckRunner) lint(ctx context.Context, d *Document) (diagnosticResult, error) {
	if r.disabled.Load() {
		return diagnosticResult{}, nil
	}
	args := append([]string{"--format=json1"}, r.args...)
	args = append(args, "-")
	cmd := exec.CommandContext(ctx, r.path, args...)
	cmd.Stdin = bytes.NewBufferString(d.Text)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if errors.Is(err, exec.ErrNotFound) {
		r.disabled.Store(true)
		r.log.Warn("shellcheck not found; disabling shellcheck diagnostics", "path", r.path)
		return diagnosticResult{}, nil
	}
	if stdout.Len() == 0 {
		if err != nil {
			return diagnosticResult{}, fmt.Errorf("%w: %s", err, stderr.String())
		}
		return diagnosticResult{}, nil
	}
	lintResult, parseErr := shellCheckDiagnostics(d.URI, stdout.Bytes())
	if parseErr != nil {
		return diagnosticResult{}, parseErr
	}
	return lintResult, nil
}

type shellCheckOutput struct {
	Comments []shellCheckComment `json:"comments"`
}

type shellCheckComment struct {
	Line      int    `json:"line"`
	EndLine   int    `json:"endLine"`
	Column    int    `json:"column"`
	EndColumn int    `json:"endColumn"`
	Level     string `json:"level"`
	Code      int    `json:"code"`
	Message   string `json:"message"`
	Fix       *struct {
		Replacements []shellCheckReplacement `json:"replacements"`
	} `json:"fix"`
}

type shellCheckReplacement struct {
	Line        int    `json:"line"`
	EndLine     int    `json:"endLine"`
	Column      int    `json:"column"`
	EndColumn   int    `json:"endColumn"`
	Replacement string `json:"replacement"`
}

func shellCheckDiagnostics(u protocol.DocumentURI, raw []byte) (diagnosticResult, error) {
	var out shellCheckOutput
	if err := json.Unmarshal(raw, &out); err != nil {
		return diagnosticResult{}, err
	}
	diags := make([]protocol.Diagnostic, 0, len(out.Comments))
	actions := make([]protocol.CodeAction, 0, len(out.Comments))
	for _, c := range out.Comments {
		code := "SC" + strconv.Itoa(c.Code)
		diag := protocol.Diagnostic{
			Range:           shellCheckRange(c),
			Severity:        shellCheckSeverity(c.Level),
			Code:            code,
			CodeDescription: &protocol.CodeDescription{Href: protocol.URI("https://www.shellcheck.net/wiki/" + code)},
			Source:          "shellcheck",
			Message:         c.Message,
		}
		diags = append(diags, diag)
		if action := shellCheckCodeAction(u, code, diag, c); action != nil {
			actions = append(actions, *action)
		}
	}
	return diagnosticResult{Diagnostics: diags, CodeActions: actions}, nil
}

func shellCheckCodeAction(u protocol.DocumentURI, code string, diag protocol.Diagnostic, c shellCheckComment) *protocol.CodeAction {
	if c.Fix == nil || len(c.Fix.Replacements) == 0 {
		return nil
	}
	edits := make([]protocol.TextEdit, 0, len(c.Fix.Replacements))
	for _, r := range c.Fix.Replacements {
		edits = append(edits, protocol.TextEdit{
			Range: protocol.Range{
				Start: shellCheckPosition(r.Line, r.Column),
				End:   shellCheckPosition(r.EndLine, r.EndColumn),
			},
			NewText: r.Replacement,
		})
	}
	return &protocol.CodeAction{
		Title:       "Apply fix for " + code,
		Kind:        protocol.QuickFix,
		Diagnostics: []protocol.Diagnostic{diag},
		Edit: &protocol.WorkspaceEdit{
			Changes: map[protocol.DocumentURI][]protocol.TextEdit{u: edits},
		},
	}
}

func shellCheckSeverity(level string) protocol.DiagnosticSeverity {
	switch level {
	case "error":
		return protocol.DiagnosticSeverityError
	case "warning":
		return protocol.DiagnosticSeverityWarning
	case "info":
		return protocol.DiagnosticSeverityInformation
	case "style":
		return protocol.DiagnosticSeverityHint
	default:
		return protocol.DiagnosticSeverityWarning
	}
}

func shellCheckRange(c shellCheckComment) protocol.Range {
	start := shellCheckPosition(c.Line, c.Column)
	endLine, endColumn := c.EndLine, c.EndColumn
	if endLine == 0 {
		endLine = c.Line
	}
	if endColumn == 0 {
		endColumn = c.Column + 1
	}
	end := shellCheckPosition(endLine, endColumn)
	if end.Line < start.Line || end.Line == start.Line && end.Character < start.Character {
		end = start
	}
	return protocol.Range{Start: start, End: end}
}

func shellCheckPosition(line, column int) protocol.Position {
	if line <= 0 {
		line = 1
	}
	if column <= 0 {
		column = 1
	}
	return protocol.Position{Line: uint32(line - 1), Character: uint32(column - 1)}
}

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

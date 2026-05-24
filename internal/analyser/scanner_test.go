// SPDX-License-Identifier: GPL-3.0-only

package analyser

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.lsp.dev/uri"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
	"mvdan.cc/sh/v3/syntax"
)

func parseBash(name, src string) *syntax.File {
	f, _ := syntax.NewParser().Parse(strings.NewReader(src), name)
	return f
}

func TestScanWorkspaceIndexesShellFiles(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "a.sh"), "greet() { echo hi; }\n")
	mustWrite(t, filepath.Join(root, "b.bash"), "farewell() { echo bye; }\n")
	mustWrite(t, filepath.Join(root, "ignore.md"), "not shell\n")
	_ = os.MkdirAll(filepath.Join(root, "node_modules"), 0o755)
	mustWrite(t, filepath.Join(root, "node_modules", "x.sh"), "should_skip() { :; }\n")

	idx := NewIndex()
	err := ScanWorkspace(context.Background(), root, idx, parseBash)
	assert.NilError(t, err)
	assert.Assert(t, cmp.Equal(idx.Len(), 2))
	assert.Assert(t, idx.Get(uri.File(filepath.Join(root, "a.sh"))) != nil)
}

func TestAllUsagesCrossFile(t *testing.T) {
	idx := NewIndex()
	idx.AddOrReplace(uri.File("/tmp/a.sh"), parseBash("a.sh", "greet() { :; }\n"))
	idx.AddOrReplace(uri.File("/tmp/b.sh"), parseBash("b.sh", "greet\nfarewell\n"))

	hits := idx.AllUsages("greet", true)
	assert.Assert(t, cmp.Len(hits, 2))
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	assert.NilError(t, os.WriteFile(path, []byte(content), 0o644))
}

// SPDX-License-Identifier: GPL-3.0-only

package analyser

import (
	"context"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"go.lsp.dev/uri"
	"mvdan.cc/sh/v3/syntax"
)

// shellExts holds the extensions we treat as shell without consulting the
// shebang. Mirrors upstream bash-language-server's default glob.
var shellExts = map[string]struct{}{
	".sh": {}, ".bash": {}, ".zsh": {}, ".dash": {}, ".ksh": {}, ".bats": {},
}

// IsShellPath reports whether path looks like a shell script. Files with a
// known extension are accepted outright; extensionless files are accepted
// when their shebang names a shell.
func IsShellPath(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	if _, ok := shellExts[ext]; ok {
		return true
	}
	if ext != "" {
		return false
	}
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	buf := make([]byte, 200)
	n, _ := f.Read(buf)
	first := string(buf[:n])
	if !strings.HasPrefix(first, "#!") {
		return false
	}
	if i := strings.IndexByte(first, '\n'); i >= 0 {
		first = first[:i]
	}
	return strings.Contains(first, "sh")
}

// ScanWorkspace walks root indexing every shell script it finds. Errors
// on individual files are logged and skipped — one broken file should not
// stop the index from coming up.
func ScanWorkspace(ctx context.Context, root string, idx *Index, parse func(name, src string) *syntax.File) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			slog.Debug("walk error", "path", path, "err", err)
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if d.IsDir() {
			if isSkippedDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !IsShellPath(path) {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			slog.Debug("read error", "path", path, "err", err)
			return nil
		}
		file := parse(path, string(src))
		idx.AddOrReplace(uri.File(path), file)
		return nil
	})
}

func isSkippedDir(name string) bool {
	switch name {
	case ".git", ".hg", ".svn", "node_modules", "vendor",
		".venv", "venv", "__pycache__", ".bun", "dist", "build", "target":
		return true
	}
	return false
}

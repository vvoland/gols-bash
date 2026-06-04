// SPDX-License-Identifier: GPL-3.0-only

package server

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type editorConfigFormat struct {
	IndentStyle string
	IndentSize  string
}

func editorConfigIndent(path string) *uint {
	if path == "" {
		return nil
	}
	files := editorConfigFiles(filepath.Dir(path))
	if len(files) == 0 {
		return nil
	}
	var cfg editorConfigFormat
	for i := len(files) - 1; i >= 0; i-- {
		cfg.apply(editorConfigFormatForFile(files[i], path))
	}
	switch cfg.IndentStyle {
	case "tab":
		var indent uint
		return &indent
	case "space":
		if n, err := strconv.Atoi(cfg.IndentSize); err == nil && n > 0 {
			u := uint(n)
			return &u
		}
	}
	return nil
}

func editorConfigFiles(dir string) []string {
	var files []string
	for {
		path := filepath.Join(dir, ".editorconfig")
		if _, err := os.Stat(path); err == nil {
			files = append(files, path)
			if editorConfigRoot(path) {
				break
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return files
}

func editorConfigRoot(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	for s.Scan() {
		key, value, ok := editorConfigKeyValue(s.Text())
		if ok && key == "root" {
			return value == "true"
		}
	}
	return false
}

func editorConfigFormatForFile(configPath, targetPath string) editorConfigFormat {
	f, err := os.Open(configPath)
	if err != nil {
		return editorConfigFormat{}
	}
	defer f.Close()
	baseDir := filepath.Dir(configPath)
	matches := false
	var cfg editorConfigFormat
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			matches = editorConfigSectionMatches(line[1:len(line)-1], baseDir, targetPath)
			continue
		}
		key, value, ok := editorConfigKeyValue(line)
		if !ok || !matches {
			continue
		}
		switch key {
		case "indent_style":
			cfg.IndentStyle = value
		case "indent_size":
			cfg.IndentSize = value
		}
	}
	return cfg
}

func editorConfigKeyValue(line string) (string, string, bool) {
	line = strings.TrimSpace(line)
	if i := strings.IndexByte(line, '#'); i >= 0 {
		line = strings.TrimSpace(line[:i])
	}
	if i := strings.IndexByte(line, ';'); i >= 0 {
		line = strings.TrimSpace(line[:i])
	}
	key, value, ok := strings.Cut(line, "=")
	if !ok {
		return "", "", false
	}
	key = strings.ToLower(strings.TrimSpace(key))
	value = strings.ToLower(strings.TrimSpace(value))
	return key, value, key != ""
}

func editorConfigSectionMatches(pattern, baseDir, targetPath string) bool {
	rel, err := filepath.Rel(baseDir, targetPath)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return false
	}
	rel = filepath.ToSlash(rel)
	pattern = filepath.ToSlash(pattern)
	if !strings.Contains(pattern, "/") {
		ok, _ := filepath.Match(pattern, filepath.Base(rel))
		return ok
	}
	ok, _ := filepath.Match(pattern, rel)
	return ok
}

func (c *editorConfigFormat) apply(next editorConfigFormat) {
	if next.IndentStyle != "" {
		c.IndentStyle = next.IndentStyle
	}
	if next.IndentSize != "" {
		c.IndentSize = next.IndentSize
	}
}

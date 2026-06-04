// SPDX-License-Identifier: GPL-3.0-only

package data

import (
	"os"
	"path/filepath"
	"sort"
)

// ExecutablesFromPath returns executable regular files found in a PATH value.
func ExecutablesFromPath(pathValue string) []string {
	if pathValue == "" {
		return nil
	}
	seen := map[string]struct{}{}
	for _, dir := range filepath.SplitList(pathValue) {
		if dir == "" {
			continue
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.Type()&os.ModeType != 0 {
				continue
			}
			info, err := entry.Info()
			if err != nil || !info.Mode().IsRegular() || info.Mode()&0o111 == 0 {
				continue
			}
			seen[entry.Name()] = struct{}{}
		}
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

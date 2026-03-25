// Copyright 2026 — see LICENSE file for terms.
package config

import (
	"os"
	"path/filepath"
)

// PathAllowPatterns returns glob patterns that allow reading files directly
// within each directory in $PATH. Each pattern is of the form "<dir>/*"
// (single-level, non-recursive). Directories are resolved through
// symlinks and deduplicated; non-existent directories are skipped.
func PathAllowPatterns() []string {
	pathVal := os.Getenv("PATH")
	if pathVal == "" {
		return nil
	}

	dirs := filepath.SplitList(pathVal)
	seen := make(map[string]struct{}, len(dirs))
	var patterns []string

	for _, dir := range dirs {
		if dir == "" {
			continue
		}
		resolved := ResolvePath(dir)
		if _, dup := seen[resolved]; dup {
			continue
		}
		seen[resolved] = struct{}{}

		// Only include directories that exist.
		info, err := os.Stat(resolved)
		if err != nil || !info.IsDir() {
			continue
		}

		patterns = append(patterns, resolved+"/*")
	}

	return patterns
}

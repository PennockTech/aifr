// Copyright 2026 — see LICENSE file for terms.
package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"

	"go.pennock.tech/aifr/pkg/protocol"
)

// PathfindParams controls path-based command searching.
type PathfindParams struct {
	SearchList string // "envvar:PATH" (default), "envvar:CLASSPATH", "dirlist:/a:/b"
}

// Pathfind searches for a command across a search list, reporting all
// matches with masking information. The command may contain glob
// metacharacters (* ? [ ) for single-level matching.
func (e *Engine) Pathfind(command string, params PathfindParams) (*protocol.PathfindResponse, error) {
	if command == "" {
		return nil, fmt.Errorf("command name is required")
	}

	spec := params.SearchList
	if spec == "" {
		spec = "envvar:PATH"
	}

	dirs, err := parseSearchList(spec)
	if err != nil {
		return nil, err
	}

	isGlob := strings.ContainsAny(command, "*?[")

	resp := &protocol.PathfindResponse{
		Command:    command,
		SearchSpec: spec,
	}

	// Track first match per filename for masking.
	firstMatch := make(map[string]string) // name → first full path

	// Deduplicate resolved directories while preserving order.
	seen := make(map[string]struct{}, len(dirs))

	for _, dir := range dirs {
		resolved := resolveDir(dir)
		if _, dup := seen[resolved]; dup {
			continue
		}
		seen[resolved] = struct{}{}

		i := len(resp.SearchList)
		resp.SearchList = append(resp.SearchList, resolved)

		entries, err := os.ReadDir(resolved)
		if err != nil {
			continue
		}

		for _, de := range entries {
			name := de.Name()

			var matched bool
			if isGlob {
				matched, _ = doublestar.Match(command, name)
			} else {
				matched = name == command
			}
			if !matched {
				continue
			}

			fullPath := filepath.Join(resolved, name)

			// Check access on the individual file (deny/sensitive may block it).
			if err := e.checker.Check(fullPath); err != nil {
				continue
			}

			info, err := de.Info()
			if err != nil {
				continue
			}

			entry := protocol.PathfindEntry{
				Path:       fullPath,
				Dir:        resolved,
				Name:       name,
				Mode:       info.Mode().String(),
				Size:       info.Size(),
				Executable: info.Mode()&0o111 != 0,
				DirIndex:   i,
			}

			if first, exists := firstMatch[name]; exists {
				entry.Masked = true
				entry.MaskedBy = first
			} else {
				firstMatch[name] = fullPath
			}

			resp.Entries = append(resp.Entries, entry)
		}
	}

	resp.Total = len(resp.Entries)
	resp.Complete = true
	return resp, nil
}

// parseSearchList parses a search list spec into a list of directories.
// Supported forms:
//   - "envvar:PATH"     — split the named environment variable
//   - "envvar:CLASSPATH" — any env var works
//   - "dirlist:/a:/b:/c" — explicit list using os.PathListSeparator
//   - ""                 — defaults to "envvar:PATH"
func parseSearchList(spec string) ([]string, error) {
	if spec == "" {
		spec = "envvar:PATH"
	}

	if strings.HasPrefix(spec, "envvar:") {
		varName := spec[len("envvar:"):]
		if varName == "" {
			return nil, fmt.Errorf("empty variable name in search_list spec %q", spec)
		}
		val := os.Getenv(varName)
		if val == "" {
			return nil, fmt.Errorf("environment variable %q is empty or unset", varName)
		}
		return filepath.SplitList(val), nil
	}

	if strings.HasPrefix(spec, "dirlist:") {
		raw := spec[len("dirlist:"):]
		if raw == "" {
			return nil, fmt.Errorf("empty directory list in search_list spec %q", spec)
		}
		return filepath.SplitList(raw), nil
	}

	return nil, fmt.Errorf("invalid search_list spec %q: must start with envvar: or dirlist:", spec)
}

// resolveDir resolves a directory path to an absolute, symlink-resolved form.
func resolveDir(dir string) string {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return dir
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return abs
	}
	return resolved
}

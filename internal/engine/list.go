// Copyright 2026 — see LICENSE file for terms.
package engine

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/bmatcuk/doublestar/v4"

	"go.pennock.tech/aifr/pkg/protocol"
)

// ListParams controls directory listing.
type ListParams struct {
	Depth      int       // 0 = immediate children, -1 = unlimited, N = N levels
	Pattern    string    // glob filter on entry name
	Type       string    // "f" = files, "d" = dirs, "l" = symlinks, "" = all
	Sort       SortOrder // sort entries (default: none = filesystem order)
	Descending bool      // reverse sort order
	Limit      int       // 0 = no limit, N = return first N entries after sorting
}

// List returns directory entries.
func (e *Engine) List(path string, params ListParams) (*protocol.ListResponse, error) {
	resolved, err := e.checkAccess(path)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(resolved)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, protocol.NewPathError(protocol.ErrNotFound, path, "path does not exist")
		}
		return nil, protocol.NewPathError(protocol.ErrNotFound, path, err.Error())
	}

	if !info.IsDir() {
		return nil, protocol.NewPathError(protocol.ErrIsDirectory, path,
			"list requires a directory, got a file")
	}

	resp := &protocol.ListResponse{
		Path:   resolved,
		Source: "filesystem",
	}

	entries, err := e.walkDir(resolved, resolved, params, 0)
	if err != nil {
		return nil, err
	}

	SortEntries(entries, params.Sort, params.Descending)
	if params.Limit > 0 && len(entries) > params.Limit {
		entries = entries[:params.Limit]
	}

	resp.Entries = entries
	resp.Total = len(entries)
	resp.Complete = true
	return resp, nil
}

// walkDir recursively walks a directory respecting depth and filters.
func (e *Engine) walkDir(root, dir string, params ListParams, currentDepth int) ([]protocol.StatEntry, error) {
	dirEntries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading directory %q: %w", dir, err)
	}

	var entries []protocol.StatEntry

	for _, de := range dirEntries {
		name := de.Name()
		fullPath := filepath.Join(dir, name)

		// Pattern filter.
		if params.Pattern != "" {
			matched, _ := doublestar.Match(params.Pattern, name)
			if !matched {
				// For recursive listing, still descend into dirs even if name doesn't match.
				if de.IsDir() && (params.Depth < 0 || currentDepth < params.Depth) {
					sub, err := e.walkDir(root, fullPath, params, currentDepth+1)
					if err != nil {
						continue // skip unreadable dirs
					}
					entries = append(entries, sub...)
				}
				continue
			}
		}

		// Type filter.
		entryType := entryTypeString(de)
		if params.Type != "" && !matchesType(params.Type, entryType) {
			// Still descend for recursion.
			if de.IsDir() && (params.Depth < 0 || currentDepth < params.Depth) {
				sub, err := e.walkDir(root, fullPath, params, currentDepth+1)
				if err != nil {
					continue
				}
				entries = append(entries, sub...)
			}
			continue
		}

		// Check access control.
		if err := e.checker.Check(fullPath); err != nil {
			continue // silently skip inaccessible entries
		}

		info, err := de.Info()
		if err != nil {
			continue
		}

		entry := protocol.StatEntry{
			Name: name,
			Path: fullPath,
			Type: entryType,
			Size: info.Size(),
			Mode: info.Mode().String(),
		}
		if !info.ModTime().IsZero() {
			entry.ModTime = info.ModTime().UTC().Format("2006-01-02T15:04:05Z")
		}
		entries = append(entries, entry)

		// Recurse into directories.
		if de.IsDir() && (params.Depth < 0 || currentDepth < params.Depth) {
			sub, err := e.walkDir(root, fullPath, params, currentDepth+1)
			if err != nil {
				continue
			}
			entries = append(entries, sub...)
		}
	}

	return entries, nil
}

func entryTypeString(de fs.DirEntry) string {
	if de.Type()&os.ModeSymlink != 0 {
		return "symlink"
	}
	if de.IsDir() {
		return "dir"
	}
	return "file"
}

func matchesType(filter, entryType string) bool {
	switch filter {
	case "f":
		return entryType == "file"
	case "d":
		return entryType == "dir"
	case "l":
		return entryType == "symlink"
	default:
		return true
	}
}

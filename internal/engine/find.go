// Copyright 2026 — see LICENSE file for terms.
package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/bmatcuk/doublestar/v4"

	"go.pennock.tech/aifr/pkg/protocol"
)

// FindParams controls file/path searching.
type FindParams struct {
	Name       string        // glob on filename
	Path       string        // glob on full relative path
	Type       string        // "f", "d", "l", or ""
	MaxDepth   int           // -1 = unlimited, 0 = root only
	MinSize    int64         // 0 = no minimum
	MaxSize    int64         // 0 = no maximum
	NewerThan  time.Duration // 0 = no filter
	Sort       SortOrder     // sort results (default: none = walk order)
	Descending bool          // reverse sort order
	Limit      int           // 0 = no limit, N = return first N results after sorting
}

// Find locates files matching the given criteria.
func (e *Engine) Find(path string, params FindParams) (*protocol.FindResponse, error) {
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
			fmt.Sprintf("find requires a directory, got %q", path))
	}

	resp := &protocol.FindResponse{
		Root:   resolved,
		Source: "filesystem",
	}

	now := time.Now()
	e.findDir(resolved, resolved, params, 0, now, resp)

	SortFindEntries(resp.Entries, params.Sort, params.Descending)
	if params.Limit > 0 && len(resp.Entries) > params.Limit {
		resp.Entries = resp.Entries[:params.Limit]
	}

	resp.Total = len(resp.Entries)
	resp.Complete = true
	return resp, nil
}

// findDir recursively searches for entries.
func (e *Engine) findDir(root, dir string, params FindParams, currentDepth int, now time.Time, resp *protocol.FindResponse) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	for _, de := range entries {
		fullPath := filepath.Join(dir, de.Name())

		info, err := de.Info()
		if err != nil {
			continue
		}

		// Check access.
		if err := e.checker.Check(fullPath); err != nil {
			continue
		}

		entryType := entryTypeString(de)

		// Type filter.
		if params.Type != "" && !matchesType(params.Type, entryType) {
			// Still recurse into dirs.
			if de.IsDir() && (params.MaxDepth < 0 || currentDepth < params.MaxDepth) {
				e.findDir(root, fullPath, params, currentDepth+1, now, resp)
			}
			continue
		}

		// Name glob filter.
		if params.Name != "" {
			matched, _ := doublestar.Match(params.Name, de.Name())
			if !matched {
				if de.IsDir() && (params.MaxDepth < 0 || currentDepth < params.MaxDepth) {
					e.findDir(root, fullPath, params, currentDepth+1, now, resp)
				}
				continue
			}
		}

		// Path glob filter.
		if params.Path != "" {
			relPath, _ := filepath.Rel(root, fullPath)
			matched, _ := doublestar.Match(params.Path, relPath)
			if !matched {
				if de.IsDir() && (params.MaxDepth < 0 || currentDepth < params.MaxDepth) {
					e.findDir(root, fullPath, params, currentDepth+1, now, resp)
				}
				continue
			}
		}

		// Size filters (files only).
		if entryType == "file" {
			if params.MinSize > 0 && info.Size() < params.MinSize {
				continue
			}
			if params.MaxSize > 0 && info.Size() > params.MaxSize {
				continue
			}
		}

		// Newer-than filter.
		if params.NewerThan > 0 {
			cutoff := now.Add(-params.NewerThan)
			if info.ModTime().Before(cutoff) {
				if de.IsDir() && (params.MaxDepth < 0 || currentDepth < params.MaxDepth) {
					e.findDir(root, fullPath, params, currentDepth+1, now, resp)
				}
				continue
			}
		}

		resp.Entries = append(resp.Entries, protocol.FindEntry{
			Path: fullPath,
			Type: entryType,
			Size: info.Size(),
			Mode: info.Mode().String(),
		})

		// Recurse into directories.
		if de.IsDir() && (params.MaxDepth < 0 || currentDepth < params.MaxDepth) {
			e.findDir(root, fullPath, params, currentDepth+1, now, resp)
		}
	}
}

// Copyright 2026 — see LICENSE file for terms.
package engine

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"

	"github.com/bmatcuk/doublestar/v4"

	"go.pennock.tech/aifr/pkg/protocol"
)

const (
	// DefaultMaxMatches caps search results.
	DefaultMaxMatches = 500
)

// SearchParams controls content search.
type SearchParams struct {
	IsRegexp   bool
	IgnoreCase bool
	Context    int    // context lines before/after
	MaxMatches int    // 0 = use default
	Include    string // glob for files to include
	Exclude    string // glob for files to exclude
	Offset     int    // skip first N matches (for pagination)
}

// Search finds content matches within a directory tree.
func (e *Engine) Search(pattern, path string, params SearchParams) (*protocol.SearchResponse, error) {
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

	maxMatches := params.MaxMatches
	if maxMatches <= 0 {
		maxMatches = DefaultMaxMatches
	}

	// Compile the pattern.
	var re *regexp.Regexp
	if params.IsRegexp {
		flags := ""
		if params.IgnoreCase {
			flags = "(?i)"
		}
		re, err = regexp.Compile(flags + pattern)
		if err != nil {
			return nil, protocol.NewError("INVALID_PATTERN", "invalid regexp: "+err.Error())
		}
	} else {
		escaped := regexp.QuoteMeta(pattern)
		if params.IgnoreCase {
			escaped = "(?i)" + escaped
		}
		re, err = regexp.Compile(escaped)
		if err != nil {
			return nil, protocol.NewError("INVALID_PATTERN", "cannot compile pattern: "+err.Error())
		}
	}

	resp := &protocol.SearchResponse{
		Pattern:  pattern,
		IsRegexp: params.IsRegexp,
		Root:     resolved,
		Source:   "filesystem",
	}

	if info.IsDir() {
		e.searchDir(resolved, resolved, re, params, maxMatches, resp)
	} else {
		e.searchFile(resolved, resolved, re, params, maxMatches, resp)
	}

	resp.TotalMatches = len(resp.Matches)
	resp.Truncated = resp.TotalMatches >= maxMatches
	resp.Complete = !resp.Truncated

	if !resp.Complete {
		tok, err := e.EncodeListContinuation(&ListContinuationToken{
			Tool:   "search",
			Path:   resolved,
			Offset: params.Offset + len(resp.Matches),
			Limit:  maxMatches,
		})
		if err != nil {
			return nil, err
		}
		resp.Continuation = tok
	}

	return resp, nil
}

// searchDir recursively searches files in a directory.
func (e *Engine) searchDir(root, dir string, re *regexp.Regexp, params SearchParams, maxMatches int, resp *protocol.SearchResponse) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if len(resp.Matches) >= maxMatches {
			return
		}

		fullPath := filepath.Join(dir, entry.Name())

		if entry.IsDir() {
			e.searchDir(root, fullPath, re, params, maxMatches, resp)
			continue
		}

		// Check access.
		if err := e.checker.Check(fullPath); err != nil {
			continue
		}

		// Apply include/exclude filters.
		if params.Include != "" {
			matched, _ := doublestar.Match(params.Include, entry.Name())
			if !matched {
				continue
			}
		}
		if params.Exclude != "" {
			matched, _ := doublestar.Match(params.Exclude, entry.Name())
			if matched {
				continue
			}
		}

		e.searchFile(root, fullPath, re, params, maxMatches, resp)
	}
}

// searchFile searches a single file for matches.
func (e *Engine) searchFile(root, path string, re *regexp.Regexp, params SearchParams, maxMatches int, resp *protocol.SearchResponse) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	resp.FilesSearched++

	// Read first bytes to check if binary.
	peek := make([]byte, BinaryDetectSize)
	n, _ := f.Read(peek)
	if isBinary(peek[:n]) {
		return // skip binary files
	}
	f.Seek(0, 0) //nolint:errcheck

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var allLines []string
	for scanner.Scan() {
		allLines = append(allLines, scanner.Text())
	}

	fileMatched := false
	relPath, err := filepath.Rel(root, path)
	if err != nil {
		relPath = path
	}

	for lineIdx, line := range allLines {
		if len(resp.Matches) >= maxMatches {
			return
		}

		loc := re.FindStringIndex(line)
		if loc == nil {
			continue
		}

		if !fileMatched {
			fileMatched = true
			resp.FilesMatched++
		}

		match := protocol.SearchMatch{
			File:   relPath,
			Line:   lineIdx + 1,
			Column: loc[0] + 1,
			Match:  line,
		}

		// Context lines.
		if params.Context > 0 {
			startCtx := max(lineIdx-params.Context, 0)
			endCtx := lineIdx + params.Context
			if endCtx >= len(allLines) {
				endCtx = len(allLines) - 1
			}

			for i := startCtx; i < lineIdx; i++ {
				match.ContextBefore = append(match.ContextBefore, allLines[i])
			}
			for i := lineIdx + 1; i <= endCtx; i++ {
				match.ContextAfter = append(match.ContextAfter, allLines[i])
			}
		}

		resp.Matches = append(resp.Matches, match)
	}
}

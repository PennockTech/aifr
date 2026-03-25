// Copyright 2026 — see LICENSE file for terms.
// Package gitprovider implements git tree/blob access via go-git.
package gitprovider

import (
	"fmt"
	"strings"
)

// GitPath represents a parsed git path: [repo:]<ref>:<path>
type GitPath struct {
	Repo string // optional repo name; empty = auto-detect
	Ref  string // branch, tag, commit hash, HEAD~N, etc.
	Path string // path within the tree (relative to repo root)
}

// ParseGitPath parses a git path string.
// Formats:
//
//	ref:path                → auto-detect repo, use ref, path
//	repo:ref:path           → named repo, ref, path
//
// The ref portion is required. The path portion is required.
// A ref cannot be empty but a path can be "" (meaning repo root).
func ParseGitPath(s string) (*GitPath, error) {
	// Count colons to determine format.
	// We need at least one colon for ref:path.
	// Two colons means repo:ref:path.
	//
	// Edge case: commit hashes don't contain colons.
	// Windows paths with drive letters (C:) are not supported (per spec: no Windows).

	first, rest, ok := strings.Cut(s, ":")
	if !ok {
		return nil, fmt.Errorf("invalid git path %q: expected ref:path or repo:ref:path", s)
	}

	// Check for a second colon after the first.
	middle, after, hasTwoParts := strings.Cut(rest, ":")

	if !hasTwoParts {
		// Format: ref:path
		if first == "" {
			return nil, fmt.Errorf("invalid git path %q: ref cannot be empty", s)
		}
		return &GitPath{
			Ref:  first,
			Path: rest,
		}, nil
	}

	// Format: repo:ref:path
	repo := first
	ref := middle
	path := after

	if repo == "" {
		return nil, fmt.Errorf("invalid git path %q: repo cannot be empty", s)
	}
	if ref == "" {
		return nil, fmt.Errorf("invalid git path %q: ref cannot be empty", s)
	}

	return &GitPath{
		Repo: repo,
		Ref:  ref,
		Path: path,
	}, nil
}

// IsGitPath returns true if the string looks like a git path (contains a colon
// that is likely a ref:path separator, not just part of a normal path).
func IsGitPath(s string) bool {
	// A git path must contain at least one colon.
	// We also exclude paths that look like they could be Windows drive letters (C:).
	idx := strings.IndexByte(s, ':')
	if idx < 0 {
		return false
	}
	// Must have something before the colon (the ref or repo name).
	if idx == 0 {
		return false
	}
	// Absolute filesystem paths require the three-part format (path:ref:file)
	// to avoid ambiguity with filesystem paths that happen to contain a colon.
	if strings.HasPrefix(s, "/") {
		return strings.Count(s, ":") >= 2
	}
	return true
}

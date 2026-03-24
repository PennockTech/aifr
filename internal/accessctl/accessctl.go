// Copyright 2026 — see LICENSE file for terms.
// Package accessctl implements path-based access control for aifr.
//
// Evaluation order for every path:
//  1. Resolve to absolute, canonical path (resolve symlinks)
//  2. Check SENSITIVE list → AccessDeniedSensitive
//  3. Check DENY list → AccessDenied
//  4. Check ALLOW list → permit
//  5. Default → AccessDenied
package accessctl

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"

	"go.pennock.tech/aifr/pkg/protocol"
)

// Checker evaluates whether a path may be accessed.
type Checker struct {
	sensitive []string
	deny      []string
	allow     []string
	cwdAllow  string // fallback when allow is empty: cwd + "/**"
}

// CheckerParams configures a new Checker.
type CheckerParams struct {
	Allow     []string // allowed path patterns
	Deny      []string // denied path patterns (evaluated after allow match, before permit)
	CredsDeny []string // additional sensitive patterns (merged with built-in)
}

// NewChecker creates a Checker from the given parameters.
// Patterns are validated at construction time.
func NewChecker(params CheckerParams) (*Checker, error) {
	c := &Checker{
		sensitive: make([]string, 0, len(sensitivePatterns)+len(params.CredsDeny)),
		deny:      params.Deny,
		allow:     params.Allow,
	}

	// Merge built-in sensitive patterns with user creds_deny.
	c.sensitive = append(c.sensitive, sensitivePatterns...)
	c.sensitive = append(c.sensitive, params.CredsDeny...)

	// Validate all patterns.
	for _, p := range c.sensitive {
		if !doublestar.ValidatePattern(p) {
			return nil, fmt.Errorf("invalid sensitive pattern: %q", p)
		}
	}
	for _, p := range c.deny {
		if !doublestar.ValidatePattern(p) {
			return nil, fmt.Errorf("invalid deny pattern: %q", p)
		}
	}
	for _, p := range c.allow {
		if !doublestar.ValidatePattern(p) {
			return nil, fmt.Errorf("invalid allow pattern: %q", p)
		}
	}

	// If allow list is empty, default to cwd-only mode.
	if len(c.allow) == 0 {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("getting cwd for default allow: %w", err)
		}
		c.cwdAllow = cwd
	}

	return c, nil
}

// Check evaluates whether the given path is accessible.
// The path is resolved to an absolute canonical path before checking.
// Returns nil if access is permitted, or an *protocol.AifrError.
func (c *Checker) Check(path string) error {
	resolved, err := resolvePath(path)
	if err != nil {
		// If we can't resolve the path (e.g., broken symlink target),
		// still check the original absolute path for sensitive patterns.
		abs, absErr := filepath.Abs(path)
		if absErr != nil {
			return protocol.NewPathError(protocol.ErrNotFound, path, "cannot resolve path")
		}
		resolved = abs
	}

	// 1. Check sensitive patterns.
	if c.matchesSensitive(resolved) {
		return protocol.NewPathError(protocol.ErrAccessDeniedSensitive, resolved,
			"path matches sensitive file pattern — the user should read this file themselves if needed")
	}

	// 2. Check deny patterns.
	if c.matchesDeny(resolved) {
		return protocol.NewPathError(protocol.ErrAccessDenied, resolved, "path is in deny list")
	}

	// 3. Check allow patterns (or cwd fallback).
	if c.matchesAllow(resolved) {
		return nil
	}

	// 4. Default deny.
	return protocol.NewPathError(protocol.ErrAccessDenied, resolved, "path is not in allow list")
}

// SensitivePatterns returns the full list of sensitive patterns for auditing.
func (c *Checker) SensitivePatterns() []string {
	result := make([]string, len(c.sensitive))
	copy(result, c.sensitive)
	return result
}

// matchesSensitive checks if path matches any sensitive pattern.
// Uses case-insensitive matching on the basename component.
func (c *Checker) matchesSensitive(path string) bool {
	// For case-insensitive basename matching, we create a version of the path
	// with the basename lowercased and check both.
	pathLower := pathWithLowercaseBasename(path)

	for _, pattern := range c.sensitive {
		if matchPath(pattern, path) || (pathLower != path && matchPath(pattern, pathLower)) {
			return true
		}
	}
	return false
}

// matchesDeny checks if path matches any deny pattern.
func (c *Checker) matchesDeny(path string) bool {
	for _, pattern := range c.deny {
		if matchPath(pattern, path) {
			return true
		}
	}
	return false
}

// matchesAllow checks if path matches any allow pattern or the cwd fallback.
func (c *Checker) matchesAllow(path string) bool {
	if c.cwdAllow != "" {
		// CWD-only mode: allow cwd itself and anything beneath it.
		return path == c.cwdAllow || strings.HasPrefix(path, c.cwdAllow+"/")
	}
	for _, pattern := range c.allow {
		if matchPath(pattern, path) {
			return true
		}
	}
	return false
}

// matchPath wraps doublestar.Match, treating errors as non-matches.
func matchPath(pattern, path string) bool {
	matched, err := doublestar.Match(pattern, path)
	if err != nil {
		return false
	}
	return matched
}

// resolvePath converts a path to absolute and resolves symlinks.
func resolvePath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", err
	}
	return resolved, nil
}

// pathWithLowercaseBasename returns the path with its basename lowercased.
func pathWithLowercaseBasename(path string) string {
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	lower := strings.ToLower(base)
	if lower == base {
		return path
	}
	return filepath.Join(dir, lower)
}

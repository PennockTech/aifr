// Copyright 2026 — see LICENSE file for terms.
// Package config handles TOML configuration loading and merging for aifr.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	toml "github.com/pelletier/go-toml/v2"
)

// Config holds the effective configuration for aifr.
type Config struct {
	Allow        []string    `toml:"allow"`
	Deny         []string    `toml:"deny"`
	CredsDeny    []string    `toml:"creds_deny"`
	PathReadable *bool       `toml:"path_readable"` // add $PATH dirs to allow list (default true)
	Git          GitConfig   `toml:"git"`
	Cache        CacheConfig `toml:"cache"`
}

// IsPathReadable returns the effective value of PathReadable (default true).
func (c *Config) IsPathReadable() bool {
	return c.PathReadable == nil || *c.PathReadable
}

// GitConfig holds git-related configuration.
type GitConfig struct {
	Repos     map[string]string `toml:"repos"`
	ReposGlob []string          `toml:"repos_glob"`
}

// CacheConfig holds MCP cache settings.
type CacheConfig struct {
	MaxEntries  int `toml:"max_entries"`
	MaxMemoryMB int `toml:"max_memory_mb"`
	TTLSeconds  int `toml:"ttl_seconds"`
}

// LoadParams controls how configuration is loaded.
type LoadParams struct {
	// ConfigPath is an explicit config file path (from --config flag).
	// If set, only this path is tried.
	ConfigPath string
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Cache: CacheConfig{
			MaxEntries:  10000,
			MaxMemoryMB: 256,
			TTLSeconds:  300,
		},
	}
}

// Load reads and merges configuration from the first available source.
// Search order: explicit path > ./.aifr.toml > $XDG_CONFIG_HOME/aifr/config.toml > ~/.config/aifr/config.toml
func Load(params LoadParams) (*Config, error) {
	cfg := DefaultConfig()

	path, err := findConfigFile(params.ConfigPath)
	if err != nil {
		return nil, err
	}
	if path == "" {
		slog.Debug("no config file found, using defaults")
		return cfg, nil
	}

	slog.Debug("loading config", "path", path)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config %q: %w", path, err)
	}

	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config %q: %w", path, err)
	}

	// Resolve tildes and symlinks in path patterns.
	cfg.Allow, err = resolvePatterns(cfg.Allow)
	if err != nil {
		return nil, fmt.Errorf("resolving allow patterns: %w", err)
	}
	cfg.Deny, err = resolvePatterns(cfg.Deny)
	if err != nil {
		return nil, fmt.Errorf("resolving deny patterns: %w", err)
	}
	cfg.CredsDeny, err = resolvePatterns(cfg.CredsDeny)
	if err != nil {
		return nil, fmt.Errorf("resolving creds_deny patterns: %w", err)
	}

	// Resolve tildes and symlinks in git repo paths.
	for name, repoPath := range cfg.Git.Repos {
		expanded, err := ExpandTilde(repoPath)
		if err != nil {
			return nil, fmt.Errorf("resolving git repo path %q: %w", name, err)
		}
		cfg.Git.Repos[name] = ResolvePath(expanded)
	}

	// Expand repos_glob patterns into the Repos map.
	if err := expandReposGlob(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// expandReposGlob expands glob patterns in Git.ReposGlob into Git.Repos.
// Explicit Repos entries take precedence over glob-discovered ones.
func expandReposGlob(cfg *Config) error {
	for _, pattern := range cfg.Git.ReposGlob {
		expanded, err := ExpandTilde(pattern)
		if err != nil {
			return fmt.Errorf("expanding repos_glob %q: %w", pattern, err)
		}

		matches, err := doublestar.FilepathGlob(expanded)
		if err != nil {
			slog.Warn("invalid repos_glob pattern", "pattern", pattern, "error", err)
			continue
		}

		for _, match := range matches {
			info, err := os.Stat(match)
			if err != nil || !info.IsDir() {
				continue
			}

			name := deriveRepoName(match)
			if _, exists := cfg.Git.Repos[name]; exists {
				continue // explicit repos take precedence
			}

			if cfg.Git.Repos == nil {
				cfg.Git.Repos = make(map[string]string)
			}
			resolved := ResolvePath(match)
			cfg.Git.Repos[name] = resolved
			slog.Debug("repos_glob: discovered repo", "name", name, "path", resolved)
		}
	}
	return nil
}

// deriveRepoName generates a repo name from a directory path.
// Strips .git suffix from the base name.
func deriveRepoName(path string) string {
	name := filepath.Base(path)
	name = strings.TrimSuffix(name, ".git")
	return name
}

// findConfigFile returns the path to the first config file found,
// or "" if none exists.
func findConfigFile(explicit string) (string, error) {
	if explicit != "" {
		expanded, err := ExpandTilde(explicit)
		if err != nil {
			return "", err
		}
		if _, err := os.Stat(expanded); err != nil {
			return "", fmt.Errorf("config file %q: %w", explicit, err)
		}
		return expanded, nil
	}

	candidates := []string{
		"./.aifr.toml",
	}

	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		candidates = append(candidates, filepath.Join(xdg, "aifr", "config.toml"))
	}

	home, err := os.UserHomeDir()
	if err == nil {
		candidates = append(candidates, filepath.Join(home, ".config", "aifr", "config.toml"))
	}

	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c, nil
		}
	}

	return "", nil
}

// ExpandTilde expands ~ and ~user prefixes in a path.
func ExpandTilde(path string) (string, error) {
	if !strings.HasPrefix(path, "~") {
		return path, nil
	}

	// ~/... — current user
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("expanding ~: %w", err)
		}
		if path == "~" {
			return home, nil
		}
		return filepath.Join(home, path[2:]), nil
	}

	// ~user/... — other user
	slashIdx := strings.IndexByte(path, '/')
	var username string
	if slashIdx < 0 {
		username = path[1:]
	} else {
		username = path[1:slashIdx]
	}

	u, err := user.Lookup(username)
	if err != nil {
		var unknownUser user.UnknownUserError
		if errors.As(err, &unknownUser) {
			return "", fmt.Errorf("unknown user %q in path %q", username, path)
		}
		return "", fmt.Errorf("looking up user %q: %w", username, err)
	}

	if slashIdx < 0 {
		return u.HomeDir, nil
	}
	return filepath.Join(u.HomeDir, path[slashIdx+1:]), nil
}

// resolvePatterns expands tildes and resolves symlinks in the non-glob
// prefix of each pattern, so that patterns match the symlink-resolved
// paths that the access checker compares against.
func resolvePatterns(patterns []string) ([]string, error) {
	result := make([]string, len(patterns))
	for i, p := range patterns {
		expanded, err := ExpandTilde(p)
		if err != nil {
			return nil, err
		}
		result[i] = resolvePatternPrefix(expanded)
	}
	return result, nil
}

// resolvePatternPrefix resolves symlinks in the literal (non-glob) prefix
// of a pattern. For example, if /home is a symlink to /export/home, then
// "/home/user/projects/**" becomes "/export/home/user/projects/**".
//
// If no portion of the prefix exists on disk, the pattern is returned
// unchanged (it may refer to a path that will be created later, or be a
// relative glob like "**/secrets/**").
func resolvePatternPrefix(pattern string) string {
	// Find the first glob metacharacter.
	globChars := "*?[{"
	firstGlob := len(pattern)
	for i, c := range pattern {
		if strings.ContainsRune(globChars, c) {
			firstGlob = i
			break
		}
	}

	// Extract the literal prefix (up to the last separator before the glob).
	literal := pattern[:firstGlob]
	if lastSep := strings.LastIndexByte(literal, '/'); lastSep >= 0 {
		literal = literal[:lastSep]
	} else {
		// No separator in the literal prefix — nothing concrete to resolve.
		return pattern
	}

	if literal == "" {
		return pattern
	}

	// Walk backward through the literal prefix to find the longest portion
	// that exists on disk, so we can resolve symlinks in it. The tail
	// (components that don't exist yet) is preserved as-is.
	tryPath := literal
	for tryPath != "" && tryPath != "/" && tryPath != "." {
		resolved, err := filepath.EvalSymlinks(tryPath)
		if err == nil {
			tail := literal[len(tryPath):]
			suffix := pattern[len(literal):]
			return resolved + tail + suffix
		}
		tryPath = filepath.Dir(tryPath)
	}

	return pattern
}

// ResolvePath resolves a concrete (non-glob) path: makes it absolute and
// resolves symlinks. Used for git repo paths and other non-pattern paths
// in the config. Falls back to filepath.Clean if the path does not exist.
func ResolvePath(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return filepath.Clean(path)
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return abs
	}
	return resolved
}

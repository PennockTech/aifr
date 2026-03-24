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

	toml "github.com/pelletier/go-toml/v2"
)

// Config holds the effective configuration for aifr.
type Config struct {
	Allow     []string    `toml:"allow"`
	Deny      []string    `toml:"deny"`
	CredsDeny []string    `toml:"creds_deny"`
	Git       GitConfig   `toml:"git"`
	Cache     CacheConfig `toml:"cache"`
}

// GitConfig holds git-related configuration.
type GitConfig struct {
	Repos map[string]string `toml:"repos"`
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

	// Resolve tildes in path patterns.
	cfg.Allow, err = resolvePatternTildes(cfg.Allow)
	if err != nil {
		return nil, fmt.Errorf("resolving allow patterns: %w", err)
	}
	cfg.Deny, err = resolvePatternTildes(cfg.Deny)
	if err != nil {
		return nil, fmt.Errorf("resolving deny patterns: %w", err)
	}
	cfg.CredsDeny, err = resolvePatternTildes(cfg.CredsDeny)
	if err != nil {
		return nil, fmt.Errorf("resolving creds_deny patterns: %w", err)
	}

	// Resolve tildes in git repo paths.
	for name, repoPath := range cfg.Git.Repos {
		resolved, err := ExpandTilde(repoPath)
		if err != nil {
			return nil, fmt.Errorf("resolving git repo path %q: %w", name, err)
		}
		cfg.Git.Repos[name] = resolved
	}

	return cfg, nil
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

// resolvePatternTildes expands tildes in a slice of glob patterns.
func resolvePatternTildes(patterns []string) ([]string, error) {
	result := make([]string, len(patterns))
	for i, p := range patterns {
		expanded, err := ExpandTilde(p)
		if err != nil {
			return nil, err
		}
		result[i] = expanded
	}
	return result, nil
}

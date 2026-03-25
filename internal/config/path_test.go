// Copyright 2026 — see LICENSE file for terms.
package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPathAllowPatterns(t *testing.T) {
	dir := t.TempDir()
	bin1 := filepath.Join(dir, "bin1")
	bin2 := filepath.Join(dir, "bin2")
	os.MkdirAll(bin1, 0o755) //nolint:errcheck
	os.MkdirAll(bin2, 0o755) //nolint:errcheck

	t.Setenv("PATH", bin1+":"+bin2)

	patterns := PathAllowPatterns()
	if len(patterns) != 2 {
		t.Fatalf("expected 2 patterns, got %d: %v", len(patterns), patterns)
	}

	for _, p := range patterns {
		if !strings.HasSuffix(p, "/*") {
			t.Errorf("pattern %q should end with /*", p)
		}
	}
}

func TestPathAllowPatternsDedup(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	os.MkdirAll(bin, 0o755) //nolint:errcheck

	// Same dir twice.
	t.Setenv("PATH", bin+":"+bin)

	patterns := PathAllowPatterns()
	if len(patterns) != 1 {
		t.Errorf("expected 1 pattern (deduped), got %d: %v", len(patterns), patterns)
	}
}

func TestPathAllowPatternsSkipsNonexistent(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	os.MkdirAll(bin, 0o755) //nolint:errcheck
	noexist := filepath.Join(dir, "noexist")

	t.Setenv("PATH", bin+":"+noexist)

	patterns := PathAllowPatterns()
	if len(patterns) != 1 {
		t.Errorf("expected 1 pattern (nonexistent skipped), got %d: %v", len(patterns), patterns)
	}
}

func TestPathAllowPatternsEmpty(t *testing.T) {
	t.Setenv("PATH", "")

	patterns := PathAllowPatterns()
	if len(patterns) != 0 {
		t.Errorf("expected 0 patterns for empty PATH, got %d", len(patterns))
	}
}

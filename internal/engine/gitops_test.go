// Copyright 2026 — see LICENSE file for terms.
package engine

import (
	"os"
	"testing"

	"go.pennock.tech/aifr/internal/accessctl"
	"go.pennock.tech/aifr/internal/config"
)

// TestOpenGitRepoDotNormalization verifies that "." is treated as
// auto-detect (same as "") rather than as an explicit filesystem path.
// This matters because auto-detect skips access control (the user is
// already in the directory), while explicit paths are checked.
func TestOpenGitRepoDotNormalization(t *testing.T) {
	dir := t.TempDir()
	initTestGitRepo(t, dir, 1)

	// Create an engine that does NOT allow the repo dir.
	// Use a non-existent allow path so nothing is permitted.
	checker, err := accessctl.NewChecker(accessctl.CheckerParams{
		Allow: []string{"/nonexistent-allow-path/**"},
	})
	if err != nil {
		t.Fatal(err)
	}
	eng, err := NewEngine(checker, config.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}

	// cd into the repo so auto-detect finds it.
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(origDir) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	// Auto-detect (empty string) should succeed — no access check.
	_, err = eng.Log("", "HEAD", LogParams{MaxCount: 1})
	if err != nil {
		t.Errorf("Log with empty repo (auto-detect) failed: %v", err)
	}

	// Bare "." should also succeed — normalized to auto-detect.
	_, err = eng.Log(".", "HEAD", LogParams{MaxCount: 1})
	if err != nil {
		t.Errorf("Log with \".\" repo failed: %v", err)
	}

	// Explicit absolute path should be denied.
	_, err = eng.Log(dir, "HEAD", LogParams{MaxCount: 1})
	if err == nil {
		t.Error("Log with explicit absolute path should have been denied")
	}

	// ".." should be denied (it's a different path, subject to access control).
	// Create a subdir and cd into it so ".." resolves to the repo root.
	subdir := dir + "/subdir"
	os.Mkdir(subdir, 0o755)
	if err := os.Chdir(subdir); err != nil {
		t.Fatal(err)
	}
	_, err = eng.Log("..", "HEAD", LogParams{MaxCount: 1})
	if err == nil {
		t.Error("Log with \"..\" should have been denied")
	}
}

// TestDotRepoConsistentAcrossCommands verifies that "." normalization
// works for all git commands, not just log.
func TestDotRepoConsistentAcrossCommands(t *testing.T) {
	dir := t.TempDir()
	initTestGitRepo(t, dir, 1)

	checker, err := accessctl.NewChecker(accessctl.CheckerParams{
		Allow: []string{"/nonexistent-allow-path/**"},
	})
	if err != nil {
		t.Fatal(err)
	}
	eng, err := NewEngine(checker, config.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(origDir) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	// All these should succeed with "." (normalized to auto-detect).
	t.Run("Refs", func(t *testing.T) {
		_, err := eng.Refs(".", true, true, true)
		if err != nil {
			t.Errorf("Refs with \".\" failed: %v", err)
		}
	})

	t.Run("Log", func(t *testing.T) {
		_, err := eng.Log(".", "HEAD", LogParams{MaxCount: 1})
		if err != nil {
			t.Errorf("Log with \".\" failed: %v", err)
		}
	})

	// All these should fail with explicit absolute path.
	t.Run("Refs_absolute_denied", func(t *testing.T) {
		_, err := eng.Refs(dir, true, true, true)
		if err == nil {
			t.Error("Refs with absolute path should have been denied")
		}
	})

	t.Run("Log_absolute_denied", func(t *testing.T) {
		_, err := eng.Log(dir, "HEAD", LogParams{MaxCount: 1})
		if err == nil {
			t.Error("Log with absolute path should have been denied")
		}
	})
}

// Copyright 2026 — see LICENSE file for terms.
package engine

import (
	"os"
	"path/filepath"
	"testing"

	"go.pennock.tech/aifr/internal/accessctl"
	"go.pennock.tech/aifr/internal/config"
)

// makePathfindTestDirs creates two directories with some "commands" for testing.
// Returns the two directory paths.
func makePathfindTestDirs(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	dir1 := filepath.Join(dir, "bin1")
	dir2 := filepath.Join(dir, "bin2")
	os.MkdirAll(dir1, 0o755) //nolint:errcheck
	os.MkdirAll(dir2, 0o755) //nolint:errcheck

	// dir1: git, git-commit, python3
	os.WriteFile(filepath.Join(dir1, "git"), []byte("#!/bin/sh\n"), 0o755)        //nolint:errcheck
	os.WriteFile(filepath.Join(dir1, "git-commit"), []byte("#!/bin/sh\n"), 0o755) //nolint:errcheck
	os.WriteFile(filepath.Join(dir1, "python3"), []byte("#!/bin/sh\n"), 0o755)    //nolint:errcheck

	// dir2: git (masked), git-rebase, curl
	os.WriteFile(filepath.Join(dir2, "git"), []byte("#!/bin/sh\n"), 0o755)        //nolint:errcheck
	os.WriteFile(filepath.Join(dir2, "git-rebase"), []byte("#!/bin/sh\n"), 0o755) //nolint:errcheck
	os.WriteFile(filepath.Join(dir2, "curl"), []byte("#!/bin/sh\n"), 0o644)       //nolint:errcheck

	return dir1, dir2
}

func makePathfindEngine(t *testing.T, allowPatterns []string) *Engine {
	t.Helper()
	checker, err := accessctl.NewChecker(accessctl.CheckerParams{
		Allow: allowPatterns,
	})
	if err != nil {
		t.Fatal(err)
	}
	eng, err := NewEngine(checker, config.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	return eng
}

func TestPathfindExactMatch(t *testing.T) {
	dir1, dir2 := makePathfindTestDirs(t)
	eng := makePathfindEngine(t, []string{dir1 + "/*", dir2 + "/*"})

	resp, err := eng.Pathfind("git", PathfindParams{
		SearchList: "dirlist:" + dir1 + ":" + dir2,
	})
	if err != nil {
		t.Fatal(err)
	}

	if resp.Total != 2 {
		t.Fatalf("expected 2 matches, got %d", resp.Total)
	}

	// First match should not be masked.
	if resp.Entries[0].Masked {
		t.Error("first match should not be masked")
	}
	if resp.Entries[0].Dir != dir1 {
		t.Errorf("first match dir = %q, want %q", resp.Entries[0].Dir, dir1)
	}

	// Second match should be masked by the first.
	if !resp.Entries[1].Masked {
		t.Error("second match should be masked")
	}
	if resp.Entries[1].MaskedBy != filepath.Join(dir1, "git") {
		t.Errorf("masked_by = %q, want %q", resp.Entries[1].MaskedBy, filepath.Join(dir1, "git"))
	}
}

func TestPathfindGlobMatch(t *testing.T) {
	dir1, dir2 := makePathfindTestDirs(t)
	eng := makePathfindEngine(t, []string{dir1 + "/*", dir2 + "/*"})

	resp, err := eng.Pathfind("git-*", PathfindParams{
		SearchList: "dirlist:" + dir1 + ":" + dir2,
	})
	if err != nil {
		t.Fatal(err)
	}

	if resp.Total != 2 {
		t.Fatalf("expected 2 matches for git-*, got %d", resp.Total)
	}

	// git-commit from dir1, git-rebase from dir2 — different names, neither masked.
	for _, e := range resp.Entries {
		if e.Masked {
			t.Errorf("entry %q should not be masked (different names)", e.Name)
		}
	}
}

func TestPathfindExecutableFlag(t *testing.T) {
	dir1, dir2 := makePathfindTestDirs(t)
	eng := makePathfindEngine(t, []string{dir1 + "/*", dir2 + "/*"})

	resp, err := eng.Pathfind("curl", PathfindParams{
		SearchList: "dirlist:" + dir1 + ":" + dir2,
	})
	if err != nil {
		t.Fatal(err)
	}

	if resp.Total != 1 {
		t.Fatalf("expected 1 match, got %d", resp.Total)
	}

	// curl has mode 0644 — not executable.
	if resp.Entries[0].Executable {
		t.Error("curl should not be marked executable (mode 0644)")
	}
}

func TestPathfindDirIndex(t *testing.T) {
	dir1, dir2 := makePathfindTestDirs(t)
	eng := makePathfindEngine(t, []string{dir1 + "/*", dir2 + "/*"})

	resp, err := eng.Pathfind("git", PathfindParams{
		SearchList: "dirlist:" + dir1 + ":" + dir2,
	})
	if err != nil {
		t.Fatal(err)
	}

	if resp.Entries[0].DirIndex != 0 {
		t.Errorf("first entry dir_index = %d, want 0", resp.Entries[0].DirIndex)
	}
	if resp.Entries[1].DirIndex != 1 {
		t.Errorf("second entry dir_index = %d, want 1", resp.Entries[1].DirIndex)
	}
}

func TestPathfindAccessDenied(t *testing.T) {
	dir1, dir2 := makePathfindTestDirs(t)
	// Only allow dir1, not dir2.
	eng := makePathfindEngine(t, []string{dir1 + "/*"})

	resp, err := eng.Pathfind("git", PathfindParams{
		SearchList: "dirlist:" + dir1 + ":" + dir2,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Should only find git in dir1.
	if resp.Total != 1 {
		t.Fatalf("expected 1 match (dir2 denied), got %d", resp.Total)
	}
}

func TestParseSearchList(t *testing.T) {
	// Set a test env var.
	t.Setenv("AIFR_TEST_PATH", "/usr/bin:/usr/local/bin")

	tests := []struct {
		spec    string
		wantLen int
		wantErr bool
	}{
		{"envvar:AIFR_TEST_PATH", 2, false},
		{"dirlist:/a:/b:/c", 3, false},
		{"", 2, false}, // defaults to envvar:PATH, but PATH varies; just check no error

		{"envvar:", 0, true},
		{"envvar:NONEXISTENT_VAR_XYZ", 0, true},
		{"dirlist:", 0, true},
		{"invalid:foo", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.spec, func(t *testing.T) {
			dirs, err := parseSearchList(tt.spec)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error")
				}
				return
			}
			if err != nil {
				if tt.spec == "" {
					// Default PATH might be empty in some test envs.
					t.Skip("PATH not set in test env")
				}
				t.Fatal(err)
			}
			if tt.spec != "" && len(dirs) != tt.wantLen {
				t.Errorf("got %d dirs, want %d", len(dirs), tt.wantLen)
			}
		})
	}
}

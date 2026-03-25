// Copyright 2026 — see LICENSE file for terms.
package gitprovider

import (
	"testing"
)

func TestParseGitPath(t *testing.T) {
	tests := []struct {
		input    string
		wantRepo string
		wantRef  string
		wantPath string
		wantErr  bool
	}{
		// ref:path
		{"main:src/handler.go", "", "main", "src/handler.go", false},
		{"HEAD:README.md", "", "HEAD", "README.md", false},
		{"HEAD~3:README.md", "", "HEAD~3", "README.md", false},
		{"a1b2c3d:pkg/auth/auth.go", "", "a1b2c3d", "pkg/auth/auth.go", false},
		{"origin/feature/login:cmd/server/main.go", "", "origin/feature/login", "cmd/server/main.go", false},
		{"main:", "", "main", "", false}, // empty path = repo root

		// repo:ref:path
		{"infra:v2.1.0:terraform/main.tf", "infra", "v2.1.0", "terraform/main.tf", false},
		{"myrepo:main:src/lib.rs", "myrepo", "main", "src/lib.rs", false},
		{"myrepo:HEAD:", "myrepo", "HEAD", "", false}, // empty path = repo root

		// Errors
		{"no-colon", "", "", "", true},
		{":path", "", "", "", true},      // empty ref
		{":ref:path", "", "", "", true},  // empty repo in 3-part
		{"repo::path", "", "", "", true}, // empty ref in 3-part
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			gp, err := ParseGitPath(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error for %q", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", tt.input, err)
			}
			if gp.Repo != tt.wantRepo {
				t.Errorf("repo = %q, want %q", gp.Repo, tt.wantRepo)
			}
			if gp.Ref != tt.wantRef {
				t.Errorf("ref = %q, want %q", gp.Ref, tt.wantRef)
			}
			if gp.Path != tt.wantPath {
				t.Errorf("path = %q, want %q", gp.Path, tt.wantPath)
			}
		})
	}
}

func TestIsGitPath(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"main:src/handler.go", true},
		{"HEAD:README.md", true},
		{"repo:ref:path", true},
		{"/absolute/path", false},
		{"relative/path", false},
		{"no-colon", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := IsGitPath(tt.input)
			if got != tt.want {
				t.Errorf("IsGitPath(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

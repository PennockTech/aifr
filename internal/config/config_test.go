// Copyright 2026 — see LICENSE file for terms.
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Cache.MaxEntries != 10000 {
		t.Errorf("default MaxEntries = %d, want 10000", cfg.Cache.MaxEntries)
	}
	if cfg.Cache.MaxMemoryMB != 256 {
		t.Errorf("default MaxMemoryMB = %d, want 256", cfg.Cache.MaxMemoryMB)
	}
	if cfg.Cache.TTLSeconds != 300 {
		t.Errorf("default TTLSeconds = %d, want 300", cfg.Cache.TTLSeconds)
	}
	if len(cfg.Allow) != 0 {
		t.Errorf("default Allow should be empty, got %v", cfg.Allow)
	}
}

func TestLoadExplicitPath(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "test.toml")

	content := `
allow = ["/home/user/projects/**"]
deny = ["**/secrets/**"]

[git.repos]
main = "/home/user/projects/myapp"

[cache]
max_entries = 5000
max_memory_mb = 128
ttl_seconds = 60
`
	if err := os.WriteFile(cfgFile, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(LoadParams{ConfigPath: cfgFile})
	if err != nil {
		t.Fatal(err)
	}

	if len(cfg.Allow) != 1 || cfg.Allow[0] != "/home/user/projects/**" {
		t.Errorf("Allow = %v, want [/home/user/projects/**]", cfg.Allow)
	}
	if len(cfg.Deny) != 1 || cfg.Deny[0] != "**/secrets/**" {
		t.Errorf("Deny = %v, want [**/secrets/**]", cfg.Deny)
	}
	if cfg.Git.Repos["main"] != "/home/user/projects/myapp" {
		t.Errorf("Git.Repos[main] = %q, want /home/user/projects/myapp", cfg.Git.Repos["main"])
	}
	if cfg.Cache.MaxEntries != 5000 {
		t.Errorf("Cache.MaxEntries = %d, want 5000", cfg.Cache.MaxEntries)
	}
}

func TestLoadMissingExplicitPath(t *testing.T) {
	_, err := Load(LoadParams{ConfigPath: "/nonexistent/config.toml"})
	if err == nil {
		t.Fatal("expected error for missing explicit config path")
	}
}

func TestLoadNoConfigFile(t *testing.T) {
	// Run from a temp dir where no .aifr.toml exists.
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir) //nolint:errcheck
	os.Chdir(dir)           //nolint:errcheck

	// Clear XDG to avoid picking up real config.
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "xdg"))

	cfg, err := Load(LoadParams{})
	if err != nil {
		t.Fatal(err)
	}

	// Should get defaults.
	if cfg.Cache.MaxEntries != 10000 {
		t.Errorf("expected default config, got MaxEntries=%d", cfg.Cache.MaxEntries)
	}
}

func TestLoadDotAifrToml(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir) //nolint:errcheck
	os.Chdir(dir)           //nolint:errcheck

	content := `allow = ["/tmp/**"]`
	if err := os.WriteFile(filepath.Join(dir, ".aifr.toml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	// Clear XDG to avoid interference.
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "xdg"))

	cfg, err := Load(LoadParams{})
	if err != nil {
		t.Fatal(err)
	}

	if len(cfg.Allow) != 1 || cfg.Allow[0] != "/tmp/**" {
		t.Errorf("Allow = %v, want [/tmp/**]", cfg.Allow)
	}
}

func TestLoadXDGConfigHome(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir) //nolint:errcheck
	os.Chdir(dir)           //nolint:errcheck

	xdgDir := filepath.Join(dir, "xdg", "aifr")
	os.MkdirAll(xdgDir, 0o755) //nolint:errcheck

	content := `deny = ["**/private/**"]`
	if err := os.WriteFile(filepath.Join(xdgDir, "config.toml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "xdg"))

	cfg, err := Load(LoadParams{})
	if err != nil {
		t.Fatal(err)
	}

	if len(cfg.Deny) != 1 || cfg.Deny[0] != "**/private/**" {
		t.Errorf("Deny = %v, want [**/private/**]", cfg.Deny)
	}
}

func TestExpandTilde(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home directory")
	}

	tests := []struct {
		input string
		want  string
	}{
		{"/absolute/path", "/absolute/path"},
		{"relative/path", "relative/path"},
		{"~/foo/bar", filepath.Join(home, "foo/bar")},
		{"~", home},
	}

	for _, tt := range tests {
		got, err := ExpandTilde(tt.input)
		if err != nil {
			t.Errorf("ExpandTilde(%q) error: %v", tt.input, err)
			continue
		}
		if got != tt.want {
			t.Errorf("ExpandTilde(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestExpandTildeUnknownUser(t *testing.T) {
	_, err := ExpandTilde("~nonexistentuserxyz123/foo")
	if err == nil {
		t.Fatal("expected error for unknown user")
	}
}

func TestLoadTildeExpansionInPaths(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home directory")
	}

	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "test.toml")

	content := `
allow = ["~/projects/**"]
deny = ["~/projects/secrets/**"]
creds_deny = ["~/creds/**"]

[git.repos]
main = "~/projects/myapp"
`
	if err := os.WriteFile(cfgFile, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(LoadParams{ConfigPath: cfgFile})
	if err != nil {
		t.Fatal(err)
	}

	wantAllow := filepath.Join(home, "projects/**")
	if len(cfg.Allow) != 1 || cfg.Allow[0] != wantAllow {
		t.Errorf("Allow = %v, want [%s]", cfg.Allow, wantAllow)
	}

	wantDeny := filepath.Join(home, "projects/secrets/**")
	if len(cfg.Deny) != 1 || cfg.Deny[0] != wantDeny {
		t.Errorf("Deny = %v, want [%s]", cfg.Deny, wantDeny)
	}

	wantCreds := filepath.Join(home, "creds/**")
	if len(cfg.CredsDeny) != 1 || cfg.CredsDeny[0] != wantCreds {
		t.Errorf("CredsDeny = %v, want [%s]", cfg.CredsDeny, wantCreds)
	}

	wantRepo := filepath.Join(home, "projects/myapp")
	if cfg.Git.Repos["main"] != wantRepo {
		t.Errorf("Git.Repos[main] = %q, want %q", cfg.Git.Repos["main"], wantRepo)
	}
}

func TestLoadInvalidTOML(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "bad.toml")

	if err := os.WriteFile(cfgFile, []byte("this is not valid toml {{{"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(LoadParams{ConfigPath: cfgFile})
	if err == nil {
		t.Fatal("expected error for invalid TOML")
	}
}

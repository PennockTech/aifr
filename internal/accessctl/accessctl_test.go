// Copyright 2026 — see LICENSE file for terms.
package accessctl

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"go.pennock.tech/aifr/pkg/protocol"
)

// helper to assert a path is allowed.
func assertAllowed(t *testing.T, c *Checker, path string) {
	t.Helper()
	if err := c.Check(path); err != nil {
		t.Errorf("expected path %q to be allowed, got error: %v", path, err)
	}
}

// helper to assert a path is denied (ACCESS_DENIED).
func assertDenied(t *testing.T, c *Checker, path string) {
	t.Helper()
	err := c.Check(path)
	if err == nil {
		t.Errorf("expected path %q to be denied, got nil", path)
		return
	}
	var ae *protocol.AifrError
	if !errors.As(err, &ae) || ae.Code != protocol.ErrAccessDenied {
		t.Errorf("expected ACCESS_DENIED for %q, got: %v", path, err)
	}
}

// helper to assert a path triggers ACCESS_DENIED_SENSITIVE.
func assertSensitive(t *testing.T, c *Checker, path string) {
	t.Helper()
	err := c.Check(path)
	if err == nil {
		t.Errorf("expected path %q to be sensitive, got nil", path)
		return
	}
	var ae *protocol.AifrError
	if !errors.As(err, &ae) || ae.Code != protocol.ErrAccessDeniedSensitive {
		t.Errorf("expected ACCESS_DENIED_SENSITIVE for %q, got: %v", path, err)
	}
}

func TestCWDFallbackMode(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir) //nolint:errcheck
	os.Chdir(dir)           //nolint:errcheck

	// Create a test file.
	testFile := filepath.Join(dir, "test.txt")
	os.WriteFile(testFile, []byte("hello"), 0o644) //nolint:errcheck

	c, err := NewChecker(CheckerParams{})
	if err != nil {
		t.Fatal(err)
	}

	// File within cwd should be allowed.
	assertAllowed(t, c, testFile)

	// File outside cwd should be denied.
	assertDenied(t, c, "/etc/hostname")
}

func TestExplicitAllowDeny(t *testing.T) {
	dir := t.TempDir()

	// Create test files.
	allowedFile := filepath.Join(dir, "allowed", "file.txt")
	deniedFile := filepath.Join(dir, "allowed", "secrets", "key.txt")
	os.MkdirAll(filepath.Dir(allowedFile), 0o755)  //nolint:errcheck
	os.MkdirAll(filepath.Dir(deniedFile), 0o755)   //nolint:errcheck
	os.WriteFile(allowedFile, []byte("ok"), 0o644) //nolint:errcheck
	os.WriteFile(deniedFile, []byte("no"), 0o644)  //nolint:errcheck

	c, err := NewChecker(CheckerParams{
		Allow: []string{dir + "/allowed/**"},
		Deny:  []string{dir + "/allowed/secrets/**"},
	})
	if err != nil {
		t.Fatal(err)
	}

	assertAllowed(t, c, allowedFile)
	assertDenied(t, c, deniedFile)
	assertDenied(t, c, "/tmp/outside")
}

func TestSensitivePatternsBlockAllowList(t *testing.T) {
	// Even if a sensitive file is in the allow list, it must be blocked.
	dir := t.TempDir()
	sshDir := filepath.Join(dir, ".ssh")
	os.MkdirAll(sshDir, 0o700) //nolint:errcheck

	keyFile := filepath.Join(sshDir, "id_rsa")
	os.WriteFile(keyFile, []byte("fake key"), 0o600) //nolint:errcheck

	c, err := NewChecker(CheckerParams{
		Allow: []string{dir + "/**"},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Should be sensitive even though it's within the allow list.
	assertSensitive(t, c, keyFile)
}

func TestCredsDenyMerge(t *testing.T) {
	dir := t.TempDir()
	customFile := filepath.Join(dir, "custom.creds")
	os.WriteFile(customFile, []byte("data"), 0o644) //nolint:errcheck

	c, err := NewChecker(CheckerParams{
		Allow:     []string{dir + "/**"},
		CredsDeny: []string{"**/*.creds"},
	})
	if err != nil {
		t.Fatal(err)
	}

	assertSensitive(t, c, customFile)
}

func TestSymlinkResolution(t *testing.T) {
	dir := t.TempDir()

	// Create a sensitive file and a symlink to it.
	sshDir := filepath.Join(dir, ".ssh")
	os.MkdirAll(sshDir, 0o700) //nolint:errcheck

	keyFile := filepath.Join(sshDir, "id_ed25519")
	os.WriteFile(keyFile, []byte("fake key"), 0o600) //nolint:errcheck

	linkFile := filepath.Join(dir, "innocent-link")
	if err := os.Symlink(keyFile, linkFile); err != nil {
		t.Skip("symlinks not supported")
	}

	c, err := NewChecker(CheckerParams{
		Allow: []string{dir + "/**"},
	})
	if err != nil {
		t.Fatal(err)
	}

	// The symlink should resolve to the sensitive file and be blocked.
	assertSensitive(t, c, linkFile)
}

func TestInvalidPattern(t *testing.T) {
	_, err := NewChecker(CheckerParams{
		Allow: []string{"[invalid"},
	})
	if err == nil {
		t.Fatal("expected error for invalid pattern")
	}
}

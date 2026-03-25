// Copyright 2026 — see LICENSE file for terms.
package accessctl

import (
	"os"
	"path/filepath"
	"testing"
)

// TestSensitivePatterns tests that all categories of sensitive files are caught
// and that legitimate files are not falsely flagged.
func TestSensitivePatterns(t *testing.T) {
	// Create a temp dir that serves as our "filesystem root" for testing.
	// We create real files so that symlink resolution and path resolution work.
	root := t.TempDir()

	// Helper to create a file at a path relative to root.
	mkFile := func(relPath string) string {
		full := filepath.Join(root, relPath)
		os.MkdirAll(filepath.Dir(full), 0o755)    //nolint:errcheck
		os.WriteFile(full, []byte("test"), 0o644) //nolint:errcheck
		return full
	}

	c, err := NewChecker(CheckerParams{
		Allow: []string{root + "/**"},
	})
	if err != nil {
		t.Fatal(err)
	}

	// ── Files that MUST be detected as sensitive ──

	sensitiveFiles := map[string][]string{
		"SSH & GPG": {
			".ssh/id_rsa",
			".ssh/id_ed25519",
			".ssh/id_ecdsa",
			".ssh/authorized_keys",
			".ssh/known_hosts",
			".ssh/config",
			".gnupg/trustdb.gpg",
			".gnupg/private-keys-v1.d/ABC123.key",
			".gpg/secring.gpg",
		},
		"Cloud credentials": {
			".aws/credentials",
			".aws/config",
			".azure/accessTokens.json",
			".azure/azureProfile.json",
			".config/gcloud/credentials.db",
			".config/gcloud/application_default_credentials.json",
			".config/gcloud/access_tokens.db",
			".boto",
			"project/service-account.json",
			"project/service-account-key.json",
		},
		"Kubernetes & container": {
			".kube/config",
			".kube/cache/discovery/something",
			".docker/config.json",
			".docker/daemon.json",
			"project/kubeconfig",
			"project/kubeconfig.yaml",
		},
		"NATS": {
			"project/operator.nk",
			"nats/context/myctx.json",
		},
		"Package manager tokens": {
			".npmrc",
			".yarnrc",
			".pypirc",
			".gem/credentials",
			".cargo/credentials",
			".cargo/credentials.toml",
			".composer/auth.json",
		},
		"Env files & secrets": {
			".env",
			".env.production",
			".env.local",
			"env.local",
			".envrc",
			"project/server.pem",
			"project/server.key",
			"project/cert.p12",
			"project/cert.pfx",
			"project/keystore.jks",
			"project/app.keystore",
			"project/secrets.yml",
			"project/secrets.yaml",
			"project/secrets.json",
			"project/vault.yml",
			"project/vault.yaml",
		},
		"Shell history": {
			".bash_history",
			".zsh_history",
			".histfile",
			".python_history",
			".node_repl_history",
			".psql_history",
			".mysql_history",
			".sqlite_history",
			".lesshst",
			".viminfo",
		},
		"Database files": {
			".pgpass",
			".my.cnf",
			".mongoshrc.js",
			".dbshell",
		},
		"OS credential stores": {
			"login.keychain",
			"login.keychain-db",
			".local/share/keyrings/default.keyring",
			".gnome2/keyrings/default.keyring",
		},
		"CI/CD & tool tokens": {
			".netrc",
			".curlrc",
			".wgetrc",
			".git-credentials",
			".config/gh/hosts.yml",
			".config/hub",
			".travis.yml",
			".vault-token",
			".terraform/state.tfstate",
			"terraform.tfvars",
		},
		"Password managers": {
			".password-store/email.gpg",
			".age/key.txt",
			"data.age",
			"age-key.txt",
			".sops.yaml",
			".1password/data",
			".op/config",
			"bitwarden-data",
			".lastpass/vault",
		},
		"Miscellaneous": {
			"tls.key",
			"tls.crt",
			"app.secret",
			"htpasswd",
			"shadow",
			"passwd",
			"master.key",
			"credentials.xml",
			"app.creds",
			"db-creds.json",
			"db-creds.yaml",
			"db-creds.yml",
			"db-creds.toml",
		},
	}

	for category, files := range sensitiveFiles {
		for _, relPath := range files {
			t.Run("sensitive/"+category+"/"+relPath, func(t *testing.T) {
				full := mkFile(relPath)
				assertSensitive(t, c, full)
			})
		}
	}

	// ── Files that MUST NOT be falsely flagged ──

	safeFiles := []string{
		"project/README.md",
		"project/src/env.go",
		"project/cmd/sensitive.go",
		"project/docs/ssh-guide.md",
		"project/main.go",
		"project/internal/config/config.go",
		"project/go.mod",
		"project/go.sum",
		"project/Taskfile.yml",
		"project/.editorconfig",
		"project/data.txt",
		"project/config.toml",
	}

	for _, relPath := range safeFiles {
		t.Run("safe/"+relPath, func(t *testing.T) {
			full := mkFile(relPath)
			assertAllowed(t, c, full)
		})
	}
}

// TestSensitivePatternCount ensures we maintain a substantial list.
func TestSensitivePatternCount(t *testing.T) {
	if len(sensitivePatterns) < 100 {
		t.Errorf("expected at least 100 sensitive patterns, got %d", len(sensitivePatterns))
	}
}

// TestCaseInsensitiveBasename tests that basename matching is case-insensitive.
func TestCaseInsensitiveBasename(t *testing.T) {
	root := t.TempDir()

	// Create a file with uppercase basename.
	sshDir := filepath.Join(root, ".ssh")
	os.MkdirAll(sshDir, 0o700) //nolint:errcheck

	keyFile := filepath.Join(sshDir, "ID_RSA")
	os.WriteFile(keyFile, []byte("fake"), 0o600) //nolint:errcheck

	c, err := NewChecker(CheckerParams{
		Allow: []string{root + "/**"},
	})
	if err != nil {
		t.Fatal(err)
	}

	// The pattern is for id_rsa (lowercase) but ID_RSA should match too.
	assertSensitive(t, c, keyFile)
}

// TestDenyTakesPrecedenceOverAllow verifies evaluation order.
func TestDenyTakesPrecedenceOverAllow(t *testing.T) {
	root := t.TempDir()
	secretFile := filepath.Join(root, "data", "secret.txt")
	normalFile := filepath.Join(root, "data", "normal.txt")
	os.MkdirAll(filepath.Dir(secretFile), 0o755) //nolint:errcheck
	os.WriteFile(secretFile, []byte("s"), 0o644) //nolint:errcheck
	os.WriteFile(normalFile, []byte("n"), 0o644) //nolint:errcheck

	c, err := NewChecker(CheckerParams{
		Allow: []string{root + "/**"},
		Deny:  []string{root + "/data/secret.*"},
	})
	if err != nil {
		t.Fatal(err)
	}

	assertDenied(t, c, secretFile)
	assertAllowed(t, c, normalFile)
}

// TestSensitiveTakesPrecedenceOverAll verifies sensitive > deny > allow.
func TestSensitiveTakesPrecedenceOverAll(t *testing.T) {
	root := t.TempDir()

	envFile := filepath.Join(root, ".env")
	os.WriteFile(envFile, []byte("SECRET=x"), 0o644) //nolint:errcheck

	c, err := NewChecker(CheckerParams{
		Allow: []string{root + "/**"},
		// Even an explicit deny for a sensitive file should return SENSITIVE, not DENIED.
		Deny: []string{root + "/.env"},
	})
	if err != nil {
		t.Fatal(err)
	}

	assertSensitive(t, c, envFile)
}

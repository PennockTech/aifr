// Copyright 2026 — see LICENSE file for terms.
package accessctl

// sensitivePatterns is the built-in list of file patterns that are always
// denied. These are compiled into matchers at Checker construction time.
// Patterns use doublestar semantics (github.com/bmatcuk/doublestar/v4).
//
// This list is an implicit deny that cannot be overridden by the allow-list.
// It returns the distinct ACCESS_DENIED_SENSITIVE error.
var sensitivePatterns = []string{
	// ── SSH & GPG ──
	"**/.ssh/*id_*",
	"**/.ssh/authorized_keys",
	"**/.ssh/known_hosts",
	"**/.ssh/config",
	"**/.gnupg/**",
	"**/.gpg/**",

	// ── Cloud provider credentials ──
	"**/.aws/credentials",
	"**/.aws/config",
	"**/.azure/accessTokens.json",
	"**/.azure/azureProfile.json",
	"**/.config/gcloud/credentials.db",
	"**/.config/gcloud/application_default_credentials.json",
	"**/.config/gcloud/access_tokens.db",
	"**/.boto",
	"**/service-account*.json",

	// ── Kubernetes & container ──
	"**/.kube/config",
	"**/.kube/cache/**",
	"**/.docker/config.json",
	"**/.docker/daemon.json",
	"**/kubeconfig*",

	// ── NATS ──
	"**/*.nk",
	"**/nats/context/*.json",
	"**/nats/context/*.json.*",

	// ── Package manager tokens ──
	"**/.npmrc",
	"**/.yarnrc",
	"**/.pypirc",
	"**/.gem/credentials",
	"**/.cargo/credentials",
	"**/.cargo/credentials.toml",
	"**/NuGet.Config",
	"**/.nuget/NuGet.Config",
	"**/.composer/auth.json",

	// ── Application secrets & env files ──
	"**/.env",
	"**/.env.*",
	"**/env.local",
	"**/.envrc",
	"**/*.pem",
	"**/*.key",
	"**/*.p12",
	"**/*.pfx",
	"**/*.jks",
	"**/*.keystore",
	"**/secrets.yml",
	"**/secrets.yaml",
	"**/secrets.json",
	"**/vault.yml",
	"**/vault.yaml",

	// ── Shell & editor history ──
	"**/.bash_history",
	"**/.zsh_history",
	"**/.histfile",
	"**/.python_history",
	"**/.node_repl_history",
	"**/.psql_history",
	"**/.mysql_history",
	"**/.sqlite_history",
	"**/.lesshst",
	"**/.viminfo",

	// ── Database files ──
	"**/.pgpass",
	"**/.my.cnf",
	"**/.mongoshrc.js",
	"**/mongos.conf",
	"**/.dbshell",

	// ── OS credential stores ──
	"**/login.keychain*",
	"**/.local/share/keyrings/**",
	"**/.gnome2/keyrings/**",
	"**/kwallet*",

	// ── CI/CD & tool tokens ──
	"**/.netrc",
	"**/.curlrc",
	"**/.wgetrc",
	"**/.git-credentials",
	"**/.config/gh/hosts.yml",
	"**/.config/hub",
	"**/.travis.yml",
	"**/netlify.toml",
	"**/.circleci/config.yml",
	"**/.github/**secrets**",
	"**/.vault-token",
	"**/.terraform/*.tfstate",
	"**/terraform.tfvars",

	// ── Browser & app data ──
	"**/.config/chromium/**/Login Data*",
	"**/.config/google-chrome/**/Login Data*",
	"**/Cookies",
	"**/Cookies-journal",

	// ── Password managers & age encryption ──
	"**/.password-store/**",
	"**/.age/*.txt",
	"**/*.age",
	"**/age-key.txt",
	"**/.sops.yaml",
	"**/.1password/**",
	"**/.op/**",
	"**/bitwarden-*",
	"**/.lastpass/**",

	// ── TLS & certificates (private) ──
	"**/tls.key",
	"**/tls.crt",

	// ── Miscellaneous secrets ──
	"**/*.secret",
	"**/htpasswd*",
	"**/shadow",
	"**/passwd",
	"**/master.key",
	"**/credentials.xml",
	"**/*.creds",
	"**/*.creds.*",
	"**/*-creds.json",
	"**/*-creds.yaml",
	"**/*-creds.yml",
	"**/*-creds.toml",
}

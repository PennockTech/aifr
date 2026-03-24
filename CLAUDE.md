# CLAUDE.md — aifr (AI File Reader)

## Project Summary

`aifr` is a read-only filesystem and git-tree access tool for AI coding agents.
It exists because AI agents (Claude Code, etc.) resort to shell pipelines
(`sed -n`, `find | grep`, `head/tail`) that trigger security checks. `aifr`
replaces all of those with a single binary that is *always safe* because it
can never write, and is *always scoped* because it enforces allow/deny lists
with a built-in sensitive-file blocklist.

The tool is dual-mode: a standalone CLI and an MCP server. Installing the MCP
server also exposes a skill file that teaches CLI usage, so the agent can use
whichever interface is appropriate.

**Language:** Go (1.22+)
**License:** TBD by maintainer
**Repository layout:** Standard Go project (`cmd/`, `internal/`, `pkg/`)

---

## Architecture Overview

```
┌─────────────────────────────────────────────────────┐
│                    aifr binary                      │
│                                                     │
│  ┌───────────┐  ┌────────────┐  ┌────────────────┐ │
│  │  CLI      │  │  MCP       │  │  Skill file    │ │
│  │  (cobra)  │  │  (stdio /  │  │  generator     │ │
│  │           │  │   http)    │  │  (emits        │ │
│  │           │  │            │  │   SKILL.md)    │ │
│  └─────┬─────┘  └─────┬──────┘  └────────────────┘ │
│        │              │                             │
│        └──────┬───────┘                             │
│               ▼                                     │
│  ┌─────────────────────────────────────────┐        │
│  │          Core Engine (internal/engine)  │        │
│  │                                         │        │
│  │  ┌───────────┐  ┌──────────────┐       │        │
│  │  │ Access    │  │ Reader       │       │        │
│  │  │ Control   │  │ (fs + git)   │       │        │
│  │  │           │  │              │       │        │
│  │  │ allow     │  │ chunk/stream │       │        │
│  │  │ deny      │  │ line/byte/   │       │        │
│  │  │ sensitive │  │ continuation │       │        │
│  │  └───────────┘  └──────────────┘       │        │
│  │                                         │        │
│  │  ┌───────────┐  ┌──────────────┐       │        │
│  │  │ Git       │  │ Cache        │       │        │
│  │  │ Provider  │  │ (MCP mode    │       │        │
│  │  │ (go-git)  │  │  only)       │       │        │
│  │  └───────────┘  └──────────────┘       │        │
│  └─────────────────────────────────────────┘        │
└─────────────────────────────────────────────────────┘
```

---

## Module / Package Layout

```
aifr/
├── cmd/
│   └── aifr/
│       └── main.go                 # Entrypoint, cobra root command
├── internal/
│   ├── accessctl/
│   │   ├── accessctl.go            # Allow/deny/sensitive evaluation
│   │   ├── accessctl_test.go
│   │   ├── sensitive.go            # Built-in sensitive file patterns
│   │   └── sensitive_test.go
│   ├── engine/
│   │   ├── engine.go               # Core read/list/search/stat orchestrator
│   │   ├── engine_test.go
│   │   ├── chunk.go                # Chunked reading logic
│   │   ├── chunk_test.go
│   │   ├── search.go               # grep/glob/find operations
│   │   └── search_test.go
│   ├── gitprovider/
│   │   ├── provider.go             # Git tree/blob resolution via go-git
│   │   ├── provider_test.go
│   │   ├── cache.go                # In-memory LRU cache for tree/blob objects
│   │   ├── cache_test.go
│   │   ├── watcher.go              # Watches .git/index for invalidation
│   │   └── watcher_test.go
│   ├── config/
│   │   ├── config.go               # Config file parsing + CLI flag merging
│   │   └── config_test.go
│   ├── mcpserver/
│   │   ├── server.go               # MCP stdio/http transport
│   │   ├── tools.go                # MCP tool definitions + handlers
│   │   ├── resources.go            # MCP resource definitions (optional)
│   │   └── skill.go                # Embedded SKILL.md content + emit command
│   └── output/
│       ├── json.go                 # JSON output formatting
│       └── text.go                 # Plain text output formatting
├── pkg/
│   └── protocol/
│       ├── types.go                # Shared request/response types
│       └── errors.go               # Error type constants
├── configs/
│   └── aifr.example.toml           # Example configuration
├── CLAUDE.md                       # This file
├── go.mod
└── go.sum
```

---

## Configuration

TOML config file. Searched in order:
1. `--config` flag
2. `./.aifr.toml`
3. `$XDG_CONFIG_HOME/aifr/config.toml`
4. `~/.config/aifr/config.toml`

```toml
# .aifr.toml

# Paths the tool is allowed to read.
# Supports globs. Paths are resolved to absolute before matching.
# If empty, the tool operates in "current directory only" mode:
# it allows the cwd and everything beneath it.
allow = [
  "/home/user/projects/**",
  "/etc/nats/**",
]

# Explicit deny patterns. Evaluated AFTER allow, so deny wins.
# The built-in sensitive-file list is always active in addition to this.
deny = [
  "/home/user/projects/secrets/**",
  "**/.env.production",
]

# Git repositories to expose tree access for.
# Each entry maps a name to a repo path.
# If empty, git operations use the repo found by walking up from cwd.
[git.repos]
main = "/home/user/projects/myapp"
infra = "/home/user/projects/infra"

# Cache settings (MCP mode only; CLI is stateless)
[cache]
max_entries = 10000          # Max cached git objects
max_memory_mb = 256          # Soft memory ceiling
ttl_seconds = 300            # TTL for cached items (git index watcher forces eviction anyway)
```

---

## Access Control Model

### Evaluation Order

For every path access request:

```
1. Resolve to absolute, canonical path (resolve symlinks)
2. Check SENSITIVE list → if match → AccessDeniedSensitive error
3. Check DENY list     → if match → AccessDenied error
4. Check ALLOW list    → if match → permit
5. Default             → AccessDenied error
```

### Error Types

| Error Code              | Meaning                                                           |
|-------------------------|-------------------------------------------------------------------|
| `ACCESS_DENIED`         | Path is outside the allow-list or in the deny-list                |
| `ACCESS_DENIED_SENSITIVE` | Path matches the built-in sensitive file list. The agent MUST NOT retry. It should tell the user to read the file themselves through regular tools if they choose to. |
| `NOT_FOUND`             | Path does not exist                                               |
| `IS_DIRECTORY`          | A file-read operation was attempted on a directory                |
| `INVALID_REF`           | Git ref does not exist                                            |
| `CHUNK_OUT_OF_RANGE`    | Requested chunk/offset exceeds file bounds                        |

### Built-in Sensitive File Patterns

The `internal/accessctl/sensitive.go` file MUST contain a comprehensive list.
This list is an *implicit deny* that is always active, cannot be overridden by
the allow-list, and returns the distinct `ACCESS_DENIED_SENSITIVE` error.

The agent implementor should maintain this list. Initial categories and
examples (target ~200+ patterns):

**SSH & GPG:**
```
**/.ssh/id_*
**/.ssh/authorized_keys
**/.ssh/known_hosts
**/.ssh/config
**/.gnupg/**
**/.gpg/**
```

**Cloud provider credentials:**
```
**/.aws/credentials
**/.aws/config
**/.azure/accessTokens.json
**/.azure/azureProfile.json
**/.config/gcloud/credentials.db
**/.config/gcloud/application_default_credentials.json
**/.config/gcloud/access_tokens.db
**/.boto
**/service-account*.json
```

**Kubernetes & container:**
```
**/.kube/config
**/.kube/cache/**
**/.docker/config.json
**/.docker/daemon.json
**/kubeconfig*
```

**Package manager tokens:**
```
**/.npmrc
**/.yarnrc
**/.pypirc
**/.gem/credentials
**/.cargo/credentials
**/.cargo/credentials.toml
**/NuGet.Config
**/.nuget/NuGet.Config
**/.composer/auth.json
```

**Application secrets & env files:**
```
**/.env
**/.env.*
**/env.local
**/.envrc
**/*.pem
**/*.key
**/*.p12
**/*.pfx
**/*.jks
**/*.keystore
**/secrets.yml
**/secrets.yaml
**/secrets.json
**/vault.yml
**/vault.yaml
```

**Shell & editor history (may contain secrets):**
```
**/.bash_history
**/.zsh_history
**/.histfile
**/.python_history
**/.node_repl_history
**/.psql_history
**/.mysql_history
**/.sqlite_history
**/.lesshst
**/.viminfo
```

**Database files:**
```
**/.pgpass
**/.my.cnf
**/.mongoshrc.js
**/mongos.conf
**/.dbshell
```

**OS credential stores:**
```
**/login.keychain*
**/.local/share/keyrings/**
**/.gnome2/keyrings/**
**/kwallet*
```

**CI/CD & tool tokens:**
```
**/.netrc
**/.curlrc
**/.wgetrc
**/.git-credentials
**/.config/gh/hosts.yml
**/.config/hub
**/.travis.yml            # often contains encrypted secrets
**/netlify.toml
**/.circleci/config.yml
**/.github/**secrets**
**/.vault-token
**/.terraform/*.tfstate   # may contain provider secrets
**/terraform.tfvars
```

**Browser & app data:**
```
**/.config/chromium/**/Login Data*
**/.config/google-chrome/**/Login Data*
**/Cookies
**/Cookies-journal
```

**Miscellaneous:**
```
**/.password-store/**
**/.age/*.txt
**/age-key.txt
**/.sops.yaml
**/.1password/**
**/.op/**
**/bitwarden-*
**/.lastpass/**
**/tls.key
**/tls.crt
**/*.secret
**/htpasswd*
**/shadow
**/passwd               # only /etc/passwd and similar
**/master.key
**/credentials.xml      # Jenkins
```

**Implementation notes:**
- Patterns use `doublestar` semantics (Go: `github.com/bmatcuk/doublestar/v4`).
- The list is compiled to a matcher at startup; there is no per-request overhead.
- Matching is case-insensitive on the basename component only (for Windows compat if ever needed).
- Symlinks are resolved before matching, so a symlink to `~/.ssh/id_rsa` is still blocked.

---

## Core Operations

### 1. `stat` — File/directory metadata

```
aifr stat <path>
aifr stat <repo>:<ref>:<path>
```

Returns: type (file/dir/symlink), size, mode, mtime. For git objects: type, size, object hash.

### 2. `read` — Chunked file reading

```
aifr read [--lines START:END] [--bytes START:END] [--chunk-id ID] <path>
aifr read [--lines START:END] [--bytes START:END] [--chunk-id ID] <repo>:<ref>:<path>
```

**Chunking modes (mutually exclusive):**

| Mode | Flag | Behavior |
|------|------|----------|
| Line range | `--lines 1:50` | 1-indexed, inclusive. `--lines 50:` means "50 to EOF" |
| Byte range | `--bytes 0:4095` | 0-indexed, inclusive. Aligns to sane boundaries (see below) |
| Continuation | `--chunk-id <id>` | Resume from a prior chunk's continuation token |
| Whole file | *(none)* | Returns entire file if ≤ default chunk size (64 KiB), else returns first chunk + continuation token |

**Sane chunk boundaries (byte mode):**

When the caller requests a byte range, `aifr` adjusts the end boundary:
- If the file is text (heuristic: no NUL bytes in first 8 KiB), extend the
  end to the next newline character (or EOF), up to 1 KiB past the requested
  end. This avoids splitting mid-line.
- If the file is binary, align to the nearest 4 KiB boundary (page-aligned),
  unless it would exceed 8 KiB past the requested end.

**Continuation token:**

Every response that does not include the entire file includes a `continuation`
field. The token is opaque to the caller and encodes:
- File identity (path + mtime, or git object hash)
- Byte offset of next chunk
- Chunk size hint

If the file has changed since the token was issued, the operation returns a
`STALE_CONTINUATION` error with a hint to re-read from the beginning.

**JSON response structure (read):**

```json
{
  "path": "/home/user/projects/myapp/main.go",
  "source": "filesystem",
  "total_size": 28430,
  "total_lines": 412,
  "chunk": {
    "start_byte": 0,
    "end_byte": 4117,
    "start_line": 1,
    "end_line": 50,
    "data": "package main\n\nimport (\n\t\"fmt\"\n...",
    "encoding": "utf-8",
    "truncated_at": "newline"
  },
  "continuation": "eyJwIjoiL2hvbWUvdXNlci9...",
  "complete": false
}
```

For git sources:
```json
{
  "path": "src/main.go",
  "source": "git",
  "repo": "main",
  "ref": "feature/auth",
  "ref_resolved": "a1b2c3d4e5f6...",
  "object_hash": "9f8e7d6c5b4a...",
  "total_size": 28430,
  "chunk": { ... },
  "continuation": "...",
  "complete": false
}
```

### 3. `list` — Directory listing

```
aifr list [--depth N] [--pattern GLOB] [--type f|d|l] <path>
aifr list [--depth N] [--pattern GLOB] [--type f|d|l] <repo>:<ref>:<path>
```

- `--depth 0` means the directory itself (just its entries).
- `--depth N` recurses N levels.
- `--depth -1` is unlimited recursion.
- Results are streamed as JSON lines when output exceeds 1000 entries.
- Each entry includes: name, type, size, mode (filesystem) or hash (git).

### 4. `search` — Content search

```
aifr search [--regexp] [--fixed-string] [--ignore-case] [--context N] \
            [--max-matches N] [--include GLOB] [--exclude GLOB] \
            <pattern> <path>
aifr search [same flags] <pattern> <repo>:<ref>:<path>
```

Implements the subset of grep semantics an agent actually needs:
- Fixed-string or regexp matching (RE2, not PCRE).
- Context lines (default 0).
- Match limiting.
- File include/exclude globs.
- Operates recursively on directories by default.
- Returns results as structured JSON (file, line number, column, match, context lines).
- For git trees, reads blobs directly—never checks out files.

**JSON response structure (search):**

```json
{
  "pattern": "func.*Handler",
  "is_regexp": true,
  "root": "/home/user/projects/myapp",
  "source": "filesystem",
  "matches": [
    {
      "file": "api/handlers.go",
      "line": 42,
      "column": 1,
      "match": "func UserHandler(w http.ResponseWriter, r *http.Request) {",
      "context_before": ["", "// UserHandler handles user requests"],
      "context_after": ["\tctx := r.Context()"]
    }
  ],
  "files_searched": 87,
  "files_matched": 3,
  "total_matches": 12,
  "truncated": false
}
```

### 5. `find` — File/path search

```
aifr find [--name GLOB] [--path GLOB] [--type f|d|l] [--max-depth N] \
          [--min-size SIZE] [--max-size SIZE] [--newer-than DURATION] \
          <path>
aifr find [--name GLOB] [--path GLOB] [--type f|d|l] [--max-depth N] \
          <repo>:<ref>:<path>
```

Returns matching paths as structured JSON. This replaces `find ... | grep ...`
pipelines.

### 6. `diff` — Compare files or refs

```
aifr diff <path-a> <path-b>
aifr diff <repo>:<ref-a>:<path> <repo>:<ref-b>:<path>
aifr diff <repo>:<ref-a>:<path> <path>      # git vs working tree
```

Returns unified diff as structured JSON with hunks, or plain text with `--format text`.

### 7. `refs` — List git refs

```
aifr refs [--branches] [--tags] [--remotes] [<repo>]
```

Returns available refs for a repository. Useful for discovering what branches/tags
exist before reading from them.

### 8. `log` — Git commit log

```
aifr log [--max-count N] [--since DATE] [--until DATE] [--path PATH] [<repo>][:<ref>]
```

Returns structured commit log (hash, author, date, message, files changed).

---

## Git Tree Access

### Path Syntax

Git objects are addressed as: `[repo:]<ref>:<path>`

- `repo` is optional. If omitted, uses the git repo found by walking up from
  cwd (same as `git` itself). If a name is given, it maps through
  `[git.repos]` in the config.
- `ref` is any valid git ref: branch name, tag name, `HEAD`, `HEAD~3`,
  short/full commit hash, etc. Resolved via `go-git`.
- `path` is relative to the repo root.

Examples:
```
main:src/handler.go                    # "main" branch of auto-detected repo
infra:v2.1.0:terraform/main.tf         # "v2.1.0" tag of repo named "infra"
HEAD~3:README.md                       # 3 commits back
a1b2c3d:pkg/auth/auth.go              # specific commit
origin/feature/login:cmd/server/main.go
```

### Git Provider Implementation

Use `github.com/go-git/go-git/v5` for all git operations.

**Object resolution:**
1. Parse the ref portion → resolve to a commit hash.
2. Get the commit's tree object.
3. Walk the tree to find the requested path.
4. Read the blob content.

**Never check out files.** All reads go directly from the git object store
(packfiles / loose objects). This is safe, fast, and has zero side effects on
the working tree.

**Ref resolution must handle:**
- Branch names (local and remote-tracking)
- Tag names (including annotated tags → peel to commit)
- Commit hashes (full and abbreviated)
- Relative refs (`HEAD~N`, `branch^2`, etc.)
- `HEAD`

### Git Index Watcher

In MCP mode, the git provider watches `.git/index` (and `.git/refs/`,
`.git/packed-refs`) using `fsnotify` to detect when:
- New commits are made
- Branches are created/deleted/updated
- Tags are added/removed
- Rebases, merges, or resets occur

On change detection:
1. Invalidate all cached tree/blob objects for the affected repo.
2. Re-resolve any refs that may have changed.
3. Do NOT eagerly re-populate the cache—let the next request do it.

**Debounce:** Coalesce rapid filesystem events (e.g., during a `git rebase`)
with a 100ms debounce window.

In CLI mode, there is no watcher. Every invocation resolves refs fresh.

### Git Cache (MCP mode)

LRU cache keyed by `(repo, object_hash)`.

Cached objects:
- Tree objects (directory listings)
- Blob content (file contents, up to a size threshold—default 1 MiB per blob)
- Ref → commit hash mappings (short TTL, invalidated by watcher)

NOT cached:
- Search results (too variable)
- Diff results (computed on the fly)

Cache is in-process memory only. No disk persistence. The MCP server is
expected to be long-lived, so the cache is useful for repeated reads of the
same ref/path within a session.

---

## CLI Design

```
aifr [global flags] <command> [command flags] [args]

Global flags:
  --config PATH       Config file path
  --format json|text  Output format (default: json)
  --quiet             Suppress non-essential output
  --version           Print version

Commands:
  read        Read file contents (chunked)
  stat        File/directory metadata
  list        Directory listing
  search      Content search (grep-like)
  find        Path/name search (find-like)
  diff        Compare files or refs
  refs        List git refs
  log         Git commit log
  config      Show effective configuration
  mcp         Start MCP server
  skill       Emit SKILL.md to stdout
  sensitive   List built-in sensitive patterns (for auditing)
```

**All commands** return structured JSON by default (for agent consumption).
`--format text` is available for human use.

**Exit codes:**
- 0: Success
- 1: General error
- 2: Access denied
- 3: Access denied (sensitive file)
- 4: Not found
- 10: Invalid arguments

---

## MCP Server Design

### Transport

Support both `stdio` (default, for Claude Code integration) and
`streamable-http` (for multi-client setups).

### Tool Definitions

The MCP server exposes these tools, each mapping 1:1 to a CLI command:

```
aifr_read       Read file contents with chunking
aifr_stat       Get file/directory metadata
aifr_list       List directory contents
aifr_search     Search file contents (grep-like)
aifr_find       Find files by name/path pattern
aifr_diff       Compare files or git refs
aifr_refs       List git branches/tags
aifr_log        Git commit history
```

Each tool's MCP description MUST include:
- One-line summary
- Parameter descriptions with examples
- Git path syntax explanation
- Error codes the tool can return
- Example invocations

### MCP Tool Descriptions

Tool descriptions should be concise but self-contained. The agent reading
them should be able to use the tool without external documentation. Example
for `aifr_read`:

```
Read file contents with optional chunking. Supports filesystem paths and git refs.

Path syntax:
  /absolute/path          → filesystem file
  relative/path           → relative to allowed roots
  branch:path             → git tree (auto-detected repo)
  reponame:ref:path       → named git repo at ref

Chunking (mutually exclusive):
  lines: "1:50"           → lines 1-50 (1-indexed, inclusive)
  bytes: "0:4095"         → bytes 0-4095 (auto-adjusts to sane boundary)
  chunk_id: "<token>"     → continue from previous chunk

Returns: file content, metadata, and continuation token if incomplete.
Errors: ACCESS_DENIED_SENSITIVE means the file looks like a credential
        and the user should be asked to read it manually if needed.
```

### MCP Resources (Optional)

Consider exposing these as MCP resources:
- `aifr://config` — current effective configuration
- `aifr://sensitive-patterns` — the sensitive file pattern list
- `aifr://git/{repo}/refs` — available refs for a repo

### Skill File

`aifr skill` emits a SKILL.md file suitable for placement in
`~/.claude/skills/` or a project's `.claude/skills/` directory.

The skill file MUST contain:
- Tool name and description
- When to use aifr instead of raw shell commands
- Complete CLI command reference with examples
- Git path syntax
- Chunked reading workflow (read → continuation → read → ...)
- Error handling guidance (especially `ACCESS_DENIED_SENSITIVE`)
- Configuration overview

The skill file content is embedded in the binary at compile time
(via `//go:embed`).

### MCP + CLI Synergy

When `aifr mcp` starts, it SHOULD log (to stderr) a message like:

```
aifr MCP server started. CLI is also available:
  aifr read <path>       # read files
  aifr search <pat> <p>  # search content
  aifr list <path>       # list directories
Run 'aifr skill' to generate a Claude Code skill file.
```

The MCP tool descriptions should mention that the same operations are
available via CLI, so the agent can choose based on context (e.g., CLI for
one-off reads, MCP for cached/streamed access).

---

## Streaming / Chunk Boundary Design

The core challenge: an agent asks for "the next chunk" and needs to get
something useful, not content split mid-token or mid-JSON-string.

### Text Files

**Line-based chunking** is the default for text files. The agent requests
a line range, and gets exactly those lines. This is the simplest and most
predictable mode.

**Byte-based chunking** with sane boundaries:
1. Read the requested byte range.
2. If the byte range ends mid-line, extend forward to the next `\n` (up to
   1 KiB overshoot).
3. If the overshoot would exceed 1 KiB, truncate at the last `\n` within
   the range instead (shrink rather than grow).
4. Report the actual byte range delivered.

**Continuation-based streaming:**
1. First `read` with no range returns the first chunk (default: 64 KiB worth
   of complete lines).
2. Response includes `continuation` token.
3. Agent calls `read --chunk-id <token>` for the next chunk.
4. Repeat until `"complete": true`.

The continuation token encodes enough state that the server doesn't need to
hold open file handles between requests. In MCP mode, the server MAY cache
the file content for the duration of the continuation sequence to ensure
consistency (invalidated if the file changes).

### Binary Files

Binary files use byte-based chunking aligned to 4 KiB boundaries. The
response includes base64-encoded data with a `"encoding": "base64"` field.

Binary detection heuristic: file contains NUL bytes in the first 8 KiB.

### Streaming in JSON

For operations that produce large result sets (search, list, find), results
are returned as a JSON object with an array field. If the result set exceeds
a threshold (default: 1000 entries for list, 500 matches for search), the
response is paginated with a continuation token, identical in semantics to
file reading.

```json
{
  "matches": [ ... ],
  "total_matches": 2847,
  "returned": 500,
  "continuation": "...",
  "complete": false
}
```

---

## Dependencies

**Required:**
- `github.com/go-git/go-git/v5` — Git operations
- `github.com/spf13/cobra` — CLI framework
- `github.com/pelletier/go-toml/v2` — Config parsing
- `github.com/bmatcuk/doublestar/v4` — Glob matching
- `github.com/fsnotify/fsnotify` — File watching (git index)

**MCP protocol:**
- Evaluate `github.com/mark3labs/mcp-go` or implement the stdio JSON-RPC
  protocol directly (it's simple enough). Prefer a maintained library if
  one exists at sufficient quality; otherwise a minimal in-tree implementation
  of the MCP tool-call subset is acceptable. Do NOT take a dependency on
  a Node.js runtime.

**Avoid:**
- CGo dependencies (keep the binary statically compilable)
- FUSE / NFS / kernel modules
- Any write-capable filesystem abstractions

---

## Testing Strategy

### Unit Tests

Every package gets `_test.go` files:
- `accessctl`: Test all three evaluation paths (sensitive, deny, allow).
  Test symlink resolution. Test glob edge cases. Test the full sensitive
  file list against known credential paths.
- `engine`: Test chunking boundary logic extensively. Test continuation
  token round-trips. Test stale detection.
- `gitprovider`: Use `go-git`'s in-memory repository for tests. Test ref
  resolution, tree walking, blob reading. Test cache invalidation.
- `config`: Test config file merging, default behavior, environment variable
  overrides.

### Integration Tests

A `testdata/` directory with:
- A small git repository (committed as test fixtures)
- Files with known content for chunk boundary testing
- Symlinks to test resolution
- A `.env` file and fake `id_rsa` to test sensitive blocking

### Test Assertions for Access Control

```go
// These MUST be tested and MUST return ACCESS_DENIED_SENSITIVE:
assertSensitive(t, "~/.ssh/id_rsa")
assertSensitive(t, "~/.ssh/id_ed25519")
assertSensitive(t, "~/.aws/credentials")
assertSensitive(t, "/home/user/.env")
assertSensitive(t, "/home/user/project/.env.production")
assertSensitive(t, "~/.kube/config")
assertSensitive(t, "~/.gnupg/private-keys-v1.d/ABC123.key")
assertSensitive(t, "~/.config/gcloud/application_default_credentials.json")
assertSensitive(t, "/etc/shadow")
assertSensitive(t, "~/.netrc")
assertSensitive(t, "~/.git-credentials")
assertSensitive(t, "~/.config/gh/hosts.yml")
assertSensitive(t, "/home/user/project/secrets.yaml")

// These MUST NOT be falsely flagged:
assertNotSensitive(t, "/home/user/project/README.md")
assertNotSensitive(t, "/home/user/project/src/env.go")
assertNotSensitive(t, "/home/user/project/cmd/sensitive.go")
assertNotSensitive(t, "/home/user/project/.envrc.example")
assertNotSensitive(t, "/home/user/project/docs/ssh-guide.md")
```

---

## Build & Release

```bash
# Build
go build -o aifr ./cmd/aifr

# With version info
go build -ldflags "-X main.version=$(git describe --tags)" -o aifr ./cmd/aifr

# Cross-compile
GOOS=linux   GOARCH=amd64 go build -o aifr-linux-amd64   ./cmd/aifr
GOOS=darwin  GOARCH=arm64 go build -o aifr-darwin-arm64   ./cmd/aifr
GOOS=freebsd GOARCH=amd64 go build -o aifr-freebsd-amd64 ./cmd/aifr
```

FreeBSD support is a first-class target (for the maintainer's NAS).

---

## Open Design Questions for Implementor

These are decisions the implementing agent (Claude Code) should make and
document as it builds. Record decisions in a `DECISIONS.md` file.

1. **MCP library choice:** Evaluate `mcp-go` vs in-tree implementation.
   The protocol surface needed is small (tool listing, tool calls, optional
   resources). Document the tradeoff.

2. **Continuation token format:** Should it be a signed token (HMAC to
   prevent tampering) or a simple base64-encoded struct? The threat model
   is that a rogue prompt injection might craft a token to read files
   outside the allow-list. The token should encode the original path and
   the access control result, so re-validation is cheap but tampering is
   caught.

3. **Large file handling:** What happens when an agent asks to `read` a
   2 GiB log file with no range? Current spec says "returns first chunk +
   continuation." Should there be a size warning threshold? (Suggest: yes,
   and the response should include `"warning": "file_large"` with the
   total size, so the agent can decide whether to continue.)

4. **Symlink policy for git trees:** Go-git doesn't always resolve git
   symlinks the same way as a working tree. Document the behavior and
   decide if we follow the symlink into the tree or return the link target
   as text.

5. **MCP notification of git ref changes:** When the watcher detects ref
   changes, should the MCP server proactively notify the client (via MCP
   notifications/logging) or silently invalidate the cache? Proactive
   notification is friendlier but requires the client to handle it.

6. **Search in git trees:** Searching across an entire tree at a ref
   requires reading every blob. For large repos this is expensive. Should
   there be a limit (e.g., max files to search, max total blob size)?
   Should search results be cached in MCP mode?

7. **`.gitignore` awareness:** When listing or searching the filesystem
   (not git tree), should `aifr` respect `.gitignore` by default? Probably
   yes for `list` and `search`, with `--no-gitignore` to override.

---

## Non-Goals

- **Writing files.** This tool never writes. Not even "safe" writes like
  creating temp files. The filesystem is read-only through this tool.
- **Executing commands.** No shell-out, no subprocess spawning.
- **Network access.** No HTTP fetching, no remote git clones. It reads
  local repos only.
- **FUSE/NFS/kernel anything.** Pure userspace, pure Go.
- **LSP/language awareness.** This is a file reader, not a code intelligence
  tool. It reads bytes and lines. Pair it with ast-grep or tree-sitter
  tools if you need structural code understanding.
- **Windows support.** Not a priority. Don't break it gratuitously, but
  don't contort the design for Windows path semantics.

---

## Implementation Order

Suggested build sequence for the implementing agent:

1. **Skeleton:** `cmd/aifr/main.go`, cobra commands, `--version`, `--help`.
2. **Config:** Parse TOML, merge with defaults, test.
3. **Access control:** `internal/accessctl/` with full sensitive list. Test thoroughly.
4. **Engine — filesystem reads:** `read`, `stat`, `list` for local files. Chunking.
5. **Engine — search/find:** `search`, `find` for local files.
6. **CLI wiring:** All commands working end-to-end with JSON output.
7. **Git provider:** `go-git` integration, ref resolution, tree/blob reading.
8. **Git operations:** `read`, `list`, `search`, `stat`, `diff` on git trees.
9. **Git commands:** `refs`, `log`.
10. **MCP server:** Tool definitions, stdio transport.
11. **MCP caching:** LRU cache, git index watcher, invalidation.
12. **Skill file:** Embedded `SKILL.md`, `aifr skill` command.
13. **Diff:** Cross-source diff (git vs git, git vs filesystem).
14. **Polish:** Error messages, edge cases, `--format text`, documentation.

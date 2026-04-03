aifr — AI File Reader
=====================

A read-only filesystem and git-tree access tool, with both a standalone CLI
and a JSON-RPC co-process mode which happens to fit the MCP schema for AI
agents.

AI agents (Claude Code, Cursor, etc.) resort to shell pipelines
(`sed -n`, `find | grep`, `head/tail`) that trigger security checks and
produce brittle, unstructured output. `aifr` replaces all of those with a
single binary that is always safe (never writes) and always scoped (enforces
allow/deny lists with a built-in sensitive-file blocklist).

The `aifr` tool can read files, read git state, read "getent" system files,
some basic system information,  and more.  Everything read-only (except the
MCP self reload task).  No network access, but some network state querying.
It never runs external commands, there is no possibility for shell injection
attacks.  All functionality is "native" to this tool.

A number of filters traditionally done with the Unix pipe handling are moved
left into aifr, to limit the output without needing a pipeline.  This avoids
triggering security checks for a shell invocation, even when using `aifr`
purely as a CLI tool, let alone when you start it as a co-process server so
that it can quickly answer many questions.  The queries often support options
to filter which data rows are returned, which columns, to sort, and more.

Outside of AI, scripts which do a lot of git operations often end up
repeatedly shelling out to git.  With this agent running as a co-process, they
can query configuration and contents and some history with a read-only tool:
safe, because it will not touch anything or break the repo; at worst, it will
fail to parse something, not break your setup.

Combine with tools such as [`ast-grep`](https://ast-grep.github.io) and
[`ripgrep`](https://github.com/burntsushi/ripgrep).  `aifr` does not seek to
replace good tools, it seeks to replace permission-triggering pipeline
invocations.


## Install

Binary releases are available; currently hosted at:
<https://github.com/PennockTech/aifr/releases>

Or you can build from source.

```sh
# From source
go install go.pennock.tech/aifr/cmd/aifr@latest

# Or build from the repo
task build
```

### MCP Server for Claude Code

```sh
claude mcp add --scope user aifr -- aifr mcp
```

This registers `aifr` as an MCP server using stdio transport. Claude Code
will then have access to all 20 aifr tools (`aifr_read`, `aifr_cat`,
`aifr_stat`, `aifr_list`, `aifr_search`, `aifr_find`, `aifr_refs`,
`aifr_log`, `aifr_diff`, `aifr_wc`, `aifr_hexdump`, `aifr_checksum`,
`aifr_pathfind`, `aifr_rev_parse`, `aifr_reflog`, `aifr_stash_list`,
`aifr_sysinfo`, `aifr_getent`, `aifr_git_config`, `aifr_self`).

### Skill File

The MCP server's built-in instructions are intentionally brief. For richer
agent guidance (tool routing, advanced patterns, error handling), install the
skill file:

```sh
# Symlink into user-level skills (recommended — stays in sync with repo)
mkdir -p ~/.claude/skills
ln -s "$(pwd)/skills/aifr" ~/.claude/skills/aifr

# Or copy if you prefer a snapshot
cp -r skills/aifr ~/.claude/skills/aifr
```

You can also generate the embedded (shorter) skill file at any time:

```sh
aifr skill > /tmp/aifr-skill.md
```

### HTTP Transport

For HTTP transport (multi-client setups):

```sh
aifr mcp --transport http --addr :8080
```


## Quick Start

```sh
# Read a file
aifr read src/main.go

# Read first 50 lines
aifr read --lines 1:50 src/main.go

# Read a file from git history
aifr read HEAD~3:src/main.go

# Concatenate all Go files, skipping vendor
aifr cat --name '*.go' --exclude-path '**/vendor/**' .

# Head: first 10 lines of each Go file with dividers
aifr cat --name '*.go' --lines 10 --divider plain --format text .

# Read with line numbers (like cat -n, but showing actual file line numbers)
aifr read -n src/main.go
aifr read -n --lines 50:100 src/main.go    # lines 50–100, numbered correctly

# Search for a pattern
aifr search 'func.*Handler' ./src/

# Find files by name
aifr find --name '*.yaml' --max-depth 2 .

# List directory
aifr list --depth -1 --type f .

# Git refs and log
aifr refs --branches --tags
aifr log --max-count 5
aifr log --oneline --max-count 10
aifr log --format=text --divider=xml --max-count 3
aifr log --verbose --max-count 1        # tree hash, parent hashes, committer

# Compare files across refs
aifr diff HEAD~1:README.md README.md

# Count lines/words/bytes
aifr wc src/main.go
aifr wc --lines src/*.go              # just line counts

# System databases
aifr getent passwd root
aifr getent passwd --fields name,gecos_name,home

# System inspection
aifr sysinfo --categories os,network
```

All commands output JSON by default. Use `--format text` for human-readable
output. Line numbering (`-n` / `--number-lines`) is available on `read` and
`cat` in both formats.


### Output Format

The default output format is JSON. Use `--format text` for human-readable,
token-efficient output suitable for AI agents and shell pipelines.

Set the `AIFR_FORMAT` environment variable to change the default:

```sh
export AIFR_FORMAT=text              # all commands default to text
aifr getent passwd root              # text output
aifr getent passwd root --format json  # flag overrides env
```

`AIFR_FORMAT` accepts a colon-separated preference list — the first format
supported by the invoked command wins:

```sh
export AIFR_FORMAT=short:text:json   # "version" uses short; others use text
```

Invalid values produce a diagnostic on stderr and exit 64 (`EX_USAGE`).


## Configuration

Create `.aifr.toml` in the working directory or `~/.config/aifr/config.toml`:

```toml
allow = ["~/projects/**"]
deny = ["~/projects/secrets/**"]

[git.repos]
myapp = "~/projects/myapp"
```

If no config is found, `aifr` operates in "current directory only" mode —
it allows the cwd and everything beneath it.

See `configs/aifr.example.toml` for the full config reference.


## Access Control

Every path request is evaluated in order:

1. Resolve symlinks to canonical path
2. **Sensitive list** (built-in, 120+ patterns) → blocked, always
3. **Deny list** (from config) → blocked
4. **Allow list** (from config, or cwd fallback) → permitted
5. **Default** → blocked

Sensitive files (SSH keys, cloud credentials, `.env`, shell history, etc.)
return a distinct `ACCESS_DENIED_SENSITIVE` error. The agent is expected to
tell the user to read the file themselves if needed, and must not retry.

Run `aifr sensitive` to see the full pattern list.


## Commands

| Command      | Description                                                 |
| ------------ | ----------------------------------------------------------- |
| `read`       | Read file contents (chunked, with continuation tokens)      |
| `cat`        | Concatenate multiple files with dividers (find+cat in one)  |
| `stat`       | File/directory metadata                                     |
| `list`       | Directory listing with depth, pattern, type filters         |
| `search`     | Content search (RE2 regexp, context lines, include/exclude) |
| `find`       | Find files by name, type, size, age                         |
| `diff`       | Compare files or git refs                                   |
| `wc`         | Count lines, words, and bytes                               |
| `hexdump`    | Hex dump of file contents                                   |
| `checksum`   | Compute file checksums                                      |
| `refs`       | List git branches, tags, remotes                            |
| `log`        | Git commit log (text/oneline/xml/json, verbose mode)        |
| `reflog`     | Show git reflog for a ref                                   |
| `stash-list` | List git stashes                                            |
| `rev-parse`  | Resolve a git ref to a commit hash                          |
| `git-config` | Query git configuration                                     |
| `pathfind`   | Find commands in PATH-like search lists                     |
| `getent`     | Query system databases (passwd, group, services, protocols) |
| `sysinfo`    | System inspection for fault diagnosis                       |
| `config`     | Show effective configuration                                |
| `sensitive`  | List built-in sensitive file patterns                       |
| `skill`      | Emit SKILL.md for Claude Code                               |
| `mcp`        | Start MCP server (stdio or HTTP)                            |
| `version`    | Print version information                                   |


## Git Path Syntax

Git objects are addressed as `[repo:]<ref>:<path>`:

| Input Spec                      | Semantic meaning         |
| ------------------------------- | ------------------------ |
| `HEAD:README.md`                | auto-detected repo, HEAD |
| `main:src/lib.go`               | branch "main"            |
| `v2.0:config.toml`              | tag "v2.0"               |
| `HEAD~3:file.go`                | 3 commits back           |
| `a1b2c3d:pkg/auth.go`           | short commit hash        |
| `myrepo:main:terraform/main.tf` | named repo from config   |

All git reads go directly through the object store — no checkout, no side
effects on the working tree.


## Building

```sh
task build              # build with version ldflags
task test               # run tests with race detector
task check              # fmt + lint + test
task release:snapshot   # goreleaser snapshot build
```

Build targets: Linux (amd64), macOS (arm64), FreeBSD (amd64).


## License

[MIT-style license](LICENSE)

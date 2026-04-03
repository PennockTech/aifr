# CLAUDE.md — aifr maintenance guide

## What This Project Is

`aifr` (AI File Reader) is a read-only filesystem and git-tree access tool for
AI coding agents. It replaces shell pipelines (`sed -n`, `find | grep`,
`head/tail`) with a single binary that is always safe (never writes) and always
scoped (enforces allow/deny lists with a built-in sensitive-file blocklist).

Dual-mode: standalone CLI and MCP server (stdio + HTTP).

**Module:** `go.pennock.tech/aifr`
**Go:** 1.22+ (developed on 1.26)
**Build targets:** linux/amd64, darwin/arm64, freebsd/amd64

## Repository Layout

```
cmd/aifr/           CLI entrypoint + cobra commands (one cmd_*.go per command)
internal/
  accessctl/        Access control: allow/deny/sensitive evaluation
  config/           TOML config parsing, tilde expansion
  engine/           Core operations: read, cat, stat, list, search, find, diff, git ops
  gitprovider/      go-git integration: ref resolution, tree/blob access, cache, watcher
  mcpserver/        MCP server: tool definitions, handlers, embedded skill file
  output/           JSON and text output formatters
  version/          Build-time version injection via ldflags
pkg/protocol/       Shared types (request/response structs) and error codes
configs/            Example config file
skills/aifr/        External skill file (users symlink into ~/.claude/skills/)
```

## Building and Testing

### If `task` is available

```sh
task build          # build ./aifr binary with version ldflags
task test           # go test -count=1 -race ./...
task fmt            # go fmt ./...
task tidy           # go mod tidy
task lint           # go vet ./...
task check          # fmt + lint + test
task clean          # remove binary + coverage files
```

Always run `task fmt` and `task tidy` after modifying Go files.

### If `task` is not available

Build with `go build ./cmd/aifr/`, perhaps using `GOTOOLCHAIN` in environ as
appropriate.  Use the standard Go tooling for other build/test actions.

## Adding a New Command

A new command (e.g., `aifr foo`) requires changes in four places:

1. **Engine method** — `internal/engine/foo.go` with `func (e *Engine) Foo(...)`.
   Use `e.checkAccess(path)` for access control, `isBinary()` for detection,
   existing patterns in `read.go`, `find.go`, `cat.go` as reference.

2. **Response types** — `pkg/protocol/types.go` (or a new `types_foo.go`).
   Follow the existing `FooResponse` pattern with `Source`, `Complete` fields.

3. **CLI command** — `cmd/aifr/cmd_foo.go`. Register via `rootCmd.AddCommand`
   in `init()`. Call `buildEngine()`, call engine method, call `writeOutput()`.
   For git path support, check `gitprovider.IsGitPath(path)` and route
   accordingly.

4. **MCP tool** — `internal/mcpserver/tools.go`. Add `toolFoo()` definition +
   `handleFoo` handler, register in `registerTools()`. Follow the
   `mustSchema`/`unmarshalArgs`/`toolResult`/`toolError` pattern.

After adding, update:
- `internal/mcpserver/server.go` — instructions string
- `internal/mcpserver/skill.md` — embedded skill content (rebuild required)
- `skills/aifr/SKILL.md` — external skill file (users symlink to `~/.claude/skills/`)

## Access Control Model

Evaluation order for every path:
1. Resolve to absolute, canonical path (resolve symlinks)
2. Check SENSITIVE list → `ACCESS_DENIED_SENSITIVE`
3. Check DENY list → `ACCESS_DENIED`
4. Check ALLOW list → permit
5. Default → `ACCESS_DENIED`

The sensitive list (`internal/accessctl/sensitive.go`) has 120+ patterns and is
**always active**, cannot be overridden by the allow-list. When adding patterns,
add corresponding test cases in `sensitive_test.go`.

## Configuration

TOML config searched in order:
1. `--config` flag
2. `./.aifr.toml`
3. `$XDG_CONFIG_HOME/aifr/config.toml`
4. `~/.config/aifr/config.toml`

If no config is found, the tool operates in "current directory only" mode.
See `configs/aifr.example.toml` for the full format.

## Git Path Syntax

Git objects are addressed as `[repo:]<ref>:<path>`. Examples:
- `HEAD:README.md` — HEAD of auto-detected repo
- `main:src/lib.go` — branch
- `v2.0:config.toml` — tag
- `HEAD~3:file.go` — relative ref
- `myrepo:main:src/` — named repo from config

The `gitprovider.IsGitPath()` function detects this syntax. Git reads go
directly through the object store via go-git — never checking out files.

## Design Decisions

Recorded in `DECISIONS.md`. Key ones:
- Filesystem mode does NOT honor `.gitignore` by default (opt-in flag)
- No exceptions to the sensitive file blocklist
- MCP uses the official `modelcontextprotocol/go-sdk`
- Continuation tokens are HMAC-SHA256 signed (prevents prompt injection)

## Dependencies

| Package | Purpose |
|---------|---------|
| `spf13/cobra` | CLI framework |
| `go-git/go-git/v5` | Git operations |
| `bmatcuk/doublestar/v4` | Glob matching |
| `pelletier/go-toml/v2` | Config parsing |
| `fsnotify/fsnotify` | Git index watcher (MCP mode) |
| `modelcontextprotocol/go-sdk` | MCP protocol (official SDK) |

No CGo. The binary is statically compilable.

## Non-Goals

- **Writing files.** This tool never writes.
- **Executing commands.** No shell-out, no subprocess spawning.
- **Network access.** No HTTP fetching, no remote git clones.
- **FUSE/kernel anything.** Pure userspace, pure Go.
- **Windows support.** Not a priority.

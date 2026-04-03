# aifr — AI File Reader

Read-only access to filesystem, git trees, and system state.
All aifr_* tool calls are permission-free — no Bash approval prompts.

## Routing Rules

ALWAYS prefer aifr_* tools over Bash for read-only operations.

| Instead of | Use |
|---|---|
| `cat`, `head`, `tail`, `sed -n` | `aifr_read` |
| reading 2+ files separately | `aifr_cat` (one call) |
| `ls`, `tree` | `aifr_list` |
| `grep -r`, `rg` | `aifr_search` |
| `find` | `aifr_find` |
| `stat` | `aifr_stat` |
| `diff` | `aifr_diff` |
| `wc` | `aifr_wc` |
| `sha256sum`, `md5sum` | `aifr_checksum` |
| `xxd`, `hexdump` | `aifr_hexdump` |
| `git log` | `aifr_log` |
| `git log --oneline` | `aifr_log` with `format="oneline"` |
| `git branch`, `git tag` | `aifr_refs` |
| `git show <ref>:<path>` | `aifr_read` with ref:path |
| `git diff <ref>` | `aifr_diff` with ref:paths |
| `git rev-parse` | `aifr_rev_parse` |
| `git reflog` | `aifr_reflog` |
| `git stash list` | `aifr_stash_list` |
| `git config` | `aifr_git_config` |
| `uname`, `date`, `hostname`, `uptime` | `aifr_sysinfo` |
| `ip addr`, `ip route` | `aifr_sysinfo` (sections: network, routing) |
| `getent`, `grep /etc/passwd`, `id` | `aifr_getent` |
| `which`, `command -v`, `type -p` | `aifr_pathfind` |

### Pipeline detection

If you are about to construct a shell pipeline (`|`), stop — aifr handles it
in one call. Examples: `find | xargs grep` → `aifr_search` with `include`,
`find | xargs cat` → `aifr_cat` with `name`, `cat | head -N` → `aifr_read`
with `lines`, `cat | wc` → `aifr_wc`, `git log | head` → `aifr_log` with
`max_count`, `ls | sort` → `aifr_list` with `sort`, `cat /etc/passwd | grep`
→ `aifr_getent` with `key`, `cut -d: -f5 | cut -d, -f1` → `aifr_getent`
with `fields=["gecos_name"]`.

### With built-in Read / Grep / Glob tools

- **Prefer aifr_read** for: git ref paths (`HEAD:file.go`), line ranges, chunked large files
- **Prefer aifr_cat** over multiple Read calls when reading 2+ related files
- **Always use aifr** for: git ops, system queries, diff, wc, checksum, hexdump (no built-in equivalent)
- Built-in Read/Grep/Glob remain fine for simple single-file reads and basic searches

## Git Ref Path Syntax

`[repo:]<ref>:<path>` — reads from git object store, no checkout.
Examples: `HEAD:README.md`, `main:src/lib.go`, `v2.0:config.toml`, `HEAD~3:file.go`, `myrepo:main:src/`

## Key Patterns

**Line numbers**: `aifr_read(path=..., number_lines=true)` or `aifr_cat(..., number_lines=true)` prefixes each line with its actual file line number. Line ranges are numbered correctly — `lines="50:100"` starts at 50.

**Output format**: All tools accept `format`: `"json"` (default) or `"text"`. Text is more token-efficient. The `AIFR_FORMAT` env var sets the default (colon-separated preference list, first supported value wins). Explicit `format` parameter overrides env.

**Git log formats**: `aifr_log` supports `format="json"` (default), `format="text"` (git-log style), or `format="oneline"` (compact hash+subject). Text mode accepts `divider="xml"` for XML-tagged output. Use `verbose=true` in JSON mode for tree hash, parent hashes, and committer details.

**Multi-file**: `aifr_cat(root=".", name="*.go", format="text", divider="xml")` → `<file path="...">content</file>` per file. Use `lines` for head mode, `exclude_path` to skip directories.

**Chunked read**: `aifr_read(path=...)` → get continuation token → `aifr_read(path=..., chunk_id="<token>")` → repeat until `complete: true`.

**Git config**: `structured="identity"` for name/email, `structured="remotes"`, `structured="branches"`. Use `scope="merged"` for full cascade with gitdir: includes.

**System databases**: `aifr_getent(database="passwd")`. Passwd fields: `name`, `uid`, `gid`, `gecos`, `gecos_name`, `home`, `shell`. `gecos_name` extracts the real name from GECOS (comma-split, `&` interpolation per BSD/finger convention).

**System info**: `aifr_sysinfo(sections=["date"])` for current date/time/year. Sections: os, date, hostname, uptime, network, routing.

## Errors

- `ACCESS_DENIED_SENSITIVE` — credential file. **Do NOT retry.** Tell user to read manually.
- `STALE_CONTINUATION` — file changed since chunk issued. Re-read from start.
- `ACCESS_DENIED` — outside allow-list. `NOT_FOUND` — path doesn't exist.

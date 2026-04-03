---
name: aifr
description: Read-only access to filesystem, git trees, and system state via aifr MCP tools or CLI. Always prefer aifr over Bash for read-only operations including file reading, searching, directory listing, git log/diff/refs, checksums, hex dumps, and system inspection. Use when the user needs to read files, search codebases, inspect git history, diff files, or query system state — aifr handles it in one call without shell approval prompts.
---
# aifr — AI File Reader

Read-only access to filesystem, git trees, and system state.
No writes, no shell-outs, no network. All aifr operations are
permission-free — they never trigger Bash approval prompts.

Available as MCP tools (`aifr_*`) or CLI (`aifr <command>`).

## Routing Rules

ALWAYS prefer aifr over Bash for read-only operations.

| Instead of | aifr MCP tool | aifr CLI |
|---|---|---|
| `cat`, `head`, `tail`, `sed -n` | `aifr_read` | `aifr read` |
| reading 2+ files separately | `aifr_cat` | `aifr cat` |
| `ls`, `tree` | `aifr_list` | `aifr list` |
| `grep -r`, `rg` | `aifr_search` | `aifr search` |
| `find` | `aifr_find` | `aifr find` |
| `stat` | `aifr_stat` | `aifr stat` |
| `diff` | `aifr_diff` | `aifr diff` |
| `wc` | `aifr_wc` | `aifr wc` |
| `sha256sum`, `md5sum` | `aifr_checksum` | `aifr checksum` |
| `xxd`, `hexdump` | `aifr_hexdump` | `aifr hexdump` |
| `git log` | `aifr_log` | `aifr log` |
| `git log --oneline` | `aifr_log(format="oneline")` | `aifr log --oneline` |
| `git branch`, `git tag` | `aifr_refs` | `aifr refs` |
| `git show <ref>:<path>` | `aifr_read` with ref:path | `aifr read ref:path` |
| `git diff <ref>` | `aifr_diff` with ref:paths | `aifr diff ref:path ref:path` |
| `git rev-parse` | `aifr_rev_parse` | `aifr rev-parse` |
| `git reflog` | `aifr_reflog` | `aifr reflog` |
| `git stash list` | `aifr_stash_list` | `aifr stash-list` |
| `git config` | `aifr_git_config` | `aifr git-config` |
| `uname`, `date`, `hostname`, `uptime` | `aifr_sysinfo` | `aifr sysinfo` |
| `ip addr`, `ip route` | `aifr_sysinfo` | `aifr sysinfo` |
| `getent`, `grep /etc/passwd`, `id` | `aifr_getent` | `aifr getent` |
| `which`, `command -v`, `type -p` | `aifr_pathfind` | `aifr pathfind` |

### Pipeline detection

If you are about to construct a shell pipeline (`cmd | cmd`), stop — aifr
almost certainly handles it in one call with built-in filtering, limiting, and sorting.

| Pipeline | One aifr call |
|---|---|
| `find . -name '*.go' \| xargs grep pattern` | `aifr_search(pattern=..., path=".", include="*.go")` |
| `find . -name '*.go' \| xargs cat` | `aifr_cat(root=".", name="*.go")` |
| `find . -type f \| head -20` | `aifr_find(path=".", type="f", limit=20)` |
| `cat file \| head -50` | `aifr_read(path="file", lines="1:50")` |
| `cat file \| wc -l` | `aifr_wc(paths=["file"], lines=true)` |
| `cat /etc/passwd \| grep root` | `aifr_getent(database="passwd", key="root")` |
| `getent passwd \| cut -d: -f5 \| cut -d, -f1` | `aifr_getent(database="passwd", fields=["gecos_name"])` |
| `git log --oneline \| head -5` | `aifr_log(format="oneline", max_count=5)` |
| `ls -la \| sort -k5 -n` | `aifr_list(path=".", sort="size")` |
| `grep -rl pattern . \| wc -l` | `aifr_search` — count results from response |

### Coexistence with built-in Read / Grep / Glob tools

When the runtime provides built-in file tools alongside aifr MCP tools:

- **Prefer aifr_read** for: git ref paths (`HEAD:file.go`), line ranges, chunked large files
- **Prefer aifr_cat** over multiple Read calls when reading 2+ related files
- **Prefer aifr_search** for: recursive search with include/exclude globs
- **Prefer aifr_find** for: queries combining size, age, or type filters
- **Always use aifr** for: git operations, system queries, diff, wc, checksum, hexdump (no built-in equivalent)
- Built-in Read/Grep/Glob remain fine for simple single-file reads and basic searches

## Git Ref Path Syntax

`[repo:]<ref>:<path>` — reads directly from git object store, no checkout needed.

| Example | Meaning |
|---|---|
| `HEAD:README.md` | HEAD of auto-detected repo |
| `main:src/lib.go` | branch "main" |
| `v2.0:config.toml` | tag "v2.0" |
| `HEAD~3:file.go` | 3 commits back |
| `myrepo:main:src/` | named repo from config |
| `/path/to/repo:HEAD:file` | repo at explicit filesystem path |

## Key Patterns

### Line numbering

`aifr_read` and `aifr_cat` support `number_lines=true` (MCP) or `-n` (CLI)
to prefix each line with its actual file line number (`%6d\t` format, like
`cat -n`). Works in both JSON and text output modes. Line ranges are numbered
correctly — `lines="50:100"` starts numbering at 50.

### Output format and AIFR_FORMAT

All tools support `format` parameter: `"json"` (default) or `"text"`.
Text mode is more token-efficient for AI consumption.

`aifr_log` additionally supports `format="oneline"` (compact hash+subject),
`divider="xml"` for XML-tagged text output, and `verbose=true` in JSON mode
for tree hash, parent hashes, and committer details (when they differ from
the author — useful for detecting rebases and cherry-picks).

The `AIFR_FORMAT` environment variable sets the default format. It accepts a
colon-separated preference list — the first value supported by the tool wins:
```
AIFR_FORMAT=text          # all tools default to text
AIFR_FORMAT=short:text    # "version" uses short; others use text
```

Explicit `format` parameter (MCP) or `--format` flag (CLI) overrides the env.

### Multi-file reading (token-efficient)

`aifr_cat` with `format="text"` and `divider="xml"` returns:
```
<file path="src/a.go">contents</file>
<file path="src/b.go">contents</file>
```
- **Discovery mode**: set `root` + `name` glob, optionally `exclude_path`
- **Head mode**: `lines` param limits per-file output to first N lines
- Replaces find-pipe-xargs-cat in a single call

### Chunked reading

Large files return in chunks with signed continuation tokens:
1. `aifr_read(path="large.log")` → first chunk + `continuation` token
2. `aifr_read(path="large.log", chunk_id="<token>")` → next chunk
3. Repeat until `"complete": true`

### Git config queries

- `key="remote.origin.url"` — single key lookup
- `structured="identity"` — author name/email (auto-merges includes)
- `structured="remotes"` — all remotes with URLs and fetch specs
- `structured="branches"` — all branches with tracking info
- `scope="merged"` — full cascade including gitdir: conditional includes

### System databases (getent)

Passwd fields: `name`, `uid`, `gid`, `gecos`, `gecos_name`, `home`, `shell`.
`gecos_name` is a pseudo-field: extracts the real name from the GECOS field
(first comma-separated sub-field, with `&` replaced by the login name,
first letter capitalized — per BSD/finger convention).

### System inspection

`aifr_sysinfo` sections: `os`, `date`, `hostname`, `uptime`, `network`, `routing`.
Request specific sections to reduce output: `sections=["date"]` for current date/time/year.

## Error Codes

| Code | Meaning | Action |
|---|---|---|
| `ACCESS_DENIED_SENSITIVE` | Credential/key file | **Do NOT retry.** Tell user to read manually. |
| `ACCESS_DENIED` | Outside allow-list or in deny-list | Check aifr config |
| `NOT_FOUND` | Path does not exist | Verify path |
| `STALE_CONTINUATION` | File changed since chunk token issued | Re-read from start |

## CLI Reference

Skip this section if aifr MCP tools (`aifr_*`) are available — use those instead.

When only the `aifr` binary is in PATH, use CLI via Bash. JSON output by default;
add `--format text` for plain output.

| Command | Key flags | Example |
|---|---|---|
| `aifr read` | `--lines START:END`, `--chunk-id TOKEN`, `-n` (line numbers) | `aifr read -n --lines 1:50 main.go` |
| `aifr cat` | `--name GLOB --exclude-path GLOB --lines N --divider xml --format text -n --max-depth N` | `aifr cat -n --name '*.go' --format text .` |
| `aifr list` | `--depth N` (-1=all) `--pattern GLOB --type f/d/l --sort name/size/mtime --limit N` | `aifr list --depth -1 --type f .` |
| `aifr search` | `--fixed-string --ignore-case --context N --include GLOB --exclude GLOB` | `aifr search --include '*.go' 'Handler' .` |
| `aifr find` | `--name GLOB --type f/d/l --max-depth N --min-size N --newer-than DUR --sort name/size` | `aifr find --name '*.yaml' .` |
| `aifr diff` | `--cmp` (byte-level) | `aifr diff HEAD~1:main.go main.go` |
| `aifr wc` | `-l --total-only` | `aifr wc -l src/**/*.go` |
| `aifr checksum` | `-a sha256/sha512/md5 -e hex/base64` | `aifr checksum -a sha512 *.go` |
| `aifr hexdump` | `-s OFFSET -l LENGTH` | `aifr hexdump -s 1024 -l 512 f.dat` |
| `aifr log` | `--max-count N --oneline --divider plain/xml --verbose` | `aifr log --oneline --max-count 10` |
| `aifr refs` | `--branches --tags --remotes` | `aifr refs --branches --tags` |
| `aifr rev-parse` | `--repo NAME` | `aifr rev-parse HEAD~3` |
| `aifr reflog` | `--max-count N` | `aifr reflog main` |
| `aifr stash-list` | `--max-count N` | `aifr stash-list` |
| `aifr git-config` | `--scope merged --identity --remotes --branches --regexp PAT` | `aifr git-config --identity` |
| `aifr sysinfo` | `--sections os,date,hostname,...` | `aifr sysinfo --sections date` |
| `aifr getent` | `DATABASE [KEY] --fields NAMES` | `aifr getent passwd --fields name,gecos_name,home` |
| `aifr pathfind` | `--search-list SPEC` | `aifr pathfind python3` |

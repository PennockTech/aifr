# aifr — AI File Reader Skill

Detailed usage guide for the `aifr` MCP server and CLI. This supplements
the server's built-in `instructions` field with tool routing, advanced
patterns, and operational guidance.


## When to use aifr instead of shell tools

Use `aifr` (via MCP tools or CLI) instead of shell pipelines whenever you
need to read files, search content, list directories, or browse git history.
`aifr` is always safe (read-only), always scoped (access-controlled), and
returns structured JSON.

| Instead of... | Use |
|---|---|
| `cat file` | `aifr_read` with `path` |
| `head -50 file` | `aifr_read` with `lines: "1:50"` |
| `find . -name '*.go' -exec cat {} +` | `aifr_cat` with `root`, `name` |
| `head -10 *.go` with dividers | `aifr_cat` with `lines: 10`, `divider: "xml"` |
| `grep -rn pattern dir/` | `aifr_search` with `pattern`, `path` |
| `find . -name '*.yaml'` | `aifr_find` with `name: "*.yaml"` |
| `ls -la dir/` | `aifr_list` with `path` |
| `stat file` | `aifr_stat` with `path` |
| `git log --oneline` | `aifr_log` |
| `git branch -a` | `aifr_refs` |
| `diff file1 file2` | `aifr_diff` with `path_a`, `path_b` |
| `git show HEAD:file` | `aifr_read` with `path: "HEAD:file"` |
| `which git` | `aifr_pathfind` with `command: "git"` |
| `compgen -c git-` | `aifr_pathfind` with `command: "git-*"` |


## Which tool to use

| I want to... | Tool | Key params |
|---|---|---|
| Read one file | `aifr_read` | `path`, optional `lines` |
| Read many files at once | `aifr_cat` | `paths` or `root`+`name`, optional `lines`, `divider` |
| Search file contents | `aifr_search` | `pattern`, `path` |
| Find files by name/pattern | `aifr_find` | `path`, `name`, optional `type`, `max_depth` |
| List a directory | `aifr_list` | `path`, optional `depth`, `pattern`, `type` |
| Get file metadata | `aifr_stat` | `path` |
| Read from git history | `aifr_read` | `path: "ref:filepath"` |
| List git refs | `aifr_refs` | optional `branches`, `tags`, `remotes` |
| View commit log | `aifr_log` | optional `ref`, `max_count` |
| Compare two files | `aifr_diff` | `path_a`, `path_b` |
| Concatenate found files | `aifr_cat` | `root`, `name`, `exclude_path` |
| Find commands in PATH | `aifr_pathfind` | `command`, optional `search_list` |


## Git path syntax

All tools that accept a `path` parameter also accept git paths:

```
HEAD:README.md                    auto-detected repo, HEAD
main:src/lib.go                   branch "main"
v2.0:config.toml                  tag "v2.0"
HEAD~3:file.go                    3 commits back
myrepo:main:terraform/main.tf     named repo from config
/path/to/repo:HEAD:README.md      repo at filesystem path
/path/to/repo:main:src/lib.go     filesystem path + branch
```

Paths starting with `/` or not containing `:` are treated as filesystem
paths. Paths containing `:` with something before it are treated as git
paths. Absolute filesystem paths require the three-part format
(`/path:ref:file`) to avoid ambiguity.

For `aifr_refs` and `aifr_log`, the `repo` parameter also accepts
filesystem paths (e.g., `repo: "/path/to/project"`).


## aifr_cat: multi-file reading

`aifr_cat` is the most efficient way to read multiple files. Two modes:

**Explicit paths** — list the files you want:
```json
{"paths": ["src/main.go", "src/lib.go", "README.md"]}
```

**Discovery mode** — find and read files matching criteria:
```json
{"root": ".", "name": "*.go", "exclude_path": "**/vendor/**"}
```

**Head mode** — first N lines of each file:
```json
{"root": ".", "name": "*.go", "lines": 10}
```

**Text output with XML dividers** (most token-efficient for reading file
contents — use this when you need to see the actual content):
```json
{"paths": ["a.go", "b.go"], "format": "text", "divider": "xml"}
```

Returns:
```xml
<file path="a.go">
package main
...
</file>
<file path="b.go">
package lib
...
</file>
```

The `divider` options are:
- `xml` — `<file path="...">content</file>` wrappers (best for parsing)
- `plain` — `--- path ---` separators (human-readable)
- `none` — raw concatenation

**Safety limits:** Default max 2 MiB total output and 1000 files. Binary
files are detected and skipped. Access-denied files record an error per
entry but don't abort the operation.


## aifr_read: chunked reading

For large files, `aifr_read` returns the first 64 KiB with a continuation
token:

1. Call `aifr_read` with `path`
2. Response includes `continuation` token if `complete: false`
3. Call `aifr_read` with `chunk_id` set to the token
4. Repeat until `complete: true`

If the file changes between chunks, the token becomes stale and returns
`STALE_CONTINUATION` — re-read from the beginning.

For files over 10 MiB with no range specified, the response includes
`warning: "file_large"` — consider using `lines` to read a specific range.


## aifr_search: content search

RE2 regexp by default. Use `regexp: false` for fixed-string matching.

```json
{"pattern": "func.*Handler", "path": "./src/", "context": 2, "include": "*.go"}
```

Returns structured matches with file, line number, column, and context
lines. Capped at 500 matches by default (`max_matches` to adjust).

Binary files are automatically skipped.


## Error handling

| Error | Meaning | Action |
|---|---|---|
| `ACCESS_DENIED_SENSITIVE` | Credentials, keys, secrets | Do NOT retry. Tell the user to read it themselves. |
| `ACCESS_DENIED` | Outside allow-list or in deny-list | Check configuration |
| `NOT_FOUND` | Path does not exist | Verify path |
| `IS_DIRECTORY` | Tried to read a directory as a file | Use `aifr_list` instead |
| `INVALID_REF` | Git ref does not exist | Use `aifr_refs` to discover refs |
| `STALE_CONTINUATION` | File changed since token issued | Re-read from beginning |
| `CHUNK_OUT_OF_RANGE` | Requested range exceeds file | Check file size with `aifr_stat` |


## Tips

- **JSON is the default output** for all tools. The structured response
  includes metadata (size, line counts, pagination) alongside content.
- **For `aifr_cat`**, use `format: "text"` with `divider: "xml"` when you
  want to read the actual file contents efficiently. Use JSON when you need
  the metadata.
- **Discovery mode excludes** use doublestar globs: `**/vendor/**` matches
  any path containing a `vendor/` segment.
- **All tools respect access control individually.** A multi-file operation
  like `aifr_cat` will skip denied files and continue with the rest.
- **Git reads never check out files.** They go directly through the object
  store, so they have zero side effects on the working tree.

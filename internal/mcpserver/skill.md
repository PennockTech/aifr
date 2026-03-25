# aifr — AI File Reader

Use `aifr` instead of shell pipelines (`sed -n`, `find | grep`, `head/tail`)
for reading files, searching content, and browsing git trees. It is always
safe (read-only) and always scoped (enforces allow/deny lists with a built-in
sensitive-file blocklist).

## When to Use aifr

- Reading file contents (instead of `cat`, `head`, `tail`, `sed -n`)
- Searching file content (instead of `grep`, `rg`)
- Finding files by name/pattern (instead of `find`)
- Listing directory contents (instead of `ls`, `find`)
- Getting file metadata (instead of `stat`)
- Reading files from git history without checkout
- Comparing files or git refs

## CLI Commands

### Read file contents
```
aifr read <path>
aifr read --lines 1:50 <path>
aifr read --chunk-id <token>
aifr read HEAD:src/main.go
aifr read myrepo:v2.0:config.toml
```

### Concatenate multiple files
```
aifr cat file1.go file2.go file3.go
aifr cat --format text --divider xml file1.go file2.go
aifr cat --name '*.go' --exclude-path '**/vendor/**' ./src/
aifr cat --name '*.go' --lines 10 --divider plain --format text .
aifr cat --name '*.yaml' --max-depth 2 ./config/
```

### Get file/directory metadata
```
aifr stat <path>
aifr stat HEAD:README.md
```

### List directory
```
aifr list <path>
aifr list --depth -1 <path>
aifr list --pattern "*.go" --type f <path>
aifr list HEAD:src/
```

### Search content (grep-like)
```
aifr search "pattern" <path>
aifr search --fixed-string "exact text" <path>
aifr search --ignore-case --context 2 "TODO" <path>
aifr search --include "*.go" "func.*Handler" ./
```

### Find files
```
aifr find --name "*.go" <path>
aifr find --type f --min-size 1024 <path>
aifr find --newer-than 24h <path>
```

### Git refs and log
```
aifr refs
aifr refs --branches --tags
aifr log --max-count 10
aifr log HEAD
```

### Compare files
```
aifr diff file1.go file2.go
aifr diff HEAD~1:main.go main.go
aifr diff main:lib.go feature:lib.go
```

### Other
```
aifr config          # show effective configuration
aifr sensitive       # list sensitive file patterns (for auditing)
aifr version         # version info
```

## Git Path Syntax

Git objects are addressed as `[repo:]<ref>:<path>`:
- `HEAD:README.md` — HEAD of auto-detected repo
- `main:src/lib.go` — main branch
- `v2.0:config.toml` — tag v2.0
- `HEAD~3:file.go` — 3 commits back
- `myrepo:main:src/` — named repo "myrepo" at branch main

## Chunked Reading

Large files are returned in chunks. Use the continuation token to read more:
1. `aifr read large-file.log` → returns first chunk + `continuation` token
2. `aifr read --chunk-id <token>` → returns next chunk
3. Repeat until `"complete": true`

## Error Handling

- `ACCESS_DENIED_SENSITIVE` — file matches sensitive pattern (credentials, keys, etc.).
  Do NOT retry. Ask the user to read it manually if needed.
- `ACCESS_DENIED` — path is outside allow-list or in deny-list.
- `NOT_FOUND` — path does not exist.
- `STALE_CONTINUATION` — file changed since continuation token was issued. Re-read from start.

## Output

All commands output JSON by default. Use `--format text` for human-readable output.

## MCP Server

`aifr mcp` starts the MCP server (stdio by default). The same operations are
available as MCP tools: `aifr_read`, `aifr_cat`, `aifr_stat`, `aifr_list`,
`aifr_search`, `aifr_find`, `aifr_refs`, `aifr_log`, `aifr_diff`.

For `aifr_cat`, use `format="text"` with `divider="xml"` for token-efficient
multi-file reading with `<file path="...">` wrappers.

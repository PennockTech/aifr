# Design Decisions

Decisions made during implementation, as requested by the spec in CLAUDE.md.

---

## 1. .gitignore Awareness (Phase 4)

**Decision:** Filesystem mode does NOT honor `.gitignore` by default.

**Rationale:** `aifr` is a "read what's on disk" tool, scoped by allow/deny
lists. Honoring `.gitignore` by default would conflate "what's tracked" with
"what's readable." Agents may need to read generated artifacts, vendored deps,
or build output that happens to be gitignored.

The git tree mode (`ref:path`) already provides the "only tracked files" view.

**Implementation:** Opt-in `--gitignore` flag for search/list/find. MCP tools
get a `gitignore: true` parameter.

---

## 2. Sensitive Pattern Exceptions

**Decision:** No exceptions to the sensitive file blocklist.

**Rationale:** The complexity of "deny `~/.ssh/id_*` but allow `~/.ssh/id_*.pub`"
creates a larger attack surface than the convenience provides. An agent can tell
the user to read the file themselves through regular tools.

---

## 3. MCP Library Choice (Phase 8)

**Decision:** Use the official SDK `github.com/modelcontextprotocol/go-sdk`
(not the unofficial `github.com/mark3labs/mcp-go`).

**Rationale:** The official SDK provides first-class support for both
`StdioTransport` and `StreamableHTTPHandler`, has a cleaner API
(`server.AddTool(def, handler)`), is actively maintained (v1.4.1), and is
pure Go. Validated by the maintainer's own `mcp-unicode` project which uses
the same SDK.

---

## 4. Continuation Token Security (Phase 3)

**Decision:** HMAC-SHA256 signed tokens with per-process random key.

**Rationale:** Prevents prompt injection from crafting tokens to escape
the allow-list. Token encodes path + mtime + offset + chunk size. The
HMAC key is 32 bytes from `crypto/rand`, generated at process startup.
In CLI mode, tokens are scoped to a single invocation; in MCP mode, the
key persists for the server lifetime.

---

## 5. No JSON Query Tool (jq)

**Decision:** Do not implement a JSON query tool in aifr.

**Context:** `jq` triggers Claude Code permission prompts because all Bash
commands are subject to permission checks — the system cannot distinguish
`jq '.key' file.json` from `rm -rf /`. This makes jq inconvenient for agents.

**Rationale:** jq is Turing-complete; any useful subset would still be a large
surface area increase that is out of scope for a filesystem/git access tool.
AI agents can instead use `aifr_read` to retrieve JSON content and parse it
in-context, avoiding the Bash permission prompt entirely. If demand grows for
structured queries, a minimal property-access-only tool may be reconsidered.

---

## 6. sysinfo Access Control Exemption

**Decision:** System metadata paths (`/proc/*`, `/etc/os-release`) are read
directly by `aifr sysinfo` without going through the access control checker.

**Rationale:** The access control model protects user data; `/proc/net/route`,
`/etc/os-release`, and similar paths are system metadata, not user files. This
matches the precedent set by `os.Hostname()` and `runtime.GOOS`, which also
bypass access control. The sysinfo command never executes subprocesses.

---

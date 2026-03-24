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

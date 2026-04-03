// Copyright 2026 — see LICENSE file for terms.
package protocol

// ChunkInfo describes a chunk of file content.
type ChunkInfo struct {
	StartByte   int64  `json:"start_byte"`
	EndByte     int64  `json:"end_byte"`
	StartLine   int    `json:"start_line"`
	EndLine     int    `json:"end_line"`
	Data        string `json:"data"`
	Encoding    string `json:"encoding"`               // "utf-8" or "base64"
	TruncatedAt string `json:"truncated_at,omitempty"` // "newline", "boundary", etc.
}

// ReadResponse is the JSON response for a read operation.
type ReadResponse struct {
	Path         string     `json:"path"`
	Source       string     `json:"source"` // "filesystem" or "git"
	Repo         string     `json:"repo,omitempty"`
	Ref          string     `json:"ref,omitempty"`
	RefResolved  string     `json:"ref_resolved,omitempty"`
	ObjectHash   string     `json:"object_hash,omitempty"`
	TotalSize    int64      `json:"total_size"`
	TotalLines   int        `json:"total_lines,omitempty"`
	Chunk        *ChunkInfo `json:"chunk"`
	Continuation string     `json:"continuation,omitempty"`
	Complete     bool       `json:"complete"`
	Warning      string     `json:"warning,omitempty"`
}

// StatEntry describes a file or directory.
type StatEntry struct {
	Name       string `json:"name"`
	Path       string `json:"path"`
	Type       string `json:"type"` // "file", "dir", "symlink"
	Size       int64  `json:"size"`
	Mode       string `json:"mode,omitempty"`
	ModTime    string `json:"mtime,omitempty"`
	ObjectHash string `json:"object_hash,omitempty"`
}

// ListResponse is the JSON response for a list operation.
type ListResponse struct {
	Path         string      `json:"path"`
	Source       string      `json:"source"`
	Entries      []StatEntry `json:"entries"`
	Total        int         `json:"total"`
	Continuation string      `json:"continuation,omitempty"`
	Complete     bool        `json:"complete"`
}

// SearchMatch describes a single search match.
type SearchMatch struct {
	File          string   `json:"file"`
	Line          int      `json:"line"`
	Column        int      `json:"column"`
	Match         string   `json:"match"`
	ContextBefore []string `json:"context_before,omitempty"`
	ContextAfter  []string `json:"context_after,omitempty"`
}

// SearchResponse is the JSON response for a search operation.
type SearchResponse struct {
	Pattern       string        `json:"pattern"`
	IsRegexp      bool          `json:"is_regexp"`
	Root          string        `json:"root"`
	Source        string        `json:"source"`
	Matches       []SearchMatch `json:"matches"`
	FilesSearched int           `json:"files_searched"`
	FilesMatched  int           `json:"files_matched"`
	TotalMatches  int           `json:"total_matches"`
	Truncated     bool          `json:"truncated"`
	Continuation  string        `json:"continuation,omitempty"`
	Complete      bool          `json:"complete"`
}

// FindEntry describes a found file/directory.
type FindEntry struct {
	Path string `json:"path"`
	Type string `json:"type"`
	Size int64  `json:"size"`
	Mode string `json:"mode,omitempty"`
}

// FindResponse is the JSON response for a find operation.
type FindResponse struct {
	Root         string      `json:"root"`
	Source       string      `json:"source"`
	Entries      []FindEntry `json:"entries"`
	Total        int         `json:"total"`
	Continuation string      `json:"continuation,omitempty"`
	Complete     bool        `json:"complete"`
}

// DiffHunk describes a single hunk in a unified diff.
type DiffHunk struct {
	OldStart int      `json:"old_start"`
	OldLines int      `json:"old_lines"`
	NewStart int      `json:"new_start"`
	NewLines int      `json:"new_lines"`
	Lines    []string `json:"lines"`
}

// ByteDiff describes the first byte-level difference between two files.
type ByteDiff struct {
	Offset int64 `json:"offset"`
	Line   int   `json:"line"`
	Column int   `json:"column"`
	ByteA  byte  `json:"byte_a"`
	ByteB  byte  `json:"byte_b"`
	SizeA  int64 `json:"size_a"`
	SizeB  int64 `json:"size_b"`
}

// DiffResponse is the JSON response for a diff operation.
type DiffResponse struct {
	PathA     string     `json:"path_a"`
	PathB     string     `json:"path_b"`
	Source    string     `json:"source"`
	Identical bool       `json:"identical"`
	Hunks     []DiffHunk `json:"hunks,omitempty"`
	ByteDiff  *ByteDiff  `json:"byte_diff,omitempty"`
}

// ErrorResponse is the JSON error response.
type ErrorResponse struct {
	Error *AifrError `json:"error"`
}

// GitRef describes a git reference.
type GitRef struct {
	Name   string `json:"name"`
	Type   string `json:"type"` // "branch", "tag", "remote"
	Hash   string `json:"hash"`
	Remote string `json:"remote,omitempty"`
}

// RefsResponse is the JSON response for a refs operation.
type RefsResponse struct {
	Repo string   `json:"repo"`
	Refs []GitRef `json:"refs"`
}

// FileChange describes a changed file and its action within a commit.
type FileChange struct {
	Path   string `json:"path"`
	Action string `json:"action"` // "A" (add), "M" (modify), "D" (delete)
}

// LogEntry describes a single git commit.
type LogEntry struct {
	Hash         string       `json:"hash"`
	Author       string       `json:"author"`
	AuthorEmail  string       `json:"author_email"`
	Date         string       `json:"date"`
	Message      string       `json:"message"`
	FilesChanged []string     `json:"files_changed,omitempty"`
	Changes      []FileChange `json:"changes,omitempty"`

	// Verbose fields — only populated when verbose=true.
	TreeHash       string   `json:"tree_hash,omitempty"`
	ParentHashes   []string `json:"parent_hashes,omitempty"`
	Committer      string   `json:"committer,omitempty"`
	CommitterEmail string   `json:"committer_email,omitempty"`
	CommitterDate  string   `json:"committer_date,omitempty"`
}

// LogResponse is the JSON response for a log operation.
type LogResponse struct {
	Repo         string     `json:"repo"`
	Ref          string     `json:"ref,omitempty"`
	Entries      []LogEntry `json:"entries"`
	Total        int        `json:"total"`
	Continuation string     `json:"continuation,omitempty"`
	Complete     bool       `json:"complete"`
}

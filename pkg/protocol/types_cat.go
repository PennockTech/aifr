// Copyright 2026 — see LICENSE file for terms.
package protocol

// CatEntry describes a single file's content in a cat response.
type CatEntry struct {
	Path      string `json:"path"`
	RelPath   string `json:"rel_path,omitempty"` // relative to root in discovery mode
	Size      int64  `json:"size"`
	Lines     int    `json:"lines"`
	Truncated bool   `json:"truncated,omitempty"` // true if --lines limited output
	Binary    bool   `json:"binary,omitempty"`    // true if binary (content omitted)
	Error     string `json:"error,omitempty"`     // access denied, not found, etc.
	Content   string `json:"content,omitempty"`   // file content (absent for binary/error)
}

// CatResponse is the JSON response for a cat operation.
type CatResponse struct {
	Source       string     `json:"source"`
	Mode         string     `json:"mode"`           // "explicit" or "discover"
	Root         string     `json:"root,omitempty"` // discovery mode root
	Files        []CatEntry `json:"files"`
	TotalFiles   int        `json:"total_files"`
	FilesRead    int        `json:"files_read"`    // successfully read
	FilesSkipped int        `json:"files_skipped"` // binary + error
	TotalBytes   int64      `json:"total_bytes"`   // sum of content bytes returned
	Truncated    bool       `json:"truncated"`
	Warning      string     `json:"warning,omitempty"`
	Complete     bool       `json:"complete"`
}

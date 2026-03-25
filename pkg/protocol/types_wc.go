// Copyright 2026 — see LICENSE file for terms.
package protocol

// WcEntry holds word-count results for a single file.
type WcEntry struct {
	Path   string `json:"path"`
	Source string `json:"source"`
	Lines  *int   `json:"lines,omitempty"`
	Words  *int   `json:"words,omitempty"`
	Bytes  *int64 `json:"bytes,omitempty"`
	Chars  *int   `json:"chars,omitempty"`
	Error  string `json:"error,omitempty"`
}

// WcResponse is the JSON response for a wc operation.
type WcResponse struct {
	Entries   []WcEntry `json:"entries,omitempty"`
	Total     WcEntry   `json:"total"`
	FileCount int       `json:"file_count"`
	Complete  bool      `json:"complete"`
}

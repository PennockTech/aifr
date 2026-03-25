// Copyright 2026 — see LICENSE file for terms.
package protocol

// ChecksumEntry holds a checksum result for a single file.
type ChecksumEntry struct {
	Path     string `json:"path"`
	Source   string `json:"source"`
	Checksum string `json:"checksum"`
	Size     int64  `json:"size"`
	Error    string `json:"error,omitempty"`
}

// ChecksumResponse is the JSON response for a checksum operation.
type ChecksumResponse struct {
	Algorithm string          `json:"algorithm"`
	Encoding  string          `json:"encoding"`
	Entries   []ChecksumEntry `json:"entries"`
	Complete  bool            `json:"complete"`
}

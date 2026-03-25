// Copyright 2026 — see LICENSE file for terms.
package protocol

// HexdumpLine represents one line of a canonical hex dump (16 bytes).
type HexdumpLine struct {
	Offset int64  `json:"offset"`
	Hex    string `json:"hex"`
	ASCII  string `json:"ascii"`
}

// HexdumpResponse is the JSON response for a hexdump operation.
type HexdumpResponse struct {
	Path      string        `json:"path"`
	Source    string        `json:"source"`
	TotalSize int64         `json:"total_size"`
	Offset    int64         `json:"offset"`
	Length    int64         `json:"length"`
	Lines     []HexdumpLine `json:"lines"`
	Complete  bool          `json:"complete"`
}

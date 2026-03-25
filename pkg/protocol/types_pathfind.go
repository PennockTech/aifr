// Copyright 2026 — see LICENSE file for terms.
package protocol

// PathfindEntry describes a single match from a path search.
type PathfindEntry struct {
	Path       string `json:"path"`
	Dir        string `json:"dir"`  // directory this was found in
	Name       string `json:"name"` // matched filename
	Mode       string `json:"mode"` // file mode string
	Size       int64  `json:"size"`
	Executable bool   `json:"executable"`          // has any execute bit
	Masked     bool   `json:"masked"`              // true if an earlier dir had a match for same name
	MaskedBy   string `json:"masked_by,omitempty"` // path of the first match (set when Masked)
	DirIndex   int    `json:"dir_index"`           // 0-based position in search list
}

// PathfindResponse is the JSON response for a pathfind operation.
type PathfindResponse struct {
	Command    string          `json:"command"`     // query pattern
	SearchList []string        `json:"search_list"` // resolved directory list
	SearchSpec string          `json:"search_spec"` // original spec
	Entries    []PathfindEntry `json:"entries"`
	Total      int             `json:"total"`
	Complete   bool            `json:"complete"`
}

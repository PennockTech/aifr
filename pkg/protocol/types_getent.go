// Copyright 2026 — see LICENSE file for terms.
package protocol

// GetentEntry holds one record from a system database, with named fields.
type GetentEntry struct {
	Fields map[string]string `json:"fields"`
}

// GetentResponse is the JSON response for a getent operation.
type GetentResponse struct {
	Database string        `json:"database"`
	Key      string        `json:"key,omitempty"`
	Fields   []string      `json:"fields"`
	Entries  []GetentEntry `json:"entries"`
	Total    int           `json:"total"`
	Complete bool          `json:"complete"`
}

// Copyright 2026 — see LICENSE file for terms.
package protocol

// ReflogEntry holds one entry from a git reflog.
type ReflogEntry struct {
	Index   int    `json:"index"`
	OldHash string `json:"old_hash"`
	NewHash string `json:"new_hash"`
	Author  string `json:"author"`
	Email   string `json:"email"`
	Date    string `json:"date"`
	Action  string `json:"action"`
}

// ReflogResponse is the JSON response for a reflog or stash-list operation.
type ReflogResponse struct {
	Repo         string        `json:"repo"`
	Ref          string        `json:"ref"`
	Entries      []ReflogEntry `json:"entries"`
	Total        int           `json:"total"`
	Continuation string        `json:"continuation,omitempty"`
	Complete     bool          `json:"complete"`
}

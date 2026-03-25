// Copyright 2026 — see LICENSE file for terms.
package protocol

// GitConfigEntry holds a single config key/value with its source scope.
type GitConfigEntry struct {
	Key    string `json:"key"`
	Value  string `json:"value"`
	Scope  string `json:"scope"`            // "system", "global", "local", "worktree", "include"
	Source string `json:"source,omitempty"` // file path
}

// GitConfigResponse is the JSON response for a git-config query.
type GitConfigResponse struct {
	Repo            string           `json:"repo"`
	Scope           string           `json:"scope"` // requested scope
	Entries         []GitConfigEntry `json:"entries"`
	Total           int              `json:"total"`
	SkippedIncludes []string         `json:"skipped_includes,omitempty"`
	RedactedKeys    int              `json:"redacted_keys,omitempty"`
	Complete        bool             `json:"complete"`
}

// GitConfigIdentity holds structured identity information.
type GitConfigIdentity struct {
	UserName        string   `json:"user_name,omitempty"`
	UserEmail       string   `json:"user_email,omitempty"`
	AuthorName      string   `json:"author_name,omitempty"`
	AuthorEmail     string   `json:"author_email,omitempty"`
	CommitterName   string   `json:"committer_name,omitempty"`
	CommitterEmail  string   `json:"committer_email,omitempty"`
	Scope           string   `json:"scope"`
	SkippedIncludes []string `json:"skipped_includes,omitempty"`
}

// GitConfigRemote holds structured remote information.
type GitConfigRemote struct {
	Name     string   `json:"name"`
	URLs     []string `json:"urls"`
	Fetch    []string `json:"fetch"`
	PushURLs []string `json:"push_urls,omitempty"`
}

// GitConfigBranch holds structured branch tracking information.
type GitConfigBranch struct {
	Name        string `json:"name"`
	Remote      string `json:"remote,omitempty"`
	Merge       string `json:"merge,omitempty"`
	Rebase      string `json:"rebase,omitempty"`
	Description string `json:"description,omitempty"`
}

// GitConfigStructuredResponse wraps structured query results.
type GitConfigStructuredResponse struct {
	Repo     string             `json:"repo"`
	Identity *GitConfigIdentity `json:"identity,omitempty"`
	Remotes  []GitConfigRemote  `json:"remotes,omitempty"`
	Branches []GitConfigBranch  `json:"branches,omitempty"`
	Complete bool               `json:"complete"`
}

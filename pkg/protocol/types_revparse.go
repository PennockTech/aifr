// Copyright 2026 — see LICENSE file for terms.
package protocol

// RevParseResponse is the JSON response for a rev-parse operation.
type RevParseResponse struct {
	Repo        string `json:"repo"`
	Ref         string `json:"ref"`
	Hash        string `json:"hash"`
	ShortHash   string `json:"short_hash"`
	AuthorName  string `json:"author_name"`
	AuthorEmail string `json:"author_email"`
	Date        string `json:"date"`
	Subject     string `json:"subject"`
}

// Copyright 2026 — see LICENSE file for terms.
package engine

import (
	"strings"

	"go.pennock.tech/aifr/pkg/protocol"
)

// RevParse resolves a git ref to its commit hash and metadata.
func (e *Engine) RevParse(repoName, ref string) (*protocol.RevParseResponse, error) {
	if ref == "" {
		ref = "HEAD"
	}

	repo, _, err := e.openGitRepo(repoName)
	if err != nil {
		return nil, err
	}

	commit, err := e.gitProvider.ResolveRef(repo, ref)
	if err != nil {
		return nil, err
	}

	h := commit.Hash.String()
	shortHash := h
	if len(shortHash) > 12 {
		shortHash = shortHash[:12]
	}

	// First line of commit message as subject.
	subject := strings.TrimSpace(commit.Message)
	if idx := strings.IndexByte(subject, '\n'); idx >= 0 {
		subject = subject[:idx]
	}

	return &protocol.RevParseResponse{
		Repo:        repoName,
		Ref:         ref,
		Hash:        h,
		ShortHash:   shortHash,
		AuthorName:  commit.Author.Name,
		AuthorEmail: commit.Author.Email,
		Date:        commit.Author.When.UTC().Format("2006-01-02T15:04:05Z"),
		Subject:     subject,
	}, nil
}

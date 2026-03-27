// Copyright 2026 — see LICENSE file for terms.
package engine

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"go.pennock.tech/aifr/pkg/protocol"
)

// ReflogParams controls reflog queries.
type ReflogParams struct {
	MaxCount int // 0 = default (50)
	Offset   int // skip first N entries (for pagination)
}

// Reflog reads the reflog for a given ref in a repo.
// Ref defaults to "HEAD" if empty.
func (e *Engine) Reflog(repoName, ref string, params ReflogParams) (*protocol.ReflogResponse, error) {
	if ref == "" {
		ref = "HEAD"
	}
	maxCount := params.MaxCount
	if maxCount <= 0 {
		maxCount = 50
	}

	_, repoPath, err := e.openGitRepo(repoName)
	if err != nil {
		return nil, err
	}

	logPath := reflogPath(repoPath, ref)
	entries, total, err := parseReflogFile(logPath, params.Offset+maxCount)
	if err != nil {
		return nil, fmt.Errorf("reading reflog for %q: %w", ref, err)
	}

	// Apply offset.
	if params.Offset > 0 && params.Offset < len(entries) {
		entries = entries[params.Offset:]
	} else if params.Offset >= len(entries) && params.Offset > 0 {
		entries = nil
	}

	// Apply limit.
	if len(entries) > maxCount {
		entries = entries[:maxCount]
	}

	complete := params.Offset+len(entries) >= total
	resp := &protocol.ReflogResponse{
		Repo:     repoName,
		Ref:      ref,
		Entries:  entries,
		Total:    total,
		Complete: complete,
	}

	if !complete {
		tok, tokErr := e.EncodeListContinuation(&ListContinuationToken{
			Tool:   "reflog",
			Path:   repoName,
			Offset: params.Offset + len(entries),
			Limit:  maxCount,
		})
		if tokErr != nil {
			return nil, tokErr
		}
		resp.Continuation = tok
	}

	return resp, nil
}

// StashList reads the stash reflog (refs/stash).
func (e *Engine) StashList(repoName string, params ReflogParams) (*protocol.ReflogResponse, error) {
	maxCount := params.MaxCount
	if maxCount <= 0 {
		maxCount = 50
	}

	_, repoPath, err := e.openGitRepo(repoName)
	if err != nil {
		return nil, err
	}

	logPath := reflogPath(repoPath, "refs/stash")
	entries, total, err := parseReflogFile(logPath, params.Offset+maxCount)
	if err != nil {
		// No stash file = no stashes. Not an error.
		if os.IsNotExist(err) {
			return &protocol.ReflogResponse{
				Repo:     repoName,
				Ref:      "refs/stash",
				Entries:  nil,
				Total:    0,
				Complete: true,
			}, nil
		}
		return nil, fmt.Errorf("reading stash list: %w", err)
	}

	// Apply offset.
	if params.Offset > 0 && params.Offset < len(entries) {
		entries = entries[params.Offset:]
	} else if params.Offset >= len(entries) && params.Offset > 0 {
		entries = nil
	}

	// Apply limit.
	if len(entries) > maxCount {
		entries = entries[:maxCount]
	}

	complete := params.Offset+len(entries) >= total
	resp := &protocol.ReflogResponse{
		Repo:     repoName,
		Ref:      "refs/stash",
		Entries:  entries,
		Total:    total,
		Complete: complete,
	}

	if !complete {
		tok, tokErr := e.EncodeListContinuation(&ListContinuationToken{
			Tool:   "stash_list",
			Path:   repoName,
			Offset: params.Offset + len(entries),
			Limit:  maxCount,
		})
		if tokErr != nil {
			return nil, tokErr
		}
		resp.Continuation = tok
	}

	return resp, nil
}

// reflogPath returns the filesystem path to a reflog file.
// For "HEAD" it's .git/logs/HEAD; for "refs/stash" it's .git/logs/refs/stash;
// for a branch name like "main" it's .git/logs/refs/heads/main.
func reflogPath(repoPath, ref string) string {
	gitDir := filepath.Join(repoPath, ".git")
	// If repoPath is already a bare repo or .git dir, adjust.
	if _, err := os.Stat(filepath.Join(repoPath, "HEAD")); err == nil {
		gitDir = repoPath
	}

	switch {
	case ref == "HEAD":
		return filepath.Join(gitDir, "logs", "HEAD")
	case strings.HasPrefix(ref, "refs/"):
		return filepath.Join(gitDir, "logs", ref)
	default:
		// Bare branch name → refs/heads/<name>
		return filepath.Join(gitDir, "logs", "refs", "heads", ref)
	}
}

// parseReflogFile reads and parses a git reflog file.
// Reflog format: <old-hash> <new-hash> <name> <<email>> <unix-ts> <tz> \t<action>
// Entries are in chronological order in the file; we reverse to show newest first.
// Returns the parsed entries, the total number of valid entries in the file, and any error.
func parseReflogFile(path string, maxCount int) ([]protocol.ReflogEntry, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()

	var rawLines []string
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		rawLines = append(rawLines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, 0, err
	}

	// Count total valid entries and collect up to maxCount (newest-first).
	var entries []protocol.ReflogEntry
	totalValid := 0
	for i := len(rawLines) - 1; i >= 0; i-- {
		entry, ok := parseReflogLine(rawLines[i], len(rawLines)-1-i)
		if ok {
			totalValid++
			if len(entries) < maxCount {
				entries = append(entries, entry)
			}
		}
	}

	return entries, totalValid, nil
}

// parseReflogLine parses a single reflog line.
// Format: <old> <new> <name> <<email>> <unix-ts> <tz-offset>\t<action>
func parseReflogLine(line string, index int) (protocol.ReflogEntry, bool) {
	// Split at tab to separate metadata from action.
	metaPart, action, hasTab := strings.Cut(line, "\t")
	if !hasTab {
		return protocol.ReflogEntry{}, false
	}

	parts := strings.Fields(metaPart)
	if len(parts) < 5 {
		return protocol.ReflogEntry{}, false
	}

	oldHash := parts[0]
	newHash := parts[1]

	// Name and email are everything between parts[2] and the last two fields (timestamp, tz).
	// The last two fields are always the unix timestamp and timezone offset.
	tsStr := parts[len(parts)-2]
	// tzStr := parts[len(parts)-1]

	// Extract name and email from the middle.
	nameEmailParts := parts[2 : len(parts)-2]
	nameEmail := strings.Join(nameEmailParts, " ")

	var name, email string
	if idx := strings.LastIndexByte(nameEmail, '<'); idx >= 0 {
		name = strings.TrimSpace(nameEmail[:idx])
		email = strings.Trim(nameEmail[idx:], "<>")
	} else {
		name = nameEmail
	}

	// Parse timestamp.
	ts, err := strconv.ParseInt(tsStr, 10, 64)
	var dateStr string
	if err == nil {
		dateStr = time.Unix(ts, 0).UTC().Format("2006-01-02T15:04:05Z")
	}

	return protocol.ReflogEntry{
		Index:   index,
		OldHash: oldHash,
		NewHash: newHash,
		Author:  name,
		Email:   email,
		Date:    dateStr,
		Action:  strings.TrimSpace(action),
	}, true
}

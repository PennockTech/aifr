// Copyright 2026 — see LICENSE file for terms.
package engine

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/utils/merkletrie"

	"go.pennock.tech/aifr/internal/gitprovider"
	"go.pennock.tech/aifr/pkg/protocol"
)

// openGitRepo opens a git repository and checks access control for
// filesystem-path repos. Named repos and CWD auto-detect skip the
// access check since they are admin-configured or implicitly allowed.
//
// Bare "." is normalized to "" (auto-detect) since it means "this
// directory" — semantically identical to the no-argument case. Other
// relative paths ("./sub", "..", "../other") remain subject to access
// control because they may reference a different repository.
func (e *Engine) openGitRepo(repoIdentifier string) (*git.Repository, string, error) {
	if repoIdentifier == "." {
		repoIdentifier = ""
	}
	repo, repoPath, err := e.gitProvider.OpenRepo(repoIdentifier)
	if err != nil {
		return nil, "", err
	}
	// For filesystem-path repos, verify the repo root is accessible.
	if gitprovider.LooksLikePath(repoIdentifier) {
		if err := e.checker.Check(repoPath); err != nil {
			return nil, "", err
		}
	}
	return repo, repoPath, nil
}

// GitStat returns metadata for a file in a git tree.
func (e *Engine) GitStat(gitPath string) (*protocol.StatEntry, error) {
	gp, err := gitprovider.ParseGitPath(gitPath)
	if err != nil {
		return nil, err
	}

	repo, _, err := e.openGitRepo(gp.Repo)
	if err != nil {
		return nil, err
	}

	tree, commit, err := e.gitProvider.GetTree(repo, gp.Ref)
	if err != nil {
		return nil, err
	}

	if gp.Path == "" || gp.Path == "." {
		// Root tree.
		return &protocol.StatEntry{
			Name:       ".",
			Path:       gp.Path,
			Type:       "dir",
			ObjectHash: tree.Hash.String(),
		}, nil
	}

	// Try as file first.
	f, err := e.gitProvider.GetBlob(tree, gp.Path)
	if err == nil {
		return &protocol.StatEntry{
			Name:       f.Name,
			Path:       gp.Path,
			Type:       "file",
			Size:       f.Size,
			ObjectHash: f.Hash.String(),
			Mode:       f.Mode.String(),
		}, nil
	}

	// Try as directory.
	subtree, treeErr := tree.Tree(gp.Path)
	if treeErr == nil {
		return &protocol.StatEntry{
			Name:       lastPathElement(gp.Path),
			Path:       gp.Path,
			Type:       "dir",
			ObjectHash: subtree.Hash.String(),
		}, nil
	}

	// Not found — use the original commit for a better error.
	_ = commit
	return nil, protocol.NewPathError(protocol.ErrNotFound, gp.Path,
		fmt.Sprintf("path not found in %s at ref %s", gp.Repo, gp.Ref))
}

// GitRead reads file contents from a git tree.
func (e *Engine) GitRead(gitPath string, params ReadParams) (*protocol.ReadResponse, error) {
	gp, err := gitprovider.ParseGitPath(gitPath)
	if err != nil {
		return nil, err
	}

	repo, _, err := e.openGitRepo(gp.Repo)
	if err != nil {
		return nil, err
	}

	tree, commit, err := e.gitProvider.GetTree(repo, gp.Ref)
	if err != nil {
		return nil, err
	}

	f, err := e.gitProvider.GetBlob(tree, gp.Path)
	if err != nil {
		return nil, err
	}

	content, err := f.Contents()
	if err != nil {
		return nil, fmt.Errorf("reading blob %s: %w", f.Hash, err)
	}

	totalLines := countTotalLines(content)

	resp := &protocol.ReadResponse{
		Path:        gp.Path,
		Source:      "git",
		Repo:        gp.Repo,
		Ref:         gp.Ref,
		RefResolved: commit.Hash.String(),
		ObjectHash:  f.Hash.String(),
		TotalSize:   f.Size,
		TotalLines:  totalLines,
	}

	// For git reads, we return the full content (git blobs are already in memory).
	// Apply line range if specified.
	if params.Lines != nil {
		lines := strings.Split(content, "\n")
		start := params.Lines.Start - 1
		end := params.Lines.End
		if end == 0 || end > len(lines) {
			end = len(lines)
		}
		if start < 0 {
			start = 0
		}
		if start >= len(lines) {
			return nil, protocol.NewPathError(protocol.ErrChunkOutOfRange, gp.Path,
				"line range out of bounds")
		}

		selected := strings.Join(lines[start:end], "\n")
		resp.Chunk = &protocol.ChunkInfo{
			StartLine: params.Lines.Start,
			EndLine:   end,
			Data:      selected,
			Encoding:  "utf-8",
		}
		resp.Complete = true
		return resp, nil
	}

	// Full content.
	encoding := "utf-8"
	data := content
	if isBinary([]byte(content[:min(len(content), BinaryDetectSize)])) {
		encoding = "base64"
		data = encodeBase64([]byte(content))
	}

	resp.Chunk = &protocol.ChunkInfo{
		StartByte: 0,
		EndByte:   f.Size - 1,
		StartLine: 1,
		EndLine:   totalLines,
		Data:      data,
		Encoding:  encoding,
	}
	resp.Complete = true
	return resp, nil
}

// GitList lists entries in a git tree.
func (e *Engine) GitList(gitPath string) (*protocol.ListResponse, error) {
	gp, err := gitprovider.ParseGitPath(gitPath)
	if err != nil {
		return nil, err
	}

	repo, _, err := e.openGitRepo(gp.Repo)
	if err != nil {
		return nil, err
	}

	tree, _, err := e.gitProvider.GetTree(repo, gp.Ref)
	if err != nil {
		return nil, err
	}

	entries, err := e.gitProvider.ListTree(tree, gp.Path)
	if err != nil {
		return nil, err
	}

	return &protocol.ListResponse{
		Path:     gp.Path,
		Source:   "git",
		Entries:  entries,
		Total:    len(entries),
		Complete: true,
	}, nil
}

// Refs lists git refs for a repository.
func (e *Engine) Refs(repoName string, branches, tags, remotes bool) (*protocol.RefsResponse, error) {
	repo, _, err := e.openGitRepo(repoName)
	if err != nil {
		return nil, err
	}

	// Default: show all.
	if !branches && !tags && !remotes {
		branches, tags, remotes = true, true, true
	}

	resp := &protocol.RefsResponse{
		Repo: repoName,
	}

	refs, err := repo.References()
	if err != nil {
		return nil, fmt.Errorf("listing refs: %w", err)
	}
	defer refs.Close()

	err = refs.ForEach(func(ref *plumbing.Reference) error {
		name := ref.Name().String()
		hash := ref.Hash().String()

		switch {
		case ref.Name().IsBranch() && branches:
			resp.Refs = append(resp.Refs, protocol.GitRef{
				Name: ref.Name().Short(),
				Type: "branch",
				Hash: hash,
			})
		case ref.Name().IsTag() && tags:
			resp.Refs = append(resp.Refs, protocol.GitRef{
				Name: ref.Name().Short(),
				Type: "tag",
				Hash: hash,
			})
		case ref.Name().IsRemote() && remotes:
			parts := strings.SplitN(strings.TrimPrefix(name, "refs/remotes/"), "/", 2)
			remote := ""
			shortName := ref.Name().Short()
			if len(parts) == 2 {
				remote = parts[0]
			}
			resp.Refs = append(resp.Refs, protocol.GitRef{
				Name:   shortName,
				Type:   "remote",
				Hash:   hash,
				Remote: remote,
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return resp, nil
}

// LogParams controls git log queries.
type LogParams struct {
	MaxCount    int    // 0 = default (20)
	Skip        int    // skip this many commits before collecting entries
	StartHash   string // for pagination: continue past this commit hash
	StartRev    string // for pagination: original revision spec to re-walk
	Verbose     bool   // include tree hash, parent hashes, committer details
	FirstParent bool   // follow only the first parent of each commit

	// Filters — applied after the walk, before pagination slicing.
	// All are optional; zero values mean "no filtering on this axis".

	// PathGlob: doublestar glob matched against each changed file's
	// path. A commit is kept if at least one changed file matches.
	PathGlob string
	// Since: keep commits whose author date is on or after this time.
	Since *time.Time
	// Until: keep commits whose author date is on or before this time.
	Until *time.Time
	// Author: regex matched against "Name <email>" of the commit author.
	Author *regexp.Regexp
	// Grep: regex matched against the commit message.
	Grep *regexp.Regexp
}

// Log returns git commit log entries for the given revision spec.
//
// The ref argument is parsed as a gitrevisions(7) range expression
// (see ParseRevSet for the supported grammar). A bare single ref
// behaves as before — walking history from that tip.
func (e *Engine) Log(repoName, ref string, params LogParams) (*protocol.LogResponse, error) {
	repo, _, err := e.openGitRepo(repoName)
	if err != nil {
		return nil, err
	}

	// Pagination continuations carry the original spec; otherwise use
	// the supplied ref. An empty spec defaults to HEAD.
	spec := ref
	if params.StartRev != "" {
		spec = params.StartRev
	}
	if spec == "" {
		spec = "HEAD"
	}

	maxCount := params.MaxCount
	if maxCount <= 0 {
		maxCount = 20
	}

	revset, err := e.gitProvider.ParseRevSet(repo, spec)
	if err != nil {
		return nil, err
	}

	walked, err := e.gitProvider.Walk(repo, revset, gitprovider.WalkOptions{
		FirstParent: params.FirstParent,
	})
	if err != nil {
		return nil, err
	}

	// Apply filters (since/until/author/grep/path) to the walked output
	// before pagination. Each kept commit retains any pre-computed
	// changed-file list so we don't recompute it when building entries.
	type filtered struct {
		commit  *object.Commit
		side    gitprovider.Side
		changes []protocol.FileChange // nil when not yet computed
	}
	kept := make([]filtered, 0, len(walked))
	for _, w := range walked {
		if !commitPassesScalarFilters(w.Commit, params) {
			continue
		}
		var changes []protocol.FileChange
		if params.PathGlob != "" {
			changes = collectCommitChanges(w.Commit)
			if !anyChangePathMatches(changes, params.PathGlob) {
				continue
			}
		}
		kept = append(kept, filtered{commit: w.Commit, side: w.Side, changes: changes})
	}

	// Apply pagination: drop everything up to and including StartHash,
	// then drop Skip more entries.
	if params.StartHash != "" {
		for i, k := range kept {
			if k.commit.Hash.String() == params.StartHash {
				kept = kept[i+1:]
				break
			}
		}
	}
	if params.Skip > 0 {
		if params.Skip >= len(kept) {
			kept = nil
		} else {
			kept = kept[params.Skip:]
		}
	}

	resp := &protocol.LogResponse{
		Repo:    repoName,
		Ref:     ref,
		Skipped: params.Skip,
	}

	hitLimit := len(kept) > maxCount
	if hitLimit {
		kept = kept[:maxCount]
	}

	for _, k := range kept {
		entry := buildLogEntry(k.commit, k.side, params.Verbose)
		// Reuse precomputed changes from the path filter, if any.
		if k.changes != nil {
			entry.Changes = k.changes
			entry.FilesChanged = entry.FilesChanged[:0]
			for _, ch := range k.changes {
				entry.FilesChanged = append(entry.FilesChanged, ch.Path)
			}
		}
		resp.Entries = append(resp.Entries, entry)
	}

	resp.Total = len(resp.Entries)
	resp.Complete = !hitLimit

	if !resp.Complete && len(resp.Entries) > 0 {
		lastHash := resp.Entries[len(resp.Entries)-1].Hash
		tok, tokErr := e.EncodeListContinuation(&ListContinuationToken{
			Tool:    "log",
			Path:    repoName,
			Limit:   maxCount,
			Hash:    lastHash,
			RevSpec: spec,
		})
		if tokErr != nil {
			return nil, tokErr
		}
		resp.Continuation = tok
	}

	return resp, nil
}

// ParseLogDate accepts RFC3339, "2006-01-02T15:04:05" (assumed UTC),
// or a date-only "YYYY-MM-DD" form (midnight UTC). Used by both the
// CLI's --since/--until flags and the MCP tool's since/until args.
func ParseLogDate(s string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized date %q (use RFC3339 or YYYY-MM-DD)", s)
}

// commitPassesScalarFilters checks the cheap (no diff required) filters.
func commitPassesScalarFilters(c *object.Commit, p LogParams) bool {
	if p.Since != nil && c.Author.When.Before(*p.Since) {
		return false
	}
	if p.Until != nil && c.Author.When.After(*p.Until) {
		return false
	}
	if p.Author != nil {
		stamp := c.Author.Name + " <" + c.Author.Email + ">"
		if !p.Author.MatchString(stamp) {
			return false
		}
	}
	if p.Grep != nil && !p.Grep.MatchString(c.Message) {
		return false
	}
	return true
}

// anyChangePathMatches reports whether any change's path satisfies the
// glob. The pattern is matched with doublestar (the same matcher used
// elsewhere in aifr) against the changed file's path.
func anyChangePathMatches(changes []protocol.FileChange, glob string) bool {
	for _, ch := range changes {
		if ok, _ := doublestar.PathMatch(glob, ch.Path); ok {
			return true
		}
	}
	return false
}

// collectCommitChanges returns the (parent[0]→commit) diff as
// FileChange records, or nil if the commit has no parent or the diff
// fails. Used to drive the PathGlob filter and to populate FileChange
// entries on the LogEntry.
func collectCommitChanges(commit *object.Commit) []protocol.FileChange {
	if commit.NumParents() == 0 {
		return nil
	}
	parent, err := commit.Parent(0)
	if err != nil {
		return nil
	}
	parentTree, err := parent.Tree()
	if err != nil {
		return nil
	}
	currentTree, err := commit.Tree()
	if err != nil {
		return nil
	}
	diffs, err := parentTree.Diff(currentTree)
	if err != nil {
		return nil
	}
	out := make([]protocol.FileChange, 0, len(diffs))
	for _, ch := range diffs {
		name := ch.To.Name
		if name == "" {
			name = ch.From.Name
		}
		action := "M"
		if a, aErr := ch.Action(); aErr == nil {
			switch a {
			case merkletrie.Insert:
				action = "A"
			case merkletrie.Delete:
				action = "D"
			case merkletrie.Modify:
				action = "M"
			}
		}
		out = append(out, protocol.FileChange{Path: name, Action: action})
	}
	return out
}

// buildLogEntry converts a walked commit into a protocol LogEntry,
// computing changed-file metadata against the commit's first parent.
// For multi-parent merges this matches `git log` defaults; combined
// (`--cc` / `-m`) merge views are out of scope for now.
func buildLogEntry(commit *object.Commit, side gitprovider.Side, verbose bool) protocol.LogEntry {
	entry := protocol.LogEntry{
		Hash:        commit.Hash.String(),
		Author:      commit.Author.Name,
		AuthorEmail: commit.Author.Email,
		Date:        commit.Author.When.UTC().Format("2006-01-02T15:04:05Z"),
		Message:     sanitizeMessage(strings.TrimSpace(commit.Message)),
		Side:        string(side),
	}

	if verbose {
		entry.TreeHash = commit.TreeHash.String()
		for _, ph := range commit.ParentHashes {
			entry.ParentHashes = append(entry.ParentHashes, ph.String())
		}
		if commit.Committer.Name != commit.Author.Name ||
			commit.Committer.Email != commit.Author.Email ||
			!commit.Committer.When.Equal(commit.Author.When) {
			entry.Committer = commit.Committer.Name
			entry.CommitterEmail = commit.Committer.Email
			entry.CommitterDate = commit.Committer.When.UTC().Format("2006-01-02T15:04:05Z")
		}
	}

	for _, ch := range collectCommitChanges(commit) {
		entry.FilesChanged = append(entry.FilesChanged, ch.Path)
		entry.Changes = append(entry.Changes, ch)
	}

	return entry
}

// sanitizeMessage replaces control characters (especially \r) in commit
// messages with visible representations to prevent terminal manipulation
// and ensure safe display in all output formats.
func sanitizeMessage(msg string) string {
	if !strings.ContainsAny(msg, "\r\x00\x01\x02\x03\x04\x05\x06\x07\x08\x0b\x0c\x0e\x0f\x10\x11\x12\x13\x14\x15\x16\x17\x18\x19\x1a\x1b\x1c\x1d\x1e\x1f\x7f") {
		return msg
	}
	var b strings.Builder
	b.Grow(len(msg))
	for _, r := range msg {
		switch {
		case r == '\n' || r == '\t':
			// Preserve newlines and tabs — they're structurally meaningful.
			b.WriteRune(r)
		case r == '\r':
			b.WriteString("\\r")
		case r == '\x1b':
			b.WriteString("\\e")
		case r < 0x20 || r == 0x7f:
			// C0 control characters and DEL: show as \xNN.
			fmt.Fprintf(&b, "\\x%02x", r)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// DiffParams controls the diff operation mode.
type DiffParams struct {
	ByteLevel bool // if true, byte-level comparison (cmp mode)
}

// Diff compares two files (filesystem or git).
func (e *Engine) Diff(pathA, pathB string, params DiffParams) (*protocol.DiffResponse, error) {
	contentA, err := e.readContentForDiff(pathA)
	if err != nil {
		return nil, fmt.Errorf("reading %q: %w", pathA, err)
	}

	contentB, err := e.readContentForDiff(pathB)
	if err != nil {
		return nil, fmt.Errorf("reading %q: %w", pathB, err)
	}

	resp := &protocol.DiffResponse{
		PathA:  pathA,
		PathB:  pathB,
		Source: "mixed",
	}

	if params.ByteLevel {
		resp.Identical, resp.ByteDiff = computeByteDiff(contentA, contentB)
	} else {
		resp.Identical = contentA == contentB
		if !resp.Identical {
			resp.Hunks = computeDiff(contentA, contentB)
		}
	}

	return resp, nil
}

// computeByteDiff finds the first byte-level difference (cmp mode).
func computeByteDiff(a, b string) (identical bool, bd *protocol.ByteDiff) {
	if a == b {
		return true, nil
	}

	dataA := []byte(a)
	dataB := []byte(b)
	n := min(len(dataA), len(dataB))

	line := 1
	col := 1
	for i := range n {
		if dataA[i] != dataB[i] {
			return false, &protocol.ByteDiff{
				Offset: int64(i),
				Line:   line,
				Column: col,
				ByteA:  dataA[i],
				ByteB:  dataB[i],
				SizeA:  int64(len(dataA)),
				SizeB:  int64(len(dataB)),
			}
		}
		if dataA[i] == '\n' {
			line++
			col = 1
		} else {
			col++
		}
	}

	// Files differ in length but share a common prefix.
	return false, &protocol.ByteDiff{
		Offset: int64(n),
		Line:   line,
		Column: col,
		SizeA:  int64(len(dataA)),
		SizeB:  int64(len(dataB)),
	}
}

// readContentForDiff reads content from a filesystem path or git path.
func (e *Engine) readContentForDiff(path string) (string, error) {
	if gitprovider.IsGitPath(path) {
		gp, err := gitprovider.ParseGitPath(path)
		if err != nil {
			return "", err
		}
		repo, _, err := e.openGitRepo(gp.Repo)
		if err != nil {
			return "", err
		}
		tree, _, err := e.gitProvider.GetTree(repo, gp.Ref)
		if err != nil {
			return "", err
		}
		f, err := e.gitProvider.GetBlob(tree, gp.Path)
		if err != nil {
			return "", err
		}
		return f.Contents()
	}

	// Filesystem path.
	resolved, err := e.checkAccess(path)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(resolved)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// computeDiff produces unified diff hunks between two strings.
func computeDiff(a, b string) []protocol.DiffHunk {
	linesA := strings.Split(a, "\n")
	linesB := strings.Split(b, "\n")

	// Simple line-by-line diff using longest common subsequence.
	// For a production tool we'd use a proper diff algorithm, but this
	// covers the basic case well.
	var hunks []protocol.DiffHunk

	// Use the go-diff library which is already a transitive dependency via go-git.
	// For now, a simple approach: find contiguous changed regions.
	i, j := 0, 0
	for i < len(linesA) || j < len(linesB) {
		// Skip common lines.
		if i < len(linesA) && j < len(linesB) && linesA[i] == linesB[j] {
			i++
			j++
			continue
		}

		// Start of a hunk.
		hunkStartA := i
		hunkStartB := j
		var lines []string

		// Collect changed lines until we find common lines again.
		for i < len(linesA) && (j >= len(linesB) || (i < len(linesA) && linesA[i] != linesB[j])) {
			// Check if this line from A appears soon in B (within 3 lines).
			found := false
			for k := j; k < len(linesB) && k < j+3; k++ {
				if linesA[i] == linesB[k] {
					found = true
					break
				}
			}
			if found {
				break
			}
			lines = append(lines, "-"+linesA[i])
			i++
		}

		for j < len(linesB) && (i >= len(linesA) || (j < len(linesB) && linesB[j] != linesA[i])) {
			found := false
			for k := i; k < len(linesA) && k < i+3; k++ {
				if linesB[j] == linesA[k] {
					found = true
					break
				}
			}
			if found {
				break
			}
			lines = append(lines, "+"+linesB[j])
			j++
		}

		if len(lines) > 0 {
			hunks = append(hunks, protocol.DiffHunk{
				OldStart: hunkStartA + 1,
				OldLines: i - hunkStartA,
				NewStart: hunkStartB + 1,
				NewLines: j - hunkStartB,
				Lines:    lines,
			})
		}
	}

	return hunks
}

func countTotalLines(s string) int {
	if s == "" {
		return 0
	}
	count := 1
	for _, c := range s {
		if c == '\n' {
			count++
		}
	}
	if s[len(s)-1] == '\n' {
		count--
	}
	return count
}

func encodeBase64(data []byte) string {
	return "base64:" + fmt.Sprintf("%d bytes", len(data))
}

func lastPathElement(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[i+1:]
		}
	}
	return path
}

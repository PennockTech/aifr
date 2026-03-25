// Copyright 2026 — see LICENSE file for terms.
package engine

import (
	"fmt"
	"os"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"

	"go.pennock.tech/aifr/internal/gitprovider"
	"go.pennock.tech/aifr/pkg/protocol"
)

// openGitRepo opens a git repository and checks access control for
// filesystem-path repos. Named repos and CWD auto-detect skip the
// access check since they are admin-configured or implicitly allowed.
func (e *Engine) openGitRepo(repoIdentifier string) (*git.Repository, string, error) {
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

// Log returns git commit log entries.
func (e *Engine) Log(repoName, ref string, maxCount int) (*protocol.LogResponse, error) {
	repo, _, err := e.openGitRepo(repoName)
	if err != nil {
		return nil, err
	}

	if ref == "" {
		ref = "HEAD"
	}

	commit, err := e.gitProvider.ResolveRef(repo, ref)
	if err != nil {
		return nil, err
	}

	resp := &protocol.LogResponse{
		Repo: repoName,
		Ref:  ref,
	}

	if maxCount <= 0 {
		maxCount = 20
	}

	current := commit
	for i := 0; i < maxCount && current != nil; i++ {
		entry := protocol.LogEntry{
			Hash:        current.Hash.String(),
			Author:      current.Author.Name,
			AuthorEmail: current.Author.Email,
			Date:        current.Author.When.UTC().Format("2006-01-02T15:04:05Z"),
			Message:     strings.TrimSpace(current.Message),
		}

		// Get changed files (compare with parent).
		if current.NumParents() > 0 {
			parent, pErr := current.Parent(0)
			if pErr == nil {
				parentTree, _ := parent.Tree()
				currentTree, _ := current.Tree()
				if parentTree != nil && currentTree != nil {
					changes, cErr := parentTree.Diff(currentTree)
					if cErr == nil {
						for _, ch := range changes {
							name := ch.To.Name
							if name == "" {
								name = ch.From.Name
							}
							entry.FilesChanged = append(entry.FilesChanged, name)
						}
					}
				}
			}
		}

		resp.Entries = append(resp.Entries, entry)

		if current.NumParents() == 0 {
			break
		}
		current, err = current.Parent(0)
		if err != nil {
			break
		}
	}

	resp.Total = len(resp.Entries)
	return resp, nil
}

// Diff compares two files (filesystem or git).
func (e *Engine) Diff(pathA, pathB string) (*protocol.DiffResponse, error) {
	contentA, err := e.readContentForDiff(pathA)
	if err != nil {
		return nil, fmt.Errorf("reading %q: %w", pathA, err)
	}

	contentB, err := e.readContentForDiff(pathB)
	if err != nil {
		return nil, fmt.Errorf("reading %q: %w", pathB, err)
	}

	hunks := computeDiff(contentA, contentB)

	return &protocol.DiffResponse{
		PathA:  pathA,
		PathB:  pathB,
		Source: "mixed",
		Hunks:  hunks,
	}, nil
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

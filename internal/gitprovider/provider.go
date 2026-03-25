// Copyright 2026 — see LICENSE file for terms.
package gitprovider

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"

	"go.pennock.tech/aifr/pkg/protocol"
)

// Provider handles git operations.
type Provider struct {
	namedRepos map[string]string // name → absolute path
}

// NewProvider creates a new git provider.
func NewProvider(namedRepos map[string]string) *Provider {
	repos := make(map[string]string, len(namedRepos))
	maps.Copy(repos, namedRepos)
	return &Provider{namedRepos: repos}
}

// OpenRepo opens a git repository by name, filesystem path, or by walking up from cwd.
//
// When name is empty, auto-detects by walking up from the current directory.
// When name matches a configured named repo, opens that.
// When name looks like a filesystem path (starts with /, ./, or ../),
// walks up from that path to find a git repository.
// Otherwise, returns an error for unknown repo name.
func (p *Provider) OpenRepo(name string) (*git.Repository, string, error) {
	if name == "" {
		return p.openRepoFromWalk("", "current directory or parents")
	}

	// Try named repo first.
	if repoPath, ok := p.namedRepos[name]; ok {
		repo, err := git.PlainOpen(repoPath)
		if err != nil {
			return nil, "", protocol.NewError(protocol.ErrInvalidRef,
				fmt.Sprintf("cannot open git repo at %q: %v", repoPath, err))
		}
		return repo, repoPath, nil
	}

	// If it looks like a filesystem path, walk up to find a repo.
	if LooksLikePath(name) {
		return p.openRepoFromWalk(name, name)
	}

	return nil, "", protocol.NewError(protocol.ErrInvalidRef,
		fmt.Sprintf("unknown git repo name %q", name))
}

// LooksLikePath returns true if name appears to be a filesystem path
// rather than a short repo name.
func LooksLikePath(name string) bool {
	return strings.HasPrefix(name, "/") ||
		strings.HasPrefix(name, "./") ||
		strings.HasPrefix(name, "../") ||
		name == "." || name == ".."
}

// openRepoFromWalk walks up from startDir (or cwd if empty) to find a git repo.
func (p *Provider) openRepoFromWalk(startDir, desc string) (*git.Repository, string, error) {
	var dir string
	if startDir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, "", fmt.Errorf("getting cwd: %w", err)
		}
		dir = cwd
	} else {
		abs, err := filepath.Abs(startDir)
		if err != nil {
			return nil, "", protocol.NewError(protocol.ErrInvalidRef,
				fmt.Sprintf("cannot resolve path %q: %v", startDir, err))
		}
		dir = abs
	}

	for {
		repo, err := git.PlainOpen(dir)
		if err == nil {
			return repo, dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return nil, "", protocol.NewError(protocol.ErrInvalidRef,
		fmt.Sprintf("no git repository found at or above %s", desc))
}

// ResolveRef resolves a ref string to a commit hash.
// Handles: branch names, tag names, commit hashes, HEAD, HEAD~N, branch^N.
func (p *Provider) ResolveRef(repo *git.Repository, ref string) (*object.Commit, error) {
	// Handle relative refs (HEAD~3, main~2, branch^1, etc.)
	baseRef, offset, isRelative := parseRelativeRef(ref)

	var hash plumbing.Hash

	if isRelative {
		commit, err := p.resolveSimpleRef(repo, baseRef)
		if err != nil {
			return nil, err
		}
		return walkParents(commit, offset)
	}

	// Try as a direct ref first.
	commit, err := p.resolveSimpleRef(repo, ref)
	if err == nil {
		return commit, nil
	}

	// Try as a short commit hash.
	if len(ref) >= 4 && len(ref) <= 40 && isHex(ref) {
		// Try to match against objects.
		iter, iterErr := repo.CommitObjects()
		if iterErr == nil {
			defer iter.Close()
			_ = iter.ForEach(func(c *object.Commit) error {
				if strings.HasPrefix(c.Hash.String(), ref) {
					hash = c.Hash
					return fmt.Errorf("found") // stop iteration
				}
				return nil
			})
			if !hash.IsZero() {
				return repo.CommitObject(hash)
			}
		}
	}

	return nil, protocol.NewError(protocol.ErrInvalidRef,
		fmt.Sprintf("cannot resolve ref %q", ref))
}

// resolveSimpleRef resolves a non-relative ref (branch, tag, HEAD, full hash).
func (p *Provider) resolveSimpleRef(repo *git.Repository, ref string) (*object.Commit, error) {
	// Try HEAD.
	if ref == "HEAD" {
		head, err := repo.Head()
		if err != nil {
			return nil, protocol.NewError(protocol.ErrInvalidRef, "cannot resolve HEAD")
		}
		return repo.CommitObject(head.Hash())
	}

	// Try as a full hash.
	if len(ref) == 40 && isHex(ref) {
		hash := plumbing.NewHash(ref)
		return repo.CommitObject(hash)
	}

	// Try as a branch (local).
	branchRef := plumbing.NewBranchReferenceName(ref)
	r, err := repo.Reference(branchRef, true)
	if err == nil {
		return repo.CommitObject(r.Hash())
	}

	// Try as a remote-tracking branch (refs/remotes/...).
	remoteRef := plumbing.NewRemoteReferenceName("origin", ref)
	r, err = repo.Reference(remoteRef, true)
	if err == nil {
		return repo.CommitObject(r.Hash())
	}

	// Try as a remote ref with explicit remote prefix (e.g., "origin/main").
	if strings.Contains(ref, "/") {
		fullRef := plumbing.ReferenceName("refs/remotes/" + ref)
		r, err = repo.Reference(fullRef, true)
		if err == nil {
			return repo.CommitObject(r.Hash())
		}
	}

	// Try as a tag.
	tagRef := plumbing.NewTagReferenceName(ref)
	r, err = repo.Reference(tagRef, true)
	if err == nil {
		// May be an annotated tag; peel to commit.
		commit, err := repo.CommitObject(r.Hash())
		if err == nil {
			return commit, nil
		}
		// Try peeling annotated tag → commit.
		tagObj, tagErr := repo.TagObject(r.Hash())
		if tagErr == nil {
			targetCommit, targetErr := tagObj.Commit()
			if targetErr == nil {
				return targetCommit, nil
			}
		}
	}

	return nil, protocol.NewError(protocol.ErrInvalidRef,
		fmt.Sprintf("cannot resolve ref %q", ref))
}

// GetTree resolves a ref to its root tree.
func (p *Provider) GetTree(repo *git.Repository, ref string) (*object.Tree, *object.Commit, error) {
	commit, err := p.ResolveRef(repo, ref)
	if err != nil {
		return nil, nil, err
	}
	tree, err := commit.Tree()
	if err != nil {
		return nil, nil, fmt.Errorf("getting tree for commit %s: %w", commit.Hash, err)
	}
	return tree, commit, nil
}

// GetBlob reads a file from a git tree.
func (p *Provider) GetBlob(tree *object.Tree, path string) (*object.File, error) {
	if path == "" || path == "." {
		return nil, protocol.NewError(protocol.ErrIsDirectory, "cannot read root tree as a file")
	}
	f, err := tree.File(path)
	if err != nil {
		return nil, protocol.NewPathError(protocol.ErrNotFound, path,
			fmt.Sprintf("path not found in git tree: %v", err))
	}
	return f, nil
}

// ListTree lists entries in a git tree at the given path.
func (p *Provider) ListTree(tree *object.Tree, path string) ([]protocol.StatEntry, error) {
	if path != "" && path != "." {
		subtree, err := tree.Tree(path)
		if err != nil {
			return nil, protocol.NewPathError(protocol.ErrNotFound, path,
				fmt.Sprintf("path not found in git tree: %v", err))
		}
		tree = subtree
	}

	var entries []protocol.StatEntry
	for _, entry := range tree.Entries {
		entryType := "file"
		if entry.Mode.IsFile() {
			entryType = "file"
		} else {
			entryType = "dir"
		}

		e := protocol.StatEntry{
			Name:       entry.Name,
			Path:       filepath.Join(path, entry.Name),
			Type:       entryType,
			ObjectHash: entry.Hash.String(),
			Mode:       entry.Mode.String(),
		}

		// Get size for files.
		if entryType == "file" {
			blob, err := tree.TreeEntryFile(&entry)
			if err == nil {
				e.Size = blob.Size
			}
		}

		entries = append(entries, e)
	}
	return entries, nil
}

// parseRelativeRef parses refs like "HEAD~3", "main~2", "branch^1".
// Returns the base ref, the offset, and whether it's a relative ref.
func parseRelativeRef(ref string) (string, int, bool) {
	// Try ~ (ancestor)
	if idx := strings.LastIndexByte(ref, '~'); idx >= 0 {
		base := ref[:idx]
		numStr := ref[idx+1:]
		if numStr == "" {
			return base, 1, true
		}
		n, err := strconv.Atoi(numStr)
		if err == nil && n >= 0 {
			return base, n, true
		}
	}

	// Try ^ (parent)
	if idx := strings.LastIndexByte(ref, '^'); idx >= 0 {
		base := ref[:idx]
		numStr := ref[idx+1:]
		if numStr == "" {
			return base, 1, true
		}
		n, err := strconv.Atoi(numStr)
		if err == nil && n >= 0 {
			return base, n, true
		}
	}

	return ref, 0, false
}

// walkParents walks N parent commits from the given commit.
func walkParents(commit *object.Commit, n int) (*object.Commit, error) {
	current := commit
	for range n {
		if current.NumParents() == 0 {
			return nil, protocol.NewError(protocol.ErrInvalidRef,
				fmt.Sprintf("commit %s has no parent (requested ~%d)", current.Hash, n))
		}
		parent, err := current.Parent(0)
		if err != nil {
			return nil, protocol.NewError(protocol.ErrInvalidRef,
				fmt.Sprintf("cannot get parent of %s: %v", current.Hash, err))
		}
		current = parent
	}
	return current, nil
}

// isHex returns true if s contains only hex characters.
func isHex(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return len(s) > 0
}

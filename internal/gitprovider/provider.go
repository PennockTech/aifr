// Copyright 2026 — see LICENSE file for terms.
package gitprovider

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
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

// ResolveRef resolves a ref string to a commit object.
//
// Delegates the bulk of the parsing to go-git's ResolveRevision, which
// understands the gitrevisions(7) grammar for single revisions:
// HEAD, branches, tags, full and prefix hashes, ~N / ^N (correctly
// honouring the Nth parent on merge commits), arbitrary chaining like
// HEAD~3^2~1, type peeling (^{commit}, ^{tree}, ^{blob}, ^{}), and text
// search (^{/regex}, :/regex). Annotated tags are peeled automatically.
//
// As a convenience also tried by the previous implementation, a bare
// "@" is accepted as an alias for HEAD, and a name that does not contain
// a slash is also tried as "origin/<name>" so that "main" resolves to
// the remote-tracking branch when no local branch of that name exists.
func (p *Provider) ResolveRef(repo *git.Repository, ref string) (*object.Commit, error) {
	if ref == "" {
		return nil, protocol.NewError(protocol.ErrInvalidRef, "empty ref")
	}

	// "@" is the gitrevisions(7) alias for HEAD; ResolveRevision in
	// go-git 5.17 does not handle it, so substitute up front.
	canonical := ref
	if canonical == "@" {
		canonical = "HEAD"
	}

	if commit, err := p.tryResolve(repo, canonical); err == nil {
		return commit, nil
	}

	// Convenience fallback: bare names without a slash also try the
	// origin remote-tracking branch (mirrors prior behaviour).
	if !strings.ContainsAny(ref, "/^~@:") {
		if commit, err := p.tryResolve(repo, "refs/remotes/origin/"+ref); err == nil {
			return commit, nil
		}
	}

	return nil, protocol.NewError(protocol.ErrInvalidRef,
		fmt.Sprintf("cannot resolve ref %q", ref))
}

// tryResolve attempts to resolve a single revision via go-git.
// Returns the underlying error so callers can decide whether to fall back.
func (p *Provider) tryResolve(repo *git.Repository, rev string) (*object.Commit, error) {
	hashPtr, err := repo.ResolveRevision(plumbing.Revision(rev))
	if err != nil {
		return nil, err
	}
	return repo.CommitObject(*hashPtr)
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

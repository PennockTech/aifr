// Copyright 2026 — see LICENSE file for terms.
package gitprovider

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// initTestRepo creates a git repo with some commits for testing.
func initTestRepo(t *testing.T) (string, *git.Repository) {
	t.Helper()
	dir := t.TempDir()

	repo, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatal(err)
	}

	wt, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}

	// Create a file and commit.
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Test\n"), 0o644)                           //nolint:errcheck
	os.MkdirAll(filepath.Join(dir, "src"), 0o755)                                                      //nolint:errcheck
	os.WriteFile(filepath.Join(dir, "src/main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644) //nolint:errcheck

	wt.Add("README.md")   //nolint:errcheck
	wt.Add("src/main.go") //nolint:errcheck

	sig := &object.Signature{
		Name:  "Test",
		Email: "test@example.com",
		When:  time.Now(),
	}

	_, err = wt.Commit("Initial commit", &git.CommitOptions{Author: sig})
	if err != nil {
		t.Fatal(err)
	}

	// Create a second commit.
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Test v2\n\nUpdated.\n"), 0o644) //nolint:errcheck
	wt.Add("README.md")                                                                     //nolint:errcheck
	_, err = wt.Commit("Update README", &git.CommitOptions{Author: sig})
	if err != nil {
		t.Fatal(err)
	}

	// Create a tag.
	head, _ := repo.Head()
	repo.CreateTag("v1.0", head.Hash(), &git.CreateTagOptions{
		Tagger:  sig,
		Message: "v1.0 release",
	}) //nolint:errcheck

	// Create a branch.
	branchRef := plumbing.NewBranchReferenceName("feature/test")
	repo.Storer.SetReference(plumbing.NewHashReference(branchRef, head.Hash())) //nolint:errcheck

	return dir, repo
}

func TestResolveRefHEAD(t *testing.T) {
	_, repo := initTestRepo(t)
	p := NewProvider(nil)

	commit, err := p.ResolveRef(repo, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if commit.Message != "Update README" {
		t.Errorf("HEAD message = %q, want 'Update README'", commit.Message)
	}
}

func TestResolveRefRelative(t *testing.T) {
	_, repo := initTestRepo(t)
	p := NewProvider(nil)

	// HEAD~1 should be the initial commit.
	commit, err := p.ResolveRef(repo, "HEAD~1")
	if err != nil {
		t.Fatal(err)
	}
	if commit.Message != "Initial commit" {
		t.Errorf("HEAD~1 message = %q, want 'Initial commit'", commit.Message)
	}
}

func TestResolveRefBranch(t *testing.T) {
	_, repo := initTestRepo(t)
	p := NewProvider(nil)

	commit, err := p.ResolveRef(repo, "feature/test")
	if err != nil {
		t.Fatal(err)
	}
	if commit.Message != "Update README" {
		t.Errorf("feature/test message = %q, want 'Update README'", commit.Message)
	}
}

func TestResolveRefTag(t *testing.T) {
	_, repo := initTestRepo(t)
	p := NewProvider(nil)

	commit, err := p.ResolveRef(repo, "v1.0")
	if err != nil {
		t.Fatal(err)
	}
	if commit.Message != "Update README" {
		t.Errorf("v1.0 message = %q, want 'Update README'", commit.Message)
	}
}

func TestResolveRefFullHash(t *testing.T) {
	_, repo := initTestRepo(t)
	p := NewProvider(nil)

	head, _ := repo.Head()
	fullHash := head.Hash().String()

	commit, err := p.ResolveRef(repo, fullHash)
	if err != nil {
		t.Fatal(err)
	}
	if commit.Hash.String() != fullHash {
		t.Errorf("hash mismatch")
	}
}

func TestResolveRefShortHash(t *testing.T) {
	_, repo := initTestRepo(t)
	p := NewProvider(nil)

	head, _ := repo.Head()
	shortHash := head.Hash().String()[:7]

	commit, err := p.ResolveRef(repo, shortHash)
	if err != nil {
		t.Fatal(err)
	}
	if commit.Hash != head.Hash() {
		t.Errorf("short hash resolved to wrong commit")
	}
}

func TestResolveRefInvalid(t *testing.T) {
	_, repo := initTestRepo(t)
	p := NewProvider(nil)

	_, err := p.ResolveRef(repo, "nonexistent-branch")
	if err == nil {
		t.Error("expected error for invalid ref")
	}
}

func TestGetBlob(t *testing.T) {
	_, repo := initTestRepo(t)
	p := NewProvider(nil)

	tree, _, err := p.GetTree(repo, "HEAD")
	if err != nil {
		t.Fatal(err)
	}

	f, err := p.GetBlob(tree, "README.md")
	if err != nil {
		t.Fatal(err)
	}

	content, err := f.Contents()
	if err != nil {
		t.Fatal(err)
	}
	if content != "# Test v2\n\nUpdated.\n" {
		t.Errorf("content = %q, want '# Test v2\\n\\nUpdated.\\n'", content)
	}
}

func TestGetBlobSubdir(t *testing.T) {
	_, repo := initTestRepo(t)
	p := NewProvider(nil)

	tree, _, err := p.GetTree(repo, "HEAD")
	if err != nil {
		t.Fatal(err)
	}

	f, err := p.GetBlob(tree, "src/main.go")
	if err != nil {
		t.Fatal(err)
	}

	content, err := f.Contents()
	if err != nil {
		t.Fatal(err)
	}
	if content != "package main\n\nfunc main() {}\n" {
		t.Errorf("content = %q", content)
	}
}

func TestGetBlobNotFound(t *testing.T) {
	_, repo := initTestRepo(t)
	p := NewProvider(nil)

	tree, _, err := p.GetTree(repo, "HEAD")
	if err != nil {
		t.Fatal(err)
	}

	_, err = p.GetBlob(tree, "nonexistent.txt")
	if err == nil {
		t.Error("expected NOT_FOUND error")
	}
}

func TestListTree(t *testing.T) {
	_, repo := initTestRepo(t)
	p := NewProvider(nil)

	tree, _, err := p.GetTree(repo, "HEAD")
	if err != nil {
		t.Fatal(err)
	}

	entries, err := p.ListTree(tree, "")
	if err != nil {
		t.Fatal(err)
	}

	// Should have README.md and src/
	if len(entries) != 2 {
		t.Errorf("entries = %d, want 2", len(entries))
		for _, e := range entries {
			t.Logf("  %s (%s)", e.Name, e.Type)
		}
	}
}

func TestListTreeSubdir(t *testing.T) {
	_, repo := initTestRepo(t)
	p := NewProvider(nil)

	tree, _, err := p.GetTree(repo, "HEAD")
	if err != nil {
		t.Fatal(err)
	}

	entries, err := p.ListTree(tree, "src")
	if err != nil {
		t.Fatal(err)
	}

	if len(entries) != 1 || entries[0].Name != "main.go" {
		t.Errorf("expected [main.go], got %v", entries)
	}
}

func TestOpenRepoAutoDetect(t *testing.T) {
	dir, _ := initTestRepo(t)
	p := NewProvider(nil)

	origDir, _ := os.Getwd()
	defer os.Chdir(origDir) //nolint:errcheck
	os.Chdir(dir)           //nolint:errcheck

	repo, repoPath, err := p.OpenRepo("")
	if err != nil {
		t.Fatal(err)
	}
	if repo == nil {
		t.Fatal("repo is nil")
	}
	if repoPath != dir {
		t.Errorf("repoPath = %q, want %q", repoPath, dir)
	}
}

func TestOpenRepoNamed(t *testing.T) {
	dir, _ := initTestRepo(t)
	p := NewProvider(map[string]string{"test": dir})

	repo, repoPath, err := p.OpenRepo("test")
	if err != nil {
		t.Fatal(err)
	}
	if repo == nil {
		t.Fatal("repo is nil")
	}
	if repoPath != dir {
		t.Errorf("repoPath = %q, want %q", repoPath, dir)
	}
}

func TestOpenRepoByPath(t *testing.T) {
	dir, _ := initTestRepo(t)
	p := NewProvider(nil)

	repo, repoPath, err := p.OpenRepo(dir)
	if err != nil {
		t.Fatal(err)
	}
	if repo == nil {
		t.Fatal("repo is nil")
	}
	if repoPath != dir {
		t.Errorf("repoPath = %q, want %q", repoPath, dir)
	}
}

func TestOpenRepoBySubdirPath(t *testing.T) {
	dir, _ := initTestRepo(t)
	p := NewProvider(nil)

	// Opening from a subdirectory should find the repo root.
	subdir := filepath.Join(dir, "src")
	repo, repoPath, err := p.OpenRepo(subdir)
	if err != nil {
		t.Fatal(err)
	}
	if repo == nil {
		t.Fatal("repo is nil")
	}
	if repoPath != dir {
		t.Errorf("repoPath = %q, want %q", repoPath, dir)
	}
}

func TestOpenRepoNamedTakesPriority(t *testing.T) {
	dir, _ := initTestRepo(t)
	// Name a repo with the same name as a directory that exists.
	// Named repos should take priority.
	p := NewProvider(map[string]string{"test": dir})

	repo, repoPath, err := p.OpenRepo("test")
	if err != nil {
		t.Fatal(err)
	}
	if repo == nil {
		t.Fatal("repo is nil")
	}
	if repoPath != dir {
		t.Errorf("repoPath = %q, want %q", repoPath, dir)
	}
}

func TestOpenRepoUnknownName(t *testing.T) {
	p := NewProvider(nil)
	_, _, err := p.OpenRepo("nonexistent")
	if err == nil {
		t.Error("expected error for unknown repo name")
	}
}

func TestOpenRepoPathNoGit(t *testing.T) {
	dir := t.TempDir() // no .git
	p := NewProvider(nil)
	_, _, err := p.OpenRepo(dir)
	if err == nil {
		t.Error("expected error for path with no git repo")
	}
}

func TestLooksLikePath(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"/home/user/repo", true},
		{"./repo", true},
		{"../repo", true},
		{".", true},
		{"..", true},
		{"myrepo", false},
		{"infra", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := LooksLikePath(tt.input); got != tt.want {
				t.Errorf("LooksLikePath(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseRelativeRef(t *testing.T) {
	tests := []struct {
		input      string
		wantBase   string
		wantOffset int
		wantRel    bool
	}{
		{"HEAD~3", "HEAD", 3, true},
		{"main~1", "main", 1, true},
		{"HEAD~", "HEAD", 1, true},
		{"branch^1", "branch", 1, true},
		{"branch^", "branch", 1, true},
		{"HEAD", "HEAD", 0, false},
		{"main", "main", 0, false},
		{"a1b2c3d", "a1b2c3d", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			base, offset, isRel := parseRelativeRef(tt.input)
			if base != tt.wantBase || offset != tt.wantOffset || isRel != tt.wantRel {
				t.Errorf("parseRelativeRef(%q) = (%q, %d, %v), want (%q, %d, %v)",
					tt.input, base, offset, isRel, tt.wantBase, tt.wantOffset, tt.wantRel)
			}
		})
	}
}

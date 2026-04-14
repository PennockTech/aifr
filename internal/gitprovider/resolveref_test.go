// Copyright 2026 — see LICENSE file for terms.
package gitprovider

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// initMergeRepo builds a small history with a merge commit so the
// difference between ^N (Nth parent) and ~N (Nth first-parent ancestor)
// can be exercised. Layout:
//
//	A --- B --- M     (main)
//	       \   /
//	        C        (feature)
//
// The returned function commits is in topological order: [A, B, C, M].
// M's parents are (B, C); the first parent is B (main side).
func initMergeRepo(t *testing.T) (string, *git.Repository, []plumbing.Hash) {
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

	sig := func(seconds int) *object.Signature {
		return &object.Signature{
			Name:  "Test",
			Email: "test@example.com",
			When:  time.Date(2024, 1, 1, 0, 0, seconds, 0, time.UTC),
		}
	}

	write := func(path, content string) {
		t.Helper()
		full := filepath.Join(dir, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := wt.Add(path); err != nil {
			t.Fatal(err)
		}
	}

	commit := func(msg string, secs int, parents ...plumbing.Hash) plumbing.Hash {
		t.Helper()
		opts := &git.CommitOptions{Author: sig(secs), Committer: sig(secs)}
		if len(parents) > 0 {
			opts.Parents = parents
		}
		h, err := wt.Commit(msg, opts)
		if err != nil {
			t.Fatal(err)
		}
		return h
	}

	// A: initial on main.
	write("a.txt", "A\n")
	hA := commit("A", 1)

	// B: another commit on main.
	write("b.txt", "B\n")
	hB := commit("B", 2)

	// Create feature branch from A by checking it out.
	featureName := plumbing.NewBranchReferenceName("feature")
	if err := repo.Storer.SetReference(plumbing.NewHashReference(featureName, hA)); err != nil {
		t.Fatal(err)
	}
	if err := wt.Checkout(&git.CheckoutOptions{Branch: featureName}); err != nil {
		t.Fatal(err)
	}

	// C: commit on feature.
	write("c.txt", "C\n")
	hC := commit("C", 3)

	// Switch back to main and create the merge.
	mainName := plumbing.NewBranchReferenceName("master")
	// go-git's PlainInit defaults to master; double-check.
	if _, err := repo.Reference(mainName, true); err != nil {
		mainName = plumbing.NewBranchReferenceName("main")
	}
	if err := wt.Checkout(&git.CheckoutOptions{Branch: mainName}); err != nil {
		t.Fatal(err)
	}

	// M: merge commit with parents (B, C). We craft this by writing the
	// union of trees and committing with explicit parents.
	write("c.txt", "C\n") // bring C's content into main
	hM := commit("M: merge feature", 4, hB, hC)

	// Tag B for ^{}/^{commit} tests.
	if _, err := repo.CreateTag("v-b", hB, &git.CreateTagOptions{
		Tagger:  sig(2),
		Message: "tag of B",
	}); err != nil {
		t.Fatal(err)
	}

	// Configure an "origin" remote pointing nowhere so that
	// refs/remotes/origin/* references work for fallback tests.
	if _, err := repo.CreateRemote(&config.RemoteConfig{
		Name: "origin",
		URLs: []string{"https://example.invalid/repo.git"},
	}); err != nil {
		t.Fatal(err)
	}
	originMain := plumbing.NewRemoteReferenceName("origin", "trunk")
	if err := repo.Storer.SetReference(plumbing.NewHashReference(originMain, hM)); err != nil {
		t.Fatal(err)
	}

	return dir, repo, []plumbing.Hash{hA, hB, hC, hM}
}

func TestResolveRefAtAlias(t *testing.T) {
	_, repo, hashes := initMergeRepo(t)
	hM := hashes[3]
	p := NewProvider(nil)

	commit, err := p.ResolveRef(repo, "@")
	if err != nil {
		t.Fatalf("@ ref failed: %v", err)
	}
	if commit.Hash != hM {
		t.Errorf("@ resolved to %s, want HEAD=%s", commit.Hash, hM)
	}
}

func TestResolveRefCaretNthParent(t *testing.T) {
	_, repo, hashes := initMergeRepo(t)
	hB, hC, hM := hashes[1], hashes[2], hashes[3]
	p := NewProvider(nil)

	cases := []struct {
		ref  string
		want plumbing.Hash
		desc string
	}{
		{"HEAD", hM, "HEAD = M"},
		{"HEAD^", hB, "HEAD^ = first parent (B)"},
		{"HEAD^1", hB, "HEAD^1 = first parent (B)"},
		{"HEAD^2", hC, "HEAD^2 = second parent (C, feature side)"},
	}
	for _, tc := range cases {
		t.Run(tc.ref, func(t *testing.T) {
			c, err := p.ResolveRef(repo, tc.ref)
			if err != nil {
				t.Fatalf("%s failed: %v", tc.desc, err)
			}
			if c.Hash != tc.want {
				t.Errorf("%s resolved to %s, want %s", tc.desc, c.Hash, tc.want)
			}
		})
	}
}

func TestResolveRefChainedRelative(t *testing.T) {
	_, repo, hashes := initMergeRepo(t)
	hA, hB := hashes[0], hashes[1]
	p := NewProvider(nil)

	// HEAD~1 = HEAD's first-parent ancestor = B
	c, err := p.ResolveRef(repo, "HEAD~1")
	if err != nil {
		t.Fatal(err)
	}
	if c.Hash != hB {
		t.Errorf("HEAD~1 = %s, want B=%s", c.Hash, hB)
	}

	// HEAD~2 = A
	c, err = p.ResolveRef(repo, "HEAD~2")
	if err != nil {
		t.Fatal(err)
	}
	if c.Hash != hA {
		t.Errorf("HEAD~2 = %s, want A=%s", c.Hash, hA)
	}

	// HEAD^^ = HEAD^.^ = B^ = A
	c, err = p.ResolveRef(repo, "HEAD^^")
	if err != nil {
		t.Fatal(err)
	}
	if c.Hash != hA {
		t.Errorf("HEAD^^ = %s, want A=%s", c.Hash, hA)
	}

	// HEAD^2~0 = C (HEAD's 2nd parent, no further walk).
	c, err = p.ResolveRef(repo, "HEAD^2~0")
	if err != nil {
		t.Fatal(err)
	}
	if c.Hash != hashes[2] {
		t.Errorf("HEAD^2~0 = %s, want C=%s", c.Hash, hashes[2])
	}
}

func TestResolveRefTypePeeling(t *testing.T) {
	_, repo, hashes := initMergeRepo(t)
	hB := hashes[1]
	p := NewProvider(nil)

	// v-b is an annotated tag of B.
	cases := []string{"v-b", "v-b^{commit}", "v-b^{}"}
	for _, ref := range cases {
		t.Run(ref, func(t *testing.T) {
			c, err := p.ResolveRef(repo, ref)
			if err != nil {
				t.Fatalf("%s failed: %v", ref, err)
			}
			if c.Hash != hB {
				t.Errorf("%s = %s, want B=%s", ref, c.Hash, hB)
			}
		})
	}
}

func TestResolveRefMessageRegex(t *testing.T) {
	_, repo, hashes := initMergeRepo(t)
	hC := hashes[2]
	p := NewProvider(nil)

	// HEAD^{/^C$} should walk back from HEAD looking for a commit whose
	// message matches /^C$/. The merge's first-parent line is M -> B -> A;
	// the second parent gets C. ResolveRevision walks the commit graph
	// (not first-parent only) for ^{/regex}.
	c, err := p.ResolveRef(repo, "HEAD^{/^C$}")
	if err != nil {
		t.Fatalf("HEAD^{/^C$} failed: %v", err)
	}
	if c.Hash != hC {
		t.Errorf("HEAD^{/^C$} = %s, want C=%s", c.Hash, hC)
	}
}

func TestResolveRefRemoteFallback(t *testing.T) {
	_, repo, hashes := initMergeRepo(t)
	hM := hashes[3]
	p := NewProvider(nil)

	// "trunk" has no local branch but exists as origin/trunk; the
	// convenience fallback in ResolveRef should pick it up.
	c, err := p.ResolveRef(repo, "trunk")
	if err != nil {
		t.Fatalf("trunk fallback failed: %v", err)
	}
	if c.Hash != hM {
		t.Errorf("trunk = %s, want M=%s", c.Hash, hM)
	}
}

func TestResolveRefEmpty(t *testing.T) {
	_, repo, _ := initMergeRepo(t)
	p := NewProvider(nil)

	if _, err := p.ResolveRef(repo, ""); err == nil {
		t.Error("expected error for empty ref")
	}
}

func TestResolveRefRangeNotAccepted(t *testing.T) {
	// ResolveRef is for single revisions only; ranges must be parsed
	// elsewhere. Document that with a test.
	_, repo, _ := initMergeRepo(t)
	p := NewProvider(nil)

	if _, err := p.ResolveRef(repo, "HEAD~1..HEAD"); err == nil {
		t.Error("expected error: ResolveRef must reject range syntax")
	}
}

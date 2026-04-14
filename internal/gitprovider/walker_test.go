// Copyright 2026 — see LICENSE file for terms.
package gitprovider

import (
	"slices"
	"testing"

	"github.com/go-git/go-git/v5/plumbing"
)

func walkedHashes(ws []WalkedCommit) []plumbing.Hash {
	out := make([]plumbing.Hash, len(ws))
	for i, w := range ws {
		out[i] = w.Commit.Hash
	}
	return out
}

func sideMap(ws []WalkedCommit) map[plumbing.Hash]Side {
	out := make(map[plumbing.Hash]Side, len(ws))
	for _, w := range ws {
		out[w.Commit.Hash] = w.Side
	}
	return out
}

func TestWalkSingleTip(t *testing.T) {
	_, repo, hashes := initMergeRepo(t)
	hA, hB, hC, hM := hashes[0], hashes[1], hashes[2], hashes[3]
	p := NewProvider(nil)

	// Default: walk full DAG from HEAD = {M, B, A, C} (every commit).
	rs, err := p.ParseRevSet(repo, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	got, err := p.Walk(repo, rs, WalkOptions{})
	if err != nil {
		t.Fatal(err)
	}
	got4 := walkedHashes(got)
	want := []plumbing.Hash{hA, hB, hC, hM}
	for _, h := range want {
		if !slices.Contains(got4, h) {
			t.Errorf("HEAD walk missing %s; got %v", h, got4)
		}
	}
	if len(got4) != 4 {
		t.Errorf("HEAD walk size = %d, want 4 (got %v)", len(got4), got4)
	}
}

func TestWalkFirstParent(t *testing.T) {
	_, repo, hashes := initMergeRepo(t)
	hA, hB, hC, hM := hashes[0], hashes[1], hashes[2], hashes[3]
	p := NewProvider(nil)

	// First-parent only from HEAD: M -> B -> A. C is on the merged-in side.
	rs, _ := p.ParseRevSet(repo, "HEAD")
	got, err := p.Walk(repo, rs, WalkOptions{FirstParent: true})
	if err != nil {
		t.Fatal(err)
	}
	hs := walkedHashes(got)
	want := []plumbing.Hash{hM, hB, hA}
	if len(hs) != 3 {
		t.Fatalf("first-parent walk size = %d, want 3 (got %v)", len(hs), hs)
	}
	for _, h := range want {
		if !slices.Contains(hs, h) {
			t.Errorf("first-parent walk missing %s", h)
		}
	}
	if slices.Contains(hs, hC) {
		t.Errorf("first-parent walk should NOT contain C, got %v", hs)
	}
}

func TestWalkTwoDotRange(t *testing.T) {
	_, repo, hashes := initMergeRepo(t)
	hA, hB, hC, hM := hashes[0], hashes[1], hashes[2], hashes[3]
	p := NewProvider(nil)

	// A..HEAD: include HEAD, exclude reachable from A.
	// Reachable from A = {A}. HEAD's reachable = {M, B, A, C}.
	// Result = {M, B, C} (all reachable from HEAD that are NOT reachable from A).
	rs, err := p.ParseRevSet(repo, hA.String()+"..HEAD")
	if err != nil {
		t.Fatal(err)
	}
	got, err := p.Walk(repo, rs, WalkOptions{})
	if err != nil {
		t.Fatal(err)
	}
	hs := walkedHashes(got)
	if slices.Contains(hs, hA) {
		t.Errorf("A..HEAD must not contain A, got %v", hs)
	}
	for _, h := range []plumbing.Hash{hM, hB, hC} {
		if !slices.Contains(hs, h) {
			t.Errorf("A..HEAD missing %s, got %v", h, hs)
		}
	}
	if len(hs) != 3 {
		t.Errorf("A..HEAD size = %d, want 3", len(hs))
	}
}

func TestWalkSymmetricDiff(t *testing.T) {
	_, repo, hashes := initMergeRepo(t)
	hA, hB, hC := hashes[0], hashes[1], hashes[2]
	p := NewProvider(nil)

	// B...C: commits reachable from B XOR reachable from C.
	// Reachable(B) = {B, A}, reachable(C) = {C, A}.
	// XOR = {B, C}. A is in both → excluded.
	rs, err := p.ParseRevSet(repo, hB.String()+"..."+hC.String())
	if err != nil {
		t.Fatal(err)
	}
	got, err := p.Walk(repo, rs, WalkOptions{})
	if err != nil {
		t.Fatal(err)
	}
	hs := walkedHashes(got)
	if slices.Contains(hs, hA) {
		t.Errorf("B...C must not contain A (common ancestor), got %v", hs)
	}
	for _, h := range []plumbing.Hash{hB, hC} {
		if !slices.Contains(hs, h) {
			t.Errorf("B...C missing %s, got %v", h, hs)
		}
	}
	if len(hs) != 2 {
		t.Errorf("B...C size = %d, want 2 (got %v)", len(hs), hs)
	}

	// Side labelling.
	sides := sideMap(got)
	if sides[hB] != SideLeft {
		t.Errorf("B side = %q, want left", sides[hB])
	}
	if sides[hC] != SideRight {
		t.Errorf("C side = %q, want right", sides[hC])
	}
}

func TestWalkCaretBangSingleCommit(t *testing.T) {
	_, repo, hashes := initMergeRepo(t)
	hM := hashes[3]
	p := NewProvider(nil)

	// HEAD^! → just the merge commit M itself.
	rs, err := p.ParseRevSet(repo, "HEAD^!")
	if err != nil {
		t.Fatal(err)
	}
	got, err := p.Walk(repo, rs, WalkOptions{})
	if err != nil {
		t.Fatal(err)
	}
	hs := walkedHashes(got)
	if len(hs) != 1 || hs[0] != hM {
		t.Errorf("HEAD^! = %v, want [M=%s]", hs, hM)
	}
}

func TestWalkCaretAtAllParents(t *testing.T) {
	_, repo, hashes := initMergeRepo(t)
	hA, hB, hC := hashes[0], hashes[1], hashes[2]
	p := NewProvider(nil)

	// HEAD^@ → all parents of HEAD as starting tips: B and C.
	// Their reachable sets: {B, A} ∪ {C, A} = {A, B, C}.
	rs, err := p.ParseRevSet(repo, "HEAD^@")
	if err != nil {
		t.Fatal(err)
	}
	got, err := p.Walk(repo, rs, WalkOptions{})
	if err != nil {
		t.Fatal(err)
	}
	hs := walkedHashes(got)
	for _, h := range []plumbing.Hash{hA, hB, hC} {
		if !slices.Contains(hs, h) {
			t.Errorf("HEAD^@ missing %s", h)
		}
	}
	if len(hs) != 3 {
		t.Errorf("HEAD^@ size = %d, want 3 (got %v)", len(hs), hs)
	}
}

func TestWalkCaretMinusN(t *testing.T) {
	_, repo, hashes := initMergeRepo(t)
	hC, hM := hashes[2], hashes[3]
	p := NewProvider(nil)

	// HEAD^-2 = HEAD^2..HEAD: include HEAD's reachable minus reachable
	// from HEAD's 2nd parent (C).
	// reachable(HEAD) = {M, B, A, C}; reachable(C) = {C, A}.
	// Result = {M, B}.
	rs, err := p.ParseRevSet(repo, "HEAD^-2")
	if err != nil {
		t.Fatal(err)
	}
	got, err := p.Walk(repo, rs, WalkOptions{})
	if err != nil {
		t.Fatal(err)
	}
	hs := walkedHashes(got)
	if slices.Contains(hs, hC) {
		t.Errorf("HEAD^-2 must not contain C, got %v", hs)
	}
	if !slices.Contains(hs, hM) {
		t.Errorf("HEAD^-2 must contain M=%s, got %v", hM, hs)
	}
}

func TestWalkExcludeReachableCap(t *testing.T) {
	_, repo, hashes := initMergeRepo(t)
	hA := hashes[0]
	p := NewProvider(nil)

	rs, err := p.ParseRevSet(repo, hA.String()+"..HEAD")
	if err != nil {
		t.Fatal(err)
	}
	// Cap of 0 (set to 1 actually since 0 means "use default") would
	// allow at most 1 commit before erroring. reachable(A)={A}, just 1
	// commit. Should succeed.
	if _, err := p.Walk(repo, rs, WalkOptions{MaxExcludeWalk: 1}); err != nil {
		t.Fatalf("MaxExcludeWalk=1 with single-commit exclude should succeed, got %v", err)
	}
	// With the default cap, also fine.
	if _, err := p.Walk(repo, rs, WalkOptions{}); err != nil {
		t.Fatalf("default MaxExcludeWalk should succeed, got %v", err)
	}
}

func TestWalkResultsInCommitterTimeOrder(t *testing.T) {
	_, repo, hashes := initMergeRepo(t)
	p := NewProvider(nil)

	rs, _ := p.ParseRevSet(repo, "HEAD")
	got, err := p.Walk(repo, rs, WalkOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i < len(got); i++ {
		prev := got[i-1].Commit.Committer.When
		cur := got[i].Commit.Committer.When
		if cur.After(prev) {
			t.Errorf("commits out of order at index %d: %v after %v", i, prev, cur)
		}
	}
	// Sanity: M is the most recent (committer second 4) and should be first.
	if got[0].Commit.Hash != hashes[3] {
		t.Errorf("first commit = %s, want M=%s", got[0].Commit.Hash, hashes[3])
	}
}

// Copyright 2026 — see LICENSE file for terms.
package engine

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"testing"
	"time"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"

	"go.pennock.tech/aifr/pkg/protocol"
)

// initEngineMergeRepo builds the same A/B/C/M topology used in the
// gitprovider tests so engine-level Log calls can exercise ranges.
//
//	A --- B --- M
//	       \   /
//	        C
func initEngineMergeRepo(t *testing.T) (string, []plumbing.Hash) {
	t.Helper()
	dir := t.TempDir()

	repo, err := gogit.PlainInit(dir, false)
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
	write := func(path, body string) {
		t.Helper()
		full := filepath.Join(dir, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := wt.Add(path); err != nil {
			t.Fatal(err)
		}
	}
	commit := func(msg string, secs int, parents ...plumbing.Hash) plumbing.Hash {
		t.Helper()
		opts := &gogit.CommitOptions{Author: sig(secs), Committer: sig(secs)}
		if len(parents) > 0 {
			opts.Parents = parents
		}
		h, err := wt.Commit(msg, opts)
		if err != nil {
			t.Fatal(err)
		}
		return h
	}

	write("a.txt", "A\n")
	hA := commit("A", 1)
	write("b.txt", "B\n")
	hB := commit("B", 2)

	feature := plumbing.NewBranchReferenceName("feature")
	if err := repo.Storer.SetReference(plumbing.NewHashReference(feature, hA)); err != nil {
		t.Fatal(err)
	}
	if err := wt.Checkout(&gogit.CheckoutOptions{Branch: feature}); err != nil {
		t.Fatal(err)
	}
	write("c.txt", "C\n")
	hC := commit("C", 3)

	mainName := plumbing.NewBranchReferenceName("master")
	if _, err := repo.Reference(mainName, true); err != nil {
		mainName = plumbing.NewBranchReferenceName("main")
	}
	if err := wt.Checkout(&gogit.CheckoutOptions{Branch: mainName}); err != nil {
		t.Fatal(err)
	}
	write("c.txt", "C\n")
	hM := commit("M: merge feature", 4, hB, hC)

	if _, err := repo.CreateTag("v-b", hB, &gogit.CreateTagOptions{
		Tagger:  sig(2),
		Message: "tag of B",
	}); err != nil {
		t.Fatal(err)
	}

	return dir, []plumbing.Hash{hA, hB, hC, hM}
}

func entryHashes(resp *protocol.LogResponse) []string {
	out := make([]string, 0, len(resp.Entries))
	for _, e := range resp.Entries {
		out = append(out, e.Hash)
	}
	return out
}

func TestLogTwoDotRangeWithTags(t *testing.T) {
	dir, hashes := initEngineMergeRepo(t)
	eng := newTestEngine(t, dir)

	// v-b..HEAD: include HEAD's reachable, exclude v-b's reachable.
	// v-b = B, HEAD = M. Reachable(B)={B,A}; Reachable(M)={M,B,A,C}.
	// Result = {M, C}.
	resp, err := eng.Log(dir, "v-b..HEAD", LogParams{})
	if err != nil {
		t.Fatalf("Log v-b..HEAD failed: %v", err)
	}
	got := entryHashes(resp)
	wantHashes := []string{hashes[3].String(), hashes[2].String()} // M, C
	if !slices.Equal(got, wantHashes) {
		t.Errorf("v-b..HEAD entries = %v, want %v", got, wantHashes)
	}
	if !resp.Complete {
		t.Error("v-b..HEAD should be complete (only 2 commits)")
	}
}

func TestLogThreeDotSymmetric(t *testing.T) {
	dir, hashes := initEngineMergeRepo(t)
	hB, hC := hashes[1], hashes[2]
	eng := newTestEngine(t, dir)

	resp, err := eng.Log(dir, hB.String()+"..."+hC.String(), LogParams{})
	if err != nil {
		t.Fatalf("Log B...C failed: %v", err)
	}
	if len(resp.Entries) != 2 {
		t.Fatalf("B...C entries len = %d, want 2 (got %v)", len(resp.Entries), entryHashes(resp))
	}
	// Each entry must carry a side label.
	sides := map[string]string{}
	for _, e := range resp.Entries {
		sides[e.Hash] = e.Side
	}
	if sides[hB.String()] != "left" {
		t.Errorf("B side = %q, want left", sides[hB.String()])
	}
	if sides[hC.String()] != "right" {
		t.Errorf("C side = %q, want right", sides[hC.String()])
	}
}

func TestLogCaretBangSingleCommit(t *testing.T) {
	dir, hashes := initEngineMergeRepo(t)
	eng := newTestEngine(t, dir)

	resp, err := eng.Log(dir, "HEAD^!", LogParams{})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Entries) != 1 {
		t.Fatalf("HEAD^! entries = %d, want 1", len(resp.Entries))
	}
	if resp.Entries[0].Hash != hashes[3].String() {
		t.Errorf("HEAD^! = %s, want M=%s", resp.Entries[0].Hash, hashes[3])
	}
}

func TestLogCaretAtAllParents(t *testing.T) {
	dir, hashes := initEngineMergeRepo(t)
	hA, hB, hC := hashes[0], hashes[1], hashes[2]
	eng := newTestEngine(t, dir)

	resp, err := eng.Log(dir, "HEAD^@", LogParams{})
	if err != nil {
		t.Fatal(err)
	}
	got := entryHashes(resp)
	wantSet := map[string]bool{hA.String(): true, hB.String(): true, hC.String(): true}
	if len(got) != 3 {
		t.Fatalf("HEAD^@ size = %d, want 3 (got %v)", len(got), got)
	}
	for _, h := range got {
		if !wantSet[h] {
			t.Errorf("HEAD^@ contains unexpected %s", h)
		}
	}
}

func TestLogExcludeToken(t *testing.T) {
	dir, hashes := initEngineMergeRepo(t)
	hA := hashes[0]
	eng := newTestEngine(t, dir)

	// "HEAD ^A" — same set as A..HEAD.
	resp, err := eng.Log(dir, "HEAD ^"+hA.String(), LogParams{})
	if err != nil {
		t.Fatal(err)
	}
	for _, h := range entryHashes(resp) {
		if h == hA.String() {
			t.Errorf("excluded A=%s appeared in result %v", hA, entryHashes(resp))
		}
	}
	if len(resp.Entries) != 3 {
		t.Errorf("HEAD ^A size = %d, want 3 (got %v)", len(resp.Entries), entryHashes(resp))
	}
}

func TestLogCaretMinusN(t *testing.T) {
	dir, hashes := initEngineMergeRepo(t)
	hC := hashes[2]
	eng := newTestEngine(t, dir)

	// HEAD^-2 = HEAD^2..HEAD = exclude C → result {M, B}
	resp, err := eng.Log(dir, "HEAD^-2", LogParams{})
	if err != nil {
		t.Fatal(err)
	}
	got := entryHashes(resp)
	for _, h := range got {
		if h == hC.String() {
			t.Errorf("HEAD^-2 must not contain C=%s, got %v", hC, got)
		}
	}
	if len(got) != 2 {
		t.Errorf("HEAD^-2 size = %d, want 2 (got %v)", len(got), got)
	}
}

func TestLogFirstParentOnMerge(t *testing.T) {
	dir, hashes := initEngineMergeRepo(t)
	hC := hashes[2]
	eng := newTestEngine(t, dir)

	// FirstParent: M -> B -> A; C should NOT appear.
	resp, err := eng.Log(dir, "HEAD", LogParams{FirstParent: true})
	if err != nil {
		t.Fatal(err)
	}
	got := entryHashes(resp)
	if len(got) != 3 {
		t.Errorf("FirstParent walk size = %d, want 3 (got %v)", len(got), got)
	}
	if slices.Contains(got, hC.String()) {
		t.Errorf("FirstParent should not contain C=%s, got %v", hC, got)
	}
}

func TestLogDefaultIncludesAllParents(t *testing.T) {
	dir, hashes := initEngineMergeRepo(t)
	hC := hashes[2]
	eng := newTestEngine(t, dir)

	resp, err := eng.Log(dir, "HEAD", LogParams{})
	if err != nil {
		t.Fatal(err)
	}
	got := entryHashes(resp)
	if !slices.Contains(got, hC.String()) {
		t.Errorf("default walk should include C=%s, got %v", hC, got)
	}
}

func TestLogPaginationAcrossRange(t *testing.T) {
	dir, _ := initEngineMergeRepo(t)
	eng := newTestEngine(t, dir)

	// Walking the full DAG yields 4 commits; cap to 2 per page.
	resp1, err := eng.Log(dir, "HEAD", LogParams{MaxCount: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp1.Entries) != 2 {
		t.Fatalf("page 1 size = %d, want 2", len(resp1.Entries))
	}
	if resp1.Complete {
		t.Error("page 1 should not be complete")
	}
	if resp1.Continuation == "" {
		t.Fatal("page 1 must include a continuation token")
	}

	tok, err := eng.DecodeListContinuation(resp1.Continuation)
	if err != nil {
		t.Fatalf("decoding continuation: %v", err)
	}
	if tok.Tool != "log" {
		t.Errorf("token tool = %q, want log", tok.Tool)
	}
	if tok.RevSpec != "HEAD" {
		t.Errorf("token RevSpec = %q, want HEAD", tok.RevSpec)
	}

	resp2, err := eng.Log(dir, "HEAD", LogParams{
		MaxCount:  2,
		StartHash: tok.Hash,
		StartRev:  tok.RevSpec,
	})
	if err != nil {
		t.Fatal(err)
	}
	// Combined pages should yield 4 unique commits.
	combined := append(entryHashes(resp1), entryHashes(resp2)...)
	seen := map[string]bool{}
	for _, h := range combined {
		seen[h] = true
	}
	if len(seen) != 4 {
		t.Errorf("pages combined = %d unique commits, want 4 (got %v)", len(seen), combined)
	}
	if !resp2.Complete {
		t.Error("page 2 should be complete (4 total commits)")
	}
}

func TestLogInvalidRangeError(t *testing.T) {
	dir, _ := initEngineMergeRepo(t)
	eng := newTestEngine(t, dir)

	_, err := eng.Log(dir, "totally-bogus-ref-xyz..HEAD", LogParams{})
	if err == nil {
		t.Error("expected error for unresolvable range")
	}
}

func TestLogPathFilter(t *testing.T) {
	dir, hashes := initEngineMergeRepo(t)
	hC, hM := hashes[2], hashes[3]
	eng := newTestEngine(t, dir)

	// Only commits that touched c.txt: C and M (M brings c.txt onto main).
	resp, err := eng.Log(dir, "HEAD", LogParams{PathGlob: "c.txt"})
	if err != nil {
		t.Fatal(err)
	}
	got := entryHashes(resp)
	wantSet := map[string]bool{hC.String(): true, hM.String(): true}
	if len(got) != 2 {
		t.Fatalf("path filter c.txt size = %d, want 2 (got %v)", len(got), got)
	}
	for _, h := range got {
		if !wantSet[h] {
			t.Errorf("path filter c.txt unexpected %s", h)
		}
	}
}

func TestLogPathFilterGlob(t *testing.T) {
	dir, hashes := initEngineMergeRepo(t)
	eng := newTestEngine(t, dir)

	// Glob **/*.txt matches every commit's introduced file.
	resp, err := eng.Log(dir, "HEAD", LogParams{PathGlob: "**/*.txt"})
	if err != nil {
		t.Fatal(err)
	}
	// A is the root commit; first-parent diff returns nothing for it
	// (no parent), so it is dropped by the path filter. We expect B, C, M.
	got := entryHashes(resp)
	if len(got) != 3 {
		t.Errorf("**/*.txt size = %d, want 3 (got %v); A has no parent so falls out", len(got), got)
	}
	for _, want := range []string{hashes[1].String(), hashes[2].String(), hashes[3].String()} {
		if !slices.Contains(got, want) {
			t.Errorf("**/*.txt missing %s", want)
		}
	}
}

func TestLogTimeRangeFilter(t *testing.T) {
	dir, hashes := initEngineMergeRepo(t)
	eng := newTestEngine(t, dir)

	// Author timestamps: A=sec1, B=sec2, C=sec3, M=sec4.
	since := time.Date(2024, 1, 1, 0, 0, 2, 0, time.UTC)
	until := time.Date(2024, 1, 1, 0, 0, 3, 0, time.UTC)
	resp, err := eng.Log(dir, "HEAD", LogParams{Since: &since, Until: &until})
	if err != nil {
		t.Fatal(err)
	}
	got := entryHashes(resp)
	wantSet := map[string]bool{hashes[1].String(): true, hashes[2].String(): true} // B, C
	if len(got) != 2 {
		t.Fatalf("time filter [sec2..sec3] size = %d, want 2 (got %v)", len(got), got)
	}
	for _, h := range got {
		if !wantSet[h] {
			t.Errorf("time filter unexpected %s", h)
		}
	}
}

func TestLogGrepFilter(t *testing.T) {
	dir, hashes := initEngineMergeRepo(t)
	hM := hashes[3]
	eng := newTestEngine(t, dir)

	re := regexp.MustCompile(`(?i)merge`)
	resp, err := eng.Log(dir, "HEAD", LogParams{Grep: re})
	if err != nil {
		t.Fatal(err)
	}
	got := entryHashes(resp)
	if len(got) != 1 || got[0] != hM.String() {
		t.Errorf("grep merge = %v, want [M=%s]", got, hM)
	}
}

func TestLogAuthorFilter(t *testing.T) {
	dir, _ := initEngineMergeRepo(t)
	eng := newTestEngine(t, dir)

	// All commits are by "Test <test@example.com>"; matching pattern keeps all 4.
	re := regexp.MustCompile(`Test <test@example\.com>`)
	resp, err := eng.Log(dir, "HEAD", LogParams{Author: re})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Entries) != 4 {
		t.Errorf("matching author kept %d, want 4", len(resp.Entries))
	}

	// Non-matching pattern drops everything.
	re2 := regexp.MustCompile(`SomebodyElse`)
	resp2, err := eng.Log(dir, "HEAD", LogParams{Author: re2})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp2.Entries) != 0 {
		t.Errorf("non-matching author kept %d, want 0", len(resp2.Entries))
	}
}

func TestLogBackwardsCompatSingleRef(t *testing.T) {
	// Tests that single-ref calls still produce ordered, populated output.
	dir, hashes := initEngineMergeRepo(t)
	eng := newTestEngine(t, dir)

	resp, err := eng.Log(dir, "HEAD", LogParams{MaxCount: 100})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Entries) != 4 {
		t.Errorf("HEAD size = %d, want 4", len(resp.Entries))
	}
	if !resp.Complete {
		t.Error("HEAD should be complete with maxCount=100")
	}
	// Most recent (M) first.
	if resp.Entries[0].Hash != hashes[3].String() {
		t.Errorf("first entry = %s, want M=%s", resp.Entries[0].Hash, hashes[3])
	}
}

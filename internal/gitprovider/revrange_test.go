// Copyright 2026 — see LICENSE file for terms.
package gitprovider

import (
	"slices"
	"testing"

	"github.com/go-git/go-git/v5/plumbing"
)

// hashSet collects hashes from tips and excludes for set-equality assertions.
func hashSet(hs []plumbing.Hash) map[plumbing.Hash]struct{} {
	m := make(map[plumbing.Hash]struct{}, len(hs))
	for _, h := range hs {
		m[h] = struct{}{}
	}
	return m
}

func tipHashes(rs *RevSet) []plumbing.Hash {
	out := make([]plumbing.Hash, 0, len(rs.Tips))
	for _, t := range rs.Tips {
		out = append(out, t.Hash)
	}
	return out
}

func TestParseRevSetEmptyDefaultsToHEAD(t *testing.T) {
	_, repo, hashes := initMergeRepo(t)
	hM := hashes[3]
	p := NewProvider(nil)

	rs, err := p.ParseRevSet(repo, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.Tips) != 1 || rs.Tips[0].Hash != hM {
		t.Errorf("empty spec: tips = %v, want [HEAD=%s]", tipHashes(rs), hM)
	}
	if len(rs.GlobalExcludes) != 0 {
		t.Errorf("empty spec: unexpected excludes %v", rs.GlobalExcludes)
	}
	if rs.IsSymmetric() {
		t.Error("empty spec must not be symmetric")
	}
}

func TestParseRevSetSingleRev(t *testing.T) {
	_, repo, hashes := initMergeRepo(t)
	hM := hashes[3]
	p := NewProvider(nil)

	rs, err := p.ParseRevSet(repo, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.Tips) != 1 || rs.Tips[0].Hash != hM {
		t.Fatalf("HEAD: tips = %v, want [%s]", tipHashes(rs), hM)
	}
}

func TestParseRevSetTwoDotRange(t *testing.T) {
	_, repo, hashes := initMergeRepo(t)
	hA, hM := hashes[0], hashes[3]
	p := NewProvider(nil)

	// A..HEAD → include HEAD, exclude A
	rs, err := p.ParseRevSet(repo, hA.String()+"..HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.Tips) != 1 || rs.Tips[0].Hash != hM {
		t.Errorf("A..HEAD: tips = %v, want [HEAD=%s]", tipHashes(rs), hM)
	}
	if len(rs.GlobalExcludes) != 1 || rs.GlobalExcludes[0] != hA {
		t.Errorf("A..HEAD: excludes = %v, want [A=%s]", rs.GlobalExcludes, hA)
	}
}

func TestParseRevSetThreeDotRange(t *testing.T) {
	_, repo, hashes := initMergeRepo(t)
	hB, hC := hashes[1], hashes[2]
	p := NewProvider(nil)

	rs, err := p.ParseRevSet(repo, hB.String()+"..."+hC.String())
	if err != nil {
		t.Fatal(err)
	}
	if !rs.IsSymmetric() {
		t.Error("expected symmetric")
	}
	if len(rs.Tips) != 2 {
		t.Fatalf("expected 2 tips, got %d", len(rs.Tips))
	}
	// Left tip is hB, right tip is hC; each excludes the other locally.
	if rs.Tips[0].Hash != hB || rs.Tips[0].Side != SideLeft {
		t.Errorf("left tip = %s/%s, want %s/left", rs.Tips[0].Hash, rs.Tips[0].Side, hB)
	}
	if rs.Tips[1].Hash != hC || rs.Tips[1].Side != SideRight {
		t.Errorf("right tip = %s/%s, want %s/right", rs.Tips[1].Hash, rs.Tips[1].Side, hC)
	}
	if len(rs.Tips[0].LocalExcludes) != 1 || rs.Tips[0].LocalExcludes[0] != hC {
		t.Errorf("left local excludes = %v, want [%s]", rs.Tips[0].LocalExcludes, hC)
	}
	if len(rs.Tips[1].LocalExcludes) != 1 || rs.Tips[1].LocalExcludes[0] != hB {
		t.Errorf("right local excludes = %v, want [%s]", rs.Tips[1].LocalExcludes, hB)
	}
}

func TestParseRevSetCaretExcludes(t *testing.T) {
	_, repo, hashes := initMergeRepo(t)
	hA, hM := hashes[0], hashes[3]
	p := NewProvider(nil)

	rs, err := p.ParseRevSet(repo, "HEAD ^"+hA.String())
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.Tips) != 1 || rs.Tips[0].Hash != hM {
		t.Errorf("tips = %v, want [HEAD=%s]", tipHashes(rs), hM)
	}
	if len(rs.GlobalExcludes) != 1 || rs.GlobalExcludes[0] != hA {
		t.Errorf("excludes = %v, want [A=%s]", rs.GlobalExcludes, hA)
	}
}

func TestParseRevSetCaretBangSingleCommit(t *testing.T) {
	// HEAD^!  — include HEAD, exclude all of HEAD's parents.
	// HEAD = M with parents (B, C).
	_, repo, hashes := initMergeRepo(t)
	hB, hC, hM := hashes[1], hashes[2], hashes[3]
	p := NewProvider(nil)

	rs, err := p.ParseRevSet(repo, "HEAD^!")
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.Tips) != 1 || rs.Tips[0].Hash != hM {
		t.Fatalf("tips = %v, want [HEAD=%s]", tipHashes(rs), hM)
	}
	got := hashSet(rs.Tips[0].LocalExcludes)
	want := hashSet([]plumbing.Hash{hB, hC})
	if len(got) != 2 {
		t.Fatalf("local excludes = %v, want {B, C}", rs.Tips[0].LocalExcludes)
	}
	for h := range want {
		if _, ok := got[h]; !ok {
			t.Errorf("local excludes missing %s", h)
		}
	}
}

func TestParseRevSetCaretAtAllParents(t *testing.T) {
	// HEAD^@  — all parents of HEAD as separate tips, no HEAD itself.
	_, repo, hashes := initMergeRepo(t)
	hB, hC := hashes[1], hashes[2]
	p := NewProvider(nil)

	rs, err := p.ParseRevSet(repo, "HEAD^@")
	if err != nil {
		t.Fatal(err)
	}
	got := hashSet(tipHashes(rs))
	if len(got) != 2 {
		t.Fatalf("expected 2 tips, got %d (%v)", len(got), tipHashes(rs))
	}
	for _, h := range []plumbing.Hash{hB, hC} {
		if _, ok := got[h]; !ok {
			t.Errorf("expected tip %s in result", h)
		}
	}
	if len(rs.GlobalExcludes) != 0 {
		t.Errorf("HEAD^@ should produce no excludes, got %v", rs.GlobalExcludes)
	}
}

func TestParseRevSetCaretMinusN(t *testing.T) {
	// HEAD^-1  → HEAD^1..HEAD  (include HEAD, exclude HEAD^1).
	// HEAD = M, HEAD^1 = B.
	_, repo, hashes := initMergeRepo(t)
	hB, hM := hashes[1], hashes[3]
	p := NewProvider(nil)

	cases := []string{"HEAD^-", "HEAD^-1"}
	for _, spec := range cases {
		t.Run(spec, func(t *testing.T) {
			rs, err := p.ParseRevSet(repo, spec)
			if err != nil {
				t.Fatal(err)
			}
			if len(rs.Tips) != 1 || rs.Tips[0].Hash != hM {
				t.Errorf("tips = %v, want [HEAD=%s]", tipHashes(rs), hM)
			}
			if len(rs.GlobalExcludes) != 1 || rs.GlobalExcludes[0] != hB {
				t.Errorf("excludes = %v, want [B=%s]", rs.GlobalExcludes, hB)
			}
		})
	}

	// HEAD^-2  → HEAD^2..HEAD = exclude C
	hC := hashes[2]
	rs, err := p.ParseRevSet(repo, "HEAD^-2")
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.GlobalExcludes) != 1 || rs.GlobalExcludes[0] != hC {
		t.Errorf("HEAD^-2: excludes = %v, want [C=%s]", rs.GlobalExcludes, hC)
	}
}

func TestParseRevSetMultipleTokens(t *testing.T) {
	_, repo, hashes := initMergeRepo(t)
	hA, hB, hM := hashes[0], hashes[1], hashes[3]
	p := NewProvider(nil)

	// "HEAD ^A"  + "B"  → 2 tips (HEAD, B), 1 exclude (A).
	rs, err := p.ParseRevSet(repo, "HEAD ^"+hA.String()+" "+hB.String())
	if err != nil {
		t.Fatal(err)
	}
	got := hashSet(tipHashes(rs))
	for _, h := range []plumbing.Hash{hM, hB} {
		if _, ok := got[h]; !ok {
			t.Errorf("tip %s missing in %v", h, tipHashes(rs))
		}
	}
	if len(rs.GlobalExcludes) != 1 || rs.GlobalExcludes[0] != hA {
		t.Errorf("excludes = %v, want [A]", rs.GlobalExcludes)
	}
}

func TestParseRevSetErrors(t *testing.T) {
	_, repo, _ := initMergeRepo(t)
	p := NewProvider(nil)

	cases := []struct {
		spec    string
		desc    string
		wantErr bool
	}{
		{"nonexistent-branch-xyz", "unknown rev", true},
		{"..HEAD", "missing left of ..", true},
		{"HEAD..", "missing right of ..", true},
		{"...HEAD", "missing left of ...", true},
		{"HEAD...", "missing right of ...", true},
		{"^HEAD", "only excludes (no positive tips)", true},
		{"HEAD^!", "valid r^! single commit", false},
		{"HEAD^-1", "valid r^-N", false},
	}
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			_, err := p.ParseRevSet(repo, tc.spec)
			if (err != nil) != tc.wantErr {
				t.Errorf("ParseRevSet(%q) err=%v, wantErr=%v", tc.spec, err, tc.wantErr)
			}
		})
	}
}

func TestParseRevSetRangeWithSymbolicRefs(t *testing.T) {
	// The motivating user case: tag1..tag2 form.
	_, repo, hashes := initMergeRepo(t)
	hB, hM := hashes[1], hashes[3]
	p := NewProvider(nil)

	// v-b is the annotated tag for B; HEAD is M.
	rs, err := p.ParseRevSet(repo, "v-b..HEAD")
	if err != nil {
		t.Fatalf("v-b..HEAD failed: %v", err)
	}
	if len(rs.Tips) != 1 || rs.Tips[0].Hash != hM {
		t.Errorf("tips = %v, want [HEAD=%s]", tipHashes(rs), hM)
	}
	if !slices.Contains(rs.GlobalExcludes, hB) {
		t.Errorf("excludes = %v, want to contain B=%s", rs.GlobalExcludes, hB)
	}
}

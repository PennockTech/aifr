// Copyright 2026 — see LICENSE file for terms.
package gitprovider

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"

	"go.pennock.tech/aifr/pkg/protocol"
)

// Side identifies which side of a symmetric-difference range a commit
// was reached from. Empty for non-symmetric inputs.
type Side string

const (
	SideNone  Side = ""
	SideLeft  Side = "left"
	SideRight Side = "right"
)

// TipSpec describes one starting point for a log walk.
type TipSpec struct {
	Hash          plumbing.Hash
	Side          Side
	LocalExcludes []plumbing.Hash // additional excludes that apply only to this tip
}

// RevSet is the parsed form of a gitrevisions(7) range expression.
//
// The walker (see Walker.Walk) starts at each Tip, prunes any commit
// whose hash (or any ancestor's hash) is in the union of GlobalExcludes
// and that tip's LocalExcludes, and emits the rest in committer-time
// order, deduped across tips.
type RevSet struct {
	Spec           string          // original input string, for token re-encoding
	Tips           []TipSpec       // OR'd tips
	GlobalExcludes []plumbing.Hash // ^rev (apply to all tips)
}

// IsSymmetric reports whether any tip carries a non-empty Side, i.e. the
// spec contained a `r1...r2` token.
func (rs *RevSet) IsSymmetric() bool {
	for _, t := range rs.Tips {
		if t.Side != SideNone {
			return true
		}
	}
	return false
}

// ParseRevSet parses a gitrevisions(7) range/revision expression.
//
// Supported tokens (whitespace-separated):
//
//	<rev>             include rev
//	^<rev>            exclude rev (and its ancestors) from all tips
//	<r1>..<r2>        include r2, exclude r1 (two-dot range)
//	<r1>...<r2>       symmetric difference (each side excludes the other)
//	<rev>^!           include rev, exclude all of rev's parents — i.e. just rev
//	<rev>^@           include all of rev's parents (no rev itself)
//	<rev>^-           equivalent to <rev>^1..<rev>
//	<rev>^-<n>        equivalent to <rev>^<n>..<rev>
//
// Each <rev> is itself resolved through Provider.ResolveRef so the full
// gitrevisions single-revision grammar applies (HEAD, branches, tags,
// hashes, ~/^ chains, ^{type}, ^{/regex}, ...).
//
// An empty spec defaults to a single tip at HEAD.
func (p *Provider) ParseRevSet(repo *git.Repository, spec string) (*RevSet, error) {
	rs := &RevSet{Spec: spec}

	tokens := strings.Fields(spec)
	if len(tokens) == 0 {
		commit, err := p.ResolveRef(repo, "HEAD")
		if err != nil {
			return nil, err
		}
		rs.Tips = []TipSpec{{Hash: commit.Hash}}
		return rs, nil
	}

	for _, tok := range tokens {
		if err := p.applyToken(repo, rs, tok); err != nil {
			return nil, err
		}
	}

	if len(rs.Tips) == 0 {
		// Spec consisted only of ^excludes — that has no positive set to
		// walk; reject rather than silently returning nothing.
		return nil, protocol.NewError(protocol.ErrInvalidRef,
			fmt.Sprintf("revision spec %q has no positive tips", spec))
	}
	return rs, nil
}

// applyToken parses one whitespace-separated token and mutates rs.
func (p *Provider) applyToken(repo *git.Repository, rs *RevSet, tok string) error {
	// ^<rev>  — exclude.
	if strings.HasPrefix(tok, "^") {
		c, err := p.ResolveRef(repo, tok[1:])
		if err != nil {
			return err
		}
		rs.GlobalExcludes = append(rs.GlobalExcludes, c.Hash)
		return nil
	}

	// <r1>...<r2>  — symmetric difference (must be tested before "..").
	if l, r, ok := splitOnce(tok, "..."); ok {
		if l == "" || r == "" {
			return protocol.NewError(protocol.ErrInvalidRef,
				fmt.Sprintf("revision token %q: both sides of \"...\" must be non-empty", tok))
		}
		lc, err := p.ResolveRef(repo, l)
		if err != nil {
			return err
		}
		rc, err := p.ResolveRef(repo, r)
		if err != nil {
			return err
		}
		rs.Tips = append(rs.Tips,
			TipSpec{Hash: lc.Hash, Side: SideLeft, LocalExcludes: []plumbing.Hash{rc.Hash}},
			TipSpec{Hash: rc.Hash, Side: SideRight, LocalExcludes: []plumbing.Hash{lc.Hash}},
		)
		return nil
	}

	// <r1>..<r2>  — two-dot range.
	if l, r, ok := splitOnce(tok, ".."); ok {
		if l == "" || r == "" {
			return protocol.NewError(protocol.ErrInvalidRef,
				fmt.Sprintf("revision token %q: both sides of \"..\" must be non-empty", tok))
		}
		lc, err := p.ResolveRef(repo, l)
		if err != nil {
			return err
		}
		rc, err := p.ResolveRef(repo, r)
		if err != nil {
			return err
		}
		rs.Tips = append(rs.Tips, TipSpec{Hash: rc.Hash})
		rs.GlobalExcludes = append(rs.GlobalExcludes, lc.Hash)
		return nil
	}

	// <rev>^!  — single-commit shortcut.
	if base, ok := strings.CutSuffix(tok, "^!"); ok {
		c, err := p.ResolveRef(repo, base)
		if err != nil {
			return err
		}
		var locals []plumbing.Hash
		for _, ph := range c.ParentHashes {
			locals = append(locals, ph)
		}
		rs.Tips = append(rs.Tips, TipSpec{Hash: c.Hash, LocalExcludes: locals})
		return nil
	}

	// <rev>^@  — all parents of rev (no rev itself).
	if base, ok := strings.CutSuffix(tok, "^@"); ok {
		c, err := p.ResolveRef(repo, base)
		if err != nil {
			return err
		}
		if len(c.ParentHashes) == 0 {
			return protocol.NewError(protocol.ErrInvalidRef,
				fmt.Sprintf("commit %s has no parents (^@ requires at least one)", c.Hash))
		}
		for _, ph := range c.ParentHashes {
			rs.Tips = append(rs.Tips, TipSpec{Hash: ph})
		}
		return nil
	}

	// <rev>^-[<n>]  — equivalent to <rev>^<n>..<rev>.
	if idx := strings.LastIndex(tok, "^-"); idx >= 0 {
		base := tok[:idx]
		nStr := tok[idx+2:]
		n := 1
		if nStr != "" {
			parsed, err := strconv.Atoi(nStr)
			if err != nil || parsed < 1 {
				// Not actually a ^-N suffix; fall through to plain rev.
				goto plain
			}
			n = parsed
		}
		tip, err := p.ResolveRef(repo, base)
		if err != nil {
			return err
		}
		exclude, err := p.ResolveRef(repo, fmt.Sprintf("%s^%d", base, n))
		if err != nil {
			return err
		}
		rs.Tips = append(rs.Tips, TipSpec{Hash: tip.Hash})
		rs.GlobalExcludes = append(rs.GlobalExcludes, exclude.Hash)
		return nil
	}

plain:
	// Plain <rev> — include.
	c, err := p.ResolveRef(repo, tok)
	if err != nil {
		return err
	}
	rs.Tips = append(rs.Tips, TipSpec{Hash: c.Hash})
	return nil
}

// splitOnce returns (before, after, true) if sep occurs in s, splitting
// on the first occurrence. The semantics differ from strings.Cut only in
// being explicit about the "found" return.
func splitOnce(s, sep string) (string, string, bool) {
	return strings.Cut(s, sep)
}

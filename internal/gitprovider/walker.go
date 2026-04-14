// Copyright 2026 — see LICENSE file for terms.
package gitprovider

import (
	"container/heap"
	"fmt"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"

	"go.pennock.tech/aifr/pkg/protocol"
)

// DefaultMaxExcludeWalk is the safety bound on how many commits the
// walker will enumerate when pre-computing the reachable set from
// ^excludes. Beyond this we stop and return an error so we never walk
// arbitrarily large histories implicitly.
const DefaultMaxExcludeWalk = 200_000

// WalkOptions controls range walks.
type WalkOptions struct {
	// FirstParent: when true, only the first parent of each commit is
	// followed (analogous to `git log --first-parent`). Default false
	// produces the full commit DAG, matching plain `git log`.
	FirstParent bool

	// MaxExcludeWalk caps the reachable-set enumeration for ^excludes.
	// 0 uses DefaultMaxExcludeWalk.
	MaxExcludeWalk int
}

// WalkedCommit is an entry yielded by Walk. Side carries the
// symmetric-difference side ("left"/"right") when applicable, otherwise
// SideNone.
type WalkedCommit struct {
	Commit *object.Commit
	Side   Side
}

// Walk traverses the RevSet and returns all matching commits in
// committer-time descending order, deduplicated across tips.
//
// Implementation: each tip is walked independently (BFS, pruning at
// commits in its effective exclusion set), then per-tip results are
// merged by committer time. This isolates per-tip LocalExcludes (used
// by symmetric difference) without threading them through a shared
// traversal.
//
// The function does NOT page; callers (engine.Log) apply skip/limit.
// For ranges with very large excluded sides the call may return
// ErrInvalidRef("excluded reachable set exceeds N commits") — in that
// case the user should narrow the spec or supply a tighter ^exclude.
func (p *Provider) Walk(repo *git.Repository, rs *RevSet, opts WalkOptions) ([]WalkedCommit, error) {
	maxEx := opts.MaxExcludeWalk
	if maxEx <= 0 {
		maxEx = DefaultMaxExcludeWalk
	}

	// Pre-compute the global reachable set for ^excludes once.
	globalExcl, err := reachableSet(repo, rs.GlobalExcludes, opts.FirstParent, maxEx)
	if err != nil {
		return nil, err
	}

	perTip := make([][]WalkedCommit, 0, len(rs.Tips))
	for _, tip := range rs.Tips {
		excl := cloneSet(globalExcl)
		if len(tip.LocalExcludes) > 0 {
			localExcl, err := reachableSet(repo, tip.LocalExcludes, opts.FirstParent, maxEx)
			if err != nil {
				return nil, err
			}
			for h := range localExcl {
				excl[h] = struct{}{}
			}
		}

		commits, err := walkTip(repo, tip.Hash, excl, opts.FirstParent)
		if err != nil {
			return nil, err
		}

		// Sort this tip's commits by committer time descending so the
		// k-way merge below is well-defined.
		sortByCommitterTimeDesc(commits)

		entries := make([]WalkedCommit, 0, len(commits))
		for _, c := range commits {
			entries = append(entries, WalkedCommit{Commit: c, Side: tip.Side})
		}
		perTip = append(perTip, entries)
	}

	return mergeByCommitterTime(perTip), nil
}

// reachableSet enumerates all commits reachable from the given starting
// hashes (inclusive), following parents per firstParent, up to maxWalk
// commits. Returns ErrInvalidRef if the cap is exceeded.
func reachableSet(repo *git.Repository, starts []plumbing.Hash, firstParent bool, maxWalk int) (map[plumbing.Hash]struct{}, error) {
	out := make(map[plumbing.Hash]struct{})
	if len(starts) == 0 {
		return out, nil
	}

	queue := make([]plumbing.Hash, 0, len(starts))
	for _, h := range starts {
		if _, ok := out[h]; ok {
			continue
		}
		out[h] = struct{}{}
		queue = append(queue, h)
	}

	for len(queue) > 0 {
		if len(out) > maxWalk {
			return nil, protocol.NewError(protocol.ErrInvalidRef,
				fmt.Sprintf("excluded reachable set exceeds %d commits; narrow the spec", maxWalk))
		}
		h := queue[0]
		queue = queue[1:]

		c, err := repo.CommitObject(h)
		if err != nil {
			// Tolerate missing parent commits (shallow clones, broken
			// repos) by dropping that branch rather than failing.
			continue
		}
		n := len(c.ParentHashes)
		if firstParent && n > 1 {
			n = 1
		}
		for i := 0; i < n; i++ {
			ph := c.ParentHashes[i]
			if _, seen := out[ph]; seen {
				continue
			}
			out[ph] = struct{}{}
			queue = append(queue, ph)
		}
	}
	return out, nil
}

// walkTip BFS-walks from a tip, skipping (and not descending into) any
// commit whose hash is in the exclusion set.
func walkTip(repo *git.Repository, tip plumbing.Hash, excl map[plumbing.Hash]struct{}, firstParent bool) ([]*object.Commit, error) {
	if _, ex := excl[tip]; ex {
		return nil, nil
	}

	var out []*object.Commit
	seen := make(map[plumbing.Hash]struct{})
	queue := []plumbing.Hash{tip}
	seen[tip] = struct{}{}

	for len(queue) > 0 {
		h := queue[0]
		queue = queue[1:]

		c, err := repo.CommitObject(h)
		if err != nil {
			continue
		}
		out = append(out, c)

		n := len(c.ParentHashes)
		if firstParent && n > 1 {
			n = 1
		}
		for i := 0; i < n; i++ {
			ph := c.ParentHashes[i]
			if _, dup := seen[ph]; dup {
				continue
			}
			if _, ex := excl[ph]; ex {
				seen[ph] = struct{}{}
				continue
			}
			seen[ph] = struct{}{}
			queue = append(queue, ph)
		}
	}
	return out, nil
}

// cloneSet returns a shallow copy of a hash set.
func cloneSet(in map[plumbing.Hash]struct{}) map[plumbing.Hash]struct{} {
	out := make(map[plumbing.Hash]struct{}, len(in))
	for k := range in {
		out[k] = struct{}{}
	}
	return out
}

// sortByCommitterTimeDesc sorts in-place by committer time, descending.
// Insertion sort is used since per-tip commit counts in practice are
// small (typically ≤ DefaultMaxExcludeWalk and usually far smaller).
func sortByCommitterTimeDesc(commits []*object.Commit) {
	for i := 1; i < len(commits); i++ {
		for j := i; j > 0 && commits[j].Committer.When.After(commits[j-1].Committer.When); j-- {
			commits[j], commits[j-1] = commits[j-1], commits[j]
		}
	}
}

// tipCursor is a position in one tip's pre-sorted commit list.
type tipCursor struct {
	entries []WalkedCommit
	pos     int
}

func (c *tipCursor) head() WalkedCommit { return c.entries[c.pos] }

// mergeByCommitterTime k-way merges per-tip output lists in
// committer-time descending order, deduplicating commits by hash.
// The first tip to emit a commit gets to label its Side.
func mergeByCommitterTime(perTip [][]WalkedCommit) []WalkedCommit {
	cursors := make([]*tipCursor, 0, len(perTip))
	for i := range perTip {
		if len(perTip[i]) == 0 {
			continue
		}
		cursors = append(cursors, &tipCursor{entries: perTip[i]})
	}
	if len(cursors) == 0 {
		return nil
	}

	h := cursorHeap(cursors)
	heap.Init(&h)

	seen := make(map[plumbing.Hash]struct{})
	out := make([]WalkedCommit, 0)

	for h.Len() > 0 {
		top := h[0]
		entry := top.head()
		top.pos++
		if top.pos >= len(top.entries) {
			heap.Pop(&h)
		} else {
			heap.Fix(&h, 0)
		}
		if _, dup := seen[entry.Commit.Hash]; dup {
			continue
		}
		seen[entry.Commit.Hash] = struct{}{}
		out = append(out, entry)
	}
	return out
}

// cursorHeap is a max-heap over tipCursors keyed by their head entry's
// committer time.
type cursorHeap []*tipCursor

func (h cursorHeap) Len() int { return len(h) }
func (h cursorHeap) Less(i, j int) bool {
	return h[i].head().Commit.Committer.When.After(h[j].head().Commit.Committer.When)
}
func (h cursorHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }
func (h *cursorHeap) Push(x any)   { *h = append(*h, x.(*tipCursor)) }
func (h *cursorHeap) Pop() any {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[:n-1]
	return x
}

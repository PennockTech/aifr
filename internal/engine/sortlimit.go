// Copyright 2026 — see LICENSE file for terms.
package engine

import (
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"go.pennock.tech/aifr/pkg/protocol"
)

// SortOrder specifies how to sort results.
type SortOrder string

const (
	SortNone    SortOrder = ""
	SortName    SortOrder = "name"
	SortPath    SortOrder = "path"
	SortSize    SortOrder = "size"
	SortMtime   SortOrder = "mtime"
	SortVersion SortOrder = "version"
)

// SortEntries sorts StatEntry slices in place.
func SortEntries(entries []protocol.StatEntry, order SortOrder, descending bool) {
	if order == SortNone || len(entries) < 2 {
		return
	}

	slices.SortStableFunc(entries, func(a, b protocol.StatEntry) int {
		var cmp int
		switch order {
		case SortName:
			cmp = strings.Compare(filepath.Base(a.Path), filepath.Base(b.Path))
		case SortPath:
			cmp = strings.Compare(a.Path, b.Path)
		case SortSize:
			cmp = compareInt64(a.Size, b.Size)
		case SortMtime:
			cmp = strings.Compare(a.ModTime, b.ModTime) // ISO format sorts lexically
		case SortVersion:
			cmp = compareVersions(filepath.Base(a.Path), filepath.Base(b.Path))
		}
		if descending {
			cmp = -cmp
		}
		return cmp
	})
}

// SortFindEntries sorts FindEntry slices in place.
func SortFindEntries(entries []protocol.FindEntry, order SortOrder, descending bool) {
	if order == SortNone || len(entries) < 2 {
		return
	}

	slices.SortStableFunc(entries, func(a, b protocol.FindEntry) int {
		var cmp int
		switch order {
		case SortName:
			cmp = strings.Compare(filepath.Base(a.Path), filepath.Base(b.Path))
		case SortPath:
			cmp = strings.Compare(a.Path, b.Path)
		case SortSize:
			cmp = compareInt64(a.Size, b.Size)
		case SortVersion:
			cmp = compareVersions(filepath.Base(a.Path), filepath.Base(b.Path))
		default:
			cmp = strings.Compare(a.Path, b.Path)
		}
		if descending {
			cmp = -cmp
		}
		return cmp
	})
}

func compareInt64(a, b int64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

// compareVersions implements version-aware comparison like sort -V.
// Splits strings into text and numeric segments, compares numerically where possible.
func compareVersions(a, b string) int {
	segsA := versionSegments(a)
	segsB := versionSegments(b)

	n := min(len(segsA), len(segsB))

	for i := range n {
		sa, sb := segsA[i], segsB[i]
		// Try numeric comparison first.
		na, errA := strconv.ParseInt(sa, 10, 64)
		nb, errB := strconv.ParseInt(sb, 10, 64)
		if errA == nil && errB == nil {
			if na != nb {
				return compareInt64(na, nb)
			}
			continue
		}
		// Fall back to lexical comparison.
		if c := strings.Compare(sa, sb); c != 0 {
			return c
		}
	}

	// Shorter wins if all segments equal so far.
	return compareInt64(int64(len(segsA)), int64(len(segsB)))
}

// versionSegments splits a string into alternating text and numeric segments.
// "v5.17.0" → ["v", "5", ".", "17", ".", "0"]
// "go-git@v5.17.0" → ["go-git@v", "5", ".", "17", ".", "0"]
func versionSegments(s string) []string {
	var segs []string
	i := 0
	for i < len(s) {
		if s[i] >= '0' && s[i] <= '9' {
			j := i
			for j < len(s) && s[j] >= '0' && s[j] <= '9' {
				j++
			}
			segs = append(segs, s[i:j])
			i = j
		} else {
			j := i
			for j < len(s) && (s[j] < '0' || s[j] > '9') {
				j++
			}
			segs = append(segs, s[i:j])
			i = j
		}
	}
	return segs
}

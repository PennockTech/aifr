// Copyright 2026 — see LICENSE file for terms.
package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// ── Find pagination ──

func TestFindLimitComplete(t *testing.T) {
	dir := t.TempDir()
	for i := range 10 {
		mkTestFile(t, dir, filepath.Join("f"+string(rune('a'+i))+".txt"), "x")
	}
	eng := newTestEngine(t, dir)

	resp, err := eng.Find(dir, FindParams{Type: "f", Limit: 3})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Total != 10 {
		t.Errorf("Total = %d, want 10", resp.Total)
	}
	if len(resp.Entries) != 3 {
		t.Errorf("len(Entries) = %d, want 3", len(resp.Entries))
	}
	if resp.Complete {
		t.Error("Complete = true, want false (results were truncated)")
	}
	if resp.Continuation == "" {
		t.Error("Continuation is empty, want non-empty token")
	}
}

func TestFindLimitNoTruncation(t *testing.T) {
	dir := t.TempDir()
	mkTestFile(t, dir, "a.txt", "x")
	mkTestFile(t, dir, "b.txt", "x")
	eng := newTestEngine(t, dir)

	resp, err := eng.Find(dir, FindParams{Type: "f", Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Total != 2 {
		t.Errorf("Total = %d, want 2", resp.Total)
	}
	if resp.Complete != true {
		t.Error("Complete = false, want true (all results fit)")
	}
	if resp.Continuation != "" {
		t.Errorf("Continuation = %q, want empty", resp.Continuation)
	}
}

func TestFindContinuation(t *testing.T) {
	dir := t.TempDir()
	for i := range 10 {
		mkTestFile(t, dir, filepath.Join("f"+string(rune('a'+i))+".txt"), "x")
	}
	eng := newTestEngine(t, dir)

	// Collect all results via pagination.
	var allPaths []string
	offset := 0
	for {
		resp, err := eng.Find(dir, FindParams{Type: "f", Limit: 3, Sort: "name", Offset: offset})
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range resp.Entries {
			allPaths = append(allPaths, e.Path)
		}
		if resp.Complete {
			break
		}
		offset += len(resp.Entries)
	}

	if len(allPaths) != 10 {
		t.Errorf("paginated through %d results, want 10", len(allPaths))
	}
}

// ── List pagination ──

func TestListLimitComplete(t *testing.T) {
	dir := t.TempDir()
	for i := range 8 {
		mkTestFile(t, dir, "f"+string(rune('a'+i))+".txt", "x")
	}
	eng := newTestEngine(t, dir)

	resp, err := eng.List(dir, ListParams{Limit: 3})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Total != 8 {
		t.Errorf("Total = %d, want 8", resp.Total)
	}
	if len(resp.Entries) != 3 {
		t.Errorf("len(Entries) = %d, want 3", len(resp.Entries))
	}
	if resp.Complete {
		t.Error("Complete = true, want false")
	}
	if resp.Continuation == "" {
		t.Error("Continuation is empty, want non-empty token")
	}
}

func TestListLimitNoTruncation(t *testing.T) {
	dir := t.TempDir()
	mkTestFile(t, dir, "a.txt", "x")
	mkTestFile(t, dir, "b.txt", "x")
	eng := newTestEngine(t, dir)

	resp, err := eng.List(dir, ListParams{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Total != 2 {
		t.Errorf("Total = %d, want 2", resp.Total)
	}
	if !resp.Complete {
		t.Error("Complete = false, want true")
	}
	if resp.Continuation != "" {
		t.Errorf("Continuation = %q, want empty", resp.Continuation)
	}
}

func TestListContinuation(t *testing.T) {
	dir := t.TempDir()
	for i := range 8 {
		mkTestFile(t, dir, "f"+string(rune('a'+i))+".txt", "x")
	}
	eng := newTestEngine(t, dir)

	var allNames []string
	offset := 0
	for {
		resp, err := eng.List(dir, ListParams{Limit: 3, Sort: "name", Offset: offset})
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range resp.Entries {
			allNames = append(allNames, e.Name)
		}
		if resp.Complete {
			break
		}
		offset += len(resp.Entries)
	}

	if len(allNames) != 8 {
		t.Errorf("paginated through %d results, want 8", len(allNames))
	}
}

// ── Search pagination ──

func TestSearchCompleteField(t *testing.T) {
	dir := t.TempDir()
	var content strings.Builder
	for range 20 {
		content.WriteString("match line\n")
	}
	mkTestFile(t, dir, "many.txt", content.String())
	eng := newTestEngine(t, dir)

	resp, err := eng.Search("match", dir, SearchParams{MaxMatches: 5})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Complete {
		t.Error("Complete = true, want false (truncated)")
	}
	if !resp.Truncated {
		t.Error("Truncated = false, want true")
	}
	if resp.Continuation == "" {
		t.Error("Continuation is empty, want non-empty token")
	}
}

func TestSearchCompleteNoTruncation(t *testing.T) {
	dir := t.TempDir()
	mkTestFile(t, dir, "small.txt", "match\nno-match\nmatch\n")
	eng := newTestEngine(t, dir)

	resp, err := eng.Search("match", dir, SearchParams{MaxMatches: 10})
	if err != nil {
		t.Fatal(err)
	}
	if !resp.Complete {
		t.Error("Complete = false, want true")
	}
	if resp.Truncated {
		t.Error("Truncated = true, want false")
	}
	if resp.Continuation != "" {
		t.Errorf("Continuation = %q, want empty", resp.Continuation)
	}
}

// ── Cat pagination ──

func TestCatMaxFilesComplete(t *testing.T) {
	dir := t.TempDir()
	for i := range 5 {
		mkTestFile(t, dir, "f"+string(rune('a'+i))+".go", "package x\n")
	}
	eng := newTestEngine(t, dir)

	resp, err := eng.Cat(nil, dir, CatParams{Name: "*.go", MaxDepth: -1, MaxFiles: 2})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Complete {
		t.Error("Complete = true, want false (max_files truncation)")
	}
	if !resp.Truncated {
		t.Error("Truncated = false, want true")
	}
}

func TestCatMaxTotalSizeComplete(t *testing.T) {
	dir := t.TempDir()
	bigContent := strings.Repeat("x", 1024)
	for i := range 5 {
		mkTestFile(t, dir, "f"+string(rune('a'+i))+".txt", bigContent)
	}
	eng := newTestEngine(t, dir)

	resp, err := eng.Cat(nil, dir, CatParams{Name: "*.txt", MaxDepth: -1, MaxTotalSize: 2048})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Complete {
		t.Error("Complete = true, want false (output_size_limit truncation)")
	}
	if !resp.Truncated {
		t.Error("Truncated = false, want true")
	}
}

// ── Read pagination (verify existing behavior) ──

func TestReadLargeFileComplete(t *testing.T) {
	dir := t.TempDir()
	// Create file larger than DefaultChunkSize (64 KiB).
	content := strings.Repeat("x", DefaultChunkSize+1024)
	path := mkTestFile(t, dir, "large.txt", content)
	eng := newTestEngine(t, dir)

	resp, err := eng.Read(path, ReadParams{})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Complete {
		t.Error("Complete = true, want false for large file")
	}
	if resp.Continuation == "" {
		t.Error("Continuation is empty for large file")
	}

	// Continue reading.
	resp2, err := eng.Read("", ReadParams{ChunkID: resp.Continuation})
	if err != nil {
		t.Fatal(err)
	}
	if !resp2.Complete {
		t.Error("Complete = false after continuation, want true")
	}
}

// ── Log pagination ──

func TestLogCompleteField(t *testing.T) {
	// Test the LogResponse.Complete field via a git repo created with go-git.
	dir := t.TempDir()
	initTestGitRepo(t, dir, 5) // create 5 commits
	eng := newTestEngine(t, dir)

	// Request fewer commits than exist.
	resp, err := eng.Log(dir, "HEAD", LogParams{MaxCount: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Entries) != 2 {
		t.Errorf("len(Entries) = %d, want 2", len(resp.Entries))
	}
	if resp.Complete {
		t.Error("Complete = true, want false (more commits exist)")
	}
	if resp.Continuation == "" {
		t.Error("Continuation is empty, want non-empty token")
	}

	// Request more commits than exist.
	resp2, err := eng.Log(dir, "HEAD", LogParams{MaxCount: 100})
	if err != nil {
		t.Fatal(err)
	}
	if !resp2.Complete {
		t.Error("Complete = false, want true (all commits returned)")
	}
	if resp2.Continuation != "" {
		t.Errorf("Continuation = %q, want empty", resp2.Continuation)
	}
}

func TestLogSkip(t *testing.T) {
	dir := t.TempDir()
	initTestGitRepo(t, dir, 5)
	eng := newTestEngine(t, dir)

	// Get all commits to know the expected order.
	all, err := eng.Log(dir, "HEAD", LogParams{MaxCount: 100})
	if err != nil {
		t.Fatal(err)
	}
	if len(all.Entries) != 5 {
		t.Fatalf("expected 5 commits, got %d", len(all.Entries))
	}

	// Skip 2, get 2 — should start at the 3rd commit.
	resp, err := eng.Log(dir, "HEAD", LogParams{MaxCount: 2, Skip: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(resp.Entries))
	}
	if resp.Entries[0].Hash != all.Entries[2].Hash {
		t.Errorf("first entry after skip=2: got %s, want %s",
			resp.Entries[0].Hash[:12], all.Entries[2].Hash[:12])
	}
	if resp.Entries[1].Hash != all.Entries[3].Hash {
		t.Errorf("second entry after skip=2: got %s, want %s",
			resp.Entries[1].Hash[:12], all.Entries[3].Hash[:12])
	}

	// Skip past all commits — should return empty.
	resp2, err := eng.Log(dir, "HEAD", LogParams{MaxCount: 10, Skip: 100})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp2.Entries) != 0 {
		t.Errorf("expected 0 entries after skipping past all, got %d", len(resp2.Entries))
	}
	if !resp2.Complete {
		t.Error("expected Complete=true when no entries returned")
	}

	// Skip 0 — same as no skip.
	resp3, err := eng.Log(dir, "HEAD", LogParams{MaxCount: 2, Skip: 0})
	if err != nil {
		t.Fatal(err)
	}
	if resp3.Entries[0].Hash != all.Entries[0].Hash {
		t.Errorf("skip=0 first entry: got %s, want %s",
			resp3.Entries[0].Hash[:12], all.Entries[0].Hash[:12])
	}
}

// initTestGitRepo creates a bare-minimum git repo with N commits using go-git.
func initTestGitRepo(t *testing.T, dir string, numCommits int) {
	t.Helper()

	// Initialize a git repo.
	repo, err := gogit.PlainInit(dir, false)
	if err != nil {
		t.Fatal(err)
	}

	wt, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}

	for i := range numCommits {
		fname := filepath.Join(dir, "file"+string(rune('a'+i))+".txt")
		os.WriteFile(fname, []byte("commit "+string(rune('0'+i))), 0o644) //nolint:errcheck
		wt.Add(filepath.Base(fname))                                      //nolint:errcheck
		_, err := wt.Commit("commit "+string(rune('0'+i)), &gogit.CommitOptions{
			Author: &object.Signature{
				Name:  "Test",
				Email: "test@example.com",
				When:  time.Now(),
			},
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

// ── Reflog pagination ──

func TestReflogCompleteField(t *testing.T) {
	dir := t.TempDir()
	// Create a git repo so openGitRepo works, then write a synthetic reflog.
	initTestGitRepo(t, dir, 1)

	logDir := filepath.Join(dir, ".git", "logs")

	// Ensure logs dir exists and overwrite with 10 entries.
	os.MkdirAll(logDir, 0o755) //nolint:errcheck
	var lines strings.Builder
	for range 10 {
		lines.WriteString("0000000000000000000000000000000000000000 ")
		lines.WriteString("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa ")
		lines.WriteString("Test User <test@example.com> 1700000000 +0000\t")
		lines.WriteString("commit: test\n")
	}
	os.WriteFile(filepath.Join(logDir, "HEAD"), []byte(lines.String()), 0o644) //nolint:errcheck

	eng := newTestEngine(t, dir)
	resp, err := eng.Reflog(dir, "HEAD", ReflogParams{MaxCount: 3})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Entries) != 3 {
		t.Errorf("len(Entries) = %d, want 3", len(resp.Entries))
	}
	if resp.Total != 10 {
		t.Errorf("Total = %d, want 10", resp.Total)
	}
	if resp.Complete {
		t.Error("Complete = true, want false (truncated)")
	}
	if resp.Continuation == "" {
		t.Error("Continuation is empty, want non-empty token")
	}
}

func TestReflogCompleteNoTruncation(t *testing.T) {
	dir := t.TempDir()
	initTestGitRepo(t, dir, 1)

	logDir := filepath.Join(dir, ".git", "logs")
	os.MkdirAll(logDir, 0o755) //nolint:errcheck

	var lines strings.Builder
	for range 3 {
		lines.WriteString("0000000000000000000000000000000000000000 ")
		lines.WriteString("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa ")
		lines.WriteString("Test User <test@example.com> 1700000000 +0000\t")
		lines.WriteString("commit: test\n")
	}
	os.WriteFile(filepath.Join(logDir, "HEAD"), []byte(lines.String()), 0o644) //nolint:errcheck

	eng := newTestEngine(t, dir)
	resp, err := eng.Reflog(dir, "HEAD", ReflogParams{MaxCount: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Entries) != 3 {
		t.Errorf("len(Entries) = %d, want 3", len(resp.Entries))
	}
	if !resp.Complete {
		t.Error("Complete = false, want true (all entries fit)")
	}
	if resp.Continuation != "" {
		t.Errorf("Continuation = %q, want empty", resp.Continuation)
	}
}

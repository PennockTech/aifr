// Copyright 2026 — see LICENSE file for terms.
package engine

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSearchFixedString(t *testing.T) {
	dir := t.TempDir()
	mkTestFile(t, dir, "a.go", "package main\n\nfunc Hello() string {\n\treturn \"hello\"\n}\n")
	mkTestFile(t, dir, "b.go", "package main\n\nfunc World() string {\n\treturn \"world\"\n}\n")
	eng := newTestEngine(t, dir)

	resp, err := eng.Search("Hello", dir, SearchParams{})
	if err != nil {
		t.Fatal(err)
	}
	if resp.TotalMatches != 1 {
		t.Errorf("total matches = %d, want 1", resp.TotalMatches)
	}
	if resp.FilesMatched != 1 {
		t.Errorf("files matched = %d, want 1", resp.FilesMatched)
	}
	if resp.FilesSearched != 2 {
		t.Errorf("files searched = %d, want 2", resp.FilesSearched)
	}
}

func TestSearchRegexp(t *testing.T) {
	dir := t.TempDir()
	mkTestFile(t, dir, "handlers.go", "func UserHandler() {}\nfunc AdminHandler() {}\nfunc helper() {}\n")
	eng := newTestEngine(t, dir)

	resp, err := eng.Search("func.*Handler", dir, SearchParams{IsRegexp: true})
	if err != nil {
		t.Fatal(err)
	}
	if resp.TotalMatches != 2 {
		t.Errorf("total matches = %d, want 2", resp.TotalMatches)
	}
}

func TestSearchIgnoreCase(t *testing.T) {
	dir := t.TempDir()
	mkTestFile(t, dir, "mixed.txt", "Hello\nhello\nHELLO\nworld\n")
	eng := newTestEngine(t, dir)

	resp, err := eng.Search("hello", dir, SearchParams{IgnoreCase: true})
	if err != nil {
		t.Fatal(err)
	}
	if resp.TotalMatches != 3 {
		t.Errorf("total matches = %d, want 3", resp.TotalMatches)
	}
}

func TestSearchWithContext(t *testing.T) {
	dir := t.TempDir()
	mkTestFile(t, dir, "ctx.txt", "line1\nline2\nTARGET\nline4\nline5\n")
	eng := newTestEngine(t, dir)

	resp, err := eng.Search("TARGET", dir, SearchParams{Context: 1})
	if err != nil {
		t.Fatal(err)
	}
	if resp.TotalMatches != 1 {
		t.Fatalf("total matches = %d, want 1", resp.TotalMatches)
	}
	m := resp.Matches[0]
	if len(m.ContextBefore) != 1 || m.ContextBefore[0] != "line2" {
		t.Errorf("context_before = %v, want [line2]", m.ContextBefore)
	}
	if len(m.ContextAfter) != 1 || m.ContextAfter[0] != "line4" {
		t.Errorf("context_after = %v, want [line4]", m.ContextAfter)
	}
}

func TestSearchMaxMatches(t *testing.T) {
	dir := t.TempDir()
	// Create a file with many matches.
	content := ""
	for i := 0; i < 100; i++ {
		content += "match\n"
	}
	mkTestFile(t, dir, "many.txt", content)
	eng := newTestEngine(t, dir)

	resp, err := eng.Search("match", dir, SearchParams{MaxMatches: 5})
	if err != nil {
		t.Fatal(err)
	}
	if resp.TotalMatches != 5 {
		t.Errorf("total matches = %d, want 5", resp.TotalMatches)
	}
	if !resp.Truncated {
		t.Error("expected truncated=true")
	}
}

func TestSearchIncludeExclude(t *testing.T) {
	dir := t.TempDir()
	mkTestFile(t, dir, "code.go", "func main() {}\n")
	mkTestFile(t, dir, "test.txt", "func main() {}\n")
	eng := newTestEngine(t, dir)

	// Include only .go files.
	resp, err := eng.Search("func", dir, SearchParams{Include: "*.go"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.FilesSearched != 1 {
		t.Errorf("files searched = %d, want 1", resp.FilesSearched)
	}

	// Exclude .go files.
	resp, err = eng.Search("func", dir, SearchParams{Exclude: "*.go"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.FilesSearched != 1 {
		t.Errorf("files searched = %d, want 1 (the .txt)", resp.FilesSearched)
	}
}

func TestSearchSkipsBinary(t *testing.T) {
	dir := t.TempDir()
	mkTestFile(t, dir, "text.txt", "findme\n")
	// Create a binary file containing the pattern.
	os.WriteFile(filepath.Join(dir, "binary.dat"), []byte("findme\x00binary"), 0o644) //nolint:errcheck
	eng := newTestEngine(t, dir)

	resp, err := eng.Search("findme", dir, SearchParams{})
	if err != nil {
		t.Fatal(err)
	}
	if resp.TotalMatches != 1 {
		t.Errorf("total matches = %d, want 1 (binary should be skipped)", resp.TotalMatches)
	}
}

func TestSearchSingleFile(t *testing.T) {
	dir := t.TempDir()
	path := mkTestFile(t, dir, "single.txt", "alpha\nbeta\ngamma\n")
	eng := newTestEngine(t, dir)

	resp, err := eng.Search("beta", path, SearchParams{})
	if err != nil {
		t.Fatal(err)
	}
	if resp.TotalMatches != 1 {
		t.Errorf("total matches = %d, want 1", resp.TotalMatches)
	}
}

func TestFindByName(t *testing.T) {
	dir := t.TempDir()
	mkTestFile(t, dir, "a.go", "")
	mkTestFile(t, dir, "b.txt", "")
	mkTestFile(t, dir, "sub/c.go", "")
	eng := newTestEngine(t, dir)

	resp, err := eng.Find(dir, FindParams{Name: "*.go", MaxDepth: -1})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Total != 2 {
		t.Errorf("total = %d, want 2", resp.Total)
	}
}

func TestFindByType(t *testing.T) {
	dir := t.TempDir()
	mkTestFile(t, dir, "file.txt", "")
	os.Mkdir(filepath.Join(dir, "subdir"), 0o755) //nolint:errcheck
	eng := newTestEngine(t, dir)

	// Find only dirs.
	resp, err := eng.Find(dir, FindParams{Type: "d", MaxDepth: -1})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Total != 1 {
		t.Errorf("dirs = %d, want 1", resp.Total)
	}
}

func TestFindBySize(t *testing.T) {
	dir := t.TempDir()
	mkTestFile(t, dir, "small.txt", "hi")
	mkTestFile(t, dir, "big.txt", "this is a much bigger file with more content")
	eng := newTestEngine(t, dir)

	resp, err := eng.Find(dir, FindParams{MinSize: 10, MaxDepth: -1})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Total != 1 {
		t.Errorf("total = %d, want 1 (big file only)", resp.Total)
	}
}

func TestFindByAge(t *testing.T) {
	dir := t.TempDir()
	path := mkTestFile(t, dir, "recent.txt", "new")
	eng := newTestEngine(t, dir)

	// File just created should be newer than 1 hour.
	resp, err := eng.Find(dir, FindParams{NewerThan: time.Hour, MaxDepth: -1})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Total != 1 {
		t.Errorf("total = %d, want 1", resp.Total)
	}

	// Make the file old.
	oldTime := time.Now().Add(-48 * time.Hour)
	os.Chtimes(path, oldTime, oldTime) //nolint:errcheck

	resp, err = eng.Find(dir, FindParams{NewerThan: time.Hour, MaxDepth: -1})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Total != 0 {
		t.Errorf("total = %d, want 0 (old file should be excluded)", resp.Total)
	}
}

func TestFindMaxDepth(t *testing.T) {
	dir := t.TempDir()
	mkTestFile(t, dir, "a.txt", "")
	mkTestFile(t, dir, "sub/b.txt", "")
	mkTestFile(t, dir, "sub/deep/c.txt", "")
	eng := newTestEngine(t, dir)

	// Depth 0: only root entries.
	resp, err := eng.Find(dir, FindParams{MaxDepth: 0, Type: "f"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Total != 1 {
		t.Errorf("depth 0: total = %d, want 1", resp.Total)
	}

	// Depth 1: root + one level.
	resp, err = eng.Find(dir, FindParams{MaxDepth: 1, Type: "f"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Total != 2 {
		t.Errorf("depth 1: total = %d, want 2", resp.Total)
	}
}

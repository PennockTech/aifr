// Copyright 2026 — see LICENSE file for terms.
package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCatExplicitPaths(t *testing.T) {
	dir := t.TempDir()
	f1 := mkTestFile(t, dir, "a.go", "package a\n")
	f2 := mkTestFile(t, dir, "b.go", "package b\n")
	f3 := mkTestFile(t, dir, "c.go", "package c\n")
	eng := newTestEngine(t, dir)

	resp, err := eng.Cat([]string{f1, f2, f3}, "", CatParams{})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Mode != "explicit" {
		t.Errorf("mode = %q, want explicit", resp.Mode)
	}
	if resp.FilesRead != 3 {
		t.Errorf("files_read = %d, want 3", resp.FilesRead)
	}
	// Verify order is preserved.
	if resp.Files[0].Content != "package a\n" {
		t.Errorf("first file content = %q, want 'package a\\n'", resp.Files[0].Content)
	}
	if resp.Files[2].Content != "package c\n" {
		t.Errorf("third file content = %q, want 'package c\\n'", resp.Files[2].Content)
	}
}

func TestCatDiscoveryMode(t *testing.T) {
	dir := t.TempDir()
	mkTestFile(t, dir, "a.go", "package a\n")
	mkTestFile(t, dir, "b.txt", "text\n")
	mkTestFile(t, dir, "sub/c.go", "package c\n")
	eng := newTestEngine(t, dir)

	resp, err := eng.Cat(nil, dir, CatParams{Name: "*.go", MaxDepth: -1})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Mode != "discover" {
		t.Errorf("mode = %q, want discover", resp.Mode)
	}
	if resp.FilesRead != 2 {
		t.Errorf("files_read = %d, want 2 (.go files only)", resp.FilesRead)
	}
}

func TestCatExcludePath(t *testing.T) {
	dir := t.TempDir()
	mkTestFile(t, dir, "main.go", "package main\n")
	mkTestFile(t, dir, "vendor/dep/dep.go", "package dep\n")
	mkTestFile(t, dir, "internal/lib.go", "package lib\n")
	eng := newTestEngine(t, dir)

	resp, err := eng.Cat(nil, dir, CatParams{
		Name:        "*.go",
		ExcludePath: "**/vendor/**",
		MaxDepth:    -1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.FilesRead != 2 {
		t.Errorf("files_read = %d, want 2 (vendor excluded)", resp.FilesRead)
	}
	for _, f := range resp.Files {
		if strings.Contains(f.Path, "vendor") {
			t.Errorf("vendor file %q should have been excluded", f.Path)
		}
	}
}

func TestCatLinesLimit(t *testing.T) {
	dir := t.TempDir()
	content := "line1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nline9\nline10\n"
	path := mkTestFile(t, dir, "multi.txt", content)
	eng := newTestEngine(t, dir)

	resp, err := eng.Cat([]string{path}, "", CatParams{Lines: 5})
	if err != nil {
		t.Fatal(err)
	}
	if resp.FilesRead != 1 {
		t.Fatalf("files_read = %d, want 1", resp.FilesRead)
	}
	entry := resp.Files[0]
	if entry.Lines != 5 {
		t.Errorf("lines = %d, want 5", entry.Lines)
	}
	if !entry.Truncated {
		t.Error("expected truncated=true for line-limited read")
	}
	if strings.Count(entry.Content, "\n") != 5 {
		t.Errorf("content has %d newlines, want 5", strings.Count(entry.Content, "\n"))
	}
}

func TestCatBinarySkip(t *testing.T) {
	dir := t.TempDir()
	binPath := filepath.Join(dir, "binary.dat")
	os.WriteFile(binPath, []byte("hello\x00world\x00binary"), 0o644) //nolint:errcheck
	textPath := mkTestFile(t, dir, "text.txt", "hello\n")
	eng := newTestEngine(t, dir)

	resp, err := eng.Cat([]string{binPath, textPath}, "", CatParams{})
	if err != nil {
		t.Fatal(err)
	}
	if resp.FilesRead != 1 {
		t.Errorf("files_read = %d, want 1 (binary skipped)", resp.FilesRead)
	}
	if resp.FilesSkipped != 1 {
		t.Errorf("files_skipped = %d, want 1", resp.FilesSkipped)
	}
	if !resp.Files[0].Binary {
		t.Error("expected first file to be marked binary")
	}
}

func TestCatAccessDenied(t *testing.T) {
	dir := t.TempDir()
	// Create a .env file which should be caught by sensitive patterns.
	envFile := filepath.Join(dir, ".env")
	os.WriteFile(envFile, []byte("SECRET=x"), 0o644) //nolint:errcheck
	textPath := mkTestFile(t, dir, "ok.txt", "fine\n")
	eng := newTestEngine(t, dir)

	resp, err := eng.Cat([]string{envFile, textPath}, "", CatParams{})
	if err != nil {
		t.Fatal(err)
	}
	// Should not crash — error recorded in entry.
	if resp.FilesSkipped != 1 {
		t.Errorf("files_skipped = %d, want 1", resp.FilesSkipped)
	}
	if resp.FilesRead != 1 {
		t.Errorf("files_read = %d, want 1", resp.FilesRead)
	}
	if resp.Files[0].Error == "" {
		t.Error("expected error for .env file")
	}
}

func TestCatMaxTotalSize(t *testing.T) {
	dir := t.TempDir()
	// Create files that together exceed a small limit.
	bigContent := strings.Repeat("x", 600) + "\n"
	mkTestFile(t, dir, "a.txt", bigContent)
	mkTestFile(t, dir, "b.txt", bigContent)
	mkTestFile(t, dir, "c.txt", bigContent)
	eng := newTestEngine(t, dir)

	resp, err := eng.Cat(nil, dir, CatParams{MaxTotalSize: 1200, MaxDepth: -1})
	if err != nil {
		t.Fatal(err)
	}
	if !resp.Truncated {
		t.Error("expected truncated=true when size limit exceeded")
	}
	if resp.Warning != "output_size_limit" {
		t.Errorf("warning = %q, want output_size_limit", resp.Warning)
	}
	// Should have read fewer than 3 files.
	if resp.FilesRead >= 3 {
		t.Errorf("files_read = %d, expected < 3", resp.FilesRead)
	}
}

func TestCatEmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := mkTestFile(t, dir, "empty.txt", "")
	eng := newTestEngine(t, dir)

	resp, err := eng.Cat([]string{path}, "", CatParams{})
	if err != nil {
		t.Fatal(err)
	}
	if resp.FilesRead != 1 {
		t.Errorf("files_read = %d, want 1", resp.FilesRead)
	}
	if resp.Files[0].Content != "" {
		t.Errorf("content = %q, want empty", resp.Files[0].Content)
	}
}

func TestCatDiscoverySorted(t *testing.T) {
	dir := t.TempDir()
	mkTestFile(t, dir, "z.go", "z\n")
	mkTestFile(t, dir, "a.go", "a\n")
	mkTestFile(t, dir, "m.go", "m\n")
	eng := newTestEngine(t, dir)

	resp, err := eng.Cat(nil, dir, CatParams{Name: "*.go", MaxDepth: -1})
	if err != nil {
		t.Fatal(err)
	}
	// Should be sorted: a.go, m.go, z.go.
	if resp.FilesRead != 3 {
		t.Fatalf("files_read = %d, want 3", resp.FilesRead)
	}
	if !strings.HasSuffix(resp.Files[0].Path, "a.go") {
		t.Errorf("first file = %q, want a.go", resp.Files[0].Path)
	}
	if !strings.HasSuffix(resp.Files[2].Path, "z.go") {
		t.Errorf("last file = %q, want z.go", resp.Files[2].Path)
	}
}

func TestCatMaxFiles(t *testing.T) {
	dir := t.TempDir()
	for i := range 10 {
		mkTestFile(t, dir, filepath.Join("sub", string(rune('a'+i))+".txt"), "data\n")
	}
	eng := newTestEngine(t, dir)

	resp, err := eng.Cat(nil, dir, CatParams{MaxDepth: -1, MaxFiles: 3})
	if err != nil {
		t.Fatal(err)
	}
	if !resp.Truncated {
		t.Error("expected truncated=true when max files exceeded")
	}
	if len(resp.Files) != 3 {
		t.Errorf("files count = %d, want 3", len(resp.Files))
	}
}

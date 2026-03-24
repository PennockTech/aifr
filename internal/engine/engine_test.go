// Copyright 2026 — see LICENSE file for terms.
package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.pennock.tech/aifr/internal/accessctl"
	"go.pennock.tech/aifr/internal/config"
	"go.pennock.tech/aifr/pkg/protocol"
)

// newTestEngine creates an engine that allows access to the given dir.
func newTestEngine(t *testing.T, allowDir string) *Engine {
	t.Helper()
	checker, err := accessctl.NewChecker(accessctl.CheckerParams{
		Allow: []string{allowDir + "/**"},
	})
	if err != nil {
		t.Fatal(err)
	}
	eng, err := NewEngine(checker, config.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	return eng
}

// mkTestFile creates a file with known content.
func mkTestFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	os.MkdirAll(filepath.Dir(path), 0o755) //nolint:errcheck
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestStatFile(t *testing.T) {
	dir := t.TempDir()
	path := mkTestFile(t, dir, "hello.txt", "hello world\n")
	eng := newTestEngine(t, dir)

	entry, err := eng.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if entry.Type != "file" {
		t.Errorf("type = %q, want file", entry.Type)
	}
	if entry.Size != 12 {
		t.Errorf("size = %d, want 12", entry.Size)
	}
	if entry.Name != "hello.txt" {
		t.Errorf("name = %q, want hello.txt", entry.Name)
	}
}

func TestStatDir(t *testing.T) {
	dir := t.TempDir()
	subDir := filepath.Join(dir, "sub")
	os.Mkdir(subDir, 0o755) //nolint:errcheck
	eng := newTestEngine(t, dir)

	entry, err := eng.Stat(subDir)
	if err != nil {
		t.Fatal(err)
	}
	if entry.Type != "dir" {
		t.Errorf("type = %q, want dir", entry.Type)
	}
}

func TestStatAccessDenied(t *testing.T) {
	eng := newTestEngine(t, "/nonexistent-allowed-dir")
	_, err := eng.Stat("/etc/hostname")
	if err == nil {
		t.Fatal("expected access denied")
	}
}

func TestReadSmallFile(t *testing.T) {
	dir := t.TempDir()
	content := "line 1\nline 2\nline 3\n"
	path := mkTestFile(t, dir, "small.txt", content)
	eng := newTestEngine(t, dir)

	resp, err := eng.Read(path, ReadParams{})
	if err != nil {
		t.Fatal(err)
	}
	if !resp.Complete {
		t.Error("expected complete=true for small file")
	}
	if resp.Chunk.Data != content {
		t.Errorf("data = %q, want %q", resp.Chunk.Data, content)
	}
	if resp.Chunk.Encoding != "utf-8" {
		t.Errorf("encoding = %q, want utf-8", resp.Chunk.Encoding)
	}
}

func TestReadLineRange(t *testing.T) {
	dir := t.TempDir()
	var lines []string
	for i := 1; i <= 10; i++ {
		lines = append(lines, strings.Repeat("x", 10))
	}
	content := strings.Join(lines, "\n") + "\n"
	path := mkTestFile(t, dir, "lines.txt", content)
	eng := newTestEngine(t, dir)

	resp, err := eng.Read(path, ReadParams{
		Lines: &LineRange{Start: 3, End: 5},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Should contain lines 3-5.
	resultLines := strings.Split(resp.Chunk.Data, "\n")
	// Filter empty trailing line from split.
	var nonEmpty []string
	for _, l := range resultLines {
		if l != "" {
			nonEmpty = append(nonEmpty, l)
		}
	}
	if len(nonEmpty) != 3 {
		t.Errorf("expected 3 lines, got %d: %v", len(nonEmpty), nonEmpty)
	}
	if resp.Chunk.StartLine != 3 {
		t.Errorf("start_line = %d, want 3", resp.Chunk.StartLine)
	}
	if resp.Chunk.EndLine != 5 {
		t.Errorf("end_line = %d, want 5", resp.Chunk.EndLine)
	}
}

func TestReadByteRange(t *testing.T) {
	dir := t.TempDir()
	content := "abcdefghij\nklmnopqrst\nuvwxyz\n"
	path := mkTestFile(t, dir, "bytes.txt", content)
	eng := newTestEngine(t, dir)

	resp, err := eng.Read(path, ReadParams{
		Bytes: &ByteRange{Start: 0, End: 10},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Should extend to newline boundary.
	if !strings.HasSuffix(resp.Chunk.Data, "\n") {
		t.Errorf("expected data to end at newline, got %q", resp.Chunk.Data)
	}
}

func TestReadLargeFileChunking(t *testing.T) {
	dir := t.TempDir()
	// Create a file larger than DefaultChunkSize.
	var builder strings.Builder
	line := strings.Repeat("a", 100) + "\n"
	for builder.Len() < DefaultChunkSize*2 {
		builder.WriteString(line)
	}
	content := builder.String()
	path := mkTestFile(t, dir, "large.txt", content)
	eng := newTestEngine(t, dir)

	resp, err := eng.Read(path, ReadParams{})
	if err != nil {
		t.Fatal(err)
	}

	if resp.Complete {
		t.Error("expected complete=false for large file")
	}
	if resp.Continuation == "" {
		t.Error("expected continuation token")
	}

	// Read the continuation.
	resp2, err := eng.Read("", ReadParams{ChunkID: resp.Continuation})
	if err != nil {
		t.Fatal(err)
	}
	if resp2.Chunk == nil {
		t.Fatal("expected chunk data from continuation")
	}
}

func TestReadContinuationStale(t *testing.T) {
	dir := t.TempDir()
	var builder strings.Builder
	line := strings.Repeat("b", 100) + "\n"
	for builder.Len() < DefaultChunkSize*2 {
		builder.WriteString(line)
	}
	path := mkTestFile(t, dir, "stale.txt", builder.String())
	eng := newTestEngine(t, dir)

	resp, err := eng.Read(path, ReadParams{})
	if err != nil {
		t.Fatal(err)
	}

	// Modify the file to make the token stale.
	os.WriteFile(path, []byte("changed"), 0o644) //nolint:errcheck

	_, err = eng.Read("", ReadParams{ChunkID: resp.Continuation})
	if err == nil {
		t.Fatal("expected stale continuation error")
	}
	ae, ok := err.(*protocol.AifrError)
	if !ok || ae.Code != protocol.ErrStaleContinuation {
		t.Errorf("expected STALE_CONTINUATION, got: %v", err)
	}
}

func TestReadBinaryFile(t *testing.T) {
	dir := t.TempDir()
	// Create a file with NUL bytes.
	data := []byte("hello\x00world\x00binary\x00data")
	path := filepath.Join(dir, "binary.dat")
	os.WriteFile(path, data, 0o644) //nolint:errcheck
	eng := newTestEngine(t, dir)

	resp, err := eng.Read(path, ReadParams{})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Chunk.Encoding != "base64" {
		t.Errorf("expected base64 encoding for binary file, got %q", resp.Chunk.Encoding)
	}
}

func TestReadIsDirectory(t *testing.T) {
	dir := t.TempDir()
	eng := newTestEngine(t, dir)

	_, err := eng.Read(dir, ReadParams{})
	if err == nil {
		t.Fatal("expected IS_DIRECTORY error")
	}
	ae, ok := err.(*protocol.AifrError)
	if !ok || ae.Code != protocol.ErrIsDirectory {
		t.Errorf("expected IS_DIRECTORY, got: %v", err)
	}
}

func TestListDir(t *testing.T) {
	dir := t.TempDir()
	mkTestFile(t, dir, "a.txt", "a")
	mkTestFile(t, dir, "b.txt", "b")
	os.Mkdir(filepath.Join(dir, "sub"), 0o755) //nolint:errcheck
	mkTestFile(t, dir, "sub/c.txt", "c")
	eng := newTestEngine(t, dir)

	// Depth 0: immediate children only.
	resp, err := eng.List(dir, ListParams{Depth: 0})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Total != 3 { // a.txt, b.txt, sub
		t.Errorf("total = %d, want 3", resp.Total)
	}

	// Depth -1: recursive.
	resp, err = eng.List(dir, ListParams{Depth: -1})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Total != 4 { // a.txt, b.txt, sub, sub/c.txt
		t.Errorf("total = %d, want 4", resp.Total)
	}
}

func TestListWithTypeFilter(t *testing.T) {
	dir := t.TempDir()
	mkTestFile(t, dir, "file.txt", "f")
	os.Mkdir(filepath.Join(dir, "subdir"), 0o755) //nolint:errcheck
	eng := newTestEngine(t, dir)

	// Files only.
	resp, err := eng.List(dir, ListParams{Type: "f"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Total != 1 {
		t.Errorf("file filter: total = %d, want 1", resp.Total)
	}

	// Dirs only.
	resp, err = eng.List(dir, ListParams{Type: "d"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Total != 1 {
		t.Errorf("dir filter: total = %d, want 1", resp.Total)
	}
}

func TestListWithPattern(t *testing.T) {
	dir := t.TempDir()
	mkTestFile(t, dir, "foo.go", "go")
	mkTestFile(t, dir, "foo.txt", "txt")
	mkTestFile(t, dir, "bar.go", "go")
	eng := newTestEngine(t, dir)

	resp, err := eng.List(dir, ListParams{Pattern: "*.go"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Total != 2 {
		t.Errorf("pattern filter: total = %d, want 2", resp.Total)
	}
}

func TestReadLargeFileWarning(t *testing.T) {
	dir := t.TempDir()
	// Create a file just over the large threshold.
	path := filepath.Join(dir, "big.txt")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	// Write lines until we exceed the threshold.
	line := strings.Repeat("x", 1000) + "\n"
	for written := int64(0); written < LargeFileThreshold+1; {
		n, _ := f.WriteString(line)
		written += int64(n)
	}
	f.Close()

	eng := newTestEngine(t, dir)
	resp, err := eng.Read(path, ReadParams{})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Warning != "file_large" {
		t.Errorf("expected warning=file_large, got %q", resp.Warning)
	}
}

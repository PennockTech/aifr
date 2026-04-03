// Copyright 2026 — see LICENSE file for terms.
package output

import (
	"strings"
	"testing"

	"go.pennock.tech/aifr/pkg/protocol"
)

// ── NumberLines ──

func TestNumberLines(t *testing.T) {
	t.Run("empty string", func(t *testing.T) {
		got := NumberLines("", 1)
		if got != "" {
			t.Errorf("NumberLines empty = %q, want %q", got, "")
		}
	})

	t.Run("single line no trailing newline", func(t *testing.T) {
		got := NumberLines("hello", 1)
		want := "     1\thello\n"
		if got != want {
			t.Errorf("NumberLines single = %q, want %q", got, want)
		}
	})

	t.Run("single line with trailing newline", func(t *testing.T) {
		got := NumberLines("hello\n", 1)
		want := "     1\thello\n"
		if got != want {
			t.Errorf("NumberLines single+nl = %q, want %q", got, want)
		}
	})

	t.Run("multiple lines", func(t *testing.T) {
		got := NumberLines("line1\nline2\nline3\n", 1)
		want := "     1\tline1\n     2\tline2\n     3\tline3\n"
		if got != want {
			t.Errorf("NumberLines multi = %q, want %q", got, want)
		}
	})

	t.Run("start at offset", func(t *testing.T) {
		got := NumberLines("first\nsecond\n", 10)
		want := "    10\tfirst\n    11\tsecond\n"
		if got != want {
			t.Errorf("NumberLines offset = %q, want %q", got, want)
		}
	})

	t.Run("large line numbers", func(t *testing.T) {
		got := NumberLines("a\n", 999999)
		want := "999999\ta\n"
		if got != want {
			t.Errorf("NumberLines large = %q, want %q", got, want)
		}
	})

	t.Run("empty lines preserved", func(t *testing.T) {
		got := NumberLines("a\n\nb\n", 1)
		want := "     1\ta\n     2\t\n     3\tb\n"
		if got != want {
			t.Errorf("NumberLines empty lines = %q, want %q", got, want)
		}
	})
}

// ── WriteReadText ──

func TestWriteReadText(t *testing.T) {
	t.Run("basic text output", func(t *testing.T) {
		resp := &protocol.ReadResponse{
			Chunk: &protocol.ChunkInfo{
				StartLine: 1,
				EndLine:   3,
				Data:      "line1\nline2\nline3\n",
				Encoding:  "utf-8",
			},
		}
		var buf strings.Builder
		WriteReadText(&buf, resp, false)
		want := "line1\nline2\nline3\n"
		if buf.String() != want {
			t.Errorf("WriteReadText basic = %q, want %q", buf.String(), want)
		}
	})

	t.Run("adds trailing newline", func(t *testing.T) {
		resp := &protocol.ReadResponse{
			Chunk: &protocol.ChunkInfo{
				Data:     "no newline at end",
				Encoding: "utf-8",
			},
		}
		var buf strings.Builder
		WriteReadText(&buf, resp, false)
		if !strings.HasSuffix(buf.String(), "\n") {
			t.Errorf("WriteReadText should add trailing newline, got %q", buf.String())
		}
	})

	t.Run("with line numbering from line 1", func(t *testing.T) {
		resp := &protocol.ReadResponse{
			Chunk: &protocol.ChunkInfo{
				StartLine: 1,
				EndLine:   2,
				Data:      "hello\nworld\n",
				Encoding:  "utf-8",
			},
		}
		var buf strings.Builder
		WriteReadText(&buf, resp, true)
		want := "     1\thello\n     2\tworld\n"
		if buf.String() != want {
			t.Errorf("WriteReadText numbered = %q, want %q", buf.String(), want)
		}
	})

	t.Run("with line numbering from offset", func(t *testing.T) {
		resp := &protocol.ReadResponse{
			Chunk: &protocol.ChunkInfo{
				StartLine: 50,
				EndLine:   52,
				Data:      "alpha\nbeta\ngamma\n",
				Encoding:  "utf-8",
			},
		}
		var buf strings.Builder
		WriteReadText(&buf, resp, true)
		want := "    50\talpha\n    51\tbeta\n    52\tgamma\n"
		if buf.String() != want {
			t.Errorf("WriteReadText numbered offset = %q, want %q", buf.String(), want)
		}
	})

	t.Run("numbering with StartLine zero defaults to 1", func(t *testing.T) {
		resp := &protocol.ReadResponse{
			Chunk: &protocol.ChunkInfo{
				StartLine: 0,
				Data:      "test\n",
				Encoding:  "utf-8",
			},
		}
		var buf strings.Builder
		WriteReadText(&buf, resp, true)
		want := "     1\ttest\n"
		if buf.String() != want {
			t.Errorf("WriteReadText zero start = %q, want %q", buf.String(), want)
		}
	})

	t.Run("nil chunk produces no output", func(t *testing.T) {
		resp := &protocol.ReadResponse{}
		var buf strings.Builder
		WriteReadText(&buf, resp, false)
		if buf.String() != "" {
			t.Errorf("WriteReadText nil chunk = %q, want empty", buf.String())
		}
	})

	t.Run("base64 chunk produces no output", func(t *testing.T) {
		resp := &protocol.ReadResponse{
			Chunk: &protocol.ChunkInfo{
				Data:     "SGVsbG8=",
				Encoding: "base64",
			},
		}
		var buf strings.Builder
		WriteReadText(&buf, resp, false)
		if buf.String() != "" {
			t.Errorf("WriteReadText base64 = %q, want empty", buf.String())
		}
	})
}

// ── WriteCatText ──

func newCatResponse(files ...protocol.CatEntry) *protocol.CatResponse {
	return &protocol.CatResponse{
		Source:     "filesystem",
		Mode:       "explicit",
		Files:      files,
		TotalFiles: len(files),
		FilesRead:  len(files),
		Complete:   true,
	}
}

func TestWriteCatTextPlain(t *testing.T) {
	t.Run("basic plain divider", func(t *testing.T) {
		resp := newCatResponse(protocol.CatEntry{
			Path:    "/tmp/foo.go",
			Content: "package main\n",
			Lines:   1,
			Size:    13,
		})
		var buf strings.Builder
		WriteCatText(&buf, resp, "plain", false)
		want := "--- /tmp/foo.go ---\npackage main\n"
		if buf.String() != want {
			t.Errorf("WriteCatText plain = %q, want %q", buf.String(), want)
		}
	})

	t.Run("plain with numbering", func(t *testing.T) {
		resp := newCatResponse(protocol.CatEntry{
			Path:    "/tmp/foo.go",
			Content: "line1\nline2\n",
			Lines:   2,
			Size:    12,
		})
		var buf strings.Builder
		WriteCatText(&buf, resp, "plain", true)
		want := "--- /tmp/foo.go ---\n     1\tline1\n     2\tline2\n"
		if buf.String() != want {
			t.Errorf("WriteCatText plain numbered = %q, want %q", buf.String(), want)
		}
	})

	t.Run("plain with error entry", func(t *testing.T) {
		resp := newCatResponse(protocol.CatEntry{
			Path:  "/tmp/secret",
			Error: "ACCESS_DENIED_SENSITIVE",
		})
		var buf strings.Builder
		WriteCatText(&buf, resp, "plain", false)
		want := "--- /tmp/secret (error: ACCESS_DENIED_SENSITIVE) ---\n"
		if buf.String() != want {
			t.Errorf("WriteCatText plain error = %q, want %q", buf.String(), want)
		}
	})

	t.Run("plain with binary entry", func(t *testing.T) {
		resp := newCatResponse(protocol.CatEntry{
			Path:   "/tmp/bin",
			Binary: true,
		})
		var buf strings.Builder
		WriteCatText(&buf, resp, "plain", false)
		want := "--- /tmp/bin (binary, skipped) ---\n"
		if buf.String() != want {
			t.Errorf("WriteCatText plain binary = %q, want %q", buf.String(), want)
		}
	})

	t.Run("plain numbering skips error and binary", func(t *testing.T) {
		resp := newCatResponse(
			protocol.CatEntry{Path: "/tmp/err", Error: "NOT_FOUND"},
			protocol.CatEntry{Path: "/tmp/bin", Binary: true},
			protocol.CatEntry{Path: "/tmp/ok", Content: "data\n", Lines: 1, Size: 5},
		)
		var buf strings.Builder
		WriteCatText(&buf, resp, "plain", true)
		got := buf.String()
		if !strings.Contains(got, "     1\tdata\n") {
			t.Errorf("expected numbered content, got %q", got)
		}
		if strings.Contains(got, "     1\tNOT_FOUND") {
			t.Error("error entry should not be numbered")
		}
	})
}

func TestWriteCatTextXML(t *testing.T) {
	t.Run("basic xml divider", func(t *testing.T) {
		resp := newCatResponse(protocol.CatEntry{
			Path:    "/tmp/foo.go",
			Content: "package main\n",
			Lines:   1,
			Size:    13,
		})
		var buf strings.Builder
		WriteCatText(&buf, resp, "xml", false)
		got := buf.String()
		if !strings.HasPrefix(got, `<file path="/tmp/foo.go">`) {
			t.Errorf("expected xml open tag, got %q", got)
		}
		if !strings.Contains(got, "package main\n") {
			t.Errorf("expected content, got %q", got)
		}
		if !strings.HasSuffix(got, "</file>\n") {
			t.Errorf("expected xml close tag, got %q", got)
		}
	})

	t.Run("xml with numbering", func(t *testing.T) {
		resp := newCatResponse(protocol.CatEntry{
			Path:    "/tmp/foo.go",
			Content: "a\nb\n",
			Lines:   2,
			Size:    4,
		})
		var buf strings.Builder
		WriteCatText(&buf, resp, "xml", true)
		got := buf.String()
		if !strings.Contains(got, "     1\ta\n") {
			t.Errorf("expected numbered line 1, got %q", got)
		}
		if !strings.Contains(got, "     2\tb\n") {
			t.Errorf("expected numbered line 2, got %q", got)
		}
	})

	t.Run("xml error entry", func(t *testing.T) {
		resp := newCatResponse(protocol.CatEntry{
			Path:  "/tmp/x",
			Error: "NOT_FOUND",
		})
		var buf strings.Builder
		WriteCatText(&buf, resp, "xml", false)
		got := buf.String()
		if !strings.Contains(got, `error="NOT_FOUND"`) {
			t.Errorf("expected error attribute, got %q", got)
		}
	})

	t.Run("xml binary entry", func(t *testing.T) {
		resp := newCatResponse(protocol.CatEntry{
			Path:   "/tmp/x",
			Binary: true,
		})
		var buf strings.Builder
		WriteCatText(&buf, resp, "xml", false)
		got := buf.String()
		if !strings.Contains(got, `binary="true"`) {
			t.Errorf("expected binary attribute, got %q", got)
		}
	})
}

func TestWriteCatTextNone(t *testing.T) {
	t.Run("none divider concatenates", func(t *testing.T) {
		resp := newCatResponse(
			protocol.CatEntry{Path: "/a", Content: "aaa\n", Lines: 1, Size: 4},
			protocol.CatEntry{Path: "/b", Content: "bbb\n", Lines: 1, Size: 4},
		)
		var buf strings.Builder
		WriteCatText(&buf, resp, "none", false)
		want := "aaa\nbbb\n"
		if buf.String() != want {
			t.Errorf("WriteCatText none = %q, want %q", buf.String(), want)
		}
	})

	t.Run("none with numbering", func(t *testing.T) {
		resp := newCatResponse(
			protocol.CatEntry{Path: "/a", Content: "x\ny\n", Lines: 2, Size: 4},
		)
		var buf strings.Builder
		WriteCatText(&buf, resp, "none", true)
		want := "     1\tx\n     2\ty\n"
		if buf.String() != want {
			t.Errorf("WriteCatText none numbered = %q, want %q", buf.String(), want)
		}
	})

	t.Run("none skips errors and binary silently", func(t *testing.T) {
		resp := newCatResponse(
			protocol.CatEntry{Path: "/err", Error: "NOT_FOUND"},
			protocol.CatEntry{Path: "/bin", Binary: true},
		)
		var buf strings.Builder
		WriteCatText(&buf, resp, "none", false)
		if buf.String() != "" {
			t.Errorf("WriteCatText none errors = %q, want empty", buf.String())
		}
	})
}

func TestWriteCatTextRelPath(t *testing.T) {
	t.Run("uses RelPath when available", func(t *testing.T) {
		resp := newCatResponse(protocol.CatEntry{
			Path:    "/long/absolute/path/file.go",
			RelPath: "file.go",
			Content: "data\n",
			Lines:   1,
			Size:    5,
		})
		var buf strings.Builder
		WriteCatText(&buf, resp, "plain", false)
		got := buf.String()
		if !strings.Contains(got, "--- file.go ---") {
			t.Errorf("expected RelPath in header, got %q", got)
		}
	})
}

func TestWriteCatTextTruncated(t *testing.T) {
	t.Run("plain truncation notice", func(t *testing.T) {
		resp := &protocol.CatResponse{
			Files:      []protocol.CatEntry{{Path: "/a", Content: "x\n", Lines: 1, Size: 2}},
			Truncated:  true,
			TotalBytes: 1024,
			Warning:    "max_total_size exceeded",
		}
		var buf strings.Builder
		WriteCatText(&buf, resp, "plain", false)
		got := buf.String()
		if !strings.Contains(got, "truncated") {
			t.Errorf("expected truncation notice, got %q", got)
		}
	})

	t.Run("xml truncation notice", func(t *testing.T) {
		resp := &protocol.CatResponse{
			Files:      []protocol.CatEntry{{Path: "/a", Content: "x\n", Lines: 1, Size: 2}},
			Truncated:  true,
			TotalBytes: 1024,
			Warning:    "max_total_size exceeded",
		}
		var buf strings.Builder
		WriteCatText(&buf, resp, "xml", false)
		got := buf.String()
		if !strings.Contains(got, "<truncated") {
			t.Errorf("expected xml truncation tag, got %q", got)
		}
	})

	t.Run("none mode no truncation notice", func(t *testing.T) {
		resp := &protocol.CatResponse{
			Files:     []protocol.CatEntry{{Path: "/a", Content: "x\n", Lines: 1, Size: 2}},
			Truncated: true,
			Warning:   "max_total_size exceeded",
		}
		var buf strings.Builder
		WriteCatText(&buf, resp, "none", false)
		got := buf.String()
		if strings.Contains(got, "truncated") {
			t.Errorf("none mode should suppress truncation notice, got %q", got)
		}
	})
}

// ── WriteCatText multi-file numbering ──

func TestWriteCatTextMultiFileNumbering(t *testing.T) {
	t.Run("each file starts numbering at 1", func(t *testing.T) {
		resp := newCatResponse(
			protocol.CatEntry{Path: "/a", Content: "aa\nbb\n", Lines: 2, Size: 6},
			protocol.CatEntry{Path: "/b", Content: "cc\ndd\nee\n", Lines: 3, Size: 9},
		)
		var buf strings.Builder
		WriteCatText(&buf, resp, "plain", true)
		got := buf.String()
		// File /a should have lines 1-2
		if !strings.Contains(got, "     1\taa\n") {
			t.Errorf("file /a should start at line 1, got %q", got)
		}
		if !strings.Contains(got, "     2\tbb\n") {
			t.Errorf("file /a should have line 2, got %q", got)
		}
		// File /b should restart at line 1
		parts := strings.SplitAfter(got, "--- /b ---\n")
		if len(parts) < 2 {
			t.Fatalf("expected /b header, got %q", got)
		}
		bContent := parts[1]
		if !strings.HasPrefix(bContent, "     1\tcc\n") {
			t.Errorf("file /b should restart at line 1, got %q", bContent)
		}
	})
}

// ── WriteSearchText ──

func TestWriteSearchText(t *testing.T) {
	t.Run("basic match", func(t *testing.T) {
		resp := &protocol.SearchResponse{
			Matches: []protocol.SearchMatch{
				{File: "main.go", Line: 10, Column: 5, Match: "func main()"},
			},
		}
		var buf strings.Builder
		WriteSearchText(&buf, resp)
		want := "main.go:10:5: func main()\n"
		if buf.String() != want {
			t.Errorf("WriteSearchText basic = %q, want %q", buf.String(), want)
		}
	})

	t.Run("with context lines", func(t *testing.T) {
		resp := &protocol.SearchResponse{
			Matches: []protocol.SearchMatch{
				{
					File:          "lib.go",
					Line:          20,
					Column:        1,
					Match:         "target line",
					ContextBefore: []string{"before1", "before2"},
					ContextAfter:  []string{"after1"},
				},
			},
		}
		var buf strings.Builder
		WriteSearchText(&buf, resp)
		got := buf.String()
		if !strings.Contains(got, "lib.go:20:1: target line") {
			t.Errorf("expected match line, got %q", got)
		}
		if !strings.Contains(got, "lib.go-18-") {
			t.Errorf("expected context before, got %q", got)
		}
		if !strings.Contains(got, "lib.go-21-") {
			t.Errorf("expected context after, got %q", got)
		}
	})

	t.Run("truncated output", func(t *testing.T) {
		resp := &protocol.SearchResponse{
			Matches:       []protocol.SearchMatch{{File: "a.go", Line: 1, Column: 1, Match: "x"}},
			Truncated:     true,
			TotalMatches:  500,
			FilesSearched: 100,
			FilesMatched:  10,
		}
		var buf strings.Builder
		WriteSearchText(&buf, resp)
		got := buf.String()
		if !strings.Contains(got, "truncated at 500 matches") {
			t.Errorf("expected truncation notice, got %q", got)
		}
	})

	t.Run("empty results", func(t *testing.T) {
		resp := &protocol.SearchResponse{}
		var buf strings.Builder
		WriteSearchText(&buf, resp)
		if buf.String() != "" {
			t.Errorf("WriteSearchText empty = %q, want empty", buf.String())
		}
	})
}

// ── WriteStatText ──

func TestWriteStatText(t *testing.T) {
	entry := &protocol.StatEntry{
		Path:    "/tmp/hello.txt",
		Type:    "file",
		Size:    42,
		Mode:    "-rw-r--r--",
		ModTime: "2026-01-01T00:00:00Z",
	}
	var buf strings.Builder
	WriteStatText(&buf, entry)
	got := buf.String()
	if !strings.Contains(got, "-rw-r--r--") {
		t.Errorf("expected mode in output, got %q", got)
	}
	if !strings.Contains(got, "42") {
		t.Errorf("expected size in output, got %q", got)
	}
	if !strings.Contains(got, "/tmp/hello.txt") {
		t.Errorf("expected path in output, got %q", got)
	}
}

// ── WriteListText ──

func TestWriteListText(t *testing.T) {
	resp := &protocol.ListResponse{
		Entries: []protocol.StatEntry{
			{Path: "/tmp/a", Type: "file", Mode: "-rw-r--r--", Size: 100},
			{Path: "/tmp/b", Type: "dir", Mode: "drwxr-xr-x", Size: 4096},
		},
	}
	var buf strings.Builder
	WriteListText(&buf, resp)
	got := buf.String()
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %q", len(lines), got)
	}
	if !strings.Contains(lines[0], "/tmp/a") {
		t.Errorf("expected /tmp/a in first line, got %q", lines[0])
	}
	if !strings.Contains(lines[1], "/tmp/b") {
		t.Errorf("expected /tmp/b in second line, got %q", lines[1])
	}
}

// ── WriteFindText ──

func TestWriteFindText(t *testing.T) {
	resp := &protocol.FindResponse{
		Entries: []protocol.FindEntry{
			{Path: "/a/b/c.go"},
			{Path: "/x/y.txt"},
		},
	}
	var buf strings.Builder
	WriteFindText(&buf, resp)
	want := "/a/b/c.go\n/x/y.txt\n"
	if buf.String() != want {
		t.Errorf("WriteFindText = %q, want %q", buf.String(), want)
	}
}

// ── WriteDiffText ──

func TestWriteDiffText(t *testing.T) {
	t.Run("identical files", func(t *testing.T) {
		resp := &protocol.DiffResponse{
			PathA:     "a.go",
			PathB:     "b.go",
			Identical: true,
		}
		var buf strings.Builder
		WriteDiffText(&buf, resp)
		got := buf.String()
		if !strings.Contains(got, "identical") {
			t.Errorf("expected identical message, got %q", got)
		}
	})

	t.Run("unified diff", func(t *testing.T) {
		resp := &protocol.DiffResponse{
			PathA: "a.go",
			PathB: "b.go",
			Hunks: []protocol.DiffHunk{
				{OldStart: 1, OldLines: 3, NewStart: 1, NewLines: 3, Lines: []string{" same", "-old", "+new"}},
			},
		}
		var buf strings.Builder
		WriteDiffText(&buf, resp)
		got := buf.String()
		if !strings.Contains(got, "--- a.go") {
			t.Errorf("expected --- header, got %q", got)
		}
		if !strings.Contains(got, "+++ b.go") {
			t.Errorf("expected +++ header, got %q", got)
		}
		if !strings.Contains(got, "@@ -1,3 +1,3 @@") {
			t.Errorf("expected hunk header, got %q", got)
		}
		if !strings.Contains(got, "-old") {
			t.Errorf("expected old line, got %q", got)
		}
		if !strings.Contains(got, "+new") {
			t.Errorf("expected new line, got %q", got)
		}
	})

	t.Run("byte diff", func(t *testing.T) {
		resp := &protocol.DiffResponse{
			PathA: "a.bin",
			PathB: "b.bin",
			ByteDiff: &protocol.ByteDiff{
				Offset: 42,
				Line:   3,
				Column: 10,
				SizeA:  100,
				SizeB:  200,
			},
		}
		var buf strings.Builder
		WriteDiffText(&buf, resp)
		got := buf.String()
		if !strings.Contains(got, "byte 42") {
			t.Errorf("expected byte offset, got %q", got)
		}
		if !strings.Contains(got, "sizes: 100 vs 200") {
			t.Errorf("expected sizes, got %q", got)
		}
	})
}

// ── WriteErrorText ──

func TestWriteErrorText(t *testing.T) {
	t.Run("with path", func(t *testing.T) {
		err := &protocol.AifrError{Code: "NOT_FOUND", Message: "file not found", Path: "/tmp/x"}
		var buf strings.Builder
		WriteErrorText(&buf, err)
		got := buf.String()
		if !strings.Contains(got, "NOT_FOUND") {
			t.Errorf("expected error code, got %q", got)
		}
		if !strings.Contains(got, "/tmp/x") {
			t.Errorf("expected path, got %q", got)
		}
	})

	t.Run("without path", func(t *testing.T) {
		err := &protocol.AifrError{Code: "ERROR", Message: "something failed"}
		var buf strings.Builder
		WriteErrorText(&buf, err)
		got := buf.String()
		if !strings.Contains(got, "ERROR") {
			t.Errorf("expected error code, got %q", got)
		}
		if strings.Contains(got, "(") {
			t.Errorf("should not contain path parens, got %q", got)
		}
	})
}

// ── numberContent helper ──

func TestNumberContent(t *testing.T) {
	t.Run("disabled returns original", func(t *testing.T) {
		got := numberContent("hello\n", false)
		if got != "hello\n" {
			t.Errorf("numberContent disabled = %q, want %q", got, "hello\n")
		}
	})

	t.Run("empty returns empty", func(t *testing.T) {
		got := numberContent("", true)
		if got != "" {
			t.Errorf("numberContent empty = %q, want empty", got)
		}
	})

	t.Run("enabled numbers from 1", func(t *testing.T) {
		got := numberContent("a\nb\n", true)
		want := "     1\ta\n     2\tb\n"
		if got != want {
			t.Errorf("numberContent enabled = %q, want %q", got, want)
		}
	})
}

// ── WriteLogText ──

func TestWriteLogText(t *testing.T) {
	t.Run("basic single commit with legacy FilesChanged", func(t *testing.T) {
		resp := &protocol.LogResponse{
			Entries: []protocol.LogEntry{
				{
					Hash:         "abc123def456abc123def456abc123def456abcdef",
					Author:       "Alice",
					AuthorEmail:  "alice@example.com",
					Date:         "2026-01-01T00:00:00Z",
					Message:      "initial commit",
					FilesChanged: []string{"README.md"},
				},
			},
			Complete: true,
		}
		var buf strings.Builder
		WriteLogText(&buf, resp)
		got := buf.String()
		if !strings.Contains(got, "commit abc123def456") {
			t.Errorf("expected 'commit' header with hash prefix, got %q", got)
		}
		if !strings.Contains(got, "Author: Alice <alice@example.com>") {
			t.Errorf("expected Author line, got %q", got)
		}
		if !strings.Contains(got, "Date:   2026-01-01T00:00:00Z") {
			t.Errorf("expected Date line, got %q", got)
		}
		if !strings.Contains(got, "    initial commit") {
			t.Errorf("expected indented message, got %q", got)
		}
		if !strings.Contains(got, "M README.md") {
			t.Errorf("expected changed file with M action, got %q", got)
		}
	})

	t.Run("changes field preferred over files_changed", func(t *testing.T) {
		resp := &protocol.LogResponse{
			Entries: []protocol.LogEntry{
				{
					Hash:        "abc123def456abc123def456abc123def456abcdef",
					Author:      "Bob",
					AuthorEmail: "bob@example.com",
					Date:        "2026-02-01T00:00:00Z",
					Message:     "add and delete",
					Changes: []protocol.FileChange{
						{Path: "new.go", Action: "A"},
						{Path: "old.go", Action: "D"},
					},
					FilesChanged: []string{"new.go", "old.go"},
				},
			},
			Complete: true,
		}
		var buf strings.Builder
		WriteLogText(&buf, resp)
		got := buf.String()
		if !strings.Contains(got, "A new.go") {
			t.Errorf("expected 'A new.go', got %q", got)
		}
		if !strings.Contains(got, "D old.go") {
			t.Errorf("expected 'D old.go', got %q", got)
		}
	})

	t.Run("multi-line message splits subject and body", func(t *testing.T) {
		resp := &protocol.LogResponse{
			Entries: []protocol.LogEntry{
				{
					Hash:        "abc123def456abc123def456abc123def456abcdef",
					Author:      "Carol",
					AuthorEmail: "carol@example.com",
					Date:        "2026-03-01T00:00:00Z",
					Message:     "feat: add logging\n\nThis adds structured logging\nto all HTTP handlers.",
				},
			},
			Complete: true,
		}
		var buf strings.Builder
		WriteLogText(&buf, resp)
		got := buf.String()
		if !strings.Contains(got, "    feat: add logging\n") {
			t.Errorf("expected indented subject, got %q", got)
		}
		if !strings.Contains(got, "    This adds structured logging\n") {
			t.Errorf("expected indented body line 1, got %q", got)
		}
		if !strings.Contains(got, "    to all HTTP handlers.\n") {
			t.Errorf("expected indented body line 2, got %q", got)
		}
	})

	t.Run("blank line between multiple commits", func(t *testing.T) {
		resp := &protocol.LogResponse{
			Entries: []protocol.LogEntry{
				{
					Hash:        "aaa111222333aaa111222333aaa111222333aaa111",
					Author:      "A",
					AuthorEmail: "a@x.com",
					Date:        "2026-01-01T00:00:00Z",
					Message:     "first",
				},
				{
					Hash:        "bbb444555666bbb444555666bbb444555666bbb444",
					Author:      "B",
					AuthorEmail: "b@x.com",
					Date:        "2026-01-02T00:00:00Z",
					Message:     "second",
				},
			},
			Complete: true,
		}
		var buf strings.Builder
		WriteLogText(&buf, resp)
		got := buf.String()
		if !strings.Contains(got, "\n\ncommit bbb444555666") {
			t.Errorf("expected blank line separator between commits, got %q", got)
		}
	})

	t.Run("continuation message when incomplete", func(t *testing.T) {
		resp := &protocol.LogResponse{
			Entries: []protocol.LogEntry{
				{
					Hash:        "abc123def456abc123def456abc123def456abcdef",
					Author:      "A",
					AuthorEmail: "a@x.com",
					Date:        "2026-01-01T00:00:00Z",
					Message:     "first",
				},
			},
			Total:        1,
			Complete:     false,
			Continuation: "tok123",
		}
		var buf strings.Builder
		WriteLogText(&buf, resp)
		got := buf.String()
		if !strings.Contains(got, "1 commits shown") {
			t.Errorf("expected continuation message, got %q", got)
		}
	})
}

// ── WriteLogText: CR in commit message ──

func TestWriteLogText_CarriageReturn(t *testing.T) {
	// Carriage returns in commit subjects must be made visible (sanitized
	// upstream in the engine), but the text formatter must not re-introduce
	// them or break structure even if sanitization is bypassed.
	resp := &protocol.LogResponse{
		Entries: []protocol.LogEntry{
			{
				Hash:        "abc123def456abc123def456abc123def456abcdef",
				Author:      "Mallory",
				AuthorEmail: "m@evil.com",
				Date:        "2026-01-01T00:00:00Z",
				Message:     "legit subject\\rmalicious overlay",
			},
		},
		Complete: true,
	}
	var buf strings.Builder
	WriteLogText(&buf, resp)
	got := buf.String()
	// The \\r should appear literally (already sanitized by engine).
	if !strings.Contains(got, `legit subject\rmalicious overlay`) {
		t.Errorf("expected sanitized CR in text output, got %q", got)
	}
	// Must not contain an actual carriage return byte.
	if strings.Contains(got, "\r") {
		t.Errorf("text output contains literal CR byte, got %q", got)
	}
}

// ── WriteLogOneline ──

func TestWriteLogOneline(t *testing.T) {
	t.Run("basic output", func(t *testing.T) {
		resp := &protocol.LogResponse{
			Entries: []protocol.LogEntry{
				{
					Hash:    "aaa111222333aaa111222333aaa111222333aaa111",
					Author:  "A",
					Date:    "2026-01-01T00:00:00Z",
					Message: "feat: add widgets\n\nDetailed body here.",
				},
				{
					Hash:    "bbb444555666bbb444555666bbb444555666bbb444",
					Author:  "B",
					Date:    "2026-01-02T00:00:00Z",
					Message: "fix: typo",
				},
			},
			Complete: true,
		}
		var buf strings.Builder
		WriteLogOneline(&buf, resp)
		got := buf.String()
		lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
		if len(lines) != 2 {
			t.Fatalf("expected 2 lines, got %d: %q", len(lines), got)
		}
		if lines[0] != "aaa111222333 feat: add widgets" {
			t.Errorf("line 0 = %q, want %q", lines[0], "aaa111222333 feat: add widgets")
		}
		if lines[1] != "bbb444555666 fix: typo" {
			t.Errorf("line 1 = %q, want %q", lines[1], "bbb444555666 fix: typo")
		}
	})

	t.Run("continuation notice", func(t *testing.T) {
		resp := &protocol.LogResponse{
			Entries: []protocol.LogEntry{
				{
					Hash:    "abc123def456abc123def456abc123def456abcdef",
					Message: "first",
				},
			},
			Total:        1,
			Complete:     false,
			Continuation: "tok",
		}
		var buf strings.Builder
		WriteLogOneline(&buf, resp)
		got := buf.String()
		if !strings.Contains(got, "1 commits shown") {
			t.Errorf("expected continuation message, got %q", got)
		}
	})

	t.Run("CR in subject is sanitized", func(t *testing.T) {
		resp := &protocol.LogResponse{
			Entries: []protocol.LogEntry{
				{
					Hash:    "abc123def456abc123def456abc123def456abcdef",
					Message: "legit\\roverlay",
				},
			},
			Complete: true,
		}
		var buf strings.Builder
		WriteLogOneline(&buf, resp)
		got := buf.String()
		if strings.Contains(got, "\r") {
			t.Errorf("oneline output contains literal CR byte, got %q", got)
		}
		if !strings.Contains(got, `legit\roverlay`) {
			t.Errorf("expected sanitized subject, got %q", got)
		}
	})
}

// ── WriteLogXML ──

func TestWriteLogXML(t *testing.T) {
	t.Run("basic XML structure", func(t *testing.T) {
		resp := &protocol.LogResponse{
			Ref: "HEAD",
			Entries: []protocol.LogEntry{
				{
					Hash:        "abc123def456abc123def456abc123def456abcdef",
					Author:      "Alice",
					AuthorEmail: "alice@example.com",
					Date:        "2026-01-01T00:00:00Z",
					Message:     "initial commit",
					Changes: []protocol.FileChange{
						{Path: "README.md", Action: "A"},
					},
				},
			},
			Total:    1,
			Complete: true,
		}
		var buf strings.Builder
		WriteLogXML(&buf, resp)
		got := buf.String()
		if !strings.Contains(got, `<log ref="HEAD"`) {
			t.Errorf("expected <log> root element, got %q", got)
		}
		if !strings.Contains(got, `<commit hash="abc123def456">`) {
			t.Errorf("expected <commit> element, got %q", got)
		}
		if !strings.Contains(got, `<author>Alice</author>`) {
			t.Errorf("expected <author>, got %q", got)
		}
		if !strings.Contains(got, `<subject>initial commit</subject>`) {
			t.Errorf("expected <subject>, got %q", got)
		}
		if !strings.Contains(got, `<file action="A">README.md</file>`) {
			t.Errorf("expected <file> with action, got %q", got)
		}
		if !strings.Contains(got, "</commit>") {
			t.Errorf("expected </commit>, got %q", got)
		}
		if !strings.Contains(got, "</log>") {
			t.Errorf("expected </log>, got %q", got)
		}
	})

	t.Run("XML injection in commit message", func(t *testing.T) {
		resp := &protocol.LogResponse{
			Ref: "HEAD",
			Entries: []protocol.LogEntry{
				{
					Hash:        "abc123def456abc123def456abc123def456abcdef",
					Author:      "Mallory",
					AuthorEmail: "m@evil.com",
					Date:        "2026-01-01T00:00:00Z",
					Message:     "</subject></commit><commit hash=\"evil\"><subject>pwned",
				},
			},
			Total:    1,
			Complete: true,
		}
		var buf strings.Builder
		WriteLogXML(&buf, resp)
		got := buf.String()
		// The malicious content must be escaped, not interpreted as XML.
		if strings.Contains(got, `<commit hash="evil">`) {
			t.Errorf("XML injection succeeded: found injected <commit> tag in %q", got)
		}
		if !strings.Contains(got, "&lt;/subject&gt;") {
			t.Errorf("expected escaped </subject> in output, got %q", got)
		}
		if !strings.Contains(got, "&lt;commit hash=&quot;evil&quot;&gt;") {
			t.Errorf("expected escaped injected commit tag, got %q", got)
		}
		// Must have exactly one <commit> open and one </commit> close.
		if strings.Count(got, "<commit ") != 1 {
			t.Errorf("expected exactly 1 <commit> tag, got %d in %q", strings.Count(got, "<commit "), got)
		}
	})

	t.Run("XML injection in author name", func(t *testing.T) {
		resp := &protocol.LogResponse{
			Ref: "HEAD",
			Entries: []protocol.LogEntry{
				{
					Hash:        "abc123def456abc123def456abc123def456abcdef",
					Author:      `</author><evil attr="x">`,
					AuthorEmail: "a@b.com",
					Date:        "2026-01-01T00:00:00Z",
					Message:     "innocent commit",
				},
			},
			Total:    1,
			Complete: true,
		}
		var buf strings.Builder
		WriteLogXML(&buf, resp)
		got := buf.String()
		if strings.Contains(got, "<evil") {
			t.Errorf("XML injection via author succeeded: %q", got)
		}
		if !strings.Contains(got, "&lt;/author&gt;") {
			t.Errorf("expected escaped </author> in output, got %q", got)
		}
	})

	t.Run("XML injection in file path", func(t *testing.T) {
		resp := &protocol.LogResponse{
			Ref: "HEAD",
			Entries: []protocol.LogEntry{
				{
					Hash:    "abc123def456abc123def456abc123def456abcdef",
					Author:  "A",
					Date:    "2026-01-01T00:00:00Z",
					Message: "add file",
					Changes: []protocol.FileChange{
						{Path: `</file></files></commit><commit hash="evil">`, Action: "A"},
					},
				},
			},
			Total:    1,
			Complete: true,
		}
		var buf strings.Builder
		WriteLogXML(&buf, resp)
		got := buf.String()
		if strings.Count(got, "<commit ") != 1 {
			t.Errorf("XML injection via file path: expected 1 <commit>, got %d in %q",
				strings.Count(got, "<commit "), got)
		}
	})

	t.Run("CR in subject is visible in XML", func(t *testing.T) {
		resp := &protocol.LogResponse{
			Ref: "HEAD",
			Entries: []protocol.LogEntry{
				{
					Hash:    "abc123def456abc123def456abc123def456abcdef",
					Author:  "A",
					Date:    "2026-01-01T00:00:00Z",
					Message: "legit\\roverlay",
				},
			},
			Total:    1,
			Complete: true,
		}
		var buf strings.Builder
		WriteLogXML(&buf, resp)
		got := buf.String()
		if strings.Contains(got, "\r") {
			t.Errorf("XML output contains literal CR byte, got %q", got)
		}
		if !strings.Contains(got, `legit\roverlay`) {
			t.Errorf("expected sanitized CR in XML, got %q", got)
		}
	})

	t.Run("multi-line message has subject and body tags", func(t *testing.T) {
		resp := &protocol.LogResponse{
			Ref: "HEAD",
			Entries: []protocol.LogEntry{
				{
					Hash:    "abc123def456abc123def456abc123def456abcdef",
					Author:  "A",
					Date:    "2026-01-01T00:00:00Z",
					Message: "feat: add thing\n\nThis is the body.\nWith multiple lines.",
				},
			},
			Total:    1,
			Complete: true,
		}
		var buf strings.Builder
		WriteLogXML(&buf, resp)
		got := buf.String()
		if !strings.Contains(got, "<subject>feat: add thing</subject>") {
			t.Errorf("expected subject tag, got %q", got)
		}
		if !strings.Contains(got, "<body>") {
			t.Errorf("expected body tag, got %q", got)
		}
		if !strings.Contains(got, "This is the body.") {
			t.Errorf("expected body content, got %q", got)
		}
	})
}

// ── xmlEscape ──

func TestXmlEscape(t *testing.T) {
	tests := []struct {
		name, input, want string
	}{
		{"no special chars", "hello world", "hello world"},
		{"ampersand", "a&b", "a&amp;b"},
		{"less than", "a<b", "a&lt;b"},
		{"greater than", "a>b", "a&gt;b"},
		{"double quote", `a"b`, "a&quot;b"},
		{"single quote", "a'b", "a&apos;b"},
		{"all specials", `<>&"'`, "&lt;&gt;&amp;&quot;&apos;"},
		{"empty", "", ""},
		{"mixed", `Author: <"Bob" & 'Alice'>`, "Author: &lt;&quot;Bob&quot; &amp; &apos;Alice&apos;&gt;"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := xmlEscape(tt.input)
			if got != tt.want {
				t.Errorf("xmlEscape(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ── WriteRefsText ──

func TestWriteRefsText(t *testing.T) {
	resp := &protocol.RefsResponse{
		Refs: []protocol.GitRef{
			{Name: "main", Type: "branch", Hash: "abc123def456abc123def456abc123def456abcdef"},
			{Name: "main", Type: "remote", Hash: "abc123def456abc123def456abc123def456abcdef", Remote: "origin"},
		},
	}
	var buf strings.Builder
	WriteRefsText(&buf, resp)
	got := buf.String()
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %q", len(lines), got)
	}
	if !strings.Contains(lines[0], "branch") || !strings.Contains(lines[0], "main") {
		t.Errorf("expected branch line, got %q", lines[0])
	}
	if !strings.Contains(lines[1], "origin/main") {
		t.Errorf("expected remote/name, got %q", lines[1])
	}
}

// ── WriteWcText ──

func intPtr(v int) *int       { return &v }
func int64Ptr(v int64) *int64 { return &v }

func TestWriteWcText(t *testing.T) {
	t.Run("single field bare number", func(t *testing.T) {
		resp := &protocol.WcResponse{
			Entries: []protocol.WcEntry{
				{Path: "/tmp/a.go", Lines: intPtr(42)},
			},
			Total:     protocol.WcEntry{Lines: intPtr(42)},
			FileCount: 1,
		}
		var buf strings.Builder
		WriteWcText(&buf, resp)
		got := buf.String()
		if got != "42 /tmp/a.go\n" {
			t.Errorf("single field single file = %q, want %q", got, "42 /tmp/a.go\n")
		}
	})

	t.Run("single field total only", func(t *testing.T) {
		resp := &protocol.WcResponse{
			Entries:   nil, // total_only mode
			Total:     protocol.WcEntry{Lines: intPtr(100)},
			FileCount: 3,
		}
		var buf strings.Builder
		WriteWcText(&buf, resp)
		got := buf.String()
		if got != "100\n" {
			t.Errorf("single field total only = %q, want %q", got, "100\n")
		}
	})

	t.Run("multiple fields key=value", func(t *testing.T) {
		resp := &protocol.WcResponse{
			Entries: []protocol.WcEntry{
				{Path: "/tmp/a.go", Lines: intPtr(10), Words: intPtr(50), Bytes: int64Ptr(200)},
			},
			Total:     protocol.WcEntry{Lines: intPtr(10), Words: intPtr(50), Bytes: int64Ptr(200)},
			FileCount: 1,
		}
		var buf strings.Builder
		WriteWcText(&buf, resp)
		got := buf.String()
		if !strings.Contains(got, "lines=10") {
			t.Errorf("expected lines=10, got %q", got)
		}
		if !strings.Contains(got, "words=50") {
			t.Errorf("expected words=50, got %q", got)
		}
		if !strings.Contains(got, "bytes=200") {
			t.Errorf("expected bytes=200, got %q", got)
		}
	})

	t.Run("multiple files shows total", func(t *testing.T) {
		resp := &protocol.WcResponse{
			Entries: []protocol.WcEntry{
				{Path: "/a", Lines: intPtr(5)},
				{Path: "/b", Lines: intPtr(10)},
			},
			Total:     protocol.WcEntry{Lines: intPtr(15)},
			FileCount: 2,
		}
		var buf strings.Builder
		WriteWcText(&buf, resp)
		got := buf.String()
		if !strings.Contains(got, "15 total") {
			t.Errorf("expected total line, got %q", got)
		}
	})

	t.Run("error entry", func(t *testing.T) {
		resp := &protocol.WcResponse{
			Entries: []protocol.WcEntry{
				{Path: "/bad", Error: "ACCESS_DENIED"},
			},
			Total:     protocol.WcEntry{Lines: intPtr(0)},
			FileCount: 1,
		}
		var buf strings.Builder
		WriteWcText(&buf, resp)
		got := buf.String()
		if !strings.Contains(got, "error") {
			t.Errorf("expected error, got %q", got)
		}
	})
}

// ── WritePathfindText ──

func TestWritePathfindText(t *testing.T) {
	resp := &protocol.PathfindResponse{
		Entries: []protocol.PathfindEntry{
			{Path: "/usr/bin/git", Masked: false},
			{Path: "/usr/local/bin/git", Masked: true, MaskedBy: "/usr/bin/git"},
		},
	}
	var buf strings.Builder
	WritePathfindText(&buf, resp)
	got := buf.String()
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	if strings.Contains(lines[0], "masked") {
		t.Errorf("first entry should not be masked, got %q", lines[0])
	}
	if !strings.Contains(lines[1], "masked by /usr/bin/git") {
		t.Errorf("second entry should be masked, got %q", lines[1])
	}
}

// ── WriteHexdumpText ──

func TestWriteHexdumpText(t *testing.T) {
	resp := &protocol.HexdumpResponse{
		Lines: []protocol.HexdumpLine{
			{Offset: 0, Hex: "48 65 6c 6c 6f", ASCII: "Hello"},
		},
	}
	var buf strings.Builder
	WriteHexdumpText(&buf, resp)
	got := buf.String()
	if !strings.Contains(got, "00000000") {
		t.Errorf("expected offset, got %q", got)
	}
	if !strings.Contains(got, "Hello") {
		t.Errorf("expected ASCII, got %q", got)
	}
}

// ── WriteChecksumText ──

func TestWriteChecksumText(t *testing.T) {
	resp := &protocol.ChecksumResponse{
		Entries: []protocol.ChecksumEntry{
			{Path: "/tmp/a.go", Checksum: "abc123"},
			{Path: "/tmp/bad", Error: "ACCESS_DENIED"},
		},
	}
	var buf strings.Builder
	WriteChecksumText(&buf, resp)
	got := buf.String()
	if !strings.Contains(got, "abc123  /tmp/a.go") {
		t.Errorf("expected checksum line, got %q", got)
	}
	if !strings.Contains(got, "error") {
		t.Errorf("expected error line, got %q", got)
	}
}

// ── WriteRevParseText ──

func TestWriteRevParseText(t *testing.T) {
	resp := &protocol.RevParseResponse{
		Hash:        "abc123def456abc123def456abc123def456abcdef",
		AuthorName:  "Alice",
		AuthorEmail: "alice@example.com",
		Date:        "2026-01-01",
		Subject:     "fix: something",
	}
	var buf strings.Builder
	WriteRevParseText(&buf, resp)
	got := buf.String()
	if !strings.Contains(got, "abc123") {
		t.Errorf("expected hash, got %q", got)
	}
	if !strings.Contains(got, "fix: something") {
		t.Errorf("expected subject, got %q", got)
	}
}

// ── WriteReflogText ──

func TestWriteReflogText(t *testing.T) {
	resp := &protocol.ReflogResponse{
		Entries: []protocol.ReflogEntry{
			{NewHash: "abc123def456abc123def456abc123def456abcdef", Author: "Alice", Date: "2026-01-01", Action: "commit: test"},
		},
	}
	var buf strings.Builder
	WriteReflogText(&buf, resp)
	got := buf.String()
	if !strings.Contains(got, "abc123def456") {
		t.Errorf("expected hash prefix, got %q", got)
	}
	if !strings.Contains(got, "commit: test") {
		t.Errorf("expected action, got %q", got)
	}
}

// ── WriteSysinfoText ──

func TestWriteSysinfoText(t *testing.T) {
	resp := &protocol.SysinfoResponse{
		OS:       &protocol.SysinfoOS{Kernel: "Linux", Release: "6.1.0", Arch: "amd64"},
		Date:     &protocol.SysinfoDate{UTC: "2026-01-01T00:00:00Z", Timezone: "UTC"},
		Hostname: "myhost",
	}
	var buf strings.Builder
	WriteSysinfoText(&buf, resp)
	got := buf.String()
	if !strings.Contains(got, "Linux") {
		t.Errorf("expected Linux, got %q", got)
	}
	if !strings.Contains(got, "myhost") {
		t.Errorf("expected hostname, got %q", got)
	}
}

// ── WriteGetentText ──

func TestWriteGetentText(t *testing.T) {
	resp := &protocol.GetentResponse{
		Database: "passwd",
		Fields:   []string{"name", "uid", "shell"},
		Entries: []protocol.GetentEntry{
			{Fields: map[string]string{"name": "root", "uid": "0", "shell": "/bin/bash"}},
		},
	}
	var buf strings.Builder
	WriteGetentText(&buf, resp)
	got := buf.String()
	if !strings.Contains(got, "name=root") {
		t.Errorf("expected name=root, got %q", got)
	}
	if !strings.Contains(got, "uid=0") {
		t.Errorf("expected uid=0, got %q", got)
	}
}

// ── WriteGitConfigText ──

func TestWriteGitConfigText(t *testing.T) {
	resp := &protocol.GitConfigResponse{
		Entries: []protocol.GitConfigEntry{
			{Key: "user.name", Value: "Alice"},
			{Key: "user.email", Value: "alice@example.com"},
		},
	}
	var buf strings.Builder
	WriteGitConfigText(&buf, resp)
	got := buf.String()
	if !strings.Contains(got, "user.name=Alice") {
		t.Errorf("expected user.name=Alice, got %q", got)
	}
	if !strings.Contains(got, "user.email=alice@example.com") {
		t.Errorf("expected user.email, got %q", got)
	}
}

// ── Regression: double line numbering ──

// TestNoDoubleNumbering verifies that WriteReadText with numberLines=true
// produces exactly one set of line number prefixes. This is a regression test
// for a bug where applyNumberLines (JSON-mode mutation) ran before the text
// formatter, which also applied numbering — producing doubled prefixes.
func TestNoDoubleNumbering(t *testing.T) {
	resp := &protocol.ReadResponse{
		Chunk: &protocol.ChunkInfo{
			StartLine: 10,
			EndLine:   12,
			Data:      "alpha\nbeta\ngamma\n",
			Encoding:  "utf-8",
		},
	}

	// Simulate the bug: apply NumberLines to the data (as applyNumberLines did),
	// then pass the mutated response to WriteReadText with numbering enabled.
	mutated := &protocol.ReadResponse{
		Chunk: &protocol.ChunkInfo{
			StartLine: resp.Chunk.StartLine,
			EndLine:   resp.Chunk.EndLine,
			Data:      NumberLines(resp.Chunk.Data, resp.Chunk.StartLine),
			Encoding:  "utf-8",
		},
	}
	var buggy strings.Builder
	WriteReadText(&buggy, mutated, true)

	// Correct: apply WriteReadText with numbering on the original data.
	var correct strings.Builder
	WriteReadText(&correct, resp, true)

	want := "    10\talpha\n    11\tbeta\n    12\tgamma\n"
	if correct.String() != want {
		t.Errorf("correct output = %q, want %q", correct.String(), want)
	}

	// The buggy path must NOT match the correct output (proves double numbering).
	if buggy.String() == correct.String() {
		t.Fatalf("expected double-numbered output to differ, but both are %q", correct.String())
	}

	// Verify the buggy output contains the tell-tale double prefix.
	if !strings.Contains(buggy.String(), "    10\t    10\t") {
		t.Errorf("expected double prefix in buggy output, got %q", buggy.String())
	}
}

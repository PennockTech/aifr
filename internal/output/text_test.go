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

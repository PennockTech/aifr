// Copyright 2026 — see LICENSE file for terms.
package engine

import (
	"bufio"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/bmatcuk/doublestar/v4"

	"go.pennock.tech/aifr/pkg/protocol"
)

const (
	// DefaultMaxTotalSize is the default cumulative output limit (2 MiB).
	DefaultMaxTotalSize = 2 * 1024 * 1024

	// DefaultMaxFiles is the default max files to process.
	DefaultMaxFiles = 1000
)

// CatParams controls the cat operation.
type CatParams struct {
	Name         string // glob on filename (discovery mode)
	ExcludePath  string // doublestar glob on relative path to exclude
	Type         string // "f", "d", "l" — default "f" in discovery mode
	MaxDepth     int    // -1 = unlimited
	Lines        int    // 0 = all, N = first N lines per file
	MaxTotalSize int64  // 0 = use default
	MaxFiles     int    // 0 = use default
}

// Cat concatenates contents of multiple files.
// If paths is non-empty, reads those files in order (explicit mode).
// If paths is empty and root is set, discovers files under root (discovery mode).
func (e *Engine) Cat(paths []string, root string, params CatParams) (*protocol.CatResponse, error) {
	maxTotal := params.MaxTotalSize
	if maxTotal <= 0 {
		maxTotal = DefaultMaxTotalSize
	}
	maxFiles := params.MaxFiles
	if maxFiles <= 0 {
		maxFiles = DefaultMaxFiles
	}

	resp := &protocol.CatResponse{
		Source: "filesystem",
	}

	if len(paths) > 0 {
		resp.Mode = "explicit"
		e.catReadFiles(paths, "", params, maxTotal, maxFiles, resp)
	} else if root != "" {
		resp.Mode = "discover"
		resolved, err := e.checkAccess(root)
		if err != nil {
			return nil, err
		}

		info, err := os.Stat(resolved)
		if err != nil {
			return nil, protocol.NewPathError(protocol.ErrNotFound, root, "path does not exist")
		}
		if !info.IsDir() {
			return nil, protocol.NewPathError(protocol.ErrIsDirectory, root, "cat discovery mode requires a directory")
		}

		resp.Root = resolved

		// Discover files.
		discovered := e.catDiscover(resolved, resolved, params, 0)

		// Sort for deterministic output.
		slices.Sort(discovered)

		e.catReadFiles(discovered, resolved, params, maxTotal, maxFiles, resp)
	} else {
		return nil, protocol.NewError("INVALID_ARGS", "cat requires either explicit paths or a root directory")
	}

	resp.TotalFiles = len(resp.Files)
	resp.Complete = !resp.Truncated
	return resp, nil
}

// catDiscover walks a directory tree collecting file paths matching filters.
func (e *Engine) catDiscover(root, dir string, params CatParams, depth int) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var result []string
	for _, de := range entries {
		fullPath := filepath.Join(dir, de.Name())

		if de.IsDir() {
			if params.MaxDepth >= 0 && depth >= params.MaxDepth {
				continue
			}
			// Recurse into subdirectories (even if name doesn't match — we're filtering files).
			sub := e.catDiscover(root, fullPath, params, depth+1)
			result = append(result, sub...)
			continue
		}

		// Type filter.
		entryType := entryTypeString(de)
		filterType := params.Type
		if filterType == "" {
			filterType = "f" // default to files only
		}
		if !matchesType(filterType, entryType) {
			continue
		}

		// Name glob filter.
		if params.Name != "" {
			matched, _ := doublestar.Match(params.Name, de.Name())
			if !matched {
				continue
			}
		}

		// Exclude-path filter.
		if params.ExcludePath != "" {
			relPath, _ := filepath.Rel(root, fullPath)
			matched, _ := doublestar.Match(params.ExcludePath, relPath)
			if matched {
				continue
			}
		}

		// Access control.
		if err := e.checker.Check(fullPath); err != nil {
			continue // silently skip inaccessible in discovery
		}

		result = append(result, fullPath)
	}
	return result
}

// catReadFiles reads a list of files and populates the response.
func (e *Engine) catReadFiles(paths []string, root string, params CatParams, maxTotal int64, maxFiles int, resp *protocol.CatResponse) {
	var totalBytes int64

	for _, path := range paths {
		if len(resp.Files) >= maxFiles {
			resp.Truncated = true
			resp.Warning = "max_files_limit"
			break
		}

		entry := protocol.CatEntry{Path: path}

		// Compute relative path if in discovery mode.
		if root != "" {
			rel, err := filepath.Rel(root, path)
			if err == nil {
				entry.RelPath = rel
			}
		}

		// Check access (for explicit mode; discovery already checked).
		resolved, err := e.checkAccess(path)
		if err != nil {
			if ae, ok := err.(*protocol.AifrError); ok {
				entry.Error = ae.Code
			} else {
				entry.Error = err.Error()
			}
			resp.Files = append(resp.Files, entry)
			resp.FilesSkipped++
			continue
		}

		info, err := os.Stat(resolved)
		if err != nil {
			entry.Error = "NOT_FOUND"
			resp.Files = append(resp.Files, entry)
			resp.FilesSkipped++
			continue
		}

		if info.IsDir() {
			entry.Error = "IS_DIRECTORY"
			resp.Files = append(resp.Files, entry)
			resp.FilesSkipped++
			continue
		}

		entry.Size = info.Size()

		// Check binary.
		if info.Size() > 0 {
			f, err := os.Open(resolved)
			if err != nil {
				entry.Error = err.Error()
				resp.Files = append(resp.Files, entry)
				resp.FilesSkipped++
				continue
			}

			peek := make([]byte, min(BinaryDetectSize, int(info.Size())))
			n, _ := f.Read(peek)
			f.Close()

			if isBinary(peek[:n]) {
				entry.Binary = true
				resp.Files = append(resp.Files, entry)
				resp.FilesSkipped++
				continue
			}
		}

		// Read content.
		var content string
		var lineCount int
		var truncated bool

		if params.Lines > 0 {
			content, lineCount, truncated, err = readFirstNLines(resolved, params.Lines)
		} else {
			content, lineCount, err = readFullFile(resolved)
		}

		if err != nil {
			entry.Error = err.Error()
			resp.Files = append(resp.Files, entry)
			resp.FilesSkipped++
			continue
		}

		// Check cumulative size limit.
		contentSize := int64(len(content))
		if totalBytes+contentSize > maxTotal {
			resp.Truncated = true
			resp.Warning = "output_size_limit"
			break
		}

		entry.Content = content
		entry.Lines = lineCount
		entry.Truncated = truncated
		totalBytes += contentSize

		resp.Files = append(resp.Files, entry)
		resp.FilesRead++
	}

	resp.TotalBytes = totalBytes
}

// readFirstNLines reads the first n lines from a file.
func readFirstNLines(path string, n int) (string, int, bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, false, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var lines []string
	count := 0
	for scanner.Scan() {
		count++
		if count <= n {
			lines = append(lines, scanner.Text())
		}
	}
	if err := scanner.Err(); err != nil {
		return "", 0, false, err
	}

	content := strings.Join(lines, "\n")
	if len(lines) > 0 {
		content += "\n"
	}

	truncated := count > n
	returnedLines := min(count, n)
	return content, returnedLines, truncated, nil
}

// readFullFile reads the entire content of a file.
func readFullFile(path string) (string, int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", 0, err
	}
	content := string(data)
	lineCount := strings.Count(content, "\n")
	if len(content) > 0 && content[len(content)-1] != '\n' {
		lineCount++ // last line without newline
	}
	return content, lineCount, nil
}

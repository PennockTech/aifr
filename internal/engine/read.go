// Copyright 2026 — see LICENSE file for terms.
package engine

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"io"
	"os"

	"go.pennock.tech/aifr/pkg/protocol"
)

// ReadParams controls how a file is read.
type ReadParams struct {
	Lines   *LineRange // nil = not specified
	Bytes   *ByteRange // nil = not specified
	ChunkID string     // continuation token; empty = not specified
}

// LineRange specifies a 1-indexed inclusive line range.
type LineRange struct {
	Start int // 1-indexed
	End   int // 0 means EOF
}

// ByteRange specifies a 0-indexed inclusive byte range.
type ByteRange struct {
	Start int64
	End   int64 // -1 means EOF
}

// Read returns file contents with optional chunking.
func (e *Engine) Read(path string, params ReadParams) (*protocol.ReadResponse, error) {
	// Handle continuation token.
	if params.ChunkID != "" {
		return e.readContinuation(params.ChunkID)
	}

	resolved, err := e.checkAccess(path)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(resolved)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, protocol.NewPathError(protocol.ErrNotFound, path, "path does not exist")
		}
		return nil, protocol.NewPathError(protocol.ErrNotFound, path, err.Error())
	}

	if info.IsDir() {
		return nil, protocol.NewPathError(protocol.ErrIsDirectory, path, "cannot read a directory")
	}

	resp := &protocol.ReadResponse{
		Path:      resolved,
		Source:    "filesystem",
		TotalSize: info.Size(),
	}

	// Large file warning.
	if info.Size() > LargeFileThreshold && params.Lines == nil && params.Bytes == nil {
		resp.Warning = "file_large"
	}

	// Dispatch by mode.
	switch {
	case params.Lines != nil:
		return e.readLines(resolved, info, resp, params.Lines)
	case params.Bytes != nil:
		return e.readBytes(resolved, info, resp, params.Bytes)
	default:
		return e.readDefault(resolved, info, resp)
	}
}

// readLines reads a line range from the file.
func (e *Engine) readLines(path string, info os.FileInfo, resp *protocol.ReadResponse, lr *LineRange) (*protocol.ReadResponse, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening %q: %w", path, err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024) // 1MB max line

	var lines []byte
	lineNum := 0
	startLine := lr.Start
	endLine := lr.End
	totalLines := 0
	startByte := int64(0)
	currentByte := int64(0)
	foundStart := false

	for scanner.Scan() {
		lineNum++
		totalLines = lineNum
		lineData := scanner.Bytes()
		lineLen := int64(len(lineData)) + 1 // +1 for newline

		if lineNum < startLine {
			currentByte += lineLen
			continue
		}

		if !foundStart {
			startByte = currentByte
			foundStart = true
		}

		if endLine > 0 && lineNum > endLine {
			break
		}

		if len(lines) > 0 {
			lines = append(lines, '\n')
		}
		lines = append(lines, lineData...)
		currentByte += lineLen
	}

	// Count remaining lines for total.
	for scanner.Scan() {
		totalLines++
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanning %q: %w", path, err)
	}

	actualEnd := lineNum
	if endLine > 0 && endLine < lineNum {
		actualEnd = endLine
	}

	resp.TotalLines = totalLines
	resp.Chunk = &protocol.ChunkInfo{
		StartByte: startByte,
		EndByte:   startByte + int64(len(lines)),
		StartLine: startLine,
		EndLine:   actualEnd,
		Data:      string(lines),
		Encoding:  "utf-8",
	}
	resp.Complete = endLine == 0 || actualEnd >= totalLines
	return resp, nil
}

// readBytes reads a byte range from the file with sane boundary adjustment.
func (e *Engine) readBytes(path string, info os.FileInfo, resp *protocol.ReadResponse, br *ByteRange) (*protocol.ReadResponse, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening %q: %w", path, err)
	}
	defer f.Close()

	start := br.Start
	end := br.End
	if end < 0 || end >= info.Size() {
		end = info.Size() - 1
	}

	if start > end || start >= info.Size() {
		return nil, protocol.NewPathError(protocol.ErrChunkOutOfRange, path,
			fmt.Sprintf("byte range %d:%d is out of bounds (file size: %d)", br.Start, br.End, info.Size()))
	}

	// Read the requested range.
	size := end - start + 1
	buf := make([]byte, size)
	n, err := f.ReadAt(buf, start)
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("reading %q: %w", path, err)
	}
	buf = buf[:n]

	// Detect binary.
	detectBuf := buf
	if len(detectBuf) > BinaryDetectSize {
		detectBuf = detectBuf[:BinaryDetectSize]
	}
	binary := isBinary(detectBuf)

	if binary {
		// Binary: align to 4 KiB boundary.
		resp.Chunk = &protocol.ChunkInfo{
			StartByte: start,
			EndByte:   start + int64(n) - 1,
			Data:      base64.StdEncoding.EncodeToString(buf),
			Encoding:  "base64",
		}
	} else {
		// Text: adjust to sane line boundary.
		buf, truncatedAt := adjustTextBoundary(f, buf, start, end, info.Size())
		resp.Chunk = &protocol.ChunkInfo{
			StartByte:   start,
			EndByte:     start + int64(len(buf)) - 1,
			Data:        string(buf),
			Encoding:    "utf-8",
			TruncatedAt: truncatedAt,
		}
	}

	// Count lines in chunk.
	resp.Chunk.StartLine, resp.Chunk.EndLine = countLinesInRange(resp.Chunk.Data)
	resp.Complete = resp.Chunk.EndByte >= info.Size()-1

	if !resp.Complete {
		tok, err := e.encodeContinuation(&continuationToken{
			Path:      path,
			ModTime:   info.ModTime().UnixNano(),
			Offset:    resp.Chunk.EndByte + 1,
			ChunkSize: int(size),
		})
		if err != nil {
			return nil, err
		}
		resp.Continuation = tok
	}

	return resp, nil
}

// readDefault reads the file with default chunking (first 64 KiB of complete lines).
func (e *Engine) readDefault(path string, info os.FileInfo, resp *protocol.ReadResponse) (*protocol.ReadResponse, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening %q: %w", path, err)
	}
	defer f.Close()

	// If file fits in one chunk, return it all.
	if info.Size() <= DefaultChunkSize {
		data, err := io.ReadAll(f)
		if err != nil {
			return nil, fmt.Errorf("reading %q: %w", path, err)
		}

		encoding := "utf-8"
		dataStr := string(data)
		if isBinary(data) {
			encoding = "base64"
			dataStr = base64.StdEncoding.EncodeToString(data)
		}

		startLine, endLine := countLinesInRange(string(data))
		resp.TotalLines = endLine
		resp.Chunk = &protocol.ChunkInfo{
			StartByte: 0,
			EndByte:   int64(len(data)) - 1,
			StartLine: startLine,
			EndLine:   endLine,
			Data:      dataStr,
			Encoding:  encoding,
		}
		resp.Complete = true
		return resp, nil
	}

	// Read first chunk.
	buf := make([]byte, DefaultChunkSize)
	n, err := f.Read(buf)
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("reading %q: %w", path, err)
	}
	buf = buf[:n]

	binary := isBinary(buf)

	if binary {
		resp.Chunk = &protocol.ChunkInfo{
			StartByte: 0,
			EndByte:   int64(n) - 1,
			Data:      base64.StdEncoding.EncodeToString(buf),
			Encoding:  "base64",
		}
	} else {
		// Truncate to last complete line.
		buf = truncateToLastNewline(buf)
		startLine, endLine := countLinesInRange(string(buf))
		resp.Chunk = &protocol.ChunkInfo{
			StartByte:   0,
			EndByte:     int64(len(buf)) - 1,
			StartLine:   startLine,
			EndLine:     endLine,
			Data:        string(buf),
			Encoding:    "utf-8",
			TruncatedAt: "newline",
		}
	}

	resp.Complete = false
	tok, err := e.encodeContinuation(&continuationToken{
		Path:      path,
		ModTime:   info.ModTime().UnixNano(),
		Offset:    int64(len(buf)),
		ChunkSize: DefaultChunkSize,
	})
	if err != nil {
		return nil, err
	}
	resp.Continuation = tok

	return resp, nil
}

// readContinuation resumes reading from a continuation token.
func (e *Engine) readContinuation(token string) (*protocol.ReadResponse, error) {
	tok, err := e.decodeContinuation(token)
	if err != nil {
		return nil, err
	}

	// Verify the file hasn't changed.
	info, err := os.Stat(tok.Path)
	if err != nil {
		return nil, protocol.NewPathError(protocol.ErrStaleContinuation, tok.Path, "file no longer exists")
	}

	if info.ModTime().UnixNano() != tok.ModTime {
		return nil, protocol.NewPathError(protocol.ErrStaleContinuation, tok.Path,
			"file has been modified since continuation token was issued")
	}

	// Re-check access (in case allow/deny changed, though unlikely in CLI mode).
	if err := e.checker.Check(tok.Path); err != nil {
		return nil, err
	}

	resp := &protocol.ReadResponse{
		Path:      tok.Path,
		Source:    "filesystem",
		TotalSize: info.Size(),
	}

	// Read from the stored offset.
	return e.readBytes(tok.Path, info, resp, &ByteRange{
		Start: tok.Offset,
		End:   tok.Offset + int64(tok.ChunkSize) - 1,
	})
}

// adjustTextBoundary adjusts the buffer to end at a newline boundary.
func adjustTextBoundary(f *os.File, buf []byte, start, end, fileSize int64) ([]byte, string) {
	if len(buf) == 0 {
		return buf, ""
	}

	// If the buffer already ends at a newline or EOF, we're done.
	if buf[len(buf)-1] == '\n' || start+int64(len(buf)) >= fileSize {
		return buf, "newline"
	}

	// Try to extend forward to the next newline, up to MaxByteOvershoot.
	extra := make([]byte, MaxByteOvershoot)
	n, _ := f.ReadAt(extra, end+1)
	extra = extra[:n]

	for i, b := range extra {
		if b == '\n' {
			return append(buf, extra[:i+1]...), "newline"
		}
	}

	// Overshoot would exceed limit. Shrink to last newline within range.
	for i := len(buf) - 1; i >= 0; i-- {
		if buf[i] == '\n' {
			return buf[:i+1], "newline"
		}
	}

	// No newline found at all; return as-is.
	return buf, "boundary"
}

// truncateToLastNewline truncates buf to the last newline.
func truncateToLastNewline(buf []byte) []byte {
	for i := len(buf) - 1; i >= 0; i-- {
		if buf[i] == '\n' {
			return buf[:i+1]
		}
	}
	return buf // no newline found
}

// countLinesInRange counts lines in data, returning (1, lineCount).
func countLinesInRange(data string) (int, int) {
	if len(data) == 0 {
		return 0, 0
	}
	count := 1
	for _, b := range data {
		if b == '\n' {
			count++
		}
	}
	// If data ends with newline, the last "line" is empty — don't count it.
	if len(data) > 0 && data[len(data)-1] == '\n' {
		count--
	}
	return 1, count
}

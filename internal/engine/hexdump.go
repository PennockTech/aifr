// Copyright 2026 — see LICENSE file for terms.
package engine

import (
	"fmt"
	"io"
	"os"
	"strings"

	"go.pennock.tech/aifr/internal/gitprovider"
	"go.pennock.tech/aifr/pkg/protocol"
)

const (
	hexdumpDefaultLength = 256
	hexdumpMaxLength     = 64 * 1024 // 64 KiB
	hexdumpBytesPerLine  = 16
)

// HexdumpParams controls offset and length for hex dump output.
type HexdumpParams struct {
	Offset int64 // starting byte offset (default 0)
	Length int64 // number of bytes to dump (0 = default 256, capped at 64 KiB)
}

// Hexdump produces a canonical hex dump of a file region.
func (e *Engine) Hexdump(path string, params HexdumpParams) (*protocol.HexdumpResponse, error) {
	if gitprovider.IsGitPath(path) {
		return e.gitHexdump(path, params)
	}
	return e.fsHexdump(path, params)
}

func (e *Engine) fsHexdump(path string, params HexdumpParams) (*protocol.HexdumpResponse, error) {
	resolved, err := e.checkAccess(path)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(resolved)
	if err != nil {
		return nil, err
	}
	totalSize := info.Size()

	length := clampLength(params.Length)
	if params.Offset >= totalSize {
		return &protocol.HexdumpResponse{
			Path:      resolved,
			Source:    "filesystem",
			TotalSize: totalSize,
			Offset:    params.Offset,
			Length:    0,
			Complete:  true,
		}, nil
	}

	f, err := os.Open(resolved)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	if params.Offset > 0 {
		if _, err := f.Seek(params.Offset, io.SeekStart); err != nil {
			return nil, err
		}
	}

	buf := make([]byte, length)
	n, err := io.ReadFull(f, buf)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return nil, err
	}
	buf = buf[:n]

	return &protocol.HexdumpResponse{
		Path:      resolved,
		Source:    "filesystem",
		TotalSize: totalSize,
		Offset:    params.Offset,
		Length:    int64(n),
		Lines:     formatHexLines(buf, params.Offset),
		Complete:  params.Offset+int64(n) >= totalSize,
	}, nil
}

func (e *Engine) gitHexdump(gitPath string, params HexdumpParams) (*protocol.HexdumpResponse, error) {
	gp, err := gitprovider.ParseGitPath(gitPath)
	if err != nil {
		return nil, err
	}

	repo, _, err := e.openGitRepo(gp.Repo)
	if err != nil {
		return nil, err
	}

	commit, err := e.gitProvider.ResolveRef(repo, gp.Ref)
	if err != nil {
		return nil, err
	}

	tree, err := commit.Tree()
	if err != nil {
		return nil, err
	}

	file, err := tree.File(gp.Path)
	if err != nil {
		return nil, err
	}

	content, err := file.Contents()
	if err != nil {
		return nil, err
	}
	data := []byte(content)
	totalSize := int64(len(data))

	length := clampLength(params.Length)
	if params.Offset >= totalSize {
		return &protocol.HexdumpResponse{
			Path:      gitPath,
			Source:    "git",
			TotalSize: totalSize,
			Offset:    params.Offset,
			Length:    0,
			Complete:  true,
		}, nil
	}

	end := min(params.Offset+length, totalSize)
	slice := data[params.Offset:end]

	return &protocol.HexdumpResponse{
		Path:      gitPath,
		Source:    "git",
		TotalSize: totalSize,
		Offset:    params.Offset,
		Length:    int64(len(slice)),
		Lines:     formatHexLines(slice, params.Offset),
		Complete:  end >= totalSize,
	}, nil
}

func clampLength(length int64) int64 {
	if length <= 0 {
		return hexdumpDefaultLength
	}
	return min(length, hexdumpMaxLength)
}

// formatHexLines produces canonical hex dump lines (like xxd or hexdump -C).
func formatHexLines(data []byte, baseOffset int64) []protocol.HexdumpLine {
	var lines []protocol.HexdumpLine

	for i := 0; i < len(data); i += hexdumpBytesPerLine {
		end := min(i+hexdumpBytesPerLine, len(data))
		chunk := data[i:end]

		// Hex: space-separated pairs, with extra space between bytes 8 and 9.
		var hexParts []string
		for j, b := range chunk {
			if j == 8 {
				hexParts = append(hexParts, "") // extra space
			}
			hexParts = append(hexParts, fmt.Sprintf("%02x", b))
		}
		hexStr := strings.Join(hexParts, " ")

		// ASCII: printable bytes shown, non-printable as '.'.
		var ascii strings.Builder
		for _, b := range chunk {
			if b >= 0x20 && b <= 0x7e {
				ascii.WriteByte(b)
			} else {
				ascii.WriteByte('.')
			}
		}

		lines = append(lines, protocol.HexdumpLine{
			Offset: baseOffset + int64(i),
			Hex:    hexStr,
			ASCII:  ascii.String(),
		})
	}

	return lines
}

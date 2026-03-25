// Copyright 2026 — see LICENSE file for terms.
package engine

import (
	"bufio"
	"os"
	"strings"
	"unicode/utf8"

	"go.pennock.tech/aifr/internal/gitprovider"
	"go.pennock.tech/aifr/pkg/protocol"
)

// WcParams controls which counts are returned.
type WcParams struct {
	Lines     bool
	Words     bool
	Bytes     bool
	Chars     bool // rune count
	TotalOnly bool // suppress per-file entries, return only combined total
}

// wcDefaults sets Lines+Words+Bytes when no flags are specified.
func (p *WcParams) wcDefaults() {
	if !p.Lines && !p.Words && !p.Bytes && !p.Chars {
		p.Lines = true
		p.Words = true
		p.Bytes = true
	}
}

// Wc counts lines, words, bytes, and/or characters for one or more paths.
func (e *Engine) Wc(paths []string, params WcParams) (*protocol.WcResponse, error) {
	params.wcDefaults()

	resp := &protocol.WcResponse{Complete: true}

	var totalLines, totalWords, totalChars int
	var totalBytes int64

	for _, path := range paths {
		var entry protocol.WcEntry
		if gitprovider.IsGitPath(path) {
			ent, err := e.gitWc(path, params)
			if err != nil {
				entry = protocol.WcEntry{Path: path, Source: "git", Error: err.Error()}
			} else {
				entry = *ent
			}
		} else {
			ent, err := e.fsWc(path, params)
			if err != nil {
				entry = protocol.WcEntry{Path: path, Source: "filesystem", Error: err.Error()}
			} else {
				entry = *ent
			}
		}

		if entry.Lines != nil {
			totalLines += *entry.Lines
		}
		if entry.Words != nil {
			totalWords += *entry.Words
		}
		if entry.Bytes != nil {
			totalBytes += *entry.Bytes
		}
		if entry.Chars != nil {
			totalChars += *entry.Chars
		}

		if !params.TotalOnly {
			resp.Entries = append(resp.Entries, entry)
		}
	}

	resp.Total = protocol.WcEntry{Path: "total", Source: "mixed"}
	if params.Lines {
		resp.Total.Lines = &totalLines
	}
	if params.Words {
		resp.Total.Words = &totalWords
	}
	if params.Bytes {
		resp.Total.Bytes = &totalBytes
	}
	if params.Chars {
		resp.Total.Chars = &totalChars
	}
	resp.FileCount = len(paths)

	return resp, nil
}

// fsWc counts for a filesystem path.
func (e *Engine) fsWc(path string, params WcParams) (*protocol.WcEntry, error) {
	resolved, err := e.checkAccess(path)
	if err != nil {
		return nil, err
	}

	entry := &protocol.WcEntry{Path: resolved, Source: "filesystem"}

	// Bytes-only optimization: stat without reading.
	if params.Bytes && !params.Lines && !params.Words && !params.Chars {
		info, err := os.Stat(resolved)
		if err != nil {
			return nil, err
		}
		sz := info.Size()
		entry.Bytes = &sz
		return entry, nil
	}

	f, err := os.Open(resolved)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var lines, words, chars int
	var bytes int64

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		lines++
		bytes += int64(len(line)) + 1 // +1 for newline
		if params.Words {
			words += len(strings.Fields(line))
		}
		if params.Chars {
			chars += utf8.RuneCountInString(line) + 1 // +1 for newline
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	if params.Lines {
		entry.Lines = &lines
	}
	if params.Words {
		entry.Words = &words
	}
	if params.Bytes {
		// Use stat for accurate byte count (handles files not ending in newline).
		info, err := os.Stat(resolved)
		if err == nil {
			sz := info.Size()
			entry.Bytes = &sz
		} else {
			entry.Bytes = &bytes
		}
	}
	if params.Chars {
		entry.Chars = &chars
	}

	return entry, nil
}

// gitWc counts for a git path.
func (e *Engine) gitWc(gitPath string, params WcParams) (*protocol.WcEntry, error) {
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

	entry := &protocol.WcEntry{Path: gitPath, Source: "git"}

	content, err := file.Contents()
	if err != nil {
		return nil, err
	}

	if params.Bytes {
		sz := int64(len(content))
		entry.Bytes = &sz
	}

	if params.Lines || params.Words || params.Chars {
		linesList := strings.Split(content, "\n")
		// strings.Split on trailing newline produces empty last element
		if len(linesList) > 0 && linesList[len(linesList)-1] == "" {
			linesList = linesList[:len(linesList)-1]
		}

		if params.Lines {
			n := len(linesList)
			entry.Lines = &n
		}
		if params.Words {
			w := len(strings.Fields(content))
			entry.Words = &w
		}
		if params.Chars {
			c := utf8.RuneCountInString(content)
			entry.Chars = &c
		}
	}

	return entry, nil
}

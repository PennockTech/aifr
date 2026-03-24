// Copyright 2026 — see LICENSE file for terms.
package engine

import (
	"os"

	"go.pennock.tech/aifr/pkg/protocol"
)

// Stat returns metadata for a file or directory.
func (e *Engine) Stat(path string) (*protocol.StatEntry, error) {
	resolved, err := e.checkAccess(path)
	if err != nil {
		return nil, err
	}

	info, err := os.Lstat(resolved)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, protocol.NewPathError(protocol.ErrNotFound, path, "path does not exist")
		}
		return nil, protocol.NewPathError(protocol.ErrNotFound, path, err.Error())
	}

	entryType := "file"
	switch {
	case info.IsDir():
		entryType = "dir"
	case info.Mode()&os.ModeSymlink != 0:
		entryType = "symlink"
	}

	return &protocol.StatEntry{
		Name:    info.Name(),
		Path:    resolved,
		Type:    entryType,
		Size:    info.Size(),
		Mode:    info.Mode().String(),
		ModTime: info.ModTime().UTC().Format("2006-01-02T15:04:05Z"),
	}, nil
}

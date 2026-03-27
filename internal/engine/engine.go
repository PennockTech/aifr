// Copyright 2026 — see LICENSE file for terms.
// Package engine implements the core read/stat/list/search/find operations for aifr.
package engine

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"go.pennock.tech/aifr/internal/accessctl"
	"go.pennock.tech/aifr/internal/config"
	"go.pennock.tech/aifr/internal/gitprovider"
	"go.pennock.tech/aifr/pkg/protocol"
)

const (
	// DefaultChunkSize is the default chunk size for file reads (64 KiB).
	DefaultChunkSize = 64 * 1024

	// LargeFileThreshold triggers a warning in read responses (10 MiB).
	LargeFileThreshold = 10 * 1024 * 1024

	// MaxByteOvershoot is the maximum bytes past the requested end for
	// text boundary alignment.
	MaxByteOvershoot = 1024

	// BinaryAlignBoundary is the alignment for binary file chunks.
	BinaryAlignBoundary = 4096

	// BinaryDetectSize is how many bytes to scan for NUL to detect binary.
	BinaryDetectSize = 8192
)

// Engine orchestrates all aifr operations.
type Engine struct {
	checker     *accessctl.Checker
	config      *config.Config
	gitProvider *gitprovider.Provider
	hmacKey     []byte // per-process HMAC key for continuation tokens
}

// NewEngine creates a new Engine.
func NewEngine(checker *accessctl.Checker, cfg *config.Config) (*Engine, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generating HMAC key: %w", err)
	}

	return &Engine{
		checker:     checker,
		config:      cfg,
		gitProvider: gitprovider.NewProvider(cfg.Git.Repos),
		hmacKey:     key,
	}, nil
}

// checkAccess verifies that the given path is accessible, returning the
// resolved absolute path.
func (e *Engine) checkAccess(path string) (string, error) {
	if err := e.checker.Check(path); err != nil {
		return "", err
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", protocol.NewPathError(protocol.ErrNotFound, path, "cannot resolve path")
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return "", protocol.NewPathError(protocol.ErrNotFound, path, "path does not exist")
		}
		return "", protocol.NewPathError(protocol.ErrNotFound, path, err.Error())
	}
	return resolved, nil
}

// continuationToken holds the state for resuming a chunked read.
type continuationToken struct {
	Path      string `json:"p"`
	ModTime   int64  `json:"m"` // Unix nanos
	Size      int64  `json:"z"` // file size at token creation
	Offset    int64  `json:"o"` // byte offset of next chunk
	ChunkSize int    `json:"s"` // chunk size hint
}

// encodeContinuation creates an HMAC-signed continuation token.
func (e *Engine) encodeContinuation(tok *continuationToken) (string, error) {
	data, err := json.Marshal(tok)
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, e.hmacKey)
	mac.Write(data)
	sig := mac.Sum(nil)

	// Combine: data + sig.
	combined := append(data, sig...)
	return base64.URLEncoding.EncodeToString(combined), nil
}

// decodeContinuation verifies and decodes a continuation token.
func (e *Engine) decodeContinuation(token string) (*continuationToken, error) {
	combined, err := base64.URLEncoding.DecodeString(token)
	if err != nil {
		return nil, protocol.NewError(protocol.ErrStaleContinuation, "invalid continuation token")
	}

	if len(combined) < sha256.Size {
		return nil, protocol.NewError(protocol.ErrStaleContinuation, "invalid continuation token")
	}

	data := combined[:len(combined)-sha256.Size]
	sig := combined[len(combined)-sha256.Size:]

	mac := hmac.New(sha256.New, e.hmacKey)
	mac.Write(data)
	expectedSig := mac.Sum(nil)

	if !hmac.Equal(sig, expectedSig) {
		return nil, protocol.NewError(protocol.ErrStaleContinuation, "continuation token signature mismatch")
	}

	var tok continuationToken
	if err := json.Unmarshal(data, &tok); err != nil {
		return nil, protocol.NewError(protocol.ErrStaleContinuation, "invalid continuation token data")
	}

	return &tok, nil
}

// ListContinuationToken holds the state for resuming a paginated list/find/search/log/reflog.
// Exported so that MCP handlers can inspect decoded tokens.
type ListContinuationToken struct {
	Tool   string `json:"t"`           // "list", "find", "search", "log", "reflog", "stash_list"
	Path   string `json:"p"`           // root path or repo name
	Offset int    `json:"o"`           // number of results already returned
	Limit  int    `json:"l"`           // page size
	Hash   string `json:"h,omitempty"` // for log: last commit hash
}

// EncodeListContinuation creates an HMAC-signed list continuation token.
func (e *Engine) EncodeListContinuation(tok *ListContinuationToken) (string, error) {
	data, err := json.Marshal(tok)
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, e.hmacKey)
	mac.Write(data)
	sig := mac.Sum(nil)

	combined := append(data, sig...)
	return base64.URLEncoding.EncodeToString(combined), nil
}

// DecodeListContinuation verifies and decodes a list continuation token.
func (e *Engine) DecodeListContinuation(token string) (*ListContinuationToken, error) {
	combined, err := base64.URLEncoding.DecodeString(token)
	if err != nil {
		return nil, protocol.NewError(protocol.ErrStaleContinuation, "invalid continuation token")
	}

	if len(combined) < sha256.Size {
		return nil, protocol.NewError(protocol.ErrStaleContinuation, "invalid continuation token")
	}

	data := combined[:len(combined)-sha256.Size]
	sig := combined[len(combined)-sha256.Size:]

	mac := hmac.New(sha256.New, e.hmacKey)
	mac.Write(data)
	expectedSig := mac.Sum(nil)

	if !hmac.Equal(sig, expectedSig) {
		return nil, protocol.NewError(protocol.ErrStaleContinuation, "continuation token signature mismatch")
	}

	var tok ListContinuationToken
	if err := json.Unmarshal(data, &tok); err != nil {
		return nil, protocol.NewError(protocol.ErrStaleContinuation, "invalid continuation token data")
	}

	return &tok, nil
}

// isBinary checks if data contains NUL bytes (binary file heuristic).
func isBinary(data []byte) bool {
	return slices.Contains(data, 0)
}

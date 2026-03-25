// Copyright 2026 — see LICENSE file for terms.
package engine

import (
	"crypto/md5"  //nolint:gosec // legacy comparison support
	"crypto/sha1" //nolint:gosec // legacy comparison support
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"os"
	"strings"

	"golang.org/x/crypto/sha3"

	"go.pennock.tech/aifr/internal/gitprovider"
	"go.pennock.tech/aifr/pkg/protocol"
)

// ChecksumParams controls checksum algorithm and output encoding.
type ChecksumParams struct {
	Algorithm string // "sha256" (default), "sha1", "sha512", "sha3-256", "sha3-512", "md5"
	Encoding  string // "hex" (default), "base64", "base64url"
}

var hashConstructors = map[string]func() hash.Hash{
	"md5":      md5.New,
	"sha1":     sha1.New,
	"sha256":   sha256.New,
	"sha512":   sha512.New,
	"sha3-256": sha3.New256,
	"sha3-512": sha3.New512,
}

func newHash(algorithm string) (hash.Hash, error) {
	fn, ok := hashConstructors[algorithm]
	if !ok {
		known := make([]string, 0, len(hashConstructors))
		for k := range hashConstructors {
			known = append(known, k)
		}
		return nil, fmt.Errorf("unknown algorithm %q (supported: %s)", algorithm, strings.Join(known, ", "))
	}
	return fn(), nil
}

func encodeHash(sum []byte, encoding string) (string, error) {
	switch encoding {
	case "hex", "":
		return hex.EncodeToString(sum), nil
	case "base64":
		return base64.StdEncoding.EncodeToString(sum), nil
	case "base64url":
		return base64.URLEncoding.EncodeToString(sum), nil
	default:
		return "", fmt.Errorf("unknown encoding %q (supported: hex, base64, base64url)", encoding)
	}
}

// Checksum computes checksums for one or more paths.
func (e *Engine) Checksum(paths []string, params ChecksumParams) (*protocol.ChecksumResponse, error) {
	if params.Algorithm == "" {
		params.Algorithm = "sha256"
	}
	if params.Encoding == "" {
		params.Encoding = "hex"
	}

	// Validate algorithm and encoding once before iterating.
	if _, err := newHash(params.Algorithm); err != nil {
		return nil, err
	}
	if _, err := encodeHash(nil, params.Encoding); err != nil {
		return nil, err
	}

	resp := &protocol.ChecksumResponse{
		Algorithm: params.Algorithm,
		Encoding:  params.Encoding,
		Complete:  true,
	}

	for _, path := range paths {
		var entry protocol.ChecksumEntry
		if gitprovider.IsGitPath(path) {
			ent, err := e.gitChecksum(path, params)
			if err != nil {
				entry = protocol.ChecksumEntry{Path: path, Source: "git", Error: err.Error()}
			} else {
				entry = *ent
			}
		} else {
			ent, err := e.fsChecksum(path, params)
			if err != nil {
				entry = protocol.ChecksumEntry{Path: path, Source: "filesystem", Error: err.Error()}
			} else {
				entry = *ent
			}
		}
		resp.Entries = append(resp.Entries, entry)
	}

	return resp, nil
}

func (e *Engine) fsChecksum(path string, params ChecksumParams) (*protocol.ChecksumEntry, error) {
	resolved, err := e.checkAccess(path)
	if err != nil {
		return nil, err
	}

	f, err := os.Open(resolved)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	h, _ := newHash(params.Algorithm) // already validated
	size, err := io.Copy(h, f)
	if err != nil {
		return nil, err
	}

	encoded, _ := encodeHash(h.Sum(nil), params.Encoding) // already validated

	return &protocol.ChecksumEntry{
		Path:     resolved,
		Source:   "filesystem",
		Checksum: encoded,
		Size:     size,
	}, nil
}

func (e *Engine) gitChecksum(gitPath string, params ChecksumParams) (*protocol.ChecksumEntry, error) {
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

	reader, err := file.Reader()
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	h, _ := newHash(params.Algorithm) // already validated
	size, err := io.Copy(h, reader)
	if err != nil {
		return nil, err
	}

	encoded, _ := encodeHash(h.Sum(nil), params.Encoding) // already validated

	return &protocol.ChecksumEntry{
		Path:     gitPath,
		Source:   "git",
		Checksum: encoded,
		Size:     size,
	}, nil
}

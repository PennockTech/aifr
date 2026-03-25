// Copyright 2026 — see LICENSE file for terms.
package main

import (
	"go.pennock.tech/aifr/internal/engine"

	"github.com/spf13/cobra"
)

var (
	checksumAlgorithm string
	checksumEncoding  string
)

var checksumCmd = &cobra.Command{
	Use:   "checksum <path>...",
	Short: "Compute file checksums",
	Long: `Compute cryptographic checksums for one or more files.
Supports filesystem paths and git refs.

Algorithms: sha256 (default), sha1, sha512, sha3-256, sha3-512, md5
Encodings: hex (default), base64, base64url

Examples:
  aifr checksum file.go
  aifr checksum -a sha512 *.go
  aifr checksum -a sha3-256 -e base64 HEAD:README.md`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		eng, err := buildEngine()
		if err != nil {
			exitWithError(err)
			return nil
		}

		resp, err := eng.Checksum(args, engine.ChecksumParams{
			Algorithm: checksumAlgorithm,
			Encoding:  checksumEncoding,
		})
		if err != nil {
			exitWithError(err)
			return nil
		}
		writeOutput(resp)
		return nil
	},
}

func init() {
	checksumCmd.Flags().StringVarP(&checksumAlgorithm, "algorithm", "a", "sha256", "hash algorithm (sha256, sha1, sha512, sha3-256, sha3-512, md5)")
	checksumCmd.Flags().StringVarP(&checksumEncoding, "encoding", "e", "hex", "output encoding (hex, base64, base64url)")
	rootCmd.AddCommand(checksumCmd)
}

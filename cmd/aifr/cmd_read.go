// Copyright 2026 — see LICENSE file for terms.
package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"go.pennock.tech/aifr/internal/engine"
	"go.pennock.tech/aifr/internal/gitprovider"
	"go.pennock.tech/aifr/pkg/protocol"
)

var (
	readLines   string
	readBytes   string
	readChunkID string
)

var readCmd = &cobra.Command{
	Use:   "read <path>",
	Short: "Read file contents (chunked)",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		eng, err := buildEngine()
		if err != nil {
			exitWithError(err)
			return nil
		}

		params := engine.ReadParams{
			ChunkID: readChunkID,
		}

		// Parse --lines flag.
		if readLines != "" {
			lr, err := parseLineRange(readLines)
			if err != nil {
				exitWithError(err)
				return nil
			}
			params.Lines = lr
		}

		// Parse --bytes flag.
		if readBytes != "" {
			br, err := parseByteRange(readBytes)
			if err != nil {
				exitWithError(err)
				return nil
			}
			params.Bytes = br
		}

		path := ""
		if len(args) > 0 {
			path = args[0]
		}

		var resp *protocol.ReadResponse
		if gitprovider.IsGitPath(path) {
			resp, err = eng.GitRead(path, params)
		} else {
			resp, err = eng.Read(path, params)
		}
		if err != nil {
			exitWithError(err)
			return nil
		}
		writeOutput(resp)
		return nil
	},
}

func init() {
	readCmd.Flags().StringVar(&readLines, "lines", "", "line range (e.g., 1:50, 50:)")
	readCmd.Flags().StringVar(&readBytes, "bytes", "", "byte range (e.g., 0:4095)")
	readCmd.Flags().StringVar(&readChunkID, "chunk-id", "", "continuation token")
	rootCmd.AddCommand(readCmd)
}

// parseLineRange parses "START:END" into a LineRange.
func parseLineRange(s string) (*engine.LineRange, error) {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid line range %q: expected START:END", s)
	}

	start, err := strconv.Atoi(parts[0])
	if err != nil {
		return nil, fmt.Errorf("invalid line range start %q: %w", parts[0], err)
	}

	end := 0 // 0 means EOF
	if parts[1] != "" {
		end, err = strconv.Atoi(parts[1])
		if err != nil {
			return nil, fmt.Errorf("invalid line range end %q: %w", parts[1], err)
		}
	}

	return &engine.LineRange{Start: start, End: end}, nil
}

// parseByteRange parses "START:END" into a ByteRange.
func parseByteRange(s string) (*engine.ByteRange, error) {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid byte range %q: expected START:END", s)
	}

	start, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid byte range start %q: %w", parts[0], err)
	}

	end := int64(-1) // -1 means EOF
	if parts[1] != "" {
		end, err = strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid byte range end %q: %w", parts[1], err)
		}
	}

	return &engine.ByteRange{Start: start, End: end}, nil
}

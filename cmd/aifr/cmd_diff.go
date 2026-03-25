// Copyright 2026 — see LICENSE file for terms.
package main

import (
	"go.pennock.tech/aifr/internal/engine"

	"github.com/spf13/cobra"
)

var diffCmp bool

var diffCmd = &cobra.Command{
	Use:   "diff <path-a> <path-b>",
	Short: "Compare files or git refs",
	Long: `Compare two files. Supports filesystem paths and git refs.

Use --cmp for byte-level comparison (like cmp): reports the first
differing byte position instead of a line-by-line diff.

Examples:
  aifr diff file1.go file2.go
  aifr diff --cmp file1.bin file2.bin
  aifr diff main:src/lib.go feature:src/lib.go
  aifr diff HEAD~1:README.md README.md`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		eng, err := buildEngine()
		if err != nil {
			exitWithError(err)
			return nil
		}

		resp, err := eng.Diff(args[0], args[1], engine.DiffParams{
			ByteLevel: diffCmp,
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
	diffCmd.Flags().BoolVar(&diffCmp, "cmp", false, "byte-level comparison (report first differing byte)")
	rootCmd.AddCommand(diffCmd)
}

// Copyright 2026 — see LICENSE file for terms.
package main

import (
	"github.com/spf13/cobra"
)

var diffCmd = &cobra.Command{
	Use:   "diff <path-a> <path-b>",
	Short: "Compare files or git refs",
	Long: `Compare two files. Supports filesystem paths and git refs.

Examples:
  aifr diff file1.go file2.go
  aifr diff main:src/lib.go feature:src/lib.go
  aifr diff HEAD~1:README.md README.md`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		eng, err := buildEngine()
		if err != nil {
			exitWithError(err)
			return nil
		}

		resp, err := eng.Diff(args[0], args[1])
		if err != nil {
			exitWithError(err)
			return nil
		}
		writeOutput(resp)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(diffCmd)
}

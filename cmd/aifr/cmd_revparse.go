// Copyright 2026 — see LICENSE file for terms.
package main

import (
	"github.com/spf13/cobra"
)

var revparseRepo string

var revparseCmd = &cobra.Command{
	Use:   "rev-parse <ref>",
	Short: "Resolve a git ref to a commit hash",
	Long: `Resolve a git ref (branch, tag, commit, HEAD~N) to its full commit hash
and metadata. Defaults to HEAD if no ref is given.

Examples:
  aifr rev-parse HEAD
  aifr rev-parse main
  aifr rev-parse --repo myrepo v2.0
  aifr rev-parse HEAD~3`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		eng, err := buildEngine()
		if err != nil {
			exitWithError(err)
			return nil
		}

		ref := ""
		if len(args) > 0 {
			ref = args[0]
		}

		resp, err := eng.RevParse(revparseRepo, ref)
		if err != nil {
			exitWithError(err)
			return nil
		}
		writeOutput(resp)
		return nil
	},
}

func init() {
	revparseCmd.Flags().StringVar(&revparseRepo, "repo", "", "named repo or filesystem path (default: auto-detect)")
	rootCmd.AddCommand(revparseCmd)
}

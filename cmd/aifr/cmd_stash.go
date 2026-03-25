// Copyright 2026 — see LICENSE file for terms.
package main

import (
	"go.pennock.tech/aifr/internal/engine"

	"github.com/spf13/cobra"
)

var stashMaxCount int

var stashCmd = &cobra.Command{
	Use:   "stash-list [repo]",
	Short: "List git stashes",
	Long: `List stashed changes from the stash reflog. Returns stash entries
with their hashes, authors, dates, and messages.

Examples:
  aifr stash-list
  aifr stash-list --max-count 5
  aifr stash-list /path/to/repo`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		eng, err := buildEngine()
		if err != nil {
			exitWithError(err)
			return nil
		}

		repoName := ""
		if len(args) > 0 {
			repoName = args[0]
		}

		resp, err := eng.StashList(repoName, engine.ReflogParams{
			MaxCount: stashMaxCount,
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
	stashCmd.Flags().IntVar(&stashMaxCount, "max-count", 50, "maximum stash entries to show")
	rootCmd.AddCommand(stashCmd)
}

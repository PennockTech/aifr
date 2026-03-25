// Copyright 2026 — see LICENSE file for terms.
package main

import (
	"github.com/spf13/cobra"
)

var (
	refsBranches bool
	refsTags     bool
	refsRemotes  bool
)

var refsCmd = &cobra.Command{
	Use:   "refs [repo]",
	Short: "List git refs (branches, tags, remotes)",
	Args:  cobra.MaximumNArgs(1),
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

		resp, err := eng.Refs(repoName, refsBranches, refsTags, refsRemotes)
		if err != nil {
			exitWithError(err)
			return nil
		}
		writeOutput(resp)
		return nil
	},
}

func init() {
	refsCmd.Flags().BoolVar(&refsBranches, "branches", false, "show branches")
	refsCmd.Flags().BoolVar(&refsTags, "tags", false, "show tags")
	refsCmd.Flags().BoolVar(&refsRemotes, "remotes", false, "show remote refs")
	rootCmd.AddCommand(refsCmd)
}

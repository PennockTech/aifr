// Copyright 2026 — see LICENSE file for terms.
package main

import (
	"github.com/spf13/cobra"

	"go.pennock.tech/aifr/internal/gitprovider"
)

var statCmd = &cobra.Command{
	Use:   "stat <path>",
	Short: "File/directory metadata",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		eng, err := buildEngine()
		if err != nil {
			exitWithError(err)
			return nil
		}
		path := args[0]
		if gitprovider.IsGitPath(path) {
			entry, err := eng.GitStat(path)
			if err != nil {
				exitWithError(err)
				return nil
			}
			writeOutput(entry)
		} else {
			entry, err := eng.Stat(path)
			if err != nil {
				exitWithError(err)
				return nil
			}
			writeOutput(entry)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(statCmd)
}

// Copyright 2026 — see LICENSE file for terms.
package main

import (
	"strings"

	"github.com/spf13/cobra"
)

var (
	logMaxCount int
)

var logCmd = &cobra.Command{
	Use:   "log [repo][:<ref>]",
	Short: "Git commit log",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		eng, err := buildEngine()
		if err != nil {
			exitWithError(err)
			return nil
		}

		repoName := ""
		ref := ""
		if len(args) > 0 {
			arg := args[0]
			// Parse repo:ref or just ref.
			if before, after, ok := strings.Cut(arg, ":"); ok {
				repoName = before
				ref = after
			} else {
				ref = arg
			}
		}

		resp, err := eng.Log(repoName, ref, logMaxCount)
		if err != nil {
			exitWithError(err)
			return nil
		}
		writeOutput(resp)
		return nil
	},
}

func init() {
	logCmd.Flags().IntVar(&logMaxCount, "max-count", 20, "maximum commits to show")
	rootCmd.AddCommand(logCmd)
}

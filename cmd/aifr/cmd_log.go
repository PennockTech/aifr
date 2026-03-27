// Copyright 2026 — see LICENSE file for terms.
package main

import (
	"strings"

	"github.com/spf13/cobra"

	"go.pennock.tech/aifr/internal/engine"
)

var (
	logMaxCount int
)

var logCmd = &cobra.Command{
	Use:   "log [repo|path][:<ref>]",
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
			// Filesystem paths (absolute or relative) are repo identifiers,
			// with an optional :ref suffix.
			if strings.HasPrefix(arg, "/") || strings.HasPrefix(arg, "./") || strings.HasPrefix(arg, "../") || arg == "." || arg == ".." {
				if before, after, ok := strings.Cut(arg, ":"); ok {
					repoName = before
					ref = after
				} else {
					repoName = arg
				}
			} else if before, after, ok := strings.Cut(arg, ":"); ok {
				// Parse repo:ref or just ref.
				repoName = before
				ref = after
			} else {
				ref = arg
			}
		}

		resp, err := eng.Log(repoName, ref, engine.LogParams{MaxCount: logMaxCount})
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

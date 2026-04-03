// Copyright 2026 — see LICENSE file for terms.
package main

import (
	"strings"

	"github.com/spf13/cobra"

	"go.pennock.tech/aifr/internal/engine"
)

var (
	logMaxCount int
	logSkip     int
	logOneline  bool
	logDivider  string
	logVerbose  bool
)

var logCmd = &cobra.Command{
	Use:   "log [repo|path][:<ref>]",
	Short: "Git commit log",
	Long: `Show git commit log with structured entries.

Output formats for --format text:
  default   git-log style with commit/Author/Date headers
  --oneline compact one-line-per-commit (hash + subject)

Divider formats for --format text (ignored with --oneline):
  plain   git-log style (default)
  xml     XML-tagged output with escaped content

Use --verbose to include tree hash, parent hashes, and committer
details (when they differ from the author) in JSON output.`,
	Args: cobra.MaximumNArgs(1),
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

		// --oneline implies text format.
		if logOneline {
			flagFormat = "oneline"
		}

		resp, err := eng.Log(repoName, ref, engine.LogParams{
			MaxCount: logMaxCount,
			Skip:     logSkip,
			Verbose:  logVerbose,
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
	logCmd.Flags().IntVar(&logMaxCount, "max-count", 20, "maximum commits to show")
	logCmd.Flags().IntVar(&logSkip, "skip", 0, "skip this many commits before showing results")
	logCmd.Flags().BoolVar(&logOneline, "oneline", false, "compact one-line-per-commit output")
	logCmd.Flags().StringVar(&logDivider, "divider", "plain", "divider format for text output: plain, xml")
	logCmd.Flags().BoolVar(&logVerbose, "verbose", false, "include tree hash, parent hashes, committer details")
	rootCmd.AddCommand(logCmd)
}

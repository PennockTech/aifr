// Copyright 2026 — see LICENSE file for terms.
package main

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"go.pennock.tech/aifr/internal/engine"
)

var (
	logMaxCount    int
	logSkip        int
	logOneline     bool
	logDivider     string
	logVerbose     bool
	logFirstParent bool
	logPath        string
	logSince       string
	logUntil       string
	logAuthor      string
	logGrep        string
)

var logCmd = &cobra.Command{
	Use:   "log [repo|path][:<revspec>]",
	Short: "Git commit log",
	Long: `Show git commit log with structured entries.

The <revspec> uses gitrevisions(7) syntax. In addition to single
revisions (HEAD, branches, tags, hashes, HEAD~3, HEAD^2, HEAD^{/regex},
…) the following range forms are accepted:

  v1..v2          commits reachable from v2 but not v1
  v1...v2         symmetric difference (each side excludes the other)
  ^rev            exclude rev (combine with positive tips, e.g. "HEAD ^v1")
  rev^!           just rev (excludes its parents)
  rev^@           all parents of rev
  rev^-[N]        equivalent to rev^N..rev (default N=1)

Output formats for --format text:
  default   git-log style with commit/Author/Date headers
  --oneline compact one-line-per-commit (hash + subject)

Divider formats for --format text (ignored with --oneline):
  plain   git-log style (default)
  xml     XML-tagged output with escaped content

Use --verbose to include tree hash, parent hashes, and committer
details (when they differ from the author) in JSON output.

Filters narrow the result set after the walk:
  --path GLOB    keep commits that touched a matching path (doublestar glob)
  --since DATE   keep commits with author date >= DATE (RFC3339 or YYYY-MM-DD)
  --until DATE   keep commits with author date <= DATE
  --author RE    keep commits whose "Name <email>" matches the regex
  --grep RE      keep commits whose message matches the regex
  --first-parent on merges, only follow the first parent`,
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

		params := engine.LogParams{
			MaxCount:    logMaxCount,
			Skip:        logSkip,
			Verbose:     logVerbose,
			FirstParent: logFirstParent,
			PathGlob:    logPath,
		}

		if logSince != "" {
			t, err := engine.ParseLogDate(logSince)
			if err != nil {
				exitWithError(fmt.Errorf("--since: %w", err))
				return nil
			}
			params.Since = &t
		}
		if logUntil != "" {
			t, err := engine.ParseLogDate(logUntil)
			if err != nil {
				exitWithError(fmt.Errorf("--until: %w", err))
				return nil
			}
			params.Until = &t
		}
		if logAuthor != "" {
			re, err := regexp.Compile(logAuthor)
			if err != nil {
				exitWithError(fmt.Errorf("--author: %w", err))
				return nil
			}
			params.Author = re
		}
		if logGrep != "" {
			re, err := regexp.Compile(logGrep)
			if err != nil {
				exitWithError(fmt.Errorf("--grep: %w", err))
				return nil
			}
			params.Grep = re
		}

		resp, err := eng.Log(repoName, ref, params)
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
	logCmd.Flags().BoolVar(&logFirstParent, "first-parent", false, "follow only the first parent of each commit")
	logCmd.Flags().StringVar(&logPath, "path", "", "keep commits touching paths matching this doublestar glob")
	logCmd.Flags().StringVar(&logSince, "since", "", "keep commits with author date >= this time (RFC3339 or YYYY-MM-DD)")
	logCmd.Flags().StringVar(&logUntil, "until", "", "keep commits with author date <= this time")
	logCmd.Flags().StringVar(&logAuthor, "author", "", "keep commits whose Name <email> matches this regex")
	logCmd.Flags().StringVar(&logGrep, "grep", "", "keep commits whose message matches this regex")
	rootCmd.AddCommand(logCmd)
}

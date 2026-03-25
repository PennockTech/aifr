// Copyright 2026 — see LICENSE file for terms.
package main

import (
	"github.com/spf13/cobra"

	"go.pennock.tech/aifr/internal/engine"
)

var (
	searchRegexp      bool
	searchFixedString bool
	searchIgnoreCase  bool
	searchContext     int
	searchMaxMatches  int
	searchInclude     string
	searchExclude     string
)

var searchCmd = &cobra.Command{
	Use:   "search <pattern> <path>",
	Short: "Content search (grep-like)",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		eng, err := buildEngine()
		if err != nil {
			exitWithError(err)
			return nil
		}

		isRegexp := searchRegexp
		if searchFixedString {
			isRegexp = false
		}

		resp, err := eng.Search(args[0], args[1], engine.SearchParams{
			IsRegexp:   isRegexp,
			IgnoreCase: searchIgnoreCase,
			Context:    searchContext,
			MaxMatches: searchMaxMatches,
			Include:    searchInclude,
			Exclude:    searchExclude,
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
	searchCmd.Flags().BoolVar(&searchRegexp, "regexp", true, "treat pattern as regexp (default)")
	searchCmd.Flags().BoolVar(&searchFixedString, "fixed-string", false, "treat pattern as fixed string")
	searchCmd.Flags().BoolVar(&searchIgnoreCase, "ignore-case", false, "case-insensitive matching")
	searchCmd.Flags().IntVar(&searchContext, "context", 0, "context lines before/after match")
	searchCmd.Flags().IntVar(&searchMaxMatches, "max-matches", 0, "max matches (0=default 500)")
	searchCmd.Flags().StringVar(&searchInclude, "include", "", "glob for files to include")
	searchCmd.Flags().StringVar(&searchExclude, "exclude", "", "glob for files to exclude")
	rootCmd.AddCommand(searchCmd)
}

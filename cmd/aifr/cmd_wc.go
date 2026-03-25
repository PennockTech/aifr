// Copyright 2026 — see LICENSE file for terms.
package main

import (
	"go.pennock.tech/aifr/internal/engine"

	"github.com/spf13/cobra"
)

var (
	wcLines     bool
	wcWords     bool
	wcBytes     bool
	wcChars     bool
	wcTotalOnly bool
)

var wcCmd = &cobra.Command{
	Use:   "wc <path>...",
	Short: "Count lines, words, and bytes",
	Long: `Count lines, words, bytes, and/or characters in one or more files.
Supports filesystem paths and git refs.

If no count flags are given, defaults to lines + words + bytes.

Examples:
  aifr wc file.go
  aifr wc -l *.go
  aifr wc --total-only -l src/*.go
  aifr wc HEAD:README.md`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		eng, err := buildEngine()
		if err != nil {
			exitWithError(err)
			return nil
		}

		resp, err := eng.Wc(args, engine.WcParams{
			Lines:     wcLines,
			Words:     wcWords,
			Bytes:     wcBytes,
			Chars:     wcChars,
			TotalOnly: wcTotalOnly,
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
	wcCmd.Flags().BoolVarP(&wcLines, "lines", "l", false, "count lines")
	wcCmd.Flags().BoolVarP(&wcWords, "words", "w", false, "count words")
	wcCmd.Flags().BoolVarP(&wcBytes, "bytes", "c", false, "count bytes")
	wcCmd.Flags().BoolVarP(&wcChars, "chars", "m", false, "count characters (runes)")
	wcCmd.Flags().BoolVar(&wcTotalOnly, "total-only", false, "only show combined total")
	rootCmd.AddCommand(wcCmd)
}

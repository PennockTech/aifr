// Copyright 2026 — see LICENSE file for terms.
package main

import (
	"go.pennock.tech/aifr/internal/engine"

	"github.com/spf13/cobra"
)

var reflogMaxCount int

var reflogCmd = &cobra.Command{
	Use:   "reflog [ref]",
	Short: "Show git reflog for a ref",
	Long: `Show the reflog for a git ref (default: HEAD). Lists recent ref
updates with timestamps and actions.

Examples:
  aifr reflog
  aifr reflog main
  aifr reflog --max-count 10 HEAD`,
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

		resp, err := eng.Reflog("", ref, engine.ReflogParams{
			MaxCount: reflogMaxCount,
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
	reflogCmd.Flags().IntVar(&reflogMaxCount, "max-count", 20, "maximum entries to show")
	rootCmd.AddCommand(reflogCmd)
}

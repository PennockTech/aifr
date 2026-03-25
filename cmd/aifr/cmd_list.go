// Copyright 2026 — see LICENSE file for terms.
package main

import (
	"github.com/spf13/cobra"

	"go.pennock.tech/aifr/internal/engine"
)

var (
	listDepth   int
	listPattern string
	listType    string
)

var listCmd = &cobra.Command{
	Use:   "list <path>",
	Short: "Directory listing",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		eng, err := buildEngine()
		if err != nil {
			exitWithError(err)
			return nil
		}

		resp, err := eng.List(args[0], engine.ListParams{
			Depth:   listDepth,
			Pattern: listPattern,
			Type:    listType,
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
	listCmd.Flags().IntVar(&listDepth, "depth", 0, "recursion depth (0=immediate, -1=unlimited)")
	listCmd.Flags().StringVar(&listPattern, "pattern", "", "glob filter on entry name")
	listCmd.Flags().StringVar(&listType, "type", "", "entry type filter (f=file, d=dir, l=symlink)")
	rootCmd.AddCommand(listCmd)
}

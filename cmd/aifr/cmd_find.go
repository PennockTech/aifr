// Copyright 2026 — see LICENSE file for terms.
package main

import (
	"time"

	"github.com/spf13/cobra"

	"go.pennock.tech/aifr/internal/engine"
)

var (
	findName      string
	findPath      string
	findType      string
	findMaxDepth  int
	findMinSize   int64
	findMaxSize   int64
	findNewerThan string
)

var findCmd = &cobra.Command{
	Use:   "find <path>",
	Short: "Find files by name/path pattern",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		eng, err := buildEngine()
		if err != nil {
			exitWithError(err)
			return nil
		}

		params := engine.FindParams{
			Name:     findName,
			Path:     findPath,
			Type:     findType,
			MaxDepth: findMaxDepth,
			MinSize:  findMinSize,
			MaxSize:  findMaxSize,
		}

		if findNewerThan != "" {
			d, err := time.ParseDuration(findNewerThan)
			if err != nil {
				exitWithError(err)
				return nil
			}
			params.NewerThan = d
		}

		resp, err := eng.Find(args[0], params)
		if err != nil {
			exitWithError(err)
			return nil
		}
		writeOutput(resp)
		return nil
	},
}

func init() {
	findCmd.Flags().StringVar(&findName, "name", "", "glob on filename")
	findCmd.Flags().StringVar(&findPath, "path", "", "glob on relative path")
	findCmd.Flags().StringVar(&findType, "type", "", "entry type (f=file, d=dir, l=symlink)")
	findCmd.Flags().IntVar(&findMaxDepth, "max-depth", -1, "max recursion depth (-1=unlimited)")
	findCmd.Flags().Int64Var(&findMinSize, "min-size", 0, "minimum file size in bytes")
	findCmd.Flags().Int64Var(&findMaxSize, "max-size", 0, "maximum file size in bytes")
	findCmd.Flags().StringVar(&findNewerThan, "newer-than", "", "duration (e.g., 24h, 7d)")
	rootCmd.AddCommand(findCmd)
}

// Copyright 2026 — see LICENSE file for terms.
package main

import (
	"os"

	"github.com/spf13/cobra"

	"go.pennock.tech/aifr/internal/engine"
	"go.pennock.tech/aifr/internal/output"
	"go.pennock.tech/aifr/pkg/protocol"
)

var (
	catName         string
	catExcludePath  string
	catType         string
	catMaxDepth     int
	catLines        int
	catDivider      string
	catMaxTotalSize int64
	catMaxFiles     int
)

var catCmd = &cobra.Command{
	Use:   "cat [flags] <path> [<path>...]",
	Short: "Concatenate contents of multiple files",
	Long: `Concatenate file contents with dividers between files. Two modes:

  Explicit:    aifr cat file1.go file2.go file3.go
  Discovery:   aifr cat --name '*.go' --exclude-path '**/vendor/**' ./src/

Discovery mode activates when --name or --exclude-path flags are set.
In discovery mode, exactly one positional arg (the root directory) is expected.

Divider formats for --format text:
  plain   --- path/to/file ---
  xml     <file path="path/to/file">content</file>
  none    raw concatenation`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		eng, err := buildEngine()
		if err != nil {
			exitWithError(err)
			return nil
		}

		params := engine.CatParams{
			Name:         catName,
			ExcludePath:  catExcludePath,
			Type:         catType,
			MaxDepth:     catMaxDepth,
			Lines:        catLines,
			MaxTotalSize: catMaxTotalSize,
			MaxFiles:     catMaxFiles,
		}

		// Determine mode.
		isDiscovery := catName != "" || catExcludePath != ""

		var resp *protocol.CatResponse
		if isDiscovery {
			resp, err = eng.Cat(nil, args[0], params)
		} else {
			resp, err = eng.Cat(args, "", params)
		}
		if err != nil {
			exitWithError(err)
			return nil
		}

		if flagFormat == "text" {
			output.WriteCatText(os.Stdout, resp, catDivider, flagNumberLines)
		} else {
			if flagNumberLines {
				applyNumberLines(resp)
			}
			writeJSON(resp)
		}
		return nil
	},
}

func init() {
	catCmd.Flags().StringVar(&catName, "name", "", "glob on filename (enables discovery mode)")
	catCmd.Flags().StringVar(&catExcludePath, "exclude-path", "", "doublestar glob to exclude by relative path")
	catCmd.Flags().StringVar(&catType, "type", "", "entry type filter (default: f in discovery)")
	catCmd.Flags().IntVar(&catMaxDepth, "max-depth", -1, "max recursion depth (-1=unlimited)")
	catCmd.Flags().IntVar(&catLines, "lines", 0, "max lines per file (0=all)")
	catCmd.Flags().StringVar(&catDivider, "divider", "plain", "divider format: plain, xml, none")
	catCmd.Flags().Int64Var(&catMaxTotalSize, "max-total-size", 0, "max total output bytes (default: 2MiB)")
	catCmd.Flags().IntVar(&catMaxFiles, "max-files", 0, "max files to read (default: 1000)")
	rootCmd.AddCommand(catCmd)
}

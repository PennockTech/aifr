// Copyright 2026 — see LICENSE file for terms.
package main

import (
	"github.com/spf13/cobra"

	"go.pennock.tech/aifr/internal/engine"
)

var pathfindSearchList string

var pathfindCmd = &cobra.Command{
	Use:   "pathfind <command>",
	Short: "Find commands in PATH-like search lists",
	Long: `Search for a command across a search list (default: $PATH).
Reports ALL matches with masking information — a better "which".

The command name may contain glob wildcards: * ? [
For example, "git-*" finds all git sub-commands.

Search list formats:
  envvar:PATH              $PATH directories (default)
  envvar:CLASSPATH         $CLASSPATH directories
  dirlist:/usr/bin:/usr/local/bin   explicit list`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		eng, err := buildEngine()
		if err != nil {
			exitWithError(err)
			return nil
		}

		resp, err := eng.Pathfind(args[0], engine.PathfindParams{
			SearchList: pathfindSearchList,
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
	pathfindCmd.Flags().StringVar(&pathfindSearchList, "search-list", "",
		`search list spec (default "envvar:PATH")`)
	rootCmd.AddCommand(pathfindCmd)
}

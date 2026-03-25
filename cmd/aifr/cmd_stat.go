// Copyright 2026 — see LICENSE file for terms.
package main

import (
	"github.com/spf13/cobra"
)

var statCmd = &cobra.Command{
	Use:   "stat <path>",
	Short: "File/directory metadata",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		eng, err := buildEngine()
		if err != nil {
			exitWithError(err)
			return nil
		}
		entry, err := eng.Stat(args[0])
		if err != nil {
			exitWithError(err)
			return nil
		}
		writeOutput(entry)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(statCmd)
}

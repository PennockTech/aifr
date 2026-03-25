// Copyright 2026 — see LICENSE file for terms.
package main

import (
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Show effective configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			exitWithError(err)
			return nil
		}
		writeOutput(cfg)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(configCmd)
}

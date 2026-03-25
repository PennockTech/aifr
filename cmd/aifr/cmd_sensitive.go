// Copyright 2026 — see LICENSE file for terms.
package main

import (
	"github.com/spf13/cobra"

	"go.pennock.tech/aifr/internal/accessctl"
)

var sensitiveCmd = &cobra.Command{
	Use:   "sensitive",
	Short: "List built-in sensitive file patterns (for auditing)",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			exitWithError(err)
			return nil
		}

		checker, err := accessctl.NewChecker(accessctl.CheckerParams{
			Allow:     cfg.Allow,
			Deny:      cfg.Deny,
			CredsDeny: cfg.CredsDeny,
		})
		if err != nil {
			exitWithError(err)
			return nil
		}

		patterns := checker.SensitivePatterns()
		writeOutput(map[string]any{
			"patterns": patterns,
			"count":    len(patterns),
		})
		return nil
	},
}

func init() {
	rootCmd.AddCommand(sensitiveCmd)
}

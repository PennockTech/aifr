// Copyright 2026 — see LICENSE file for terms.
package main

import (
	"os"

	"github.com/spf13/cobra"

	"go.pennock.tech/aifr/internal/mcpserver"
)

var skillCmd = &cobra.Command{
	Use:   "skill",
	Short: "Emit SKILL.md to stdout",
	Long:  "Outputs a skill file suitable for ~/.claude/skills/ or .claude/skills/",
	RunE: func(cmd *cobra.Command, args []string) error {
		return mcpserver.EmitSkill(os.Stdout)
	},
}

func init() {
	rootCmd.AddCommand(skillCmd)
}

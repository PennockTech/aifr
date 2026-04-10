// Copyright 2026 — see LICENSE file for terms.
package main

import "github.com/spf13/cobra"

var hookCmd = &cobra.Command{
	Use:   "hook",
	Short: "Hooks for AI coding agent integration",
	Long: `Commands designed for use as hooks in AI coding agents such as Claude Code.

These sub-commands read hook payloads from stdin and write hook responses
to stdout, following the agent's hook protocol.

Example Claude Code configuration:

  {
    "hooks": {
      "PreToolUse": [
        {
          "matcher": "Bash",
          "hooks": [
            {
              "type": "command",
              "command": "aifr hook check-command"
            }
          ]
        }
      ]
    }
  }`,
}

func init() {
	rootCmd.AddCommand(hookCmd)
}

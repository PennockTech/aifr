// Copyright 2026 — see LICENSE file for terms.
package main

import "github.com/spf13/cobra"

var hookCmd = &cobra.Command{
	Use:   "hook",
	Short: "Hooks for AI coding agent integration",
	Long: `Commands designed for use as hooks in AI coding agents such as Claude Code.

These sub-commands read hook payloads from stdin and write hook responses
to stdout, following the agent's hook protocol. The check-command sub-command
auto-detects whether it was invoked from a PreToolUse or PermissionRequest
hook and emits the matching response shape.

Recommended configuration — wire check-command into PermissionRequest so it
only fires when a Bash call would otherwise prompt the user; calls that are
already permitted run untouched:

  {
    "hooks": {
      "PermissionRequest": [
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
  }

Alternative — use PreToolUse instead if you want every Bash call (including
already-permitted ones) routed through aifr when it can handle them:

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

// Copyright 2026 — see LICENSE file for terms.
package main

import (
	"encoding/json"
	"io"
	"os"

	"github.com/spf13/cobra"

	"go.pennock.tech/aifr/internal/hookcmd"
)

var checkCommandMCP bool

var checkCommandCmd = &cobra.Command{
	Use:   "check-command",
	Short: "Suggest aifr alternatives for Bash tool calls",
	Long: `Reads a Claude Code hook payload from stdin, analyzes the shell
command, and if aifr can handle it, outputs a hook response denying the
Bash call and suggesting the aifr alternative.

Supports both PreToolUse and PermissionRequest hooks: the response shape
is selected automatically from the payload's hook_event_name field, so
the same binary can be wired into either event without configuration.

If the command is not something aifr handles, exits silently (exit 0,
no output) so the Bash call continues through normal permission evaluation.

Pipelines ending in | head -n N or | tail -n N are recognized and mapped
to the appropriate aifr limit parameter (--max-count, --limit, --lines, etc.).

When --mcp is set, or when an aifr MCP server is detected in .mcp.json,
suggestions reference MCP tool calls instead of CLI sub-commands.

Recognized commands: cat, head, tail, grep/rg, find, ls, wc, stat,
diff, sed -n, sha256sum/md5sum, hexdump/xxd, git log, git diff.

Recommended Claude Code settings — wire into PermissionRequest so the
hook only fires when a Bash call would otherwise prompt the user, and
already-permitted commands run without interception:

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

Use PreToolUse instead if you want every Bash call routed through aifr
when it can handle the command, even calls that are already permitted:

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
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		input, err := io.ReadAll(os.Stdin)
		if err != nil {
			return err
		}

		result, err := hookcmd.CheckCommand(input, checkCommandMCP)
		if err != nil {
			return err
		}
		if result == nil {
			return nil
		}

		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	},
}

func init() {
	checkCommandCmd.Flags().BoolVar(&checkCommandMCP, "mcp", false,
		"suggest MCP tool calls (auto-detected from .mcp.json and $AIFR_MCP if not set)")
	hookCmd.AddCommand(checkCommandCmd)
}

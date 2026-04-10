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
	Long: `Reads a Claude Code PreToolUse hook payload from stdin, analyzes the
shell command, and if aifr can handle it, outputs a hook response denying
the Bash call and suggesting the aifr alternative.

If the command is not something aifr handles, exits silently (exit 0,
no output) so the Bash call continues through normal permission evaluation.

Pipelines ending in | head -n N or | tail -n N are recognized and mapped
to the appropriate aifr limit parameter (--max-count, --limit, --lines, etc.).

When --mcp is set, or when an aifr MCP server is detected in .mcp.json,
suggestions reference MCP tool calls instead of CLI sub-commands.

Recognized commands: cat, head, tail, grep/rg, find, ls, wc, stat,
diff, sed -n, sha256sum/md5sum, hexdump/xxd, git log, git diff.

Usage in Claude Code settings:

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

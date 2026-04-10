// Copyright 2026 — see LICENSE file for terms.
package main

import (
	"encoding/json"
	"io"
	"os"

	"github.com/spf13/cobra"

	"go.pennock.tech/aifr/internal/hookcmd"
)

var checkCommandCmd = &cobra.Command{
	Use:   "check-command",
	Short: "Suggest aifr alternatives for Bash tool calls",
	Long: `Reads a Claude Code PreToolUse hook payload from stdin, analyzes the
shell command, and if aifr can handle it, outputs a hook response denying
the Bash call and suggesting the aifr alternative.

If the command is not something aifr handles, exits silently (exit 0,
no output) so the Bash call proceeds normally.

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

		result, err := hookcmd.CheckCommand(input)
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
	hookCmd.AddCommand(checkCommandCmd)
}

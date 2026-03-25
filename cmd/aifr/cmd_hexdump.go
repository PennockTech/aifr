// Copyright 2026 — see LICENSE file for terms.
package main

import (
	"go.pennock.tech/aifr/internal/engine"

	"github.com/spf13/cobra"
)

var (
	hexdumpOffset int64
	hexdumpLength int64
)

var hexdumpCmd = &cobra.Command{
	Use:   "hexdump <path>",
	Short: "Hex dump of file contents",
	Long: `Display a canonical hex dump of a file region.
Supports filesystem paths and git refs.

Output format matches xxd/hexdump -C: offset, hex bytes, ASCII.
Default: 256 bytes from offset 0. Maximum: 64 KiB per call.

Examples:
  aifr hexdump binary.dat
  aifr hexdump -s 1024 -l 512 binary.dat
  aifr hexdump HEAD:binary.dat`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		eng, err := buildEngine()
		if err != nil {
			exitWithError(err)
			return nil
		}

		resp, err := eng.Hexdump(args[0], engine.HexdumpParams{
			Offset: hexdumpOffset,
			Length: hexdumpLength,
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
	hexdumpCmd.Flags().Int64VarP(&hexdumpOffset, "offset", "s", 0, "starting byte offset")
	hexdumpCmd.Flags().Int64VarP(&hexdumpLength, "length", "l", 0, "bytes to dump (default 256, max 65536)")
	rootCmd.AddCommand(hexdumpCmd)
}

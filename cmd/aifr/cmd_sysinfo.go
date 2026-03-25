// Copyright 2026 — see LICENSE file for terms.
package main

import (
	"strings"

	"go.pennock.tech/aifr/internal/engine"

	"github.com/spf13/cobra"
)

var sysinfoSections string

var sysinfoCmd = &cobra.Command{
	Use:   "sysinfo",
	Short: "System inspection for fault diagnosis",
	Long: `Gather system information: OS, date, hostname, uptime, network
interfaces, routing table. No files are written, no commands are executed.

Sections: os, date, hostname, uptime, network, routing (default: all)

Examples:
  aifr sysinfo
  aifr sysinfo --sections date
  aifr sysinfo --sections os,date,hostname`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		eng, err := buildEngine()
		if err != nil {
			exitWithError(err)
			return nil
		}

		var sections []string
		if sysinfoSections != "" {
			sections = strings.Split(sysinfoSections, ",")
		}

		resp, err := eng.Sysinfo(engine.SysinfoParams{
			Sections: sections,
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
	sysinfoCmd.Flags().StringVar(&sysinfoSections, "sections", "", "comma-separated list of sections (os,date,hostname,uptime,network,routing)")
	rootCmd.AddCommand(sysinfoCmd)
}

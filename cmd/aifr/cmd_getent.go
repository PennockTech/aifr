// Copyright 2026 — see LICENSE file for terms.
package main

import (
	"strings"

	"go.pennock.tech/aifr/internal/engine"

	"github.com/spf13/cobra"
)

var getentFields string

var getentCmd = &cobra.Command{
	Use:   "getent <database> [key]",
	Short: "Query system databases (passwd, group, services, protocols)",
	Long: `Query system databases without shell pipelines. Reads flat files
directly (/etc/passwd, /etc/group, /etc/services, /etc/protocols).

Databases: passwd, group, services, protocols
Key: optional name or numeric ID to look up a single entry
Fields: comma-separated list to restrict output (default: all)

Passwd fields:    name, uid, gid, gecos, home, shell
Group fields:     name, gid, members
Services fields:  name, port, protocol, aliases
Protocols fields: name, number, aliases

Examples:
  aifr getent passwd
  aifr getent passwd root
  aifr getent passwd 1000
  aifr getent passwd --fields name,uid,home
  aifr getent group docker
  aifr getent services 443
  aifr getent services --fields name,port https`,
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		eng, err := buildEngine()
		if err != nil {
			exitWithError(err)
			return nil
		}

		params := engine.GetentParams{
			Database: args[0],
		}
		if len(args) > 1 {
			params.Key = args[1]
		}
		if getentFields != "" {
			params.Fields = strings.Split(getentFields, ",")
		}

		resp, err := eng.Getent(params)
		if err != nil {
			exitWithError(err)
			return nil
		}
		writeOutput(resp)
		return nil
	},
}

func init() {
	getentCmd.Flags().StringVar(&getentFields, "fields", "", "comma-separated list of fields to return")
	rootCmd.AddCommand(getentCmd)
}

// Copyright 2026 — see LICENSE file for terms.
// Command aifr is a read-only filesystem and git-tree access tool for AI agents.
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"go.pennock.tech/aifr/internal/accessctl"
	"go.pennock.tech/aifr/internal/config"
	"go.pennock.tech/aifr/internal/engine"
	"go.pennock.tech/aifr/internal/version"
	"go.pennock.tech/aifr/pkg/protocol"
)

var (
	flagConfig string
	flagFormat string
	flagQuiet  bool
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(protocol.ExitError)
	}
}

var rootCmd = &cobra.Command{
	Use:   "aifr",
	Short: "AI File Reader — read-only filesystem and git-tree access for AI agents",
	Long: `aifr is a read-only filesystem and git-tree access tool for AI coding agents.
It replaces shell pipelines (sed, find|grep, head/tail) with a single binary
that is always safe (never writes) and always scoped (enforces allow/deny lists
with a built-in sensitive-file blocklist).`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		info := map[string]string{
			"version":    version.Version,
			"commit":     version.Commit,
			"build_date": version.BuildDate,
		}
		if flagFormat == "text" {
			fmt.Printf("aifr %s (commit %s, built %s)\n",
				version.Version, version.Commit, version.BuildDate)
			return
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(info) //nolint:errcheck
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&flagConfig, "config", "", "config file path")
	rootCmd.PersistentFlags().StringVar(&flagFormat, "format", "json", "output format (json|text)")
	rootCmd.PersistentFlags().BoolVar(&flagQuiet, "quiet", false, "suppress non-essential output")

	rootCmd.Version = version.Version
	rootCmd.AddCommand(versionCmd)
}

// exitWithError writes an error response and exits with the appropriate code.
func exitWithError(err error) {
	code := protocol.ExitCodeForError(err)
	if ae, ok := err.(*protocol.AifrError); ok {
		writeJSON(&protocol.ErrorResponse{Error: ae})
	} else {
		writeJSON(&protocol.ErrorResponse{
			Error: &protocol.AifrError{Code: "ERROR", Message: err.Error()},
		})
	}
	os.Exit(code)
}

// writeJSON encodes v as JSON to stdout.
func writeJSON(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		fmt.Fprintf(os.Stderr, "aifr: failed to encode JSON: %v\n", err)
		os.Exit(protocol.ExitError)
	}
}

// writeOutput writes the response in the selected format.
func writeOutput(v any) {
	// For now, always JSON. --format text will be added in polish phase.
	writeJSON(v)
}

// loadConfig loads the effective configuration.
func loadConfig() (*config.Config, error) {
	return config.Load(config.LoadParams{ConfigPath: flagConfig})
}

// buildEngine constructs the engine from the current configuration.
func buildEngine() (*engine.Engine, error) {
	cfg, err := loadConfig()
	if err != nil {
		return nil, err
	}

	checker, err := accessctl.NewChecker(accessctl.CheckerParams{
		Allow:     cfg.Allow,
		Deny:      cfg.Deny,
		CredsDeny: cfg.CredsDeny,
	})
	if err != nil {
		return nil, err
	}

	return engine.NewEngine(checker, cfg)
}

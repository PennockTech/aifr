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
	"go.pennock.tech/aifr/internal/output"
	"go.pennock.tech/aifr/internal/version"
	"go.pennock.tech/aifr/pkg/protocol"
)

var (
	flagConfig       string
	flagFormat       string
	flagQuiet        bool
	flagNoRedact     bool
	flagNumberLines  bool
	flagVersionShort bool
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "aifr: %s\n", err)
		os.Exit(protocol.ExitUsage)
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
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Resolve flagFormat: explicit --format flag > AIFR_FORMAT env > "json".
		// Pass empty explicit when the flag wasn't set on the command line,
		// so ResolveFormat consults the environment variable.
		explicit := flagFormat
		if !cmd.Flags().Changed("format") {
			explicit = ""
		}
		supported := cliSupportedFormats(cmd)
		resolved, err := output.ResolveFormat(explicit, supported, "json")
		if err != nil {
			return err
		}
		flagFormat = resolved
		return nil
	},
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
		if flagFormat == "short" || flagVersionShort {
			fmt.Println(version.Version)
			return
		} else if flagFormat == "text" {
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
	rootCmd.PersistentFlags().StringVar(&flagFormat, "format", "", "output format (json|text); default from $AIFR_FORMAT or json")
	rootCmd.PersistentFlags().BoolVar(&flagQuiet, "quiet", false, "suppress non-essential output")
	rootCmd.PersistentFlags().BoolVar(&flagNoRedact, "no-redact", false, "do not redact sensitive config values")
	rootCmd.PersistentFlags().BoolVarP(&flagNumberLines, "number-lines", "n", false, "prefix each line with its file line number")

	versionCmd.Flags().BoolVarP(&flagVersionShort, "short", "s", false, "just the version")

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
	// Handle oneline format for log commands.
	if flagFormat == "oneline" {
		if resp, ok := v.(*protocol.LogResponse); ok {
			output.WriteLogOneline(os.Stdout, resp)
			return
		}
		// Non-log types fall through to JSON.
		writeJSON(v)
		return
	}

	if flagNumberLines && flagFormat != "text" {
		applyNumberLines(v)
	}
	if flagFormat != "text" {
		writeJSON(v)
		return
	}
	w := os.Stdout
	switch resp := v.(type) {
	case *protocol.ReadResponse:
		output.WriteReadText(w, resp, flagNumberLines)
	case *protocol.StatEntry:
		output.WriteStatText(w, resp)
	case *protocol.ListResponse:
		output.WriteListText(w, resp)
	case *protocol.SearchResponse:
		output.WriteSearchText(w, resp)
	case *protocol.FindResponse:
		output.WriteFindText(w, resp)
	case *protocol.DiffResponse:
		output.WriteDiffText(w, resp)
	case *protocol.LogResponse:
		if logDivider == "xml" {
			output.WriteLogXML(w, resp)
		} else {
			output.WriteLogText(w, resp)
		}
	case *protocol.RefsResponse:
		output.WriteRefsText(w, resp)
	case *protocol.WcResponse:
		output.WriteWcText(w, resp)
	case *protocol.PathfindResponse:
		output.WritePathfindText(w, resp)
	case *protocol.HexdumpResponse:
		output.WriteHexdumpText(w, resp)
	case *protocol.ChecksumResponse:
		output.WriteChecksumText(w, resp)
	case *protocol.RevParseResponse:
		output.WriteRevParseText(w, resp)
	case *protocol.ReflogResponse:
		output.WriteReflogText(w, resp)
	case *protocol.SysinfoResponse:
		output.WriteSysinfoText(w, resp)
	case *protocol.GetentResponse:
		output.WriteGetentText(w, resp)
	case *protocol.GitConfigResponse:
		output.WriteGitConfigText(w, resp)
	case *protocol.GitConfigStructuredResponse:
		output.WriteGitConfigStructuredText(w, resp)
	case *protocol.CatResponse:
		output.WriteCatText(w, resp, "plain", flagNumberLines)
	case *protocol.ErrorResponse:
		output.WriteErrorText(w, resp.Error)
	default:
		// Types without a dedicated text formatter fall back to JSON.
		writeJSON(v)
	}
}

// applyNumberLines mutates response content to include line numbers (for JSON mode).
// In text mode, the text formatters handle numbering directly.
func applyNumberLines(v any) {
	switch resp := v.(type) {
	case *protocol.ReadResponse:
		if resp.Chunk != nil && resp.Chunk.Encoding == "utf-8" {
			startLine := resp.Chunk.StartLine
			if startLine < 1 {
				startLine = 1
			}
			resp.Chunk.Data = output.NumberLines(resp.Chunk.Data, startLine)
		}
	case *protocol.CatResponse:
		for i := range resp.Files {
			entry := &resp.Files[i]
			if entry.Content != "" && !entry.Binary && entry.Error == "" {
				entry.Content = output.NumberLines(entry.Content, 1)
			}
		}
	}
}

// cliSupportedFormats returns the list of output formats a command supports.
func cliSupportedFormats(cmd *cobra.Command) []string {
	switch cmd.Name() {
	case "version":
		return []string{"json", "text", "short"}
	case "log":
		return []string{"json", "text", "oneline"}
	default:
		return []string{"json", "text"}
	}
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

	allow := cfg.Allow
	if cfg.IsPathReadable() {
		allow = append(allow, config.PathAllowPatterns()...)
	}

	checker, err := accessctl.NewChecker(accessctl.CheckerParams{
		Allow:     allow,
		Deny:      cfg.Deny,
		CredsDeny: cfg.CredsDeny,
	})
	if err != nil {
		return nil, err
	}

	return engine.NewEngine(checker, cfg)
}

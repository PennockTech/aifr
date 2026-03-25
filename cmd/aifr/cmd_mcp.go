// Copyright 2026 — see LICENSE file for terms.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"go.pennock.tech/aifr/internal/mcpserver"
)

var (
	mcpTransport string
	mcpAddr      string
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Start MCP server",
	Long: `Start the aifr MCP server.

The server exposes all aifr operations as MCP tools.
Use --transport stdio (default) for Claude Code integration,
or --transport http for multi-client setups.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		eng, err := buildEngine()
		if err != nil {
			exitWithError(err)
			return nil
		}

		srv := mcpserver.New(eng)
		srv.SetReloadFunc(buildEngine)
		srv.NoRedact = flagNoRedact

		ctx, cancel := signal.NotifyContext(context.Background(),
			os.Interrupt, syscall.SIGTERM)
		defer cancel()

		switch mcpTransport {
		case "stdio":
			fmt.Fprintln(os.Stderr, "aifr MCP server started (stdio). Press Ctrl+C to stop.")
			fmt.Fprintln(os.Stderr, "CLI is also available: aifr read <path>, aifr search <pat> <p>")
			return srv.RunStdio(ctx)

		case "http":
			handler := srv.HTTPHandler()
			mux := http.NewServeMux()
			mux.Handle("/mcp", handler)
			mux.Handle("/mcp/", handler)

			httpSrv := &http.Server{
				Addr:    mcpAddr,
				Handler: mux,
			}

			go func() {
				<-ctx.Done()
				httpSrv.Close() //nolint:errcheck
			}()

			slog.Info("aifr MCP server started", "transport", "http", "addr", mcpAddr)
			fmt.Fprintf(os.Stderr, "aifr MCP server listening on %s/mcp\n", mcpAddr)
			if err := httpSrv.ListenAndServe(); err != http.ErrServerClosed {
				return err
			}
			return nil

		default:
			return fmt.Errorf("unknown transport %q (use stdio or http)", mcpTransport)
		}
	},
}

func init() {
	mcpCmd.Flags().StringVar(&mcpTransport, "transport", "stdio", "transport mode (stdio|http)")
	mcpCmd.Flags().StringVar(&mcpAddr, "addr", ":8080", "HTTP listen address (for http transport)")
	rootCmd.AddCommand(mcpCmd)
}

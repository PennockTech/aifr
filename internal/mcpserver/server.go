// Copyright 2026 — see LICENSE file for terms.
// Package mcpserver implements the MCP server for aifr.
package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"go.pennock.tech/aifr/internal/engine"
	"go.pennock.tech/aifr/internal/version"
)

// Server wraps the MCP SDK server with aifr's engine.
type Server struct {
	sdkServer *mcp.Server
	engine    *engine.Engine
}

// New creates a new MCP server with all aifr tools registered.
func New(eng *engine.Engine) *Server {
	sdkServer := mcp.NewServer(
		&mcp.Implementation{
			Name:    "aifr",
			Version: version.Version,
		},
		&mcp.ServerOptions{
			Instructions: "aifr is a read-only filesystem and git-tree access tool. " +
				"Use aifr_read to read files, aifr_cat to read multiple files with dividers, " +
				"aifr_search to search content, aifr_list to list directories, " +
				"aifr_find to find files, aifr_stat for metadata, aifr_refs for git refs, " +
				"aifr_log for git history, aifr_diff to compare files, " +
				"aifr_pathfind to find commands in PATH-like search lists, " +
				"aifr_wc to count lines/words/bytes, aifr_checksum for file checksums, " +
				"aifr_hexdump for binary hex dumps, aifr_rev_parse to resolve git refs, " +
				"and aifr_sysinfo for system inspection (OS, date, uptime, network, routing). " +
				"For aifr_cat, use format=\"text\" with divider=\"xml\" for token-efficient output.",
		},
	)

	s := &Server{
		sdkServer: sdkServer,
		engine:    eng,
	}

	s.registerTools()
	return s
}

// RunStdio starts the MCP server on stdio transport.
func (s *Server) RunStdio(ctx context.Context) error {
	slog.Info("aifr MCP server started (stdio)")
	return s.sdkServer.Run(ctx, &mcp.StdioTransport{})
}

// HTTPHandler returns an HTTP handler for the streamable HTTP transport.
func (s *Server) HTTPHandler() http.Handler {
	return mcp.NewStreamableHTTPHandler(
		func(r *http.Request) *mcp.Server { return s.sdkServer },
		&mcp.StreamableHTTPOptions{Stateless: true},
	)
}

// toolResult creates a successful tool result with JSON content.
func toolResult(v any) (*mcp.CallToolResult, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshaling result: %w", err)
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(data)}},
	}, nil
}

// toolError creates an error tool result (visible to the LLM).
func toolError(msg string) (*mcp.CallToolResult, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: msg}},
		IsError: true,
	}, nil
}

// Copyright 2026 — see LICENSE file for terms.
package hookcmd

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// detectMCPAvailable checks whether an aifr MCP server is likely available
// in the current Claude Code session.
//
// Detection order:
//  1. AIFR_MCP environment variable (any non-empty value → true)
//  2. .mcp.json in the given working directory
func detectMCPAvailable(cwd string) bool {
	if os.Getenv("AIFR_MCP") != "" {
		return true
	}
	if cwd != "" {
		if checkMCPConfig(filepath.Join(cwd, ".mcp.json")) {
			return true
		}
	}
	return false
}

// checkMCPConfig reads a .mcp.json file and returns true if it contains
// an aifr MCP server entry (matched by server name or command basename).
func checkMCPConfig(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var config struct {
		MCPServers map[string]struct {
			Command string `json:"command"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal(data, &config); err != nil {
		return false
	}
	for name, server := range config.MCPServers {
		if name == "aifr" {
			return true
		}
		if filepath.Base(server.Command) == "aifr" {
			return true
		}
	}
	return false
}

// Copyright 2026 — see LICENSE file for terms.
package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"go.pennock.tech/aifr/internal/engine"
	"go.pennock.tech/aifr/internal/gitprovider"
)

func (s *Server) registerTools() {
	s.sdkServer.AddTool(toolRead(), s.handleRead)
	s.sdkServer.AddTool(toolStat(), s.handleStat)
	s.sdkServer.AddTool(toolList(), s.handleList)
	s.sdkServer.AddTool(toolSearch(), s.handleSearch)
	s.sdkServer.AddTool(toolFind(), s.handleFind)
	s.sdkServer.AddTool(toolRefs(), s.handleRefs)
	s.sdkServer.AddTool(toolLog(), s.handleLog)
	s.sdkServer.AddTool(toolDiff(), s.handleDiff)
}

// ── Tool Definitions ──

func toolRead() *mcp.Tool {
	return &mcp.Tool{
		Name: "aifr_read",
		Description: `Read file contents with optional chunking. Supports filesystem paths and git refs.

Path syntax:
  /absolute/path          → filesystem file
  relative/path           → relative to allowed roots
  branch:path             → git tree (auto-detected repo)
  reponame:ref:path       → named git repo at ref

Chunking (mutually exclusive):
  lines: "1:50"           → lines 1-50 (1-indexed, inclusive)
  chunk_id: "<token>"     → continue from previous chunk

Returns: file content, metadata, and continuation token if incomplete.
Errors: ACCESS_DENIED_SENSITIVE means the file looks like a credential
        and the user should be asked to read it manually if needed.`,
		InputSchema: mustSchema(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path":     map[string]any{"type": "string", "description": "File path or git ref:path"},
				"lines":    map[string]any{"type": "string", "description": "Line range e.g. '1:50' or '50:'"},
				"chunk_id": map[string]any{"type": "string", "description": "Continuation token from previous read"},
			},
			"required": []string{"path"},
		}),
	}
}

func toolStat() *mcp.Tool {
	return &mcp.Tool{
		Name:        "aifr_stat",
		Description: "Get file/directory metadata. Returns type, size, mode, mtime. Supports filesystem paths and git refs.",
		InputSchema: mustSchema(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{"type": "string", "description": "File path or git ref:path"},
			},
			"required": []string{"path"},
		}),
	}
}

func toolList() *mcp.Tool {
	return &mcp.Tool{
		Name:        "aifr_list",
		Description: "List directory contents. Supports depth control, glob pattern filtering, and type filtering. Supports filesystem and git refs.",
		InputSchema: mustSchema(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path":    map[string]any{"type": "string", "description": "Directory path or git ref:path"},
				"depth":   map[string]any{"type": "integer", "description": "Recursion depth (0=immediate, -1=unlimited)", "default": 0},
				"pattern": map[string]any{"type": "string", "description": "Glob filter on entry name"},
				"type":    map[string]any{"type": "string", "description": "Entry type filter: f=file, d=dir, l=symlink"},
			},
			"required": []string{"path"},
		}),
	}
}

func toolSearch() *mcp.Tool {
	return &mcp.Tool{
		Name: "aifr_search",
		Description: `Search file contents (grep-like). RE2 regexp or fixed-string matching.
Returns structured matches with file, line, column, and optional context lines.`,
		InputSchema: mustSchema(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"pattern":     map[string]any{"type": "string", "description": "Search pattern (RE2 regexp by default)"},
				"path":        map[string]any{"type": "string", "description": "Directory or file to search"},
				"regexp":      map[string]any{"type": "boolean", "description": "Treat pattern as regexp (default true)", "default": true},
				"ignore_case": map[string]any{"type": "boolean", "description": "Case-insensitive matching"},
				"context":     map[string]any{"type": "integer", "description": "Context lines before/after match"},
				"max_matches": map[string]any{"type": "integer", "description": "Max matches (default 500)"},
				"include":     map[string]any{"type": "string", "description": "Glob for files to include"},
				"exclude":     map[string]any{"type": "string", "description": "Glob for files to exclude"},
			},
			"required": []string{"pattern", "path"},
		}),
	}
}

func toolFind() *mcp.Tool {
	return &mcp.Tool{
		Name:        "aifr_find",
		Description: "Find files by name/path pattern, type, size, or age. Returns matching paths as structured JSON.",
		InputSchema: mustSchema(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path":       map[string]any{"type": "string", "description": "Directory to search"},
				"name":       map[string]any{"type": "string", "description": "Glob on filename"},
				"type":       map[string]any{"type": "string", "description": "Entry type: f=file, d=dir, l=symlink"},
				"max_depth":  map[string]any{"type": "integer", "description": "Max recursion depth (-1=unlimited)", "default": -1},
				"min_size":   map[string]any{"type": "integer", "description": "Minimum file size in bytes"},
				"max_size":   map[string]any{"type": "integer", "description": "Maximum file size in bytes"},
				"newer_than": map[string]any{"type": "string", "description": "Duration e.g. '24h', '7d'"},
			},
			"required": []string{"path"},
		}),
	}
}

func toolRefs() *mcp.Tool {
	return &mcp.Tool{
		Name:        "aifr_refs",
		Description: "List git branches, tags, and remote refs for a repository.",
		InputSchema: mustSchema(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"repo":     map[string]any{"type": "string", "description": "Named repo (empty = auto-detect)"},
				"branches": map[string]any{"type": "boolean", "description": "Show branches"},
				"tags":     map[string]any{"type": "boolean", "description": "Show tags"},
				"remotes":  map[string]any{"type": "boolean", "description": "Show remote refs"},
			},
		}),
	}
}

func toolLog() *mcp.Tool {
	return &mcp.Tool{
		Name:        "aifr_log",
		Description: "Git commit log with structured entries (hash, author, date, message, files changed).",
		InputSchema: mustSchema(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"repo":      map[string]any{"type": "string", "description": "Named repo (empty = auto-detect)"},
				"ref":       map[string]any{"type": "string", "description": "Git ref (default HEAD)"},
				"max_count": map[string]any{"type": "integer", "description": "Max commits (default 20)", "default": 20},
			},
		}),
	}
}

func toolDiff() *mcp.Tool {
	return &mcp.Tool{
		Name: "aifr_diff",
		Description: `Compare two files. Supports filesystem paths and git refs.
Examples: diff file1.go file2.go, diff main:lib.go feature:lib.go, diff HEAD~1:README.md README.md`,
		InputSchema: mustSchema(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path_a": map[string]any{"type": "string", "description": "First path (filesystem or git ref:path)"},
				"path_b": map[string]any{"type": "string", "description": "Second path (filesystem or git ref:path)"},
			},
			"required": []string{"path_a", "path_b"},
		}),
	}
}

// ── Tool Handlers ──

func (s *Server) handleRead(_ context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args struct {
		Path    string `json:"path"`
		Lines   string `json:"lines"`
		ChunkID string `json:"chunk_id"`
	}
	if err := unmarshalArgs(req, &args); err != nil {
		return toolError(err.Error())
	}

	params := engine.ReadParams{ChunkID: args.ChunkID}
	if args.Lines != "" {
		lr, err := parseLineRangeMCP(args.Lines)
		if err != nil {
			return toolError(err.Error())
		}
		params.Lines = lr
	}

	if gitprovider.IsGitPath(args.Path) {
		resp, err := s.engine.GitRead(args.Path, params)
		if err != nil {
			return toolError(err.Error())
		}
		return toolResult(resp)
	}
	resp, err := s.engine.Read(args.Path, params)
	if err != nil {
		return toolError(err.Error())
	}
	return toolResult(resp)
}

func (s *Server) handleStat(_ context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args struct {
		Path string `json:"path"`
	}
	if err := unmarshalArgs(req, &args); err != nil {
		return toolError(err.Error())
	}

	if gitprovider.IsGitPath(args.Path) {
		resp, err := s.engine.GitStat(args.Path)
		if err != nil {
			return toolError(err.Error())
		}
		return toolResult(resp)
	}
	resp, err := s.engine.Stat(args.Path)
	if err != nil {
		return toolError(err.Error())
	}
	return toolResult(resp)
}

func (s *Server) handleList(_ context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args struct {
		Path    string `json:"path"`
		Depth   int    `json:"depth"`
		Pattern string `json:"pattern"`
		Type    string `json:"type"`
	}
	if err := unmarshalArgs(req, &args); err != nil {
		return toolError(err.Error())
	}

	if gitprovider.IsGitPath(args.Path) {
		resp, err := s.engine.GitList(args.Path)
		if err != nil {
			return toolError(err.Error())
		}
		return toolResult(resp)
	}
	resp, err := s.engine.List(args.Path, engine.ListParams{
		Depth:   args.Depth,
		Pattern: args.Pattern,
		Type:    args.Type,
	})
	if err != nil {
		return toolError(err.Error())
	}
	return toolResult(resp)
}

func (s *Server) handleSearch(_ context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args struct {
		Pattern    string `json:"pattern"`
		Path       string `json:"path"`
		Regexp     *bool  `json:"regexp"`
		IgnoreCase bool   `json:"ignore_case"`
		Context    int    `json:"context"`
		MaxMatches int    `json:"max_matches"`
		Include    string `json:"include"`
		Exclude    string `json:"exclude"`
	}
	if err := unmarshalArgs(req, &args); err != nil {
		return toolError(err.Error())
	}

	isRegexp := true
	if args.Regexp != nil {
		isRegexp = *args.Regexp
	}

	resp, err := s.engine.Search(args.Pattern, args.Path, engine.SearchParams{
		IsRegexp:   isRegexp,
		IgnoreCase: args.IgnoreCase,
		Context:    args.Context,
		MaxMatches: args.MaxMatches,
		Include:    args.Include,
		Exclude:    args.Exclude,
	})
	if err != nil {
		return toolError(err.Error())
	}
	return toolResult(resp)
}

func (s *Server) handleFind(_ context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args struct {
		Path      string `json:"path"`
		Name      string `json:"name"`
		Type      string `json:"type"`
		MaxDepth  int    `json:"max_depth"`
		MinSize   int64  `json:"min_size"`
		MaxSize   int64  `json:"max_size"`
		NewerThan string `json:"newer_than"`
	}
	if err := unmarshalArgs(req, &args); err != nil {
		return toolError(err.Error())
	}

	params := engine.FindParams{
		Name:     args.Name,
		Type:     args.Type,
		MaxDepth: args.MaxDepth,
		MinSize:  args.MinSize,
		MaxSize:  args.MaxSize,
	}

	if args.NewerThan != "" {
		d, err := parseDurationMCP(args.NewerThan)
		if err != nil {
			return toolError(err.Error())
		}
		params.NewerThan = d
	}

	resp, err := s.engine.Find(args.Path, params)
	if err != nil {
		return toolError(err.Error())
	}
	return toolResult(resp)
}

func (s *Server) handleRefs(_ context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args struct {
		Repo     string `json:"repo"`
		Branches bool   `json:"branches"`
		Tags     bool   `json:"tags"`
		Remotes  bool   `json:"remotes"`
	}
	if err := unmarshalArgs(req, &args); err != nil {
		return toolError(err.Error())
	}
	resp, err := s.engine.Refs(args.Repo, args.Branches, args.Tags, args.Remotes)
	if err != nil {
		return toolError(err.Error())
	}
	return toolResult(resp)
}

func (s *Server) handleLog(_ context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args struct {
		Repo     string `json:"repo"`
		Ref      string `json:"ref"`
		MaxCount int    `json:"max_count"`
	}
	if err := unmarshalArgs(req, &args); err != nil {
		return toolError(err.Error())
	}
	resp, err := s.engine.Log(args.Repo, args.Ref, args.MaxCount)
	if err != nil {
		return toolError(err.Error())
	}
	return toolResult(resp)
}

func (s *Server) handleDiff(_ context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args struct {
		PathA string `json:"path_a"`
		PathB string `json:"path_b"`
	}
	if err := unmarshalArgs(req, &args); err != nil {
		return toolError(err.Error())
	}
	resp, err := s.engine.Diff(args.PathA, args.PathB)
	if err != nil {
		return toolError(err.Error())
	}
	return toolResult(resp)
}

// ── Helpers ──

func mustSchema(v any) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		panic("invalid schema: " + err.Error())
	}
	return data
}

func unmarshalArgs(req *mcp.CallToolRequest, dst any) error {
	data, err := json.Marshal(req.Params.Arguments)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dst)
}

func parseLineRangeMCP(s string) (*engine.LineRange, error) {
	// Reuse the same parsing logic as the CLI.
	// Format: "START:END" or "START:" (to EOF)
	var start, end int
	n, _ := fmt.Sscanf(s, "%d:%d", &start, &end)
	if n < 1 {
		return nil, fmt.Errorf("invalid line range %q", s)
	}
	return &engine.LineRange{Start: start, End: end}, nil
}

func parseDurationMCP(s string) (time.Duration, error) {
	return time.ParseDuration(s)
}

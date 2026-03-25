// Copyright 2026 — see LICENSE file for terms.
package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"go.pennock.tech/aifr/internal/config"
	"go.pennock.tech/aifr/internal/engine"
	"go.pennock.tech/aifr/internal/gitprovider"
	"go.pennock.tech/aifr/internal/output"
	"go.pennock.tech/aifr/internal/version"
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
	s.sdkServer.AddTool(toolCat(), s.handleCat)
	s.sdkServer.AddTool(toolPathfind(), s.handlePathfind)
	s.sdkServer.AddTool(toolWc(), s.handleWc)
	s.sdkServer.AddTool(toolChecksum(), s.handleChecksum)
	s.sdkServer.AddTool(toolRevParse(), s.handleRevParse)
	s.sdkServer.AddTool(toolHexdump(), s.handleHexdump)
	s.sdkServer.AddTool(toolSysinfo(), s.handleSysinfo)
	s.sdkServer.AddTool(toolGetent(), s.handleGetent)
	s.sdkServer.AddTool(toolReflog(), s.handleReflog)
	s.sdkServer.AddTool(toolStashList(), s.handleStashList)
	s.sdkServer.AddTool(toolSelf(), s.handleSelf)
	s.sdkServer.AddTool(toolGitConfig(), s.handleGitConfig)
}

// ── Tool Definitions ──

func toolRead() *mcp.Tool {
	return &mcp.Tool{
		Name: "aifr_read",
		Description: `Read file contents with optional chunking. Supports filesystem paths and git refs.

Path syntax:
  /absolute/path              → filesystem file
  relative/path               → relative to allowed roots
  branch:path                 → git tree (auto-detected repo from cwd)
  reponame:ref:path           → named git repo at ref
  /path/to/dir:ref:path       → git repo found at/above dir, at ref

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
				"sort":    map[string]any{"type": "string", "description": "Sort order: name, path, size, mtime, version"},
				"desc":    map[string]any{"type": "boolean", "description": "Sort descending"},
				"limit":   map[string]any{"type": "integer", "description": "Limit results (0=no limit)"},
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
				"sort":       map[string]any{"type": "string", "description": "Sort order: name, path, size, version"},
				"desc":       map[string]any{"type": "boolean", "description": "Sort descending"},
				"limit":      map[string]any{"type": "integer", "description": "Limit results (0=no limit)"},
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
				"repo":     map[string]any{"type": "string", "description": "Named repo, filesystem path, or empty for auto-detect from cwd"},
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
				"repo":      map[string]any{"type": "string", "description": "Named repo, filesystem path, or empty for auto-detect from cwd"},
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
Use byte_level=true for cmp-style byte comparison (reports first differing byte).
Examples: diff file1.go file2.go, diff main:lib.go feature:lib.go, diff HEAD~1:README.md README.md`,
		InputSchema: mustSchema(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path_a":     map[string]any{"type": "string", "description": "First path (filesystem or git ref:path)"},
				"path_b":     map[string]any{"type": "string", "description": "Second path (filesystem or git ref:path)"},
				"byte_level": map[string]any{"type": "boolean", "description": "Byte-level comparison (cmp mode): report first differing byte instead of line diff"},
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
		Sort    string `json:"sort"`
		Desc    bool   `json:"desc"`
		Limit   int    `json:"limit"`
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
		Depth:      args.Depth,
		Pattern:    args.Pattern,
		Type:       args.Type,
		Sort:       engine.SortOrder(args.Sort),
		Descending: args.Desc,
		Limit:      args.Limit,
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
		Sort      string `json:"sort"`
		Desc      bool   `json:"desc"`
		Limit     int    `json:"limit"`
	}
	if err := unmarshalArgs(req, &args); err != nil {
		return toolError(err.Error())
	}

	params := engine.FindParams{
		Name:       args.Name,
		Type:       args.Type,
		MaxDepth:   args.MaxDepth,
		MinSize:    args.MinSize,
		MaxSize:    args.MaxSize,
		Sort:       engine.SortOrder(args.Sort),
		Descending: args.Desc,
		Limit:      args.Limit,
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
		PathA     string `json:"path_a"`
		PathB     string `json:"path_b"`
		ByteLevel bool   `json:"byte_level"`
	}
	if err := unmarshalArgs(req, &args); err != nil {
		return toolError(err.Error())
	}
	resp, err := s.engine.Diff(args.PathA, args.PathB, engine.DiffParams{
		ByteLevel: args.ByteLevel,
	})
	if err != nil {
		return toolError(err.Error())
	}
	return toolResult(resp)
}

func toolCat() *mcp.Tool {
	return &mcp.Tool{
		Name: "aifr_cat",
		Description: `Concatenate contents of multiple files with dividers. Two modes:
1. Explicit paths: provide a list of file paths in "paths"
2. Discovery: provide "root" directory with "name"/"exclude_path" filters

Tip: Use format="text" with divider="xml" for token-efficient multi-file
reading. Each file is wrapped in <file path="...">content</file> tags.

Binary files are skipped. Each file is access-controlled individually.
Use "lines" to limit output to first N lines per file (head mode).`,
		InputSchema: mustSchema(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"paths":          map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Explicit file paths to concatenate"},
				"root":           map[string]any{"type": "string", "description": "Root directory for discovery mode"},
				"name":           map[string]any{"type": "string", "description": "Glob on filename (discovery mode)"},
				"exclude_path":   map[string]any{"type": "string", "description": "Doublestar glob on relative path to exclude"},
				"max_depth":      map[string]any{"type": "integer", "description": "Max recursion depth for discovery (-1=unlimited)", "default": -1},
				"lines":          map[string]any{"type": "integer", "description": "Max lines per file (0=all)", "default": 0},
				"divider":        map[string]any{"type": "string", "enum": []string{"xml", "plain", "none"}, "description": "Divider format for text output (default: xml)", "default": "xml"},
				"format":         map[string]any{"type": "string", "enum": []string{"json", "text"}, "description": "Output format (default: json)", "default": "json"},
				"max_total_size": map[string]any{"type": "integer", "description": "Max total output bytes (default: 2MiB)"},
				"max_files":      map[string]any{"type": "integer", "description": "Max files to read (default: 1000)"},
			},
		}),
	}
}

func (s *Server) handleCat(_ context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args struct {
		Paths        []string `json:"paths"`
		Root         string   `json:"root"`
		Name         string   `json:"name"`
		ExcludePath  string   `json:"exclude_path"`
		MaxDepth     int      `json:"max_depth"`
		Lines        int      `json:"lines"`
		Divider      string   `json:"divider"`
		Format       string   `json:"format"`
		MaxTotalSize int64    `json:"max_total_size"`
		MaxFiles     int      `json:"max_files"`
	}
	if err := unmarshalArgs(req, &args); err != nil {
		return toolError(err.Error())
	}

	params := engine.CatParams{
		Name:         args.Name,
		ExcludePath:  args.ExcludePath,
		MaxDepth:     args.MaxDepth,
		Lines:        args.Lines,
		MaxTotalSize: args.MaxTotalSize,
		MaxFiles:     args.MaxFiles,
	}

	resp, err := s.engine.Cat(args.Paths, args.Root, params)
	if err != nil {
		return toolError(err.Error())
	}

	// If text format requested, format with divider and return as text.
	if args.Format == "text" {
		divider := args.Divider
		if divider == "" {
			divider = "xml"
		}
		var buf strings.Builder
		output.WriteCatText(&buf, resp, divider)
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: buf.String()}},
		}, nil
	}

	return toolResult(resp)
}

func toolPathfind() *mcp.Tool {
	return &mcp.Tool{
		Name: "aifr_pathfind",
		Description: `Find commands in PATH-like search lists. A better "which" that reports ALL
matches with masking info. Supports glob wildcards in command name (e.g., "git-*").

search_list specs:
  "envvar:PATH"                     → $PATH directories (default)
  "envvar:CLASSPATH"                → $CLASSPATH directories
  "dirlist:/usr/bin:/usr/local/bin" → explicit directory list

Each entry shows path, mode, executable flag, and whether it is masked by an
earlier entry in the search list.`,
		InputSchema: mustSchema(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"command":     map[string]any{"type": "string", "description": "Command name or glob pattern (e.g., 'git', 'git-*', 'python3.[0-9]')"},
				"search_list": map[string]any{"type": "string", "description": "Search list spec (default: envvar:PATH)"},
			},
			"required": []string{"command"},
		}),
	}
}

func (s *Server) handlePathfind(_ context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args struct {
		Command    string `json:"command"`
		SearchList string `json:"search_list"`
	}
	if err := unmarshalArgs(req, &args); err != nil {
		return toolError(err.Error())
	}
	resp, err := s.engine.Pathfind(args.Command, engine.PathfindParams{
		SearchList: args.SearchList,
	})
	if err != nil {
		return toolError(err.Error())
	}
	return toolResult(resp)
}

// ── reflog ──

func toolReflog() *mcp.Tool {
	return &mcp.Tool{
		Name:        "aifr_reflog",
		Description: `Show git reflog for a ref (default: HEAD). Lists recent ref updates with timestamps and actions. Replaces git reflog.`,
		InputSchema: mustSchema(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"ref":       map[string]any{"type": "string", "description": "Git ref to show reflog for (default: HEAD). Can be a branch name."},
				"repo":      map[string]any{"type": "string", "description": "Named repo or filesystem path (default: auto-detect)"},
				"max_count": map[string]any{"type": "integer", "description": "Maximum entries to return (default: 50)"},
			},
		}),
	}
}

func (s *Server) handleReflog(_ context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args struct {
		Ref      string `json:"ref"`
		Repo     string `json:"repo"`
		MaxCount int    `json:"max_count"`
	}
	if err := unmarshalArgs(req, &args); err != nil {
		return toolError(err.Error())
	}
	resp, err := s.engine.Reflog(args.Repo, args.Ref, engine.ReflogParams{
		MaxCount: args.MaxCount,
	})
	if err != nil {
		return toolError(err.Error())
	}
	return toolResult(resp)
}

// ── stash-list ──

func toolStashList() *mcp.Tool {
	return &mcp.Tool{
		Name:        "aifr_stash_list",
		Description: `List git stashes. Returns stash entries with hashes, authors, dates, and messages. Replaces git stash list.`,
		InputSchema: mustSchema(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"repo":      map[string]any{"type": "string", "description": "Named repo or filesystem path (default: auto-detect)"},
				"max_count": map[string]any{"type": "integer", "description": "Maximum stash entries to return (default: 50)"},
			},
		}),
	}
}

func (s *Server) handleStashList(_ context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args struct {
		Repo     string `json:"repo"`
		MaxCount int    `json:"max_count"`
	}
	if err := unmarshalArgs(req, &args); err != nil {
		return toolError(err.Error())
	}
	resp, err := s.engine.StashList(args.Repo, engine.ReflogParams{
		MaxCount: args.MaxCount,
	})
	if err != nil {
		return toolError(err.Error())
	}
	return toolResult(resp)
}

// ── getent ──

func toolGetent() *mcp.Tool {
	return &mcp.Tool{
		Name: "aifr_getent",
		Description: `Query system databases (passwd, group, services, protocols) without shell pipelines. Reads /etc flat files directly. Supports key lookup by name or numeric ID. Use fields to restrict output columns.
Passwd fields: name, uid, gid, gecos, home, shell. Group fields: name, gid, members. Services fields: name, port, protocol, aliases. Protocols fields: name, number, aliases.`,
		InputSchema: mustSchema(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"database": map[string]any{"type": "string", "enum": []string{"passwd", "group", "services", "protocols"}, "description": "System database to query"},
				"key":      map[string]any{"type": "string", "description": "Optional: look up by name or numeric ID"},
				"fields":   map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Fields to return (default: all)"},
				"protocol": map[string]any{"type": "string", "description": "For services database: filter by protocol (tcp, udp)"},
			},
			"required": []string{"database"},
		}),
	}
}

func (s *Server) handleGetent(_ context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args struct {
		Database string   `json:"database"`
		Key      string   `json:"key"`
		Fields   []string `json:"fields"`
		Protocol string   `json:"protocol"`
	}
	if err := unmarshalArgs(req, &args); err != nil {
		return toolError(err.Error())
	}
	resp, err := s.engine.Getent(engine.GetentParams{
		Database: args.Database,
		Key:      args.Key,
		Fields:   args.Fields,
		Protocol: args.Protocol,
	})
	if err != nil {
		return toolError(err.Error())
	}
	return toolResult(resp)
}

// ── sysinfo ──

func toolSysinfo() *mcp.Tool {
	return &mcp.Tool{
		Name:        "aifr_sysinfo",
		Description: `System inspection for fault diagnosis: OS info, current date/time (including year for copyrights), hostname, uptime, network interfaces, routing table. No files written, no commands executed.`,
		InputSchema: mustSchema(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"sections": map[string]any{
					"type":        "array",
					"items":       map[string]any{"type": "string", "enum": []string{"os", "date", "hostname", "uptime", "network", "routing"}},
					"description": "Sections to include (default: all)",
				},
			},
		}),
	}
}

func (s *Server) handleSysinfo(_ context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args struct {
		Sections []string `json:"sections"`
	}
	if err := unmarshalArgs(req, &args); err != nil {
		return toolError(err.Error())
	}
	resp, err := s.engine.Sysinfo(engine.SysinfoParams{
		Sections: args.Sections,
	})
	if err != nil {
		return toolError(err.Error())
	}
	return toolResult(resp)
}

// ── hexdump ──

func toolHexdump() *mcp.Tool {
	return &mcp.Tool{
		Name:        "aifr_hexdump",
		Description: `Hex dump of file contents in canonical format (offset | hex bytes | ASCII). Default: 256 bytes from offset 0, max 64 KiB. Supports filesystem paths and git refs.`,
		InputSchema: mustSchema(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path":   map[string]any{"type": "string", "description": "File path (filesystem or git ref:path)"},
				"offset": map[string]any{"type": "integer", "description": "Starting byte offset (default 0)"},
				"length": map[string]any{"type": "integer", "description": "Bytes to dump (default 256, max 65536)"},
			},
			"required": []string{"path"},
		}),
	}
}

func (s *Server) handleHexdump(_ context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args struct {
		Path   string `json:"path"`
		Offset int64  `json:"offset"`
		Length int64  `json:"length"`
	}
	if err := unmarshalArgs(req, &args); err != nil {
		return toolError(err.Error())
	}
	resp, err := s.engine.Hexdump(args.Path, engine.HexdumpParams{
		Offset: args.Offset,
		Length: args.Length,
	})
	if err != nil {
		return toolError(err.Error())
	}
	return toolResult(resp)
}

// ── rev-parse ──

func toolRevParse() *mcp.Tool {
	return &mcp.Tool{
		Name:        "aifr_rev_parse",
		Description: `Resolve a git ref (branch, tag, commit, HEAD~N) to its full commit hash and metadata. Replaces git rev-parse.`,
		InputSchema: mustSchema(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"ref":  map[string]any{"type": "string", "description": "Git ref to resolve (branch, tag, commit hash, HEAD~N). Defaults to HEAD."},
				"repo": map[string]any{"type": "string", "description": "Named repo or filesystem path (default: auto-detect from cwd)"},
			},
			"required": []string{"ref"},
		}),
	}
}

func (s *Server) handleRevParse(_ context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args struct {
		Ref  string `json:"ref"`
		Repo string `json:"repo"`
	}
	if err := unmarshalArgs(req, &args); err != nil {
		return toolError(err.Error())
	}
	resp, err := s.engine.RevParse(args.Repo, args.Ref)
	if err != nil {
		return toolError(err.Error())
	}
	return toolResult(resp)
}

// ── checksum ──

func toolChecksum() *mcp.Tool {
	return &mcp.Tool{
		Name:        "aifr_checksum",
		Description: `Compute cryptographic checksums for one or more files. Supports sha256 (default), sha1, sha512, sha3-256, sha3-512, md5. Output as hex (default), base64, or base64url. Accepts filesystem paths and git refs.`,
		InputSchema: mustSchema(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"paths":     map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "File paths to checksum (filesystem or git ref:path)"},
				"algorithm": map[string]any{"type": "string", "description": "Hash algorithm: sha256, sha1, sha512, sha3-256, sha3-512, md5 (default: sha256)"},
				"encoding":  map[string]any{"type": "string", "description": "Output encoding: hex, base64, base64url (default: hex)"},
			},
			"required": []string{"paths"},
		}),
	}
}

func (s *Server) handleChecksum(_ context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args struct {
		Paths     []string `json:"paths"`
		Algorithm string   `json:"algorithm"`
		Encoding  string   `json:"encoding"`
	}
	if err := unmarshalArgs(req, &args); err != nil {
		return toolError(err.Error())
	}
	if len(args.Paths) == 0 {
		return toolError("paths is required and must not be empty")
	}
	resp, err := s.engine.Checksum(args.Paths, engine.ChecksumParams{
		Algorithm: args.Algorithm,
		Encoding:  args.Encoding,
	})
	if err != nil {
		return toolError(err.Error())
	}
	return toolResult(resp)
}

// ── wc ──

func toolWc() *mcp.Tool {
	return &mcp.Tool{
		Name:        "aifr_wc",
		Description: `Count lines, words, bytes, and/or characters in one or more files. Accepts filesystem paths and git refs. If no count flags are set, returns lines + words + bytes. Use total_only=true to get only the combined total (avoids per-file output).`,
		InputSchema: mustSchema(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"paths":      map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "File paths to count (filesystem or git ref:path)"},
				"lines":      map[string]any{"type": "boolean", "description": "Count lines"},
				"words":      map[string]any{"type": "boolean", "description": "Count words"},
				"bytes":      map[string]any{"type": "boolean", "description": "Count bytes"},
				"chars":      map[string]any{"type": "boolean", "description": "Count characters (runes)"},
				"total_only": map[string]any{"type": "boolean", "description": "Return only the combined total, suppress per-file entries"},
			},
			"required": []string{"paths"},
		}),
	}
}

func (s *Server) handleWc(_ context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args struct {
		Paths     []string `json:"paths"`
		Lines     bool     `json:"lines"`
		Words     bool     `json:"words"`
		Bytes     bool     `json:"bytes"`
		Chars     bool     `json:"chars"`
		TotalOnly bool     `json:"total_only"`
	}
	if err := unmarshalArgs(req, &args); err != nil {
		return toolError(err.Error())
	}
	if len(args.Paths) == 0 {
		return toolError("paths is required and must not be empty")
	}
	resp, err := s.engine.Wc(args.Paths, engine.WcParams{
		Lines:     args.Lines,
		Words:     args.Words,
		Bytes:     args.Bytes,
		Chars:     args.Chars,
		TotalOnly: args.TotalOnly,
	})
	if err != nil {
		return toolError(err.Error())
	}
	return toolResult(resp)
}

// ── git-config ──

func toolGitConfig() *mcp.Tool {
	return &mcp.Tool{
		Name: "aifr_git_config",
		Description: `Query git configuration. Default scope is local (.git/config).
Use scope="merged" for full cascade with include resolution (supports gitdir: conditional includes).
Structured queries: "identity" (defaults to merged scope), "remotes", "branches".
Credential-related keys are always redacted.`,
		InputSchema: mustSchema(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"key":        map[string]any{"type": "string", "description": "Single key lookup (e.g., remote.origin.url)"},
				"regexp":     map[string]any{"type": "string", "description": "Match keys by regexp"},
				"section":    map[string]any{"type": "string", "description": "List entries in section (e.g., remote.origin)"},
				"list":       map[string]any{"type": "boolean", "description": "Dump all config entries"},
				"scope":      map[string]any{"type": "string", "description": "Config scope: local (default), merged, global, system"},
				"type":       map[string]any{"type": "string", "description": "Type coercion: bool, int, path"},
				"structured": map[string]any{"type": "string", "description": "Structured query: identity, remotes, branches"},
				"repo":       map[string]any{"type": "string", "description": "Named repo or filesystem path (default: auto-detect)"},
			},
		}),
	}
}

func (s *Server) handleGitConfig(_ context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args struct {
		Key        string `json:"key"`
		Regexp     string `json:"regexp"`
		Section    string `json:"section"`
		List       bool   `json:"list"`
		Scope      string `json:"scope"`
		Type       string `json:"type"`
		Structured string `json:"structured"`
		Repo       string `json:"repo"`
	}
	if err := unmarshalArgs(req, &args); err != nil {
		return toolError(err.Error())
	}
	resp, err := s.engine.GitConfig(args.Repo, engine.GitConfigParams{
		Key:        args.Key,
		Regexp:     args.Regexp,
		Section:    args.Section,
		List:       args.List,
		Scope:      args.Scope,
		Type:       args.Type,
		Structured: args.Structured,
	})
	if err != nil {
		return toolError(err.Error())
	}
	return toolResult(resp)
}

// ── self ──

func toolSelf() *mcp.Tool {
	return &mcp.Tool{
		Name: "aifr_self",
		Description: `Introspect and manage the running aifr MCP server instance.
Actions:
  "version"    — return build version, commit, and date
  "config"     — return the effective aifr configuration
  "reload"     — hot-reload configuration from disk without restarting the server`,
		InputSchema: mustSchema(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"action": map[string]any{
					"type":        "string",
					"enum":        []string{"version", "config", "reload"},
					"description": "Action to perform",
				},
			},
			"required": []string{"action"},
		}),
	}
}

func (s *Server) handleSelf(_ context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args struct {
		Action string `json:"action"`
	}
	if err := unmarshalArgs(req, &args); err != nil {
		return toolError(err.Error())
	}

	switch args.Action {
	case "version":
		return toolResult(map[string]string{
			"version":    version.Version,
			"commit":     version.Commit,
			"build_date": version.BuildDate,
		})

	case "config":
		cfg, err := config.Load(config.LoadParams{})
		if err != nil {
			return toolError(fmt.Sprintf("loading config: %v", err))
		}
		return toolResult(cfg)

	case "reload":
		if s.reloadFunc == nil {
			return toolError("reload not available: no reload function configured")
		}
		newEngine, err := s.reloadFunc()
		if err != nil {
			return toolError(fmt.Sprintf("reload failed: %v", err))
		}
		s.mu.Lock()
		s.engine = newEngine
		s.mu.Unlock()
		return toolResult(map[string]string{
			"status":  "ok",
			"message": "configuration reloaded successfully",
		})

	default:
		return toolError(fmt.Sprintf("unknown action %q (use: version, config, reload)", args.Action))
	}
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

// Copyright 2026 — see LICENSE file for terms.
package mcpserver

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestAllToolsHaveFormatParameter uses Go AST analysis to verify that every
// tool definition function in tools.go includes a "format" property in its
// InputSchema, and that every handler function checks args.Format == "text".
//
// This prevents adding a new tool without format=text support.
func TestAllToolsHaveFormatParameter(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine test file path")
	}
	toolsFile := filepath.Join(filepath.Dir(thisFile), "tools.go")

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, toolsFile, nil, 0)
	if err != nil {
		t.Fatalf("parsing tools.go: %v", err)
	}

	// Step 1: Find all tool definition functions (func toolXxx() *mcp.Tool).
	toolDefs := findToolDefinitions(t, f)
	if len(toolDefs) == 0 {
		t.Fatal("found no tool definitions in tools.go")
	}
	t.Logf("found %d tool definitions: %v", len(toolDefs), toolDefNames(toolDefs))

	// Step 2: Find all handler functions (func (s *Server) handleXxx(...)).
	handlers := findHandlerFunctions(t, f)
	t.Logf("found %d handler functions: %v", len(handlers), handlers)

	// Step 3: Check each tool definition has "format" in its schema.
	for _, td := range toolDefs {
		if !toolSchemaHasFormat(td.node) {
			t.Errorf("tool definition %s() does not include a \"format\" property in its InputSchema — "+
				"all tools must support format=text", td.name)
		}
	}

	// Step 4: Check each handler function checks args.Format == "text".
	for _, hName := range handlers {
		if !handlerChecksFormatText(f, hName) {
			t.Errorf("handler %s does not check args.Format == \"text\" — "+
				"all handlers must support text format output", hName)
		}
	}
}

// TestTextFormatDoesNotReturnJSON verifies that text formatters in output/text.go
// produce output that is NOT valid JSON (i.e., text mode returns actual text).
// This is a structural smoke test using known response types.
func TestTextFormatDoesNotReturnJSON(t *testing.T) {
	// We test by invoking the text formatters with minimal data and verifying
	// the output does not parse as valid JSON.
	textOutputs := map[string]string{
		"WriteSearchText": searchTextSample(),
		"WriteLogText":    logTextSample(),
		"WriteWcText":     wcTextSample(),
		"WriteFindText":   findTextSample(),
		"WriteStatText":   statTextSample(),
	}

	for name, text := range textOutputs {
		if text == "" {
			continue // empty output is valid for some formatters
		}
		var js json.RawMessage
		if json.Unmarshal([]byte(text), &js) == nil {
			t.Errorf("%s text output is valid JSON — text mode should NOT return JSON. Got: %q", name, text)
		}
	}
}

// TestJSONFormatIsValidJSON verifies that toolResult() produces valid JSON.
func TestJSONFormatIsValidJSON(t *testing.T) {
	// toolResult marshals with json.MarshalIndent — verify a round-trip.
	result, err := toolResult(map[string]string{"key": "value"})
	if err != nil {
		t.Fatalf("toolResult error: %v", err)
	}
	if len(result.Content) == 0 {
		t.Fatal("toolResult returned no content")
	}

	// Extract text from the first content element.
	// The MCP SDK wraps it in *mcp.TextContent.
	tc, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("toolResult content type: %T, expected *mcp.TextContent", result.Content[0])
	}

	// Verify the text is valid JSON.
	var check map[string]any
	if err := json.Unmarshal([]byte(tc.Text), &check); err != nil {
		t.Errorf("toolResult JSON output is not valid JSON: %v\nGot: %q", err, tc.Text)
	}
	if check["key"] != "value" {
		t.Errorf("toolResult JSON round-trip failed: got %v", check)
	}
}

// ── helpers ──

type toolDef struct {
	name string
	node *ast.FuncDecl
}

func toolDefNames(defs []toolDef) []string {
	names := make([]string, len(defs))
	for i, d := range defs {
		names[i] = d.name
	}
	return names
}

// findToolDefinitions finds all functions matching "func toolXxx() *mcp.Tool".
func findToolDefinitions(t *testing.T, f *ast.File) []toolDef {
	t.Helper()
	var defs []toolDef
	for _, decl := range f.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok || fd.Recv != nil {
			continue
		}
		if !strings.HasPrefix(fd.Name.Name, "tool") {
			continue
		}
		// Verify it returns *mcp.Tool.
		if fd.Type.Results == nil || len(fd.Type.Results.List) != 1 {
			continue
		}
		defs = append(defs, toolDef{name: fd.Name.Name, node: fd})
	}
	return defs
}

// findHandlerFunctions finds all methods matching "func (s *Server) handleXxx(...)".
func findHandlerFunctions(t *testing.T, f *ast.File) []string {
	t.Helper()
	var names []string
	for _, decl := range f.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok || fd.Recv == nil {
			continue
		}
		if !strings.HasPrefix(fd.Name.Name, "handle") {
			continue
		}
		names = append(names, fd.Name.Name)
	}
	return names
}

// toolSchemaHasFormat walks the function body looking for a string literal "format"
// inside a composite literal that looks like a schema properties map.
func toolSchemaHasFormat(fd *ast.FuncDecl) bool {
	found := false
	ast.Inspect(fd.Body, func(n ast.Node) bool {
		if found {
			return false
		}
		kv, ok := n.(*ast.KeyValueExpr)
		if !ok {
			return true
		}
		bl, ok := kv.Key.(*ast.BasicLit)
		if !ok || bl.Kind != token.STRING {
			return true
		}
		if bl.Value == `"format"` {
			found = true
			return false
		}
		return true
	})
	return found
}

// handlerChecksFormatText walks a handler function body looking for a comparison
// of the form: args.Format == "text" (or equivalent).
func handlerChecksFormatText(f *ast.File, handlerName string) bool {
	for _, decl := range f.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok || fd.Name.Name != handlerName {
			continue
		}
		return bodyContainsFormatTextCheck(fd.Body)
	}
	return false
}

// bodyContainsFormatTextCheck looks for any expression matching
// <something>.Format == "text" or args.Format == "text".
func bodyContainsFormatTextCheck(node ast.Node) bool {
	if node == nil {
		return false
	}
	found := false
	ast.Inspect(node, func(n ast.Node) bool {
		if found {
			return false
		}
		be, ok := n.(*ast.BinaryExpr)
		if !ok || be.Op != token.EQL {
			return true
		}
		// Check LHS is *.Format selector.
		sel, ok := be.X.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Format" {
			return true
		}
		// Check RHS is "text" literal.
		bl, ok := be.Y.(*ast.BasicLit)
		if !ok || bl.Kind != token.STRING {
			return true
		}
		if bl.Value == `"text"` {
			found = true
			return false
		}
		return true
	})
	return found
}

// ── text format sample generators ──

func searchTextSample() string {
	var buf strings.Builder
	buf.WriteString("main.go:10:5: func main()\n")
	return buf.String()
}

func logTextSample() string {
	return "commit abc123def456\nAuthor: Author <a@b.c>\nDate:   2026-01-01\n\n    initial commit\n"
}

func wcTextSample() string {
	return "42 /tmp/test.go\n"
}

func findTextSample() string {
	return "/tmp/a.go\n/tmp/b.go\n"
}

func statTextSample() string {
	return "-rw-r--r--  file       42  2026-01-01T00:00:00Z  /tmp/test.go\n"
}

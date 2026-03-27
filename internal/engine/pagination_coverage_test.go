// Copyright 2026 — see LICENSE file for terms.
package engine

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestAllPaginatingToolsHavePaginationTests uses Go AST analysis to ensure
// every response type in pkg/protocol that has a "Complete bool" field has
// at least one test in internal/engine that checks the .Complete field.
//
// This prevents adding a new paginating tool without corresponding test
// coverage for its pagination behavior.
func TestAllPaginatingToolsHavePaginationTests(t *testing.T) {
	// Determine paths relative to this test file.
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine test file path")
	}
	engineDir := filepath.Dir(thisFile)
	protocolDir := filepath.Join(engineDir, "..", "..", "pkg", "protocol")

	// Step 1: Find all response structs with a "Complete bool" field.
	paginatingTypes := findPaginatingTypes(t, protocolDir)
	if len(paginatingTypes) == 0 {
		t.Fatal("found no response types with Complete bool field — check protocolDir path")
	}
	t.Logf("found %d paginating response types: %v", len(paginatingTypes), paginatingTypes)

	// Step 2: Find all test functions that reference .Complete in their body.
	testFuncsWithComplete := findTestFuncsCheckingComplete(t, engineDir)
	t.Logf("found %d test functions checking .Complete: %v", len(testFuncsWithComplete), testFuncsWithComplete)

	// Step 3: For each paginating type, verify coverage.
	for _, typeName := range paginatingTypes {
		toolName := strings.TrimSuffix(typeName, "Response")
		if !hasMatchingTest(toolName, testFuncsWithComplete) {
			t.Errorf("paginating type %s (tool prefix %q) has no test that checks .Complete — "+
				"add a test whose name contains %q and that asserts on the .Complete field",
				typeName, toolName, toolName)
		}
	}
}

// findPaginatingTypes parses all Go files in dir and returns the names of
// struct types that have both a "Complete bool" field and a "Continuation string"
// field — indicating they support pagination with resumption.
func findPaginatingTypes(t *testing.T, dir string) []string {
	t.Helper()
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, nil, 0)
	if err != nil {
		t.Fatalf("parsing %s: %v", dir, err)
	}

	var types []string
	for _, pkg := range pkgs {
		for _, file := range pkg.Files {
			for _, decl := range file.Decls {
				gd, ok := decl.(*ast.GenDecl)
				if !ok || gd.Tok != token.TYPE {
					continue
				}
				for _, spec := range gd.Specs {
					ts, ok := spec.(*ast.TypeSpec)
					if !ok {
						continue
					}
					st, ok := ts.Type.(*ast.StructType)
					if !ok {
						continue
					}
					hasComplete := false
					hasContinuation := false
					for _, field := range st.Fields.List {
						if len(field.Names) == 0 {
							continue
						}
						name := field.Names[0].Name
						if name == "Complete" {
							if ident, ok := field.Type.(*ast.Ident); ok && ident.Name == "bool" {
								hasComplete = true
							}
						}
						if name == "Continuation" {
							if ident, ok := field.Type.(*ast.Ident); ok && ident.Name == "string" {
								hasContinuation = true
							}
						}
					}
					if hasComplete && hasContinuation {
						types = append(types, ts.Name.Name)
					}
				}
			}
		}
	}
	return types
}

// findTestFuncsCheckingComplete parses all _test.go files in dir and returns
// the names of Test* functions whose body contains a selector expression
// accessing ".Complete".
func findTestFuncsCheckingComplete(t *testing.T, dir string) []string {
	t.Helper()
	fset := token.NewFileSet()
	// ParseDir with filter to include test files.
	pkgs, err := parser.ParseDir(fset, dir, nil, 0)
	if err != nil {
		t.Fatalf("parsing %s: %v", dir, err)
	}

	var funcs []string
	for _, pkg := range pkgs {
		for _, file := range pkg.Files {
			for _, decl := range file.Decls {
				fd, ok := decl.(*ast.FuncDecl)
				if !ok || !strings.HasPrefix(fd.Name.Name, "Test") {
					continue
				}
				if bodyReferencesComplete(fd.Body) {
					funcs = append(funcs, fd.Name.Name)
				}
			}
		}
	}
	return funcs
}

// bodyReferencesComplete walks an AST node and returns true if any
// SelectorExpr has Sel.Name == "Complete".
func bodyReferencesComplete(node ast.Node) bool {
	if node == nil {
		return false
	}
	found := false
	ast.Inspect(node, func(n ast.Node) bool {
		if found {
			return false
		}
		sel, ok := n.(*ast.SelectorExpr)
		if ok && sel.Sel.Name == "Complete" {
			found = true
			return false
		}
		return true
	})
	return found
}

// hasMatchingTest checks if any test function name contains the tool prefix
// (case-insensitive).
func hasMatchingTest(toolPrefix string, testFuncs []string) bool {
	lower := strings.ToLower(toolPrefix)
	for _, fn := range testFuncs {
		if strings.Contains(strings.ToLower(fn), lower) {
			return true
		}
	}
	return false
}

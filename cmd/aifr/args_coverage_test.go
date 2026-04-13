// Copyright 2026 — see LICENSE file for terms.
package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestVariadicCommandsUseFullArgs uses Go AST analysis to verify that every
// cobra command declared with MinimumNArgs(1) passes the full `args` slice
// to functions, rather than subscripting `args[0]` and silently dropping
// extra positional arguments.
//
// This prevents a class of bug where shell glob expansion (e.g. nats-pi-*)
// produces multiple arguments but only the first is processed.
func TestVariadicCommandsUseFullArgs(t *testing.T) {
	cmdFiles, err := filepath.Glob(filepath.Join(".", "cmd_*.go"))
	if err != nil {
		t.Fatal(err)
	}

	// If running from the repo root, try the cmd/aifr directory.
	if len(cmdFiles) == 0 {
		cmdFiles, err = filepath.Glob(filepath.Join("cmd", "aifr", "cmd_*.go"))
		if err != nil {
			t.Fatal(err)
		}
	}

	if len(cmdFiles) == 0 {
		t.Fatal("found no cmd_*.go files")
	}

	fset := token.NewFileSet()

	for _, path := range cmdFiles {
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading %s: %v", path, err)
		}

		f, err := parser.ParseFile(fset, path, src, 0)
		if err != nil {
			t.Fatalf("parsing %s: %v", path, err)
		}

		checkVariadicCommands(t, fset, f, path)
	}
}

// checkVariadicCommands finds cobra.Command declarations in the file that
// use MinimumNArgs(1), then verifies their RunE bodies do not subscript
// args[0] (which would silently discard extra positional arguments).
func checkVariadicCommands(t *testing.T, fset *token.FileSet, f *ast.File, filename string) {
	t.Helper()

	// Walk all top-level variable declarations to find cobra commands.
	for _, decl := range f.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.VAR {
			continue
		}
		for _, spec := range genDecl.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok || len(vs.Values) != 1 {
				continue
			}

			// Look for &cobra.Command{...} composite literals.
			unary, ok := vs.Values[0].(*ast.UnaryExpr)
			if !ok || unary.Op != token.AND {
				continue
			}
			compLit, ok := unary.X.(*ast.CompositeLit)
			if !ok {
				continue
			}

			if !isCobraCommandType(compLit.Type) {
				continue
			}

			varName := vs.Names[0].Name

			if !hasMinimumNArgs1(compLit) {
				continue
			}

			// Found a variadic command. Check its RunE body.
			runE := findRunEBody(compLit)
			if runE == nil {
				continue
			}

			if containsArgsSubscript(runE) {
				t.Errorf("%s: command %q uses MinimumNArgs(1) but subscripts args[N] "+
					"in its RunE — this silently drops extra positional arguments. "+
					"Pass the full args slice instead.",
					filename, varName)
			}
		}
	}
}

// isCobraCommandType checks if a type expression refers to cobra.Command.
func isCobraCommandType(expr ast.Expr) bool {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	ident, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	return ident.Name == "cobra" && sel.Sel.Name == "Command"
}

// hasMinimumNArgs1 checks if a cobra.Command composite literal contains
// Args: cobra.MinimumNArgs(1).
func hasMinimumNArgs1(lit *ast.CompositeLit) bool {
	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		keyIdent, ok := kv.Key.(*ast.Ident)
		if !ok || keyIdent.Name != "Args" {
			continue
		}

		// Check for cobra.MinimumNArgs(1).
		call, ok := kv.Value.(*ast.CallExpr)
		if !ok {
			continue
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			continue
		}
		pkg, ok := sel.X.(*ast.Ident)
		if !ok {
			continue
		}
		if pkg.Name != "cobra" || sel.Sel.Name != "MinimumNArgs" {
			continue
		}
		if len(call.Args) == 1 {
			bl, ok := call.Args[0].(*ast.BasicLit)
			if ok && bl.Value == "1" {
				return true
			}
		}
	}
	return false
}

// findRunEBody finds the RunE field's function literal body in a cobra.Command
// composite literal.
func findRunEBody(lit *ast.CompositeLit) *ast.BlockStmt {
	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		keyIdent, ok := kv.Key.(*ast.Ident)
		if !ok || keyIdent.Name != "RunE" {
			continue
		}
		funcLit, ok := kv.Value.(*ast.FuncLit)
		if !ok {
			continue
		}
		return funcLit.Body
	}
	return nil
}

// containsArgsSubscript walks an AST node looking for index expressions of
// the form args[N] where args is an identifier and N is any expression.
func containsArgsSubscript(node ast.Node) bool {
	found := false
	ast.Inspect(node, func(n ast.Node) bool {
		if found {
			return false
		}
		indexExpr, ok := n.(*ast.IndexExpr)
		if !ok {
			return true
		}
		ident, ok := indexExpr.X.(*ast.Ident)
		if !ok {
			return true
		}
		if strings.EqualFold(ident.Name, "args") {
			found = true
			return false
		}
		return true
	})
	return found
}

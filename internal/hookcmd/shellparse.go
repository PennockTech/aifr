// Copyright 2026 — see LICENSE file for terms.

// Package hookcmd implements command analysis for AI coding agent hooks.
// It parses shell commands from hook payloads and suggests aifr alternatives
// when the command can be safely handled by aifr.
package hookcmd

import (
	"path/filepath"
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

// parsedCommand represents a shell command extracted from a parsed AST.
type parsedCommand struct {
	Name string   // base command name (e.g., "cat", "grep")
	Args []string // argument values (unquoted where possible)
}

// parseShellCommand parses a shell command string into a primary command and
// an optional pipeline modifier. Returns nil if the command is too complex
// to analyze (multiple statements, subshells, control operators, etc.).
//
// Two-stage pipelines where the second stage is head or tail are recognized
// and returned as a modifier on the first stage.
func parseShellCommand(command string) (*parsedCommand, PipelineModifier) {
	parser := syntax.NewParser(syntax.Variant(syntax.LangBash))
	file, err := parser.Parse(strings.NewReader(command), "")
	if err != nil {
		return nil, PipelineModifier{}
	}

	if len(file.Stmts) != 1 {
		return nil, PipelineModifier{}
	}

	stmt := file.Stmts[0]
	if stmt.Background || stmt.Negated || stmt.Coprocess {
		return nil, PipelineModifier{}
	}

	switch cmd := stmt.Cmd.(type) {
	case *syntax.CallExpr:
		parsed := extractCall(cmd)
		return parsed, PipelineModifier{}

	case *syntax.BinaryCmd:
		if cmd.Op != syntax.Pipe {
			return nil, PipelineModifier{} // &&, ||
		}
		return parsePipelineCmd(cmd)

	default:
		// if, for, while, case, subshell, function decl, etc.
		return nil, PipelineModifier{}
	}
}

// parsePipelineCmd handles a two-stage pipeline (cmd | head/tail).
// Returns nil for 3+ stage pipelines or when the right side isn't head/tail.
func parsePipelineCmd(bc *syntax.BinaryCmd) (*parsedCommand, PipelineModifier) {
	// Both sides must be simple commands (not nested pipelines or control structures).
	leftCall, ok := bc.X.Cmd.(*syntax.CallExpr)
	if !ok {
		return nil, PipelineModifier{}
	}
	rightCall, ok := bc.Y.Cmd.(*syntax.CallExpr)
	if !ok {
		return nil, PipelineModifier{}
	}

	right := extractCall(rightCall)
	if right == nil {
		return nil, PipelineModifier{}
	}

	mod := pipeTailModifier(right)
	if !mod.IsSet() {
		return nil, PipelineModifier{}
	}

	left := extractCall(leftCall)
	return left, mod
}

// pipeTailModifier checks if a parsed command is head or tail and extracts
// the line count as a PipelineModifier.
func pipeTailModifier(cmd *parsedCommand) PipelineModifier {
	switch cmd.Name {
	case "head":
		return PipelineModifier{HeadLines: parseHeadTailN(cmd.Args, 10)}
	case "tail":
		if hasFlag(cmd.Args, "-f", "--follow", "-F") {
			return PipelineModifier{}
		}
		return PipelineModifier{TailLines: parseHeadTailN(cmd.Args, 10)}
	default:
		return PipelineModifier{}
	}
}

// extractCall extracts a parsedCommand from a CallExpr AST node.
// Variable assignments (LANG=C cmd) are automatically excluded since the
// parser places them in CallExpr.Assigns, not Args.
func extractCall(ce *syntax.CallExpr) *parsedCommand {
	if len(ce.Args) == 0 {
		return nil
	}

	name := wordValue(ce.Args[0])
	if name == "" {
		return nil
	}

	args := make([]string, len(ce.Args)-1)
	for i, w := range ce.Args[1:] {
		args[i] = wordValue(w)
	}

	return &parsedCommand{
		Name: filepath.Base(name),
		Args: args,
	}
}

// wordValue extracts the effective string value from a shell Word,
// stripping quotes where possible. For words containing parameter expansions
// or command substitutions, falls back to the printed shell representation.
func wordValue(w *syntax.Word) string {
	if s := w.Lit(); s != "" {
		return s
	}

	if len(w.Parts) == 1 {
		switch p := w.Parts[0].(type) {
		case *syntax.Lit:
			return p.Value
		case *syntax.SglQuoted:
			return p.Value
		case *syntax.DblQuoted:
			return dblQuotedLiteral(p)
		}
	}

	var buf strings.Builder
	syntax.NewPrinter().Print(&buf, w)
	return buf.String()
}

// dblQuotedLiteral extracts the literal content of a double-quoted string.
// If the string contains expansions, falls back to printer output.
func dblQuotedLiteral(dq *syntax.DblQuoted) string {
	var sb strings.Builder
	for _, p := range dq.Parts {
		lit, ok := p.(*syntax.Lit)
		if !ok {
			var buf strings.Builder
			syntax.NewPrinter().Print(&buf, dq)
			return buf.String()
		}
		sb.WriteString(lit.Value)
	}
	return sb.String()
}

// nonFlags returns elements of args that don't start with '-'.
// Redirections are not present in args (the parser handles them separately).
func nonFlags(args []string) []string {
	var out []string
	for _, t := range args {
		if !strings.HasPrefix(t, "-") {
			out = append(out, t)
		}
	}
	return out
}

// hasFlag reports whether any element of tokens exactly matches one of the given flags.
func hasFlag(tokens []string, flags ...string) bool {
	for _, t := range tokens {
		for _, f := range flags {
			if t == f {
				return true
			}
		}
	}
	return false
}

// shellQuote returns s quoted for shell use if it contains special characters.
func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	safe := true
	for _, r := range s {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') ||
			r == '/' || r == '.' || r == '_' || r == '-' || r == ':' || r == '~' || r == '+' || r == '@') {
			safe = false
			break
		}
	}
	if safe {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

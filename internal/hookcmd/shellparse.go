// Copyright 2026 — see LICENSE file for terms.

// Package hookcmd implements command analysis for AI coding agent hooks.
// It parses shell commands from hook payloads and suggests aifr alternatives
// when the command can be safely handled by aifr.
package hookcmd

import "strings"

// tokenize splits a shell command into tokens, handling single quotes,
// double quotes, and backslash escaping. It does not expand variables or globs.
func tokenize(s string) []string {
	var tokens []string
	var cur strings.Builder
	inSingle := false
	inDouble := false
	escaped := false

	for _, r := range s {
		if escaped {
			cur.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' && !inSingle {
			escaped = true
			continue
		}
		if r == '\'' && !inDouble {
			inSingle = !inSingle
			continue
		}
		if r == '"' && !inSingle {
			inDouble = !inDouble
			continue
		}
		if (r == ' ' || r == '\t') && !inSingle && !inDouble {
			if cur.Len() > 0 {
				tokens = append(tokens, cur.String())
				cur.Reset()
			}
			continue
		}
		cur.WriteRune(r)
	}
	if cur.Len() > 0 {
		tokens = append(tokens, cur.String())
	}
	return tokens
}

// hasShellOperators reports whether the command contains shell operators
// (pipes, chains, backgrounding) outside of quoted strings.
// Commands with these operators are too complex for simple replacement.
func hasShellOperators(s string) bool {
	inSingle := false
	inDouble := false
	escaped := false
	runes := []rune(s)

	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if escaped {
			escaped = false
			continue
		}
		if r == '\\' && !inSingle {
			escaped = true
			continue
		}
		if r == '\'' && !inDouble {
			inSingle = !inSingle
			continue
		}
		if r == '"' && !inSingle {
			inDouble = !inDouble
			continue
		}
		if inSingle || inDouble {
			continue
		}
		switch r {
		case '|':
			return true
		case ';':
			return true
		case '&':
			return true
		case '(':
			return true
		case '`':
			return true
		case '\n':
			return true
		}
		// $( subshell
		if r == '$' && i+1 < len(runes) && runes[i+1] == '(' {
			return true
		}
	}
	return false
}

// splitPipeline splits a command by unquoted pipe operators.
// It recognizes || as a logical OR (kept in the stage, not split) and only
// splits on single | pipe operators.
func splitPipeline(s string) []string {
	var stages []string
	var cur strings.Builder
	inSingle := false
	inDouble := false
	escaped := false
	runes := []rune(s)

	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if escaped {
			cur.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' && !inSingle {
			cur.WriteRune(r)
			escaped = true
			continue
		}
		if r == '\'' && !inDouble {
			inSingle = !inSingle
			cur.WriteRune(r)
			continue
		}
		if r == '"' && !inSingle {
			inDouble = !inDouble
			cur.WriteRune(r)
			continue
		}
		if r == '|' && !inSingle && !inDouble {
			// || is logical OR, not a pipe — keep in current stage.
			if i+1 < len(runes) && runes[i+1] == '|' {
				cur.WriteRune(r)
				cur.WriteRune(runes[i+1])
				i++
				continue
			}
			stages = append(stages, cur.String())
			cur.Reset()
			continue
		}
		cur.WriteRune(r)
	}
	if cur.Len() > 0 {
		stages = append(stages, cur.String())
	}
	return stages
}

// baseName returns the last path component of cmd (strips directory prefix).
func baseName(cmd string) string {
	if idx := strings.LastIndex(cmd, "/"); idx >= 0 {
		return cmd[idx+1:]
	}
	return cmd
}

// nonFlags returns tokens that don't start with '-', skipping redirections.
func nonFlags(tokens []string) []string {
	var out []string
	skipNext := false
	for _, t := range tokens {
		if skipNext {
			skipNext = false
			continue
		}
		// Skip output/input redirections and their targets.
		if t == ">" || t == ">>" || t == "<" || t == "<<" || t == "2>" || t == "2>>" || t == "&>" {
			skipNext = true
			continue
		}
		// Skip tokens that are redirection targets embedded with operator.
		if len(t) > 1 && (t[0] == '>' || (t[0] == '2' && len(t) > 2 && t[1] == '>')) {
			continue
		}
		if strings.HasPrefix(t, "-") {
			continue
		}
		out = append(out, t)
	}
	return out
}

// hasFlag reports whether any token exactly matches one of the given flags.
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

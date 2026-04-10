// Copyright 2026 — see LICENSE file for terms.
package hookcmd

import (
	"fmt"
	"strconv"
	"strings"
)

// Suggestion represents an aifr command that can replace a shell command.
type Suggestion struct {
	// Original is the original shell command (or base command name).
	Original string
	// AifrCommand is the suggested aifr invocation.
	AifrCommand string
}

// AnalyzeCommand checks if a shell command can be replaced by an aifr command.
// Returns nil if no suggestion applies.
func AnalyzeCommand(command string) *Suggestion {
	command = strings.TrimSpace(command)
	if command == "" {
		return nil
	}

	// Already an aifr invocation — nothing to suggest.
	if strings.HasPrefix(command, "aifr ") || command == "aifr" {
		return nil
	}

	// Skip complex commands with pipes, chains, or subshells.
	if hasShellOperators(command) {
		return nil
	}

	tokens := tokenize(command)
	if len(tokens) == 0 {
		return nil
	}

	// Skip environment variable assignments (VAR=value cmd ...).
	start := 0
	for start < len(tokens) && strings.Contains(tokens[start], "=") && !strings.HasPrefix(tokens[start], "-") {
		start++
	}
	if start >= len(tokens) {
		return nil
	}
	tokens = tokens[start:]

	baseCmd := baseName(tokens[0])
	args := tokens[1:]

	switch baseCmd {
	case "cat":
		return suggestCat(args)
	case "head":
		return suggestHead(args)
	case "tail":
		return suggestTail(args)
	case "grep", "egrep", "fgrep", "rg":
		return suggestSearch(baseCmd, args)
	case "find":
		return suggestFind(args)
	case "ls":
		return suggestList(args)
	case "wc":
		return suggestWc(args)
	case "stat":
		return suggestStat(args)
	case "diff":
		return suggestDiff(args)
	case "sha256sum", "sha1sum", "md5sum", "shasum", "sha384sum", "sha512sum", "b2sum":
		return suggestChecksum(baseCmd, args)
	case "hexdump", "xxd", "od":
		return suggestHexdump(baseCmd, args)
	case "sed":
		return suggestSed(args)
	case "git":
		return suggestGit(args)
	default:
		return nil
	}
}

func makeSuggestion(original, aifrCmd string) *Suggestion {
	return &Suggestion{
		Original:    original,
		AifrCommand: aifrCmd,
	}
}

func suggestCat(args []string) *Suggestion {
	files := nonFlags(args)
	if len(files) == 0 {
		return nil // reading from stdin
	}
	if len(files) == 1 {
		return makeSuggestion("cat", "aifr read "+shellQuote(files[0]))
	}
	parts := make([]string, len(files))
	for i, f := range files {
		parts[i] = shellQuote(f)
	}
	return makeSuggestion("cat", "aifr cat "+strings.Join(parts, " "))
}

func suggestHead(args []string) *Suggestion {
	n := 10 // default for head
	var file string

	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-n" && i+1 < len(args):
			if v, err := strconv.Atoi(args[i+1]); err == nil {
				n = v
			}
			i++
		case strings.HasPrefix(a, "-n"):
			if v, err := strconv.Atoi(a[2:]); err == nil {
				n = v
			}
		case len(a) > 1 && a[0] == '-' && isDigits(a[1:]):
			if v, err := strconv.Atoi(a[1:]); err == nil {
				n = v
			}
		case !strings.HasPrefix(a, "-"):
			if file == "" {
				file = a
			}
		}
	}
	if file == "" {
		return nil
	}
	return makeSuggestion("head", fmt.Sprintf("aifr read --lines=1:%d %s", n, shellQuote(file)))
}

func suggestTail(args []string) *Suggestion {
	if hasFlag(args, "-f", "--follow", "-F") {
		return nil // tail -f is a live-follow, aifr can't do that
	}

	n := 10 // default for tail
	var file string

	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-n" && i+1 < len(args):
			if v, err := strconv.Atoi(args[i+1]); err == nil {
				n = v
			}
			i++
		case strings.HasPrefix(a, "-n"):
			if v, err := strconv.Atoi(a[2:]); err == nil {
				n = v
			}
		case len(a) > 1 && a[0] == '-' && isDigits(a[1:]):
			if v, err := strconv.Atoi(a[1:]); err == nil {
				n = v
			}
		case !strings.HasPrefix(a, "-"):
			if file == "" {
				file = a
			}
		}
	}
	if file == "" {
		return nil
	}
	return makeSuggestion("tail", fmt.Sprintf("aifr read --lines=-%d: %s", n, shellQuote(file)))
}

func suggestSearch(baseCmd string, args []string) *Suggestion {
	// Extract pattern and path from grep/rg arguments.
	// We handle: grep [flags] pattern [path...]
	// Skip if reading from stdin (no path and no -r/-R).
	var pattern string
	var path string
	recursive := false

	positional := 0
	for i := 0; i < len(args); i++ {
		a := args[i]
		if strings.HasPrefix(a, "-") {
			// Check for recursive flags
			if a == "-r" || a == "-R" || a == "--recursive" {
				recursive = true
				continue
			}
			// Flags that take a value argument
			if flagTakesValue(a) && i+1 < len(args) {
				i++
			}
			continue
		}
		switch positional {
		case 0:
			pattern = a
		case 1:
			path = a
		}
		positional++
	}

	if pattern == "" {
		return nil
	}

	// If no path and not recursive, it's likely reading stdin.
	if path == "" && !recursive {
		return nil
	}

	cmd := "aifr search " + shellQuote(pattern)
	if path != "" {
		cmd += " " + shellQuote(path)
	} else {
		cmd += " ."
	}
	return makeSuggestion(baseCmd, cmd)
}

func suggestFind(args []string) *Suggestion {
	// find [path] [expressions...]
	// Common: find . -name "*.go" -type f
	var path string
	var name string
	var ftype string

	for i := 0; i < len(args); i++ {
		a := args[i]
		switch a {
		case "-name", "-iname":
			if i+1 < len(args) {
				name = args[i+1]
				i++
			}
		case "-type":
			if i+1 < len(args) {
				ftype = args[i+1]
				i++
			}
		default:
			if !strings.HasPrefix(a, "-") && path == "" {
				path = a
			}
		}
	}

	if path == "" {
		path = "."
	}

	cmd := "aifr find " + shellQuote(path)
	if name != "" {
		cmd += " --name=" + shellQuote(name)
	}
	if ftype != "" {
		cmd += " --type=" + ftype
	}
	return makeSuggestion("find", cmd)
}

func suggestList(args []string) *Suggestion {
	// ls [flags] [path...]
	files := nonFlags(args)
	path := "."
	if len(files) > 0 {
		path = files[0]
	}
	// If using complex ls flags beyond basic listing, skip.
	if hasFlag(args, "-R", "--recursive") {
		return makeSuggestion("ls", "aifr find "+shellQuote(path))
	}
	return makeSuggestion("ls", "aifr list "+shellQuote(path))
}

func suggestWc(args []string) *Suggestion {
	files := nonFlags(args)
	if len(files) == 0 {
		return nil // reading from stdin
	}
	parts := make([]string, len(files))
	for i, f := range files {
		parts[i] = shellQuote(f)
	}
	cmd := "aifr wc"
	if hasFlag(args, "-l") {
		cmd += " -l"
	}
	if hasFlag(args, "-w") {
		cmd += " -w"
	}
	if hasFlag(args, "-c") {
		cmd += " -c"
	}
	if hasFlag(args, "-m") {
		cmd += " -m"
	}
	cmd += " " + strings.Join(parts, " ")
	return makeSuggestion("wc", cmd)
}

func suggestStat(args []string) *Suggestion {
	files := nonFlags(args)
	if len(files) == 0 {
		return nil
	}
	return makeSuggestion("stat", "aifr stat "+shellQuote(files[0]))
}

func suggestDiff(args []string) *Suggestion {
	files := nonFlags(args)
	if len(files) != 2 {
		return nil // diff needs exactly 2 paths for aifr
	}
	return makeSuggestion("diff", "aifr diff "+shellQuote(files[0])+" "+shellQuote(files[1]))
}

func suggestChecksum(baseCmd string, args []string) *Suggestion {
	files := nonFlags(args)
	if len(files) == 0 {
		return nil
	}

	algo := "sha256"
	switch baseCmd {
	case "sha1sum":
		algo = "sha1"
	case "sha256sum":
		algo = "sha256"
	case "sha384sum":
		algo = "sha384"
	case "sha512sum":
		algo = "sha512"
	case "md5sum":
		algo = "md5"
	case "b2sum":
		// aifr may not support blake2; skip.
		return nil
	case "shasum":
		// shasum defaults to sha1; check for -a flag.
		algo = "sha1"
		for i := 0; i < len(args); i++ {
			if args[i] == "-a" && i+1 < len(args) {
				algo = "sha" + args[i+1]
				i++
			}
		}
	}

	parts := make([]string, len(files))
	for i, f := range files {
		parts[i] = shellQuote(f)
	}
	return makeSuggestion(baseCmd, fmt.Sprintf("aifr checksum -a %s %s", algo, strings.Join(parts, " ")))
}

func suggestHexdump(_ string, args []string) *Suggestion {
	files := nonFlags(args)
	if len(files) == 0 {
		return nil
	}
	return makeSuggestion("hexdump", "aifr hexdump "+shellQuote(files[0]))
}

func suggestSed(args []string) *Suggestion {
	// Only handle the common read-only pattern: sed -n 'NP' or sed -n 'N,Mp' file
	if !hasFlag(args, "-n") {
		return nil // without -n, sed may be doing transformations
	}

	var script string
	var file string
	sawN := false

	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "-n" {
			sawN = true
			continue
		}
		if strings.HasPrefix(a, "-") {
			continue
		}
		if !sawN {
			continue
		}
		if script == "" {
			script = a
		} else if file == "" {
			file = a
		}
	}

	if script == "" || file == "" {
		return nil
	}

	// Parse patterns like "5p", "10,20p", "5,10p"
	script = strings.TrimSuffix(script, "p")
	if script == "" {
		return nil
	}

	parts := strings.SplitN(script, ",", 2)
	if len(parts) == 1 {
		// Single line: sed -n '5p'
		if _, err := strconv.Atoi(parts[0]); err != nil {
			return nil
		}
		return makeSuggestion("sed",
			fmt.Sprintf("aifr read --lines=%s:%s %s", parts[0], parts[0], shellQuote(file)))
	}

	// Range: sed -n '5,10p'
	if _, err := strconv.Atoi(parts[0]); err != nil {
		return nil
	}
	if _, err := strconv.Atoi(parts[1]); err != nil {
		return nil
	}
	return makeSuggestion("sed",
		fmt.Sprintf("aifr read --lines=%s:%s %s", parts[0], parts[1], shellQuote(file)))
}

func suggestGit(args []string) *Suggestion {
	if len(args) == 0 {
		return nil
	}
	switch args[0] {
	case "log":
		return makeSuggestion("git log", "aifr log")
	case "diff":
		return suggestGitDiff(args[1:])
	default:
		return nil
	}
}

func suggestGitDiff(args []string) *Suggestion {
	// git diff [ref] [-- file...]
	// Only handle simple cases.
	refs := nonFlags(args)
	// Filter out "--" separator
	var clean []string
	for _, r := range refs {
		if r != "--" {
			clean = append(clean, r)
		}
	}
	if len(clean) == 2 {
		return makeSuggestion("git diff",
			fmt.Sprintf("aifr diff %s:%s %s:%s", clean[0], ".", clean[1], "."))
	}
	return makeSuggestion("git diff", "aifr diff")
}

// isDigits reports whether s is non-empty and contains only ASCII digits.
func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// flagTakesValue reports whether a grep/rg flag expects a following argument.
func flagTakesValue(flag string) bool {
	switch flag {
	case "-e", "-f", "--regexp", "--file",
		"-m", "--max-count",
		"-A", "--after-context",
		"-B", "--before-context",
		"-C", "--context",
		"--include", "--exclude", "--exclude-dir",
		"--color", "--colour",
		"-d", "--directories",
		"-D", "--devices",
		"--label",
		"--binary-files":
		return true
	}
	return false
}

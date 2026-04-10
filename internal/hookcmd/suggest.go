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
	// AifrCommand is the suggested aifr CLI invocation.
	AifrCommand string
	// ToolName is the MCP tool name (e.g., "aifr_read").
	ToolName string
	// ToolArgs are the MCP tool parameters.
	ToolArgs map[string]any
}

// PipelineModifier captures a trailing | head or | tail in a pipeline.
type PipelineModifier struct {
	HeadLines int // > 0 if piped to head -n N
	TailLines int // > 0 if piped to tail -n N
}

// IsSet reports whether any pipeline modifier is active.
func (m PipelineModifier) IsSet() bool {
	return m.HeadLines > 0 || m.TailLines > 0
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

	// Split pipeline and check for trailing head/tail.
	stages := splitPipeline(command)
	var mod PipelineModifier
	var baseStage string

	switch len(stages) {
	case 1:
		baseStage = stages[0]
	case 2:
		mod = parsePipeTail(stages[1])
		if !mod.IsSet() {
			return nil // unknown pipe target
		}
		baseStage = stages[0]
	default:
		return nil // 3+ stage pipelines are too complex
	}

	baseStage = strings.TrimSpace(baseStage)

	// Check for non-pipe shell operators in the base stage.
	if hasShellOperators(baseStage) {
		return nil
	}

	tokens := tokenize(baseStage)
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
		return suggestCat(args, mod)
	case "head":
		return suggestHead(args, mod)
	case "tail":
		return suggestTail(args, mod)
	case "grep", "egrep", "fgrep", "rg":
		return suggestSearch(baseCmd, args, mod)
	case "find":
		return suggestFind(args, mod)
	case "ls":
		return suggestList(args, mod)
	case "wc":
		return suggestWc(args, mod)
	case "stat":
		return suggestStat(args, mod)
	case "diff":
		return suggestDiff(args, mod)
	case "sha256sum", "sha1sum", "md5sum", "shasum", "sha384sum", "sha512sum", "b2sum":
		return suggestChecksum(baseCmd, args, mod)
	case "hexdump", "xxd", "od":
		return suggestHexdump(baseCmd, args, mod)
	case "sed":
		return suggestSed(args, mod)
	case "git":
		return suggestGit(args, mod)
	default:
		return nil
	}
}

// parsePipeTail parses the tail of a pipeline (the part after |) to detect
// head -n N or tail -n N patterns.
func parsePipeTail(stage string) PipelineModifier {
	stage = strings.TrimSpace(stage)
	tokens := tokenize(stage)
	if len(tokens) == 0 {
		return PipelineModifier{}
	}
	cmd := baseName(tokens[0])
	args := tokens[1:]

	switch cmd {
	case "head":
		return PipelineModifier{HeadLines: parseHeadTailN(args, 10)}
	case "tail":
		if hasFlag(args, "-f", "--follow", "-F") {
			return PipelineModifier{}
		}
		return PipelineModifier{TailLines: parseHeadTailN(args, 10)}
	default:
		return PipelineModifier{}
	}
}

// parseHeadTailN extracts the line count from head/tail arguments.
// Handles -n N, -nN, and -N forms. Returns defaultN if not found.
func parseHeadTailN(args []string, defaultN int) int {
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-n" && i+1 < len(args):
			if v, err := strconv.Atoi(args[i+1]); err == nil {
				return v
			}
			i++
		case strings.HasPrefix(a, "-n"):
			if v, err := strconv.Atoi(a[2:]); err == nil {
				return v
			}
		case len(a) > 1 && a[0] == '-' && isDigits(a[1:]):
			if v, err := strconv.Atoi(a[1:]); err == nil {
				return v
			}
		}
	}
	return defaultN
}

func makeSuggestion(original, aifrCmd, toolName string, toolArgs map[string]any) *Suggestion {
	return &Suggestion{
		Original:    original,
		AifrCommand: aifrCmd,
		ToolName:    toolName,
		ToolArgs:    toolArgs,
	}
}

func suggestCat(args []string, mod PipelineModifier) *Suggestion {
	files := nonFlags(args)
	if len(files) == 0 {
		return nil // reading from stdin
	}

	if mod.HeadLines > 0 {
		if len(files) != 1 {
			return nil // multi-file cat with head doesn't map cleanly
		}
		lines := fmt.Sprintf("1:%d", mod.HeadLines)
		return makeSuggestion("cat",
			fmt.Sprintf("aifr read --lines=%s %s", lines, shellQuote(files[0])),
			"aifr_read",
			map[string]any{"path": files[0], "lines": lines})
	}

	if mod.TailLines > 0 {
		if len(files) != 1 {
			return nil
		}
		lines := fmt.Sprintf("-%d:", mod.TailLines)
		return makeSuggestion("cat",
			fmt.Sprintf("aifr read --lines=%s %s", lines, shellQuote(files[0])),
			"aifr_read",
			map[string]any{"path": files[0], "lines": lines})
	}

	if len(files) == 1 {
		return makeSuggestion("cat",
			"aifr read "+shellQuote(files[0]),
			"aifr_read",
			map[string]any{"path": files[0]})
	}
	parts := make([]string, len(files))
	for i, f := range files {
		parts[i] = shellQuote(f)
	}
	return makeSuggestion("cat",
		"aifr cat "+strings.Join(parts, " "),
		"aifr_cat",
		map[string]any{"paths": files})
}

func suggestHead(args []string, mod PipelineModifier) *Suggestion {
	if mod.IsSet() {
		return nil // head | head or head | tail is unusual
	}

	n := 10
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
	lines := fmt.Sprintf("1:%d", n)
	return makeSuggestion("head",
		fmt.Sprintf("aifr read --lines=%s %s", lines, shellQuote(file)),
		"aifr_read",
		map[string]any{"path": file, "lines": lines})
}

func suggestTail(args []string, mod PipelineModifier) *Suggestion {
	if mod.IsSet() {
		return nil // tail | head or tail | tail is unusual
	}
	if hasFlag(args, "-f", "--follow", "-F") {
		return nil // tail -f is a live-follow, aifr can't do that
	}

	n := 10
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
	lines := fmt.Sprintf("-%d:", n)
	return makeSuggestion("tail",
		fmt.Sprintf("aifr read --lines=%s %s", lines, shellQuote(file)),
		"aifr_read",
		map[string]any{"path": file, "lines": lines})
}

func suggestSearch(baseCmd string, args []string, mod PipelineModifier) *Suggestion {
	if mod.TailLines > 0 {
		return nil
	}

	var pattern string
	var path string
	recursive := false

	positional := 0
	for i := 0; i < len(args); i++ {
		a := args[i]
		if strings.HasPrefix(a, "-") {
			if a == "-r" || a == "-R" || a == "--recursive" {
				recursive = true
				continue
			}
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
	if path == "" && !recursive {
		return nil // likely reading stdin
	}

	effectivePath := path
	if effectivePath == "" {
		effectivePath = "."
	}

	toolArgs := map[string]any{
		"pattern": pattern,
		"path":    effectivePath,
	}

	cmd := "aifr search"
	if mod.HeadLines > 0 {
		cmd += fmt.Sprintf(" --max-matches=%d", mod.HeadLines)
		toolArgs["max_matches"] = mod.HeadLines
	}
	cmd += " " + shellQuote(pattern) + " " + shellQuote(effectivePath)

	return makeSuggestion(baseCmd, cmd, "aifr_search", toolArgs)
}

func suggestFind(args []string, mod PipelineModifier) *Suggestion {
	if mod.TailLines > 0 {
		return nil
	}

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

	toolArgs := map[string]any{"path": path}
	cmd := "aifr find " + shellQuote(path)

	if name != "" {
		cmd += " --name=" + shellQuote(name)
		toolArgs["name"] = name
	}
	if ftype != "" {
		cmd += " --type=" + ftype
		toolArgs["type"] = ftype
	}
	if mod.HeadLines > 0 {
		cmd += fmt.Sprintf(" --limit=%d", mod.HeadLines)
		toolArgs["limit"] = mod.HeadLines
	}

	return makeSuggestion("find", cmd, "aifr_find", toolArgs)
}

func suggestList(args []string, mod PipelineModifier) *Suggestion {
	if mod.TailLines > 0 {
		return nil
	}

	files := nonFlags(args)
	path := "."
	if len(files) > 0 {
		path = files[0]
	}

	if hasFlag(args, "-R", "--recursive") {
		toolArgs := map[string]any{"path": path}
		cmd := "aifr find " + shellQuote(path)
		if mod.HeadLines > 0 {
			cmd += fmt.Sprintf(" --limit=%d", mod.HeadLines)
			toolArgs["limit"] = mod.HeadLines
		}
		return makeSuggestion("ls", cmd, "aifr_find", toolArgs)
	}

	toolArgs := map[string]any{"path": path}
	cmd := "aifr list " + shellQuote(path)
	if mod.HeadLines > 0 {
		cmd += fmt.Sprintf(" --limit=%d", mod.HeadLines)
		toolArgs["limit"] = mod.HeadLines
	}
	return makeSuggestion("ls", cmd, "aifr_list", toolArgs)
}

func suggestWc(args []string, mod PipelineModifier) *Suggestion {
	if mod.IsSet() {
		return nil // wc output with head/tail doesn't map usefully
	}

	files := nonFlags(args)
	if len(files) == 0 {
		return nil // reading from stdin
	}

	toolArgs := map[string]any{"paths": files}

	parts := make([]string, len(files))
	for i, f := range files {
		parts[i] = shellQuote(f)
	}
	cmd := "aifr wc"
	if hasFlag(args, "-l") {
		cmd += " -l"
		toolArgs["lines"] = true
	}
	if hasFlag(args, "-w") {
		cmd += " -w"
		toolArgs["words"] = true
	}
	if hasFlag(args, "-c") {
		cmd += " -c"
		toolArgs["bytes"] = true
	}
	if hasFlag(args, "-m") {
		cmd += " -m"
		toolArgs["chars"] = true
	}
	cmd += " " + strings.Join(parts, " ")
	return makeSuggestion("wc", cmd, "aifr_wc", toolArgs)
}

func suggestStat(args []string, mod PipelineModifier) *Suggestion {
	if mod.IsSet() {
		return nil
	}
	files := nonFlags(args)
	if len(files) == 0 {
		return nil
	}
	return makeSuggestion("stat",
		"aifr stat "+shellQuote(files[0]),
		"aifr_stat",
		map[string]any{"path": files[0]})
}

func suggestDiff(args []string, mod PipelineModifier) *Suggestion {
	if mod.IsSet() {
		return nil
	}
	files := nonFlags(args)
	if len(files) != 2 {
		return nil
	}
	return makeSuggestion("diff",
		"aifr diff "+shellQuote(files[0])+" "+shellQuote(files[1]),
		"aifr_diff",
		map[string]any{"path_a": files[0], "path_b": files[1]})
}

func suggestChecksum(baseCmd string, args []string, mod PipelineModifier) *Suggestion {
	if mod.IsSet() {
		return nil
	}
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
		return nil // aifr may not support blake2
	case "shasum":
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
	return makeSuggestion(baseCmd,
		fmt.Sprintf("aifr checksum -a %s %s", algo, strings.Join(parts, " ")),
		"aifr_checksum",
		map[string]any{"paths": files, "algorithm": algo})
}

func suggestHexdump(_ string, args []string, mod PipelineModifier) *Suggestion {
	if mod.IsSet() {
		return nil
	}
	files := nonFlags(args)
	if len(files) == 0 {
		return nil
	}
	return makeSuggestion("hexdump",
		"aifr hexdump "+shellQuote(files[0]),
		"aifr_hexdump",
		map[string]any{"path": files[0]})
}

func suggestSed(args []string, mod PipelineModifier) *Suggestion {
	if mod.IsSet() {
		return nil
	}
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

	script = strings.TrimSuffix(script, "p")
	if script == "" {
		return nil
	}

	parts := strings.SplitN(script, ",", 2)
	if len(parts) == 1 {
		if _, err := strconv.Atoi(parts[0]); err != nil {
			return nil
		}
		lines := parts[0] + ":" + parts[0]
		return makeSuggestion("sed",
			fmt.Sprintf("aifr read --lines=%s %s", lines, shellQuote(file)),
			"aifr_read",
			map[string]any{"path": file, "lines": lines})
	}

	if _, err := strconv.Atoi(parts[0]); err != nil {
		return nil
	}
	if _, err := strconv.Atoi(parts[1]); err != nil {
		return nil
	}
	lines := parts[0] + ":" + parts[1]
	return makeSuggestion("sed",
		fmt.Sprintf("aifr read --lines=%s %s", lines, shellQuote(file)),
		"aifr_read",
		map[string]any{"path": file, "lines": lines})
}

func suggestGit(args []string, mod PipelineModifier) *Suggestion {
	if len(args) == 0 {
		return nil
	}
	switch args[0] {
	case "log":
		return suggestGitLog(args[1:], mod)
	case "diff":
		return suggestGitDiff(args[1:], mod)
	default:
		return nil
	}
}

func suggestGitLog(args []string, mod PipelineModifier) *Suggestion {
	if mod.TailLines > 0 {
		return nil
	}

	oneline := false
	var maxCount int
	var ref string

	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--oneline":
			oneline = true
		case a == "-n" && i+1 < len(args):
			maxCount, _ = strconv.Atoi(args[i+1])
			i++
		case strings.HasPrefix(a, "-n") && len(a) > 2 && isDigits(a[2:]):
			maxCount, _ = strconv.Atoi(a[2:])
		case strings.HasPrefix(a, "--max-count="):
			maxCount, _ = strconv.Atoi(strings.TrimPrefix(a, "--max-count="))
		case !strings.HasPrefix(a, "-"):
			if ref == "" {
				ref = a
			}
		}
	}

	// Pipeline head overrides/sets max-count.
	if mod.HeadLines > 0 {
		maxCount = mod.HeadLines
	}

	toolArgs := map[string]any{}
	cmd := "aifr log"

	if oneline {
		cmd += " --oneline"
		toolArgs["format"] = "oneline"
	}
	if maxCount > 0 {
		cmd += fmt.Sprintf(" --max-count=%d", maxCount)
		toolArgs["max_count"] = maxCount
	}
	if ref != "" {
		cmd += " " + shellQuote(ref)
		toolArgs["ref"] = ref
	}

	return makeSuggestion("git log", cmd, "aifr_log", toolArgs)
}

func suggestGitDiff(args []string, mod PipelineModifier) *Suggestion {
	if mod.IsSet() {
		return nil // diff output with head/tail doesn't map cleanly
	}

	refs := nonFlags(args)
	var clean []string
	for _, r := range refs {
		if r != "--" {
			clean = append(clean, r)
		}
	}
	if len(clean) == 2 {
		return makeSuggestion("git diff",
			fmt.Sprintf("aifr diff %s:%s %s:%s", clean[0], ".", clean[1], "."),
			"aifr_diff",
			map[string]any{"path_a": clean[0] + ":.", "path_b": clean[1] + ":."})
	}
	return makeSuggestion("git diff", "aifr diff", "aifr_diff", map[string]any{})
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

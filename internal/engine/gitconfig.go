// Copyright 2026 — see LICENSE file for terms.
package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	gitconfig "github.com/go-git/go-git/v5/plumbing/format/config"

	iconfig "go.pennock.tech/aifr/internal/config"
	"go.pennock.tech/aifr/pkg/protocol"
)

// GitConfigParams controls git config queries.
type GitConfigParams struct {
	Scope      string // "local" (default), "merged", "global", "system"
	Key        string // single key lookup
	Regexp     string // pattern match on keys
	Section    string // section listing (e.g., "remote.origin")
	List       bool   // dump all entries
	Type       string // type coercion: "bool", "int", "path"
	Structured string // "identity", "remotes", "branches"
}

// sensitiveConfigPrefixes are keys whose values are always redacted.
var sensitiveConfigPrefixes = []string{
	"credential.",
	"http.extraheader",
	"smtp.pass",
	"sendemail.smtppass",
	"imap.pass",
	"github.token",
}

const maxIncludeDepth = 10

// configEntry is an internal entry with scope tracking.
type configEntry struct {
	key    string // section.subsection.key or section.key
	value  string
	scope  string // "system", "global", "local", "worktree", "include"
	source string // file path
}

// GitConfig queries git configuration for a repository.
func (e *Engine) GitConfig(repoName string, params GitConfigParams) (*protocol.GitConfigResponse, error) {
	scope := params.Scope
	if scope == "" {
		scope = "local"
	}

	// For structured queries, override scope defaults.
	if params.Structured == "identity" && scope == "local" {
		scope = "merged"
	}

	_, repoPath, err := e.openGitRepo(repoName)
	if err != nil {
		return nil, err
	}

	gitDir := findGitDir(repoPath)

	entries, skippedIncludes, err := e.loadGitConfig(gitDir, scope)
	if err != nil {
		return nil, err
	}

	// Handle structured queries separately.
	if params.Structured != "" {
		return e.structuredConfigQuery(repoName, scope, params.Structured, entries, skippedIncludes)
	}

	// Filter entries based on query mode.
	var filtered []configEntry
	switch {
	case params.Key != "":
		for _, ent := range entries {
			if strings.EqualFold(ent.key, params.Key) {
				filtered = append(filtered, ent)
			}
		}
		// Last value wins for single key lookup — return only the last.
		if len(filtered) > 1 {
			filtered = filtered[len(filtered)-1:]
		}

	case params.Regexp != "":
		re, err := regexp.Compile(params.Regexp)
		if err != nil {
			return nil, fmt.Errorf("invalid regexp %q: %w", params.Regexp, err)
		}
		for _, ent := range entries {
			if re.MatchString(ent.key) {
				filtered = append(filtered, ent)
			}
		}

	case params.Section != "":
		prefix := strings.ToLower(params.Section) + "."
		for _, ent := range entries {
			if strings.HasPrefix(strings.ToLower(ent.key), prefix) {
				filtered = append(filtered, ent)
			}
		}

	case params.List:
		filtered = entries

	default:
		return nil, fmt.Errorf("specify one of: key, regexp, section, list, or structured")
	}

	// Apply type coercion and build response.
	var result []protocol.GitConfigEntry
	redacted := 0
	for _, ent := range filtered {
		if isSensitiveKey(ent.key) {
			result = append(result, protocol.GitConfigEntry{
				Key:    ent.key,
				Value:  "<redacted>",
				Scope:  ent.scope,
				Source: ent.source,
			})
			redacted++
			continue
		}

		value := ent.value
		if params.Type != "" {
			value = coerceType(value, params.Type)
		}

		result = append(result, protocol.GitConfigEntry{
			Key:    ent.key,
			Value:  value,
			Scope:  ent.scope,
			Source: ent.source,
		})
	}

	return &protocol.GitConfigResponse{
		Repo:            repoName,
		Scope:           scope,
		Entries:         result,
		Total:           len(result),
		SkippedIncludes: skippedIncludes,
		RedactedKeys:    redacted,
		Complete:        true,
	}, nil
}

// structuredConfigQuery handles --identity, --remotes, --branches.
func (e *Engine) structuredConfigQuery(repoName, scope, structured string, entries []configEntry, skippedIncludes []string) (*protocol.GitConfigResponse, error) {
	// Build a lookup map (last value wins).
	lookup := make(map[string]string)
	for _, ent := range entries {
		lookup[strings.ToLower(ent.key)] = ent.value
	}

	// Collect all entries matching the structured query into response entries.
	var result []protocol.GitConfigEntry

	switch structured {
	case "identity":
		keys := []string{"user.name", "user.email", "author.name", "author.email", "committer.name", "committer.email"}
		for _, k := range keys {
			if v, ok := lookup[k]; ok {
				result = append(result, protocol.GitConfigEntry{Key: k, Value: v, Scope: scope})
			}
		}

	case "remotes":
		// Collect all remote.*.url, remote.*.fetch, remote.*.pushurl entries.
		remotes := make(map[string]map[string][]string) // name → field → values
		for _, ent := range entries {
			lower := strings.ToLower(ent.key)
			if !strings.HasPrefix(lower, "remote.") {
				continue
			}
			parts := strings.SplitN(ent.key, ".", 3)
			if len(parts) != 3 {
				continue
			}
			name := parts[1]
			field := strings.ToLower(parts[2])
			if remotes[name] == nil {
				remotes[name] = make(map[string][]string)
			}
			remotes[name][field] = append(remotes[name][field], ent.value)
		}
		for name, fields := range remotes {
			for _, url := range fields["url"] {
				result = append(result, protocol.GitConfigEntry{Key: "remote." + name + ".url", Value: url, Scope: scope})
			}
			for _, f := range fields["fetch"] {
				result = append(result, protocol.GitConfigEntry{Key: "remote." + name + ".fetch", Value: f, Scope: scope})
			}
			for _, p := range fields["pushurl"] {
				result = append(result, protocol.GitConfigEntry{Key: "remote." + name + ".pushurl", Value: p, Scope: scope})
			}
		}

	case "branches":
		branches := make(map[string]map[string]string) // name → field → value
		for _, ent := range entries {
			lower := strings.ToLower(ent.key)
			if !strings.HasPrefix(lower, "branch.") {
				continue
			}
			parts := strings.SplitN(ent.key, ".", 3)
			if len(parts) != 3 {
				continue
			}
			name := parts[1]
			field := strings.ToLower(parts[2])
			if branches[name] == nil {
				branches[name] = make(map[string]string)
			}
			branches[name][field] = ent.value
		}
		for name, fields := range branches {
			for field, value := range fields {
				result = append(result, protocol.GitConfigEntry{Key: "branch." + name + "." + field, Value: value, Scope: scope})
			}
		}

	default:
		return nil, fmt.Errorf("unknown structured query %q (use: identity, remotes, branches)", structured)
	}

	return &protocol.GitConfigResponse{
		Repo:            repoName,
		Scope:           scope,
		Entries:         result,
		Total:           len(result),
		SkippedIncludes: skippedIncludes,
		Complete:        true,
	}, nil
}

// loadGitConfig loads config entries from the requested scope(s).
func (e *Engine) loadGitConfig(gitDir, scope string) ([]configEntry, []string, error) {
	var allEntries []configEntry
	var allSkipped []string

	switch scope {
	case "system":
		ents, skipped, err := loadConfigFile("/etc/gitconfig", "system", "", 0)
		if err != nil && !os.IsNotExist(err) {
			return nil, nil, err
		}
		allEntries = append(allEntries, ents...)
		allSkipped = append(allSkipped, skipped...)

	case "global":
		path := globalConfigPath()
		ents, skipped, err := loadConfigFile(path, "global", "", 0)
		if err != nil && !os.IsNotExist(err) {
			return nil, nil, err
		}
		allEntries = append(allEntries, ents...)
		allSkipped = append(allSkipped, skipped...)

	case "local":
		path := filepath.Join(gitDir, "config")
		ents, skipped, err := loadConfigFile(path, "local", "", 0)
		if err != nil && !os.IsNotExist(err) {
			return nil, nil, err
		}
		allEntries = append(allEntries, ents...)
		allSkipped = append(allSkipped, skipped...)

	case "merged":
		// System.
		ents, skipped, err := loadConfigFile("/etc/gitconfig", "system", "", 0)
		if err == nil {
			allEntries = append(allEntries, ents...)
			allSkipped = append(allSkipped, skipped...)
		}

		// Global (with include resolution).
		globalPath := globalConfigPath()
		ents, skipped, err = e.loadConfigWithIncludes(globalPath, "global", gitDir, 0)
		if err == nil {
			allEntries = append(allEntries, ents...)
			allSkipped = append(allSkipped, skipped...)
		}

		// Local (with include resolution).
		localPath := filepath.Join(gitDir, "config")
		ents, skipped, err = e.loadConfigWithIncludes(localPath, "local", gitDir, 0)
		if err == nil {
			allEntries = append(allEntries, ents...)
			allSkipped = append(allSkipped, skipped...)
		}

		// Worktree.
		worktreePath := filepath.Join(gitDir, "config.worktree")
		ents, skipped, err = loadConfigFile(worktreePath, "worktree", "", 0)
		if err == nil {
			allEntries = append(allEntries, ents...)
			allSkipped = append(allSkipped, skipped...)
		}

	default:
		return nil, nil, fmt.Errorf("unknown scope %q (use: local, merged, global, system)", scope)
	}

	return allEntries, allSkipped, nil
}

// loadConfigFile parses a single git config file without following includes.
func loadConfigFile(path, scope, _ string, _ int) ([]configEntry, []string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}

	cfg := gitconfig.New()
	if err := gitconfig.NewDecoder(strings.NewReader(string(data))).Decode(cfg); err != nil {
		return nil, nil, fmt.Errorf("parsing %s: %w", path, err)
	}

	var entries []configEntry
	var skipped []string

	for _, sec := range cfg.Sections {
		sectionName := sec.Name
		for _, opt := range sec.Options {
			key := strings.ToLower(sectionName) + "." + opt.Key
			entries = append(entries, configEntry{key: key, value: opt.Value, scope: scope, source: path})
		}
		for _, sub := range sec.Subsections {
			for _, opt := range sub.Options {
				key := strings.ToLower(sectionName) + "." + sub.Name + "." + opt.Key
				entries = append(entries, configEntry{key: key, value: opt.Value, scope: scope, source: path})
			}
		}
	}

	// Detect includes but don't follow them in this function.
	for _, sec := range cfg.Sections {
		if strings.EqualFold(sec.Name, "include") {
			for _, opt := range sec.Options {
				if strings.EqualFold(opt.Key, "path") {
					skipped = append(skipped, "include: "+opt.Value)
				}
			}
		}
		if strings.EqualFold(sec.Name, "includeIf") {
			for _, sub := range sec.Subsections {
				skipped = append(skipped, "includeIf: "+sub.Name)
			}
		}
	}

	return entries, skipped, nil
}

// loadConfigWithIncludes parses a config file and resolves includes.
func (e *Engine) loadConfigWithIncludes(path, scope, gitDir string, depth int) ([]configEntry, []string, error) {
	if depth > maxIncludeDepth {
		return nil, []string{fmt.Sprintf("include depth exceeded at %s", path)}, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}

	cfg := gitconfig.New()
	if err := gitconfig.NewDecoder(strings.NewReader(string(data))).Decode(cfg); err != nil {
		return nil, nil, fmt.Errorf("parsing %s: %w", path, err)
	}

	var entries []configEntry
	var skipped []string
	dir := filepath.Dir(path)

	for _, sec := range cfg.Sections {
		sectionName := sec.Name

		// Handle [include] sections.
		if strings.EqualFold(sectionName, "include") {
			for _, opt := range sec.Options {
				if strings.EqualFold(opt.Key, "path") {
					incPath := resolveIncludePath(opt.Value, dir)
					ents, sk, err := e.loadConfigWithIncludes(incPath, "include", gitDir, depth+1)
					if err != nil {
						skipped = append(skipped, fmt.Sprintf("include %s: %v", opt.Value, err))
					} else {
						entries = append(entries, ents...)
						skipped = append(skipped, sk...)
					}
				}
			}
			continue
		}

		// Handle [includeIf "condition"] sections.
		if strings.EqualFold(sectionName, "includeIf") {
			for _, sub := range sec.Subsections {
				condition := sub.Name
				includePath := sub.Options.Get("path")
				if includePath == "" {
					continue
				}

				if matchesIncludeCondition(condition, gitDir) {
					incPath := resolveIncludePath(includePath, dir)
					ents, sk, err := e.loadConfigWithIncludes(incPath, "include", gitDir, depth+1)
					if err != nil {
						skipped = append(skipped, fmt.Sprintf("includeIf %q: %v", condition, err))
					} else {
						entries = append(entries, ents...)
						skipped = append(skipped, sk...)
					}
				} else if !isSupportedCondition(condition) {
					skipped = append(skipped, "includeIf: "+condition)
				}
				// Supported but non-matching conditions are silently skipped.
			}
			continue
		}

		// Regular section.
		for _, opt := range sec.Options {
			key := strings.ToLower(sectionName) + "." + opt.Key
			entries = append(entries, configEntry{key: key, value: opt.Value, scope: scope, source: path})
		}
		for _, sub := range sec.Subsections {
			for _, opt := range sub.Options {
				key := strings.ToLower(sectionName) + "." + sub.Name + "." + opt.Key
				entries = append(entries, configEntry{key: key, value: opt.Value, scope: scope, source: path})
			}
		}
	}

	return entries, skipped, nil
}

// matchesIncludeCondition evaluates a conditional include condition.
func matchesIncludeCondition(condition, gitDir string) bool {
	switch {
	case strings.HasPrefix(condition, "gitdir:"):
		return matchesGitDir(condition[7:], gitDir, false)
	case strings.HasPrefix(condition, "gitdir/i:"):
		return matchesGitDir(condition[9:], gitDir, true)
	default:
		return false
	}
}

// isSupportedCondition returns true if we can evaluate this condition type.
func isSupportedCondition(condition string) bool {
	return strings.HasPrefix(condition, "gitdir:") || strings.HasPrefix(condition, "gitdir/i:")
}

// matchesGitDir checks if a gitdir pattern matches the repo's .git directory.
func matchesGitDir(pattern, gitDir string, caseInsensitive bool) bool {
	// Expand tilde.
	if strings.HasPrefix(pattern, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return false
		}
		pattern = home + pattern[1:]
	}

	// Trailing / → append **
	if strings.HasSuffix(pattern, "/") {
		pattern += "**"
	}

	// No absolute prefix → prepend **/
	if !strings.HasPrefix(pattern, "/") {
		pattern = "**/" + pattern
	}

	target := gitDir
	if caseInsensitive {
		pattern = strings.ToLower(pattern)
		target = strings.ToLower(target)
	}

	// Also try with trailing /
	matched, _ := doublestar.Match(pattern, target)
	if !matched {
		matched, _ = doublestar.Match(pattern, target+"/")
	}
	return matched
}

// resolveIncludePath resolves an include path relative to the config file's directory.
func resolveIncludePath(path, configDir string) string {
	if strings.HasPrefix(path, "~/") {
		expanded, err := iconfig.ExpandTilde(path)
		if err == nil {
			return expanded
		}
	}
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(configDir, path)
}

// findGitDir locates the .git directory for a repo path.
func findGitDir(repoPath string) string {
	gitDir := filepath.Join(repoPath, ".git")
	if info, err := os.Stat(gitDir); err == nil && info.IsDir() {
		return gitDir
	}
	// Bare repo or already a .git dir.
	if _, err := os.Stat(filepath.Join(repoPath, "HEAD")); err == nil {
		return repoPath
	}
	return gitDir
}

// globalConfigPath returns the path to the global git config.
func globalConfigPath() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		path := filepath.Join(xdg, "git", "config")
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	xdgPath := filepath.Join(home, ".config", "git", "config")
	if _, err := os.Stat(xdgPath); err == nil {
		return xdgPath
	}
	return filepath.Join(home, ".gitconfig")
}

// isSensitiveKey checks if a config key should be redacted.
func isSensitiveKey(key string) bool {
	lower := strings.ToLower(key)
	for _, prefix := range sensitiveConfigPrefixes {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return false
}

// coerceType applies type coercion to a config value.
func coerceType(value, typ string) string {
	switch typ {
	case "bool":
		switch strings.ToLower(value) {
		case "true", "yes", "on", "1":
			return "true"
		case "false", "no", "off", "0", "":
			return "false"
		}
		return value
	case "int":
		v := strings.TrimSpace(value)
		if len(v) == 0 {
			return "0"
		}
		last := v[len(v)-1]
		switch last {
		case 'k', 'K':
			return strings.TrimSuffix(v, string(last)) + " (* 1024)"
		case 'm', 'M':
			return strings.TrimSuffix(v, string(last)) + " (* 1048576)"
		case 'g', 'G':
			return strings.TrimSuffix(v, string(last)) + " (* 1073741824)"
		}
		return v
	case "path":
		expanded, err := iconfig.ExpandTilde(value)
		if err != nil {
			return value
		}
		return expanded
	}
	return value
}

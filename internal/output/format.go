// Copyright 2026 — see LICENSE file for terms.
package output

import (
	"os"
	"strings"
)

// ResolveFormat determines the effective output format.
// It takes the explicitly requested format (from a flag or MCP parameter) and
// a list of formats the caller supports. If explicit is non-empty, it is used
// directly (and validated). Otherwise AIFR_FORMAT is consulted: a
// colon-separated preference-ordered list where the first supported value wins.
// If nothing matches, defaultFormat is returned.
//
// Returns the resolved format and an error if an explicit value or every
// AIFR_FORMAT entry is unsupported.
func ResolveFormat(explicit string, supported []string, defaultFormat string) (string, error) {
	set := make(map[string]bool, len(supported))
	for _, s := range supported {
		set[s] = true
	}

	// Explicit value (CLI --format or MCP parameter) takes precedence.
	if explicit != "" {
		if set[explicit] {
			return explicit, nil
		}
		return "", &UnsupportedFormatError{
			Requested: explicit,
			Supported: supported,
		}
	}

	// Fall back to AIFR_FORMAT environment variable.
	env := os.Getenv("AIFR_FORMAT")
	if env != "" {
		for _, candidate := range strings.Split(env, ":") {
			candidate = strings.TrimSpace(candidate)
			if candidate == "" {
				continue
			}
			if set[candidate] {
				return candidate, nil
			}
		}
		return "", &UnsupportedFormatError{
			Requested: env,
			Supported: supported,
			FromEnv:   true,
		}
	}

	return defaultFormat, nil
}

// UnsupportedFormatError is returned when no requested format is supported.
type UnsupportedFormatError struct {
	Requested string
	Supported []string
	FromEnv   bool
}

func (e *UnsupportedFormatError) Error() string {
	source := "format"
	if e.FromEnv {
		source = "AIFR_FORMAT"
	}
	return "unsupported " + source + " " + quote(e.Requested) +
		" (supported: " + strings.Join(e.Supported, ", ") + ")"
}

func quote(s string) string {
	return "\"" + s + "\""
}

// Copyright 2026 — see LICENSE file for terms.
// Package version holds build-time version information set via ldflags.
package version

const HARDCODED_VERSION = "v0.0.3-dev"

// Set by ldflags at build time.
var (
	Version   = HARDCODED_VERSION
	Commit    = "unknown"
	BuildDate = "unknown"
	BuiltBy   = ""
)

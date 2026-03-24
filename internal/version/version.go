// Copyright 2026 — see LICENSE file for terms.
// Package version holds build-time version information set via ldflags.
package version

// Set by ldflags at build time.
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildDate = "unknown"
)

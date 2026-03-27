// Copyright 2026 — see LICENSE file for terms.
// Package version holds build-time version information set via ldflags.
package version

// This is explicitly named screaming-case for matching with 'sed'.
// DO NOT RENAME without also updating Taskfile.yml and checking other non-Go files for references to it.
// Our release flow makes two commits to edit this.
//
// This should be "latest release + 0.0.1 with -dev suffix"
// OR if at the point where we're tagging, then that explicit tag.
const HARDCODED_VERSION = "v0.3.0"

// Set by ldflags at build time.
var (
	Version   = HARDCODED_VERSION
	Commit    = "unknown"
	BuildDate = "unknown"
	BuiltBy   = ""
)

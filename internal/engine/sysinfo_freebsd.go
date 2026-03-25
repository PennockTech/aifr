// Copyright 2026 — see LICENSE file for terms.
//go:build freebsd

package engine

import (
	"go.pennock.tech/aifr/pkg/protocol"
)

func fillOSDetails(info *protocol.SysinfoOS) {
	// On FreeBSD, kernel version requires x/sys/unix for reliable access.
	// We report runtime.GOOS/GOARCH from the common code.
}

// gatherRouting is not supported on FreeBSD without os/exec.
func gatherRouting() []protocol.SysinfoRoute {
	return nil
}

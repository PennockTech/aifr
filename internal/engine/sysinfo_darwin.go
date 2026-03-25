// Copyright 2026 — see LICENSE file for terms.
//go:build darwin

package engine

import (
	"go.pennock.tech/aifr/pkg/protocol"
)

func fillOSDetails(info *protocol.SysinfoOS) {
	// On Darwin, kernel version is available but requires x/sys/unix
	// or cgo for reliable uname access. We report runtime.GOOS/GOARCH
	// from the common code; additional details are left empty.
}

// gatherRouting is not supported on Darwin without os/exec.
func gatherRouting() []protocol.SysinfoRoute {
	return nil
}

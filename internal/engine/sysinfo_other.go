// Copyright 2026 — see LICENSE file for terms.
//go:build !linux && !darwin && !freebsd

package engine

import (
	"go.pennock.tech/aifr/pkg/protocol"
)

func fillOSDetails(_ *protocol.SysinfoOS) {
	// No platform-specific OS details available.
}

func gatherUptime() *protocol.SysinfoUptime {
	return nil
}

func gatherRouting() []protocol.SysinfoRoute {
	return nil
}

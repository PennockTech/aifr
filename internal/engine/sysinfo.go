// Copyright 2026 — see LICENSE file for terms.
package engine

import (
	"fmt"
	"net"
	"os"
	"runtime"
	"slices"
	"time"

	"go.pennock.tech/aifr/pkg/protocol"
)

// SysinfoParams controls which sections to include.
type SysinfoParams struct {
	Sections []string // empty = all; options: "os", "date", "hostname", "uptime", "network", "routing"
}

func (p *SysinfoParams) include(section string) bool {
	if len(p.Sections) == 0 {
		return true
	}
	return slices.Contains(p.Sections, section)
}

// Sysinfo gathers system information for fault diagnosis.
// This does NOT use os/exec — all information comes from syscalls,
// /proc reads, or stdlib functions. Access control is not applied to
// system metadata paths (/proc/*, /etc/os-release).
func (e *Engine) Sysinfo(params SysinfoParams) (*protocol.SysinfoResponse, error) {
	resp := &protocol.SysinfoResponse{Complete: true}

	if params.include("os") {
		resp.OS = gatherOS()
	}

	if params.include("date") {
		resp.Date = gatherDate()
	}

	if params.include("hostname") {
		h, err := os.Hostname()
		if err == nil {
			resp.Hostname = h
		}
	}

	if params.include("uptime") {
		resp.Uptime = gatherUptime()
	}

	if params.include("network") {
		resp.Network = gatherNetwork()
	}

	if params.include("routing") {
		resp.Routing = gatherRouting()
	}

	return resp, nil
}

func gatherDate() *protocol.SysinfoDate {
	now := time.Now()
	zoneName, offset := now.Zone()
	return &protocol.SysinfoDate{
		UTC:      now.UTC().Format(time.RFC3339),
		Local:    now.Format(time.RFC3339),
		DateOnly: now.Format("2006-01-02"),
		YearOnly: now.Format("2006"),
		Timezone: zoneName,
		Offset:   offset,
	}
}

func gatherNetwork() []protocol.SysinfoIface {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}

	var result []protocol.SysinfoIface
	for _, iface := range ifaces {
		entry := protocol.SysinfoIface{
			Name:  iface.Name,
			Flags: iface.Flags.String(),
		}
		if len(iface.HardwareAddr) > 0 {
			entry.HWAddr = iface.HardwareAddr.String()
		}

		addrs, err := iface.Addrs()
		if err == nil {
			for _, addr := range addrs {
				entry.Addresses = append(entry.Addresses, addr.String())
			}
		}

		result = append(result, entry)
	}
	return result
}

// formatUptime converts seconds to a human-readable string like "3d 14h 22m".
func formatUptime(secs float64) string {
	total := int(secs)
	days := total / 86400
	hours := (total % 86400) / 3600
	minutes := (total % 3600) / 60

	switch {
	case days > 0:
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	case hours > 0:
		return fmt.Sprintf("%dh %dm", hours, minutes)
	default:
		return fmt.Sprintf("%dm", minutes)
	}
}

// gatherOS returns OS information. Platform-specific parts are in
// sysinfo_linux.go, sysinfo_darwin.go, sysinfo_other.go.
func gatherOS() *protocol.SysinfoOS {
	info := &protocol.SysinfoOS{
		Kernel: runtime.GOOS,
		Arch:   runtime.GOARCH,
	}
	fillOSDetails(info)
	return info
}

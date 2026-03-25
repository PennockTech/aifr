// Copyright 2026 — see LICENSE file for terms.
//go:build linux

package engine

import (
	"bufio"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"syscall"

	"go.pennock.tech/aifr/pkg/protocol"
)

func fillOSDetails(info *protocol.SysinfoOS) {
	var uname syscall.Utsname
	if err := syscall.Uname(&uname); err == nil {
		info.Release = unix83ToString(uname.Release[:])
	}

	// Parse /etc/os-release for distro info.
	if f, err := os.Open("/etc/os-release"); err == nil {
		defer f.Close()
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := scanner.Text()
			key, val, ok := strings.Cut(line, "=")
			if !ok {
				continue
			}
			val = strings.Trim(val, "\"")
			switch key {
			case "NAME":
				info.Distro = val
			case "VERSION_ID":
				info.Version = val
			}
		}
	}
}

// unix83ToString converts a [65]int8 (utsname field) to a Go string.
func unix83ToString(b []int8) string {
	var buf []byte
	for _, c := range b {
		if c == 0 {
			break
		}
		buf = append(buf, byte(c))
	}
	return string(buf)
}

// gatherRouting parses /proc/net/route on Linux.
func gatherRouting() []protocol.SysinfoRoute {
	f, err := os.Open("/proc/net/route")
	if err != nil {
		return nil
	}
	defer f.Close()

	var routes []protocol.SysinfoRoute
	scanner := bufio.NewScanner(f)

	// Skip header line.
	if !scanner.Scan() {
		return nil
	}

	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 11 {
			continue
		}

		dest := parseHexIP(fields[1])
		gw := parseHexIP(fields[2])
		metric, _ := strconv.Atoi(fields[6])
		mask := parseHexIP(fields[7])

		// Convert dest + mask to CIDR.
		cidr := ipMaskToCIDR(dest, mask)

		routes = append(routes, protocol.SysinfoRoute{
			Destination: cidr,
			Gateway:     gw.String(),
			Interface:   fields[0],
			Flags:       routeFlags(fields[3]),
			Metric:      metric,
		})
	}

	return routes
}

// parseHexIP parses a hex-encoded little-endian IPv4 address from /proc/net/route.
func parseHexIP(s string) net.IP {
	b, err := hex.DecodeString(s)
	if err != nil || len(b) != 4 {
		return net.IPv4zero
	}
	// /proc/net/route uses host byte order (little-endian on x86).
	v := binary.LittleEndian.Uint32(b)
	return net.IPv4(byte(v>>24), byte(v>>16), byte(v>>8), byte(v))
}

// ipMaskToCIDR converts an IP and mask to CIDR notation.
func ipMaskToCIDR(ip, mask net.IP) string {
	ip4 := ip.To4()
	mask4 := mask.To4()
	if ip4 == nil || mask4 == nil {
		return ip.String()
	}
	ones, _ := net.IPMask(mask4).Size()
	return fmt.Sprintf("%s/%d", ip4.String(), ones)
}

// routeFlags converts numeric route flags to a human-readable string.
func routeFlags(s string) string {
	n, _ := strconv.ParseUint(s, 16, 32)
	var flags []string
	if n&0x0001 != 0 {
		flags = append(flags, "U") // up
	}
	if n&0x0002 != 0 {
		flags = append(flags, "G") // gateway
	}
	if n&0x0004 != 0 {
		flags = append(flags, "H") // host
	}
	return strings.Join(flags, "")
}

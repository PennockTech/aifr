// Copyright 2026 — see LICENSE file for terms.
package protocol

// SysinfoOS describes the operating system.
type SysinfoOS struct {
	Kernel  string `json:"kernel"`
	Arch    string `json:"arch"`
	Release string `json:"release,omitempty"`
	Distro  string `json:"distro,omitempty"`
	Version string `json:"version,omitempty"`
}

// SysinfoDate holds current date/time information.
type SysinfoDate struct {
	UTC      string `json:"utc"`
	Local    string `json:"local"`
	DateOnly string `json:"date_only"`
	YearOnly string `json:"year_only"`
	Timezone string `json:"timezone"`
	Offset   int    `json:"tz_offset"`
}

// SysinfoIface describes a network interface.
type SysinfoIface struct {
	Name      string   `json:"name"`
	Flags     string   `json:"flags"`
	Addresses []string `json:"addresses"`
	HWAddr    string   `json:"hw_addr,omitempty"`
}

// SysinfoRoute describes a routing table entry.
type SysinfoRoute struct {
	Destination string `json:"destination"`
	Gateway     string `json:"gateway"`
	Interface   string `json:"interface"`
	Flags       string `json:"flags,omitempty"`
	Metric      int    `json:"metric,omitempty"`
}

// SysinfoUptime describes system uptime.
type SysinfoUptime struct {
	Seconds float64 `json:"seconds"`
	Human   string  `json:"human"` // e.g. "3d 14h 22m"
}

// SysinfoResponse is the JSON response for a sysinfo operation.
type SysinfoResponse struct {
	OS       *SysinfoOS     `json:"os,omitempty"`
	Date     *SysinfoDate   `json:"date,omitempty"`
	Hostname string         `json:"hostname,omitempty"`
	Uptime   *SysinfoUptime `json:"uptime,omitempty"`
	Network  []SysinfoIface `json:"network,omitempty"`
	Routing  []SysinfoRoute `json:"routing,omitempty"`
	Complete bool           `json:"complete"`
}

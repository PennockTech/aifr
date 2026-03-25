// Copyright 2026 — see LICENSE file for terms.
package engine

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"go.pennock.tech/aifr/pkg/protocol"
)

// GetentParams controls the getent query.
type GetentParams struct {
	Database string   // "passwd", "group", "services", "protocols"
	Key      string   // optional: lookup by name or numeric ID
	Fields   []string // optional: restrict to these fields (default: all)
	Protocol string   // optional: for services database, filter by protocol (tcp, udp, etc.)
}

// databaseSpec describes how to parse a system database file.
type databaseSpec struct {
	path      string
	delimiter string // field separator
	fields    []string
	keyFields []int // field indices that can match a key lookup
}

var databases = map[string]databaseSpec{
	"passwd": {
		path:      "/etc/passwd",
		delimiter: ":",
		fields:    []string{"name", "uid", "gid", "gecos", "home", "shell"},
		keyFields: []int{0, 2}, // name (0) and uid (2, after dropping password)
	},
	"group": {
		path:      "/etc/group",
		delimiter: ":",
		fields:    []string{"name", "gid", "members"},
		keyFields: []int{0, 1}, // name (0) and gid (1, after dropping password)
	},
	"services": {
		path:      "/etc/services",
		delimiter: " ", // whitespace-delimited, special parsing
		fields:    []string{"name", "port", "protocol", "aliases"},
		keyFields: []int{0, 1}, // name (0) and port (1)
	},
	"protocols": {
		path:      "/etc/protocols",
		delimiter: " ", // whitespace-delimited, special parsing
		fields:    []string{"name", "number", "aliases"},
		keyFields: []int{0, 1}, // name (0) and number (1)
	},
}

// blockedDatabases are databases we refuse to query (defense in depth).
var blockedDatabases = map[string]bool{
	"shadow":  true,
	"gshadow": true,
}

// Getent queries a system database (passwd, group, services, protocols).
// System database files are read directly without access control checks —
// they are world-readable system metadata, not user data.
// Shadow databases are always refused.
func (e *Engine) Getent(params GetentParams) (*protocol.GetentResponse, error) {
	if blockedDatabases[params.Database] {
		return nil, protocol.NewError("ACCESS_DENIED_SENSITIVE",
			fmt.Sprintf("database %q contains sensitive data and cannot be queried", params.Database))
	}

	spec, ok := databases[params.Database]
	if !ok {
		known := make([]string, 0, len(databases))
		for k := range databases {
			known = append(known, k)
		}
		return nil, fmt.Errorf("unknown database %q (supported: %s)", params.Database, strings.Join(known, ", "))
	}

	// Determine which fields to return.
	outputFields := spec.fields
	if len(params.Fields) > 0 {
		// Validate requested fields.
		valid := make(map[string]bool, len(spec.fields))
		for _, f := range spec.fields {
			valid[f] = true
		}
		for _, f := range params.Fields {
			if !valid[f] {
				return nil, fmt.Errorf("unknown field %q for database %q (available: %s)",
					f, params.Database, strings.Join(spec.fields, ", "))
			}
		}
		outputFields = params.Fields
	}

	var entries []protocol.GetentEntry
	var err error

	switch params.Database {
	case "passwd":
		entries, err = parseColonFile(spec.path, passwdMapper, params.Key)
	case "group":
		entries, err = parseColonFile(spec.path, groupMapper, params.Key)
	case "services":
		entries, err = parseServicesFile(spec.path, params.Key, params.Protocol)
	case "protocols":
		entries, err = parseProtocolsFile(spec.path, params.Key)
	}
	if err != nil {
		return nil, err
	}

	// Filter to requested fields.
	if len(params.Fields) > 0 {
		for i := range entries {
			filtered := make(map[string]string, len(outputFields))
			for _, f := range outputFields {
				if v, ok := entries[i].Fields[f]; ok {
					filtered[f] = v
				}
			}
			entries[i].Fields = filtered
		}
	}

	return &protocol.GetentResponse{
		Database: params.Database,
		Key:      params.Key,
		Fields:   outputFields,
		Entries:  entries,
		Total:    len(entries),
		Complete: true,
	}, nil
}

// fieldMapper converts split fields to a named map.
type fieldMapper func(parts []string) (map[string]string, []string)

// passwdMapper parses a line from /etc/passwd.
// Format: name:password:uid:gid:gecos:home:shell
// We skip the password field (always "x").
func passwdMapper(parts []string) (map[string]string, []string) {
	if len(parts) < 7 {
		return nil, nil
	}
	m := map[string]string{
		"name":  parts[0],
		"uid":   parts[2],
		"gid":   parts[3],
		"gecos": parts[4],
		"home":  parts[5],
		"shell": parts[6],
	}
	// Key candidates: name (parts[0]), uid (parts[2])
	return m, []string{parts[0], parts[2]}
}

// groupMapper parses a line from /etc/group.
// Format: name:password:gid:members
// We skip the password field.
func groupMapper(parts []string) (map[string]string, []string) {
	if len(parts) < 4 {
		return nil, nil
	}
	m := map[string]string{
		"name":    parts[0],
		"gid":     parts[2],
		"members": parts[3],
	}
	return m, []string{parts[0], parts[2]}
}

// parseColonFile parses a colon-delimited file (passwd, group).
func parseColonFile(path string, mapper fieldMapper, key string) ([]protocol.GetentEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var entries []protocol.GetentEntry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || line[0] == '#' {
			continue
		}
		parts := strings.Split(line, ":")
		fields, keys := mapper(parts)
		if fields == nil {
			continue
		}

		if key != "" {
			matched := false
			for _, k := range keys {
				if k == key {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}

		entries = append(entries, protocol.GetentEntry{Fields: fields})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return entries, nil
}

// parseServicesFile parses /etc/services.
// Format: name  port/protocol  [aliases...]  [# comment]
// If protoFilter is non-empty, only entries matching that protocol are returned.
func parseServicesFile(path, key, protoFilter string) ([]protocol.GetentEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var entries []protocol.GetentEntry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		// Strip comments.
		if idx := strings.IndexByte(line, '#'); idx >= 0 {
			line = line[:idx]
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		name := parts[0]
		portProto := parts[1]
		port, proto, ok := strings.Cut(portProto, "/")
		if !ok {
			continue
		}

		// Filter by protocol if specified (e.g., "tcp", "udp").
		if protoFilter != "" && proto != protoFilter {
			continue
		}

		var aliases string
		if len(parts) > 2 {
			aliases = strings.Join(parts[2:], " ")
		}

		fields := map[string]string{
			"name":     name,
			"port":     port,
			"protocol": proto,
			"aliases":  aliases,
		}

		if key != "" && name != key && port != key {
			// Also check aliases.
			matched := false
			for _, a := range parts[2:] {
				if a == key {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}

		entries = append(entries, protocol.GetentEntry{Fields: fields})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return entries, nil
}

// parseProtocolsFile parses /etc/protocols.
// Format: name  number  [aliases...]  [# comment]
func parseProtocolsFile(path, key string) ([]protocol.GetentEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var entries []protocol.GetentEntry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if idx := strings.IndexByte(line, '#'); idx >= 0 {
			line = line[:idx]
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		name := parts[0]
		number := parts[1]

		var aliases string
		if len(parts) > 2 {
			aliases = strings.Join(parts[2:], " ")
		}

		fields := map[string]string{
			"name":    name,
			"number":  number,
			"aliases": aliases,
		}

		if key != "" && name != key && number != key {
			matched := false
			for _, a := range parts[2:] {
				if a == key {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}

		entries = append(entries, protocol.GetentEntry{Fields: fields})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return entries, nil
}

// Copyright 2026 — see LICENSE file for terms.
package output

import (
	"fmt"
	"io"
	"strings"

	"go.pennock.tech/aifr/pkg/protocol"
)

// WriteStatText writes a StatEntry in human-readable format.
func WriteStatText(w io.Writer, entry *protocol.StatEntry) {
	fmt.Fprintf(w, "%s  %s  %8d  %s  %s\n",
		entry.Mode, entry.Type, entry.Size, entry.ModTime, entry.Path)
}

// NumberLines prepends line numbers to each line in data, starting at startLine.
// Format matches cat -n: right-justified 6-wide number, tab, then content.
func NumberLines(data string, startLine int) string {
	if data == "" {
		return data
	}
	lines := strings.Split(data, "\n")
	// If data ends with \n, Split produces a trailing empty element — drop it.
	trailingNewline := strings.HasSuffix(data, "\n")
	if trailingNewline && len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	var buf strings.Builder
	for i, line := range lines {
		fmt.Fprintf(&buf, "%6d\t%s\n", startLine+i, line)
	}
	return buf.String()
}

// WriteReadText writes read response data in human-readable format.
// If numberLines is true, each line is prefixed with its file line number.
func WriteReadText(w io.Writer, resp *protocol.ReadResponse, numberLines bool) {
	if resp.Chunk != nil && resp.Chunk.Encoding == "utf-8" {
		data := resp.Chunk.Data
		if numberLines {
			startLine := resp.Chunk.StartLine
			if startLine < 1 {
				startLine = 1
			}
			data = NumberLines(data, startLine)
		}
		io.WriteString(w, data) //nolint:errcheck
		if !strings.HasSuffix(data, "\n") {
			io.WriteString(w, "\n") //nolint:errcheck
		}
	}
}

// WriteListText writes directory entries in human-readable format.
func WriteListText(w io.Writer, resp *protocol.ListResponse) {
	for _, entry := range resp.Entries {
		fmt.Fprintf(w, "%s  %s  %8d  %s\n", entry.Type, entry.Mode, entry.Size, entry.Path)
	}
}

// WriteErrorText writes an error in human-readable format.
func WriteErrorText(w io.Writer, err *protocol.AifrError) {
	fmt.Fprintf(w, "error: %s: %s", err.Code, err.Message)
	if err.Path != "" {
		fmt.Fprintf(w, " (%s)", err.Path)
	}
	fmt.Fprintln(w)
}

// WriteSearchText writes search results in human-readable format.
func WriteSearchText(w io.Writer, resp *protocol.SearchResponse) {
	for _, m := range resp.Matches {
		for _, line := range m.ContextBefore {
			fmt.Fprintf(w, "%s-%d-  %s\n", m.File, m.Line-len(m.ContextBefore), line)
		}
		fmt.Fprintf(w, "%s:%d:%d: %s\n", m.File, m.Line, m.Column, m.Match)
		for _, line := range m.ContextAfter {
			fmt.Fprintf(w, "%s-%d-  %s\n", m.File, m.Line+1, line)
		}
	}
	if resp.Truncated {
		fmt.Fprintf(w, "... truncated at %d matches (%d files searched, %d matched)\n",
			resp.TotalMatches, resp.FilesSearched, resp.FilesMatched)
	}
}

// WriteFindText writes find results in human-readable format.
func WriteFindText(w io.Writer, resp *protocol.FindResponse) {
	for _, entry := range resp.Entries {
		fmt.Fprintln(w, entry.Path)
	}
}

// WriteDiffText writes a diff response in unified diff format.
func WriteDiffText(w io.Writer, resp *protocol.DiffResponse) {
	if resp.Identical {
		fmt.Fprintf(w, "Files %s and %s are identical\n", resp.PathA, resp.PathB)
		return
	}
	if resp.ByteDiff != nil {
		bd := resp.ByteDiff
		fmt.Fprintf(w, "%s %s differ: byte %d, line %d, column %d\n",
			resp.PathA, resp.PathB, bd.Offset, bd.Line, bd.Column)
		if bd.SizeA != bd.SizeB {
			fmt.Fprintf(w, "  sizes: %d vs %d\n", bd.SizeA, bd.SizeB)
		}
		return
	}
	fmt.Fprintf(w, "--- %s\n+++ %s\n", resp.PathA, resp.PathB)
	for _, h := range resp.Hunks {
		fmt.Fprintf(w, "@@ -%d,%d +%d,%d @@\n", h.OldStart, h.OldLines, h.NewStart, h.NewLines)
		for _, line := range h.Lines {
			fmt.Fprintln(w, line)
		}
	}
}

// WriteCatText writes a cat response with the specified divider format.
// Divider is one of "plain", "xml", or "none".
// If numberLines is true, each file's content is prefixed with line numbers starting at 1.
func WriteCatText(w io.Writer, resp *protocol.CatResponse, divider string, numberLines bool) {
	for _, entry := range resp.Files {
		displayPath := entry.RelPath
		if displayPath == "" {
			displayPath = entry.Path
		}

		switch divider {
		case "xml":
			writeCatXML(w, entry, displayPath, numberLines)
		case "none":
			writeCatNone(w, entry, numberLines)
		default: // "plain"
			writeCatPlain(w, entry, displayPath, numberLines)
		}
	}

	if resp.Truncated {
		switch divider {
		case "xml":
			fmt.Fprintf(w, "<truncated files_shown=\"%d\" total_bytes=\"%d\" warning=%q />\n",
				len(resp.Files), resp.TotalBytes, resp.Warning)
		case "none":
			// no summary in none mode
		default:
			fmt.Fprintf(w, "--- truncated: %d files shown, %d bytes, %s ---\n",
				len(resp.Files), resp.TotalBytes, resp.Warning)
		}
	}
}

func numberContent(content string, numberLines bool) string {
	if !numberLines || content == "" {
		return content
	}
	return NumberLines(content, 1)
}

func writeCatPlain(w io.Writer, entry protocol.CatEntry, path string, numberLines bool) {
	switch {
	case entry.Error != "":
		fmt.Fprintf(w, "--- %s (error: %s) ---\n", path, entry.Error)
	case entry.Binary:
		fmt.Fprintf(w, "--- %s (binary, skipped) ---\n", path)
	default:
		fmt.Fprintf(w, "--- %s ---\n", path)
		content := numberContent(entry.Content, numberLines)
		io.WriteString(w, content) //nolint:errcheck
		if len(content) > 0 && content[len(content)-1] != '\n' {
			io.WriteString(w, "\n") //nolint:errcheck
		}
	}
}

func writeCatXML(w io.Writer, entry protocol.CatEntry, path string, numberLines bool) {
	switch {
	case entry.Error != "":
		fmt.Fprintf(w, "<file path=%q error=%q />\n", path, entry.Error)
	case entry.Binary:
		fmt.Fprintf(w, "<file path=%q binary=\"true\" />\n", path)
	default:
		fmt.Fprintf(w, "<file path=%q>\n", path)
		content := numberContent(entry.Content, numberLines)
		io.WriteString(w, content) //nolint:errcheck
		if len(content) > 0 && content[len(content)-1] != '\n' {
			io.WriteString(w, "\n") //nolint:errcheck
		}
		fmt.Fprintln(w, "</file>")
	}
}

func writeCatNone(w io.Writer, entry protocol.CatEntry, numberLines bool) {
	if entry.Error != "" || entry.Binary {
		return // skip silently in none mode
	}
	content := numberContent(entry.Content, numberLines)
	io.WriteString(w, content) //nolint:errcheck
}

// WriteLogText writes a git log response in human-readable format.
//
// The output mimics familiar git-log style with clear visual separation
// between commits. Multi-line commit messages are split into a subject
// line and indented body. File changes use A/M/D action indicators
// (from the Changes field) when available, falling back to "M" for
// the legacy FilesChanged field.
func WriteLogText(w io.Writer, resp *protocol.LogResponse) {
	for i, e := range resp.Entries {
		if i > 0 {
			fmt.Fprintln(w)
		}
		hash := e.Hash
		if len(hash) > 12 {
			hash = hash[:12]
		}
		fmt.Fprintf(w, "commit %s\n", hash)
		fmt.Fprintf(w, "Author: %s <%s>\n", e.Author, e.AuthorEmail)
		fmt.Fprintf(w, "Date:   %s\n", e.Date)

		// Split message into subject and body.
		subject, body := splitMessage(e.Message)
		fmt.Fprintf(w, "\n    %s\n", subject)
		if body != "" {
			fmt.Fprintln(w)
			for _, line := range strings.Split(body, "\n") {
				fmt.Fprintf(w, "    %s\n", line)
			}
		}

		// Prefer Changes (with action) over legacy FilesChanged.
		if len(e.Changes) > 0 {
			fmt.Fprintln(w)
			for _, ch := range e.Changes {
				fmt.Fprintf(w, "  %s %s\n", ch.Action, ch.Path)
			}
		} else if len(e.FilesChanged) > 0 {
			fmt.Fprintln(w)
			for _, f := range e.FilesChanged {
				fmt.Fprintf(w, "  M %s\n", f)
			}
		}
	}

	if !resp.Complete && resp.Continuation != "" {
		nextSkip := resp.Skipped + resp.Total
		fmt.Fprintf(w, "\n... %d commits shown, more available (use --skip %d or continuation token)\n", resp.Total, nextSkip)
	}
}

// splitMessage separates a commit message into subject (first line) and body (rest).
func splitMessage(msg string) (subject, body string) {
	msg = strings.TrimSpace(msg)
	if idx := strings.Index(msg, "\n"); idx >= 0 {
		subject = strings.TrimSpace(msg[:idx])
		body = strings.TrimSpace(msg[idx+1:])
	} else {
		subject = msg
	}
	return
}

// WriteLogOneline writes a compact one-line-per-commit log.
func WriteLogOneline(w io.Writer, resp *protocol.LogResponse) {
	for _, e := range resp.Entries {
		hash := e.Hash
		if len(hash) > 12 {
			hash = hash[:12]
		}
		subject, _ := splitMessage(e.Message)
		fmt.Fprintf(w, "%s %s\n", hash, subject)
	}
	if !resp.Complete && resp.Continuation != "" {
		nextSkip := resp.Skipped + resp.Total
		fmt.Fprintf(w, "... %d commits shown, more available (use --skip %d or continuation token)\n", resp.Total, nextSkip)
	}
}

// WriteLogXML writes a git log response in XML format with proper escaping.
//
// All text content (author, message, file paths) is XML-escaped to prevent
// injection via crafted commit messages that contain XML markup.
func WriteLogXML(w io.Writer, resp *protocol.LogResponse) {
	fmt.Fprintf(w, "<log ref=%s total=\"%d\" complete=\"%t\">\n",
		xmlAttr(resp.Ref), resp.Total, resp.Complete)

	for _, e := range resp.Entries {
		hash := e.Hash
		if len(hash) > 12 {
			hash = hash[:12]
		}
		fmt.Fprintf(w, "<commit hash=%s>\n", xmlAttr(hash))
		fmt.Fprintf(w, "<author>%s</author>\n", xmlEscape(e.Author))
		fmt.Fprintf(w, "<email>%s</email>\n", xmlEscape(e.AuthorEmail))
		fmt.Fprintf(w, "<date>%s</date>\n", xmlEscape(e.Date))

		subject, body := splitMessage(e.Message)
		fmt.Fprintf(w, "<subject>%s</subject>\n", xmlEscape(subject))
		if body != "" {
			fmt.Fprintf(w, "<body>\n%s\n</body>\n", xmlEscape(body))
		}

		if len(e.Changes) > 0 {
			fmt.Fprintln(w, "<files>")
			for _, ch := range e.Changes {
				fmt.Fprintf(w, "<file action=%s>%s</file>\n", xmlAttr(ch.Action), xmlEscape(ch.Path))
			}
			fmt.Fprintln(w, "</files>")
		} else if len(e.FilesChanged) > 0 {
			fmt.Fprintln(w, "<files>")
			for _, f := range e.FilesChanged {
				fmt.Fprintf(w, "<file action=\"M\">%s</file>\n", xmlEscape(f))
			}
			fmt.Fprintln(w, "</files>")
		}

		fmt.Fprintln(w, "</commit>")
	}

	fmt.Fprintln(w, "</log>")
}

// xmlEscape escapes text for safe inclusion in XML character data.
// It replaces &, <, >, ", and ' with their XML entity equivalents.
func xmlEscape(s string) string {
	// Fast path: no special chars.
	if !strings.ContainsAny(s, `&<>"'`) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s) + 10)
	for _, r := range s {
		switch r {
		case '&':
			b.WriteString("&amp;")
		case '<':
			b.WriteString("&lt;")
		case '>':
			b.WriteString("&gt;")
		case '"':
			b.WriteString("&quot;")
		case '\'':
			b.WriteString("&apos;")
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// xmlAttr formats a string as a quoted XML attribute value.
func xmlAttr(s string) string {
	return `"` + xmlEscape(s) + `"`
}

// WriteRefsText writes git refs in human-readable format.
func WriteRefsText(w io.Writer, resp *protocol.RefsResponse) {
	for _, r := range resp.Refs {
		if r.Remote != "" {
			fmt.Fprintf(w, "%s  %s  %s/%s\n", r.Hash[:12], r.Type, r.Remote, r.Name)
		} else {
			fmt.Fprintf(w, "%s  %s  %s\n", r.Hash[:12], r.Type, r.Name)
		}
	}
}

// WriteWcText writes word-count results in compact text format.
// When a single count field is active and total_only is set, emits just the number.
// Otherwise emits key=value pairs per line.
func WriteWcText(w io.Writer, resp *protocol.WcResponse) {
	// Determine which fields are present by checking the total entry.
	fields := wcActiveFields(&resp.Total)
	singleField := len(fields) == 1

	for _, e := range resp.Entries {
		if e.Error != "" {
			fmt.Fprintf(w, "%s: error: %s\n", e.Path, e.Error)
			continue
		}
		writeWcEntry(w, &e, fields, singleField, e.Path)
	}

	// Print total line if multiple files.
	if len(resp.Entries) > 1 {
		writeWcEntry(w, &resp.Total, fields, singleField, "total")
	} else if len(resp.Entries) == 0 {
		// total_only mode: just the total.
		writeWcEntry(w, &resp.Total, fields, singleField, "")
	}
}

type wcField struct {
	key string
	val string
}

func wcActiveFields(e *protocol.WcEntry) []string {
	var fields []string
	if e.Lines != nil {
		fields = append(fields, "lines")
	}
	if e.Words != nil {
		fields = append(fields, "words")
	}
	if e.Bytes != nil {
		fields = append(fields, "bytes")
	}
	if e.Chars != nil {
		fields = append(fields, "chars")
	}
	return fields
}

func wcFieldValue(e *protocol.WcEntry, key string) string {
	switch key {
	case "lines":
		if e.Lines != nil {
			return fmt.Sprintf("%d", *e.Lines)
		}
	case "words":
		if e.Words != nil {
			return fmt.Sprintf("%d", *e.Words)
		}
	case "bytes":
		if e.Bytes != nil {
			return fmt.Sprintf("%d", *e.Bytes)
		}
	case "chars":
		if e.Chars != nil {
			return fmt.Sprintf("%d", *e.Chars)
		}
	}
	return ""
}

func writeWcEntry(w io.Writer, e *protocol.WcEntry, fields []string, singleField bool, label string) {
	if singleField {
		v := wcFieldValue(e, fields[0])
		if label == "" {
			fmt.Fprintf(w, "%s\n", v)
		} else {
			fmt.Fprintf(w, "%s %s\n", v, label)
		}
		return
	}
	var parts []string
	for _, f := range fields {
		parts = append(parts, f+"="+wcFieldValue(e, f))
	}
	line := strings.Join(parts, " ")
	if label != "" {
		fmt.Fprintf(w, "%s %s\n", line, label)
	} else {
		fmt.Fprintf(w, "%s\n", line)
	}
}

// WritePathfindText writes pathfind results in human-readable format.
func WritePathfindText(w io.Writer, resp *protocol.PathfindResponse) {
	for _, e := range resp.Entries {
		masked := ""
		if e.Masked {
			masked = fmt.Sprintf(" (masked by %s)", e.MaskedBy)
		}
		fmt.Fprintf(w, "%s%s\n", e.Path, masked)
	}
}

// WriteHexdumpText writes a hex dump in canonical format.
func WriteHexdumpText(w io.Writer, resp *protocol.HexdumpResponse) {
	for _, line := range resp.Lines {
		fmt.Fprintf(w, "%08x  %s  |%s|\n", line.Offset, line.Hex, line.ASCII)
	}
}

// WriteChecksumText writes checksums in the standard hash-space-path format.
func WriteChecksumText(w io.Writer, resp *protocol.ChecksumResponse) {
	for _, e := range resp.Entries {
		if e.Error != "" {
			fmt.Fprintf(w, "%s: error: %s\n", e.Path, e.Error)
			continue
		}
		fmt.Fprintf(w, "%s  %s\n", e.Checksum, e.Path)
	}
}

// WriteRevParseText writes a resolved ref in compact format.
func WriteRevParseText(w io.Writer, resp *protocol.RevParseResponse) {
	fmt.Fprintf(w, "%s %s <%s> %s\n", resp.Hash, resp.AuthorName, resp.AuthorEmail, resp.Date)
	if resp.Subject != "" {
		fmt.Fprintf(w, "  %s\n", resp.Subject)
	}
}

// WriteReflogText writes reflog entries in human-readable format.
func WriteReflogText(w io.Writer, resp *protocol.ReflogResponse) {
	for _, e := range resp.Entries {
		fmt.Fprintf(w, "%s %s %s %s\n", e.NewHash[:12], e.Date, e.Author, e.Action)
	}
}

// WriteSysinfoText writes system info in human-readable format.
func WriteSysinfoText(w io.Writer, resp *protocol.SysinfoResponse) {
	if resp.OS != nil {
		fmt.Fprintf(w, "os: %s %s %s\n", resp.OS.Kernel, resp.OS.Release, resp.OS.Arch)
		if resp.OS.Distro != "" {
			fmt.Fprintf(w, "distro: %s\n", resp.OS.Distro)
		}
	}
	if resp.Date != nil {
		fmt.Fprintf(w, "date: %s (%s)\n", resp.Date.UTC, resp.Date.Timezone)
	}
	if resp.Hostname != "" {
		fmt.Fprintf(w, "hostname: %s\n", resp.Hostname)
	}
	if resp.Uptime != nil {
		fmt.Fprintf(w, "uptime: %s\n", resp.Uptime.Human)
	}
	for _, iface := range resp.Network {
		fmt.Fprintf(w, "net %s: %s\n", iface.Name, strings.Join(iface.Addresses, ", "))
	}
	for _, r := range resp.Routing {
		fmt.Fprintf(w, "route %s via %s dev %s\n", r.Destination, r.Gateway, r.Interface)
	}
}

// WriteGetentText writes getent results in human-readable format.
func WriteGetentText(w io.Writer, resp *protocol.GetentResponse) {
	for _, e := range resp.Entries {
		var parts []string
		for _, f := range resp.Fields {
			if v, ok := e.Fields[f]; ok {
				parts = append(parts, f+"="+v)
			}
		}
		fmt.Fprintln(w, strings.Join(parts, " "))
	}
}

// WriteGitConfigText writes git config entries in human-readable format.
func WriteGitConfigText(w io.Writer, resp *protocol.GitConfigResponse) {
	for _, e := range resp.Entries {
		fmt.Fprintf(w, "%s=%s\n", e.Key, e.Value)
	}
}

// WriteGitConfigStructuredText writes structured git config in human-readable format.
func WriteGitConfigStructuredText(w io.Writer, resp *protocol.GitConfigStructuredResponse) {
	if resp.Identity != nil {
		id := resp.Identity
		if id.UserName != "" {
			fmt.Fprintf(w, "user.name=%s\n", id.UserName)
		}
		if id.UserEmail != "" {
			fmt.Fprintf(w, "user.email=%s\n", id.UserEmail)
		}
	}
	for _, r := range resp.Remotes {
		for _, u := range r.URLs {
			fmt.Fprintf(w, "remote.%s.url=%s\n", r.Name, u)
		}
		for _, f := range r.Fetch {
			fmt.Fprintf(w, "remote.%s.fetch=%s\n", r.Name, f)
		}
	}
	for _, b := range resp.Branches {
		if b.Remote != "" {
			fmt.Fprintf(w, "branch.%s.remote=%s\n", b.Name, b.Remote)
		}
		if b.Merge != "" {
			fmt.Fprintf(w, "branch.%s.merge=%s\n", b.Name, b.Merge)
		}
	}
}

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

// WriteReadText writes read response data in human-readable format.
func WriteReadText(w io.Writer, resp *protocol.ReadResponse) {
	if resp.Chunk != nil && resp.Chunk.Encoding == "utf-8" {
		io.WriteString(w, resp.Chunk.Data) //nolint:errcheck
		if !strings.HasSuffix(resp.Chunk.Data, "\n") {
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

// WriteCatText writes a cat response with the specified divider format.
// Divider is one of "plain", "xml", or "none".
func WriteCatText(w io.Writer, resp *protocol.CatResponse, divider string) {
	for _, entry := range resp.Files {
		displayPath := entry.RelPath
		if displayPath == "" {
			displayPath = entry.Path
		}

		switch divider {
		case "xml":
			writeCatXML(w, entry, displayPath)
		case "none":
			writeCatNone(w, entry)
		default: // "plain"
			writeCatPlain(w, entry, displayPath)
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

func writeCatPlain(w io.Writer, entry protocol.CatEntry, path string) {
	switch {
	case entry.Error != "":
		fmt.Fprintf(w, "--- %s (error: %s) ---\n", path, entry.Error)
	case entry.Binary:
		fmt.Fprintf(w, "--- %s (binary, skipped) ---\n", path)
	default:
		fmt.Fprintf(w, "--- %s ---\n", path)
		io.WriteString(w, entry.Content) //nolint:errcheck
		if len(entry.Content) > 0 && entry.Content[len(entry.Content)-1] != '\n' {
			io.WriteString(w, "\n") //nolint:errcheck
		}
	}
}

func writeCatXML(w io.Writer, entry protocol.CatEntry, path string) {
	switch {
	case entry.Error != "":
		fmt.Fprintf(w, "<file path=%q error=%q />\n", path, entry.Error)
	case entry.Binary:
		fmt.Fprintf(w, "<file path=%q binary=\"true\" />\n", path)
	default:
		fmt.Fprintf(w, "<file path=%q>\n", path)
		io.WriteString(w, entry.Content) //nolint:errcheck
		if len(entry.Content) > 0 && entry.Content[len(entry.Content)-1] != '\n' {
			io.WriteString(w, "\n") //nolint:errcheck
		}
		fmt.Fprintln(w, "</file>")
	}
}

func writeCatNone(w io.Writer, entry protocol.CatEntry) {
	if entry.Error != "" || entry.Binary {
		return // skip silently in none mode
	}
	io.WriteString(w, entry.Content) //nolint:errcheck
}

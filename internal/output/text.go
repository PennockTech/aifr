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

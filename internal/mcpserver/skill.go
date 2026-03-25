// Copyright 2026 — see LICENSE file for terms.
package mcpserver

import (
	_ "embed"
	"io"
)

//go:embed skill.md
var skillContent string

// EmitSkill writes the SKILL.md content to w.
func EmitSkill(w io.Writer) error {
	_, err := io.WriteString(w, skillContent)
	return err
}

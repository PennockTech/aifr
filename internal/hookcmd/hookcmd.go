// Copyright 2026 — see LICENSE file for terms.
package hookcmd

import (
	"encoding/json"
	"fmt"
)

// HookInput is the JSON payload received from a Claude Code hook on stdin.
type HookInput struct {
	SessionID     string          `json:"session_id"`
	CWD           string          `json:"cwd"`
	ToolName      string          `json:"tool_name"`
	ToolInput     json.RawMessage `json:"tool_input"`
	HookEventName string          `json:"hook_event_name"`
}

// BashInput is the tool_input for a Bash tool call.
type BashInput struct {
	Command string `json:"command"`
}

// HookOutput is the JSON response for a Claude Code hook.
type HookOutput struct {
	HookSpecificOutput *HookDecision `json:"hookSpecificOutput"`
}

// HookDecision describes the hook's permission decision.
type HookDecision struct {
	HookEventName string `json:"hookEventName"`
	Decision      string `json:"permissionDecision"`
	Reason        string `json:"permissionDecisionReason,omitempty"`
}

// CheckCommand parses a PreToolUse hook payload and returns a hook output
// denying the command with an aifr suggestion, or nil if no suggestion applies.
//
// When forceMCP is true, suggestions always reference MCP tool calls.
// Otherwise, MCP availability is auto-detected from the working directory's
// .mcp.json and the AIFR_MCP environment variable.
func CheckCommand(input []byte, forceMCP bool) (*HookOutput, error) {
	var hi HookInput
	if err := json.Unmarshal(input, &hi); err != nil {
		return nil, err
	}

	if hi.ToolName != "Bash" {
		return nil, nil
	}

	var bi BashInput
	if err := json.Unmarshal(hi.ToolInput, &bi); err != nil {
		return nil, err
	}

	suggestion := AnalyzeCommand(bi.Command)
	if suggestion == nil {
		return nil, nil
	}

	mcpMode := forceMCP || detectMCPAvailable(hi.CWD)

	var reason string
	if mcpMode {
		reason = formatMCPReason(suggestion)
	} else {
		reason = formatCLIReason(suggestion)
	}

	return &HookOutput{
		HookSpecificOutput: &HookDecision{
			HookEventName: "PreToolUse",
			Decision:      "deny",
			Reason:        reason,
		},
	}, nil
}

func formatCLIReason(s *Suggestion) string {
	return "This " + s.Original +
		" invocation can be handled by aifr with access controls. Use: " +
		s.AifrCommand
}

func formatMCPReason(s *Suggestion) string {
	argsJSON, _ := json.Marshal(s.ToolArgs)
	return fmt.Sprintf(
		"This %s invocation can be handled by aifr with access controls. Use the %s tool: %s",
		s.Original, s.ToolName, string(argsJSON))
}

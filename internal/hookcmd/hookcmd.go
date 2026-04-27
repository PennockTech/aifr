// Copyright 2026 — see LICENSE file for terms.
package hookcmd

import (
	"encoding/json"
	"fmt"
)

// HookInput is the JSON payload received from a Claude Code hook on stdin.
// Both PreToolUse and PermissionRequest deliver these fields with the same
// names; PermissionRequest also includes transcript_path, permission_mode,
// and permission_suggestions, which we ignore.
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

// HookDecision describes the hook's permission decision in event-agnostic
// form. The on-the-wire JSON shape is selected by HookEventName via
// MarshalJSON: PermissionRequest emits a nested decision object, while
// PreToolUse (the default) emits flat permissionDecision fields.
type HookDecision struct {
	HookEventName string
	Decision      string
	Reason        string
}

type preToolUseDecisionJSON struct {
	HookEventName string `json:"hookEventName"`
	Decision      string `json:"permissionDecision"`
	Reason        string `json:"permissionDecisionReason,omitempty"`
}

type permissionRequestInner struct {
	Behavior string `json:"behavior"`
	Message  string `json:"message,omitempty"`
}

type permissionRequestDecisionJSON struct {
	HookEventName string                 `json:"hookEventName"`
	Decision      permissionRequestInner `json:"decision"`
}

// MarshalJSON emits the response shape that matches HookEventName.
// PermissionRequest is documented to use hookSpecificOutput.decision
// {behavior, message}; PreToolUse uses flat permissionDecision /
// permissionDecisionReason fields. Unknown or empty events default to
// the PreToolUse shape for backward compatibility.
func (d *HookDecision) MarshalJSON() ([]byte, error) {
	if d.HookEventName == "PermissionRequest" {
		return json.Marshal(permissionRequestDecisionJSON{
			HookEventName: d.HookEventName,
			Decision: permissionRequestInner{
				Behavior: d.Decision,
				Message:  d.Reason,
			},
		})
	}
	name := d.HookEventName
	if name == "" {
		name = "PreToolUse"
	}
	return json.Marshal(preToolUseDecisionJSON{
		HookEventName: name,
		Decision:      d.Decision,
		Reason:        d.Reason,
	})
}

// CheckCommand parses a PreToolUse or PermissionRequest hook payload and
// returns a hook output denying the command with an aifr suggestion, or nil
// if no suggestion applies. The output JSON shape is selected automatically
// to match the input's hook_event_name.
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

	eventName := hi.HookEventName
	if eventName == "" {
		eventName = "PreToolUse"
	}

	return &HookOutput{
		HookSpecificOutput: &HookDecision{
			HookEventName: eventName,
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

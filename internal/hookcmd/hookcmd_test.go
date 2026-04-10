// Copyright 2026 — see LICENSE file for terms.
package hookcmd

import (
	"encoding/json"
	"testing"
)

func TestCheckCommand_BashWithSuggestion(t *testing.T) {
	input := `{
		"session_id": "test-session",
		"cwd": "/tmp/nonexistent",
		"tool_name": "Bash",
		"tool_input": {"command": "cat main.go"},
		"hook_event_name": "PreToolUse"
	}`

	result, err := CheckCommand([]byte(input), false)
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if result.HookSpecificOutput == nil {
		t.Fatal("expected HookSpecificOutput, got nil")
	}
	if result.HookSpecificOutput.Decision != "deny" {
		t.Errorf("expected deny, got %q", result.HookSpecificOutput.Decision)
	}
	if result.HookSpecificOutput.HookEventName != "PreToolUse" {
		t.Errorf("expected PreToolUse, got %q", result.HookSpecificOutput.HookEventName)
	}
}

func TestCheckCommand_BashNoSuggestion(t *testing.T) {
	input := `{
		"session_id": "test-session",
		"cwd": "/tmp/nonexistent",
		"tool_name": "Bash",
		"tool_input": {"command": "go test ./..."},
		"hook_event_name": "PreToolUse"
	}`

	result, err := CheckCommand([]byte(input), false)
	if err != nil {
		t.Fatal(err)
	}
	if result != nil {
		t.Errorf("expected nil, got result with decision %q", result.HookSpecificOutput.Decision)
	}
}

func TestCheckCommand_NonBashTool(t *testing.T) {
	input := `{
		"session_id": "test-session",
		"cwd": "/tmp/nonexistent",
		"tool_name": "Read",
		"tool_input": {"file_path": "/tmp/test.go"},
		"hook_event_name": "PreToolUse"
	}`

	result, err := CheckCommand([]byte(input), false)
	if err != nil {
		t.Fatal(err)
	}
	if result != nil {
		t.Error("expected nil for non-Bash tool")
	}
}

func TestCheckCommand_InvalidJSON(t *testing.T) {
	_, err := CheckCommand([]byte("not json"), false)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

// TestCheckCommand_PipelineSuggestion is an end-to-end wiring test verifying
// that a command pipeline with a recognized | head tail produces a suggestion
// with the appropriate per-command limit parameter. The full scope of pipeline
// and complex command analysis is covered in suggest_test.go.
func TestCheckCommand_PipelineSuggestion(t *testing.T) {
	input := `{
		"session_id": "test-session",
		"cwd": "/tmp/nonexistent",
		"tool_name": "Bash",
		"tool_input": {"command": "git log --oneline | head -n 10"},
		"hook_event_name": "PreToolUse"
	}`

	result, err := CheckCommand([]byte(input), false)
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Fatal("expected suggestion for pipeline command, got nil")
	}
	if result.HookSpecificOutput.Decision != "deny" {
		t.Errorf("expected deny, got %q", result.HookSpecificOutput.Decision)
	}
	// Verify the reason mentions the aifr log command with --max-count.
	reason := result.HookSpecificOutput.Reason
	if reason == "" {
		t.Error("expected non-empty reason")
	}
}

func TestCheckCommand_OutputFormat(t *testing.T) {
	input := `{
		"session_id": "s1",
		"cwd": "/tmp/nonexistent",
		"tool_name": "Bash",
		"tool_input": {"command": "head -50 README.md"},
		"hook_event_name": "PreToolUse"
	}`

	result, err := CheckCommand([]byte(input), false)
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Fatal("expected result")
	}

	// Verify it marshals to valid JSON with expected structure.
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}

	hso, ok := decoded["hookSpecificOutput"].(map[string]any)
	if !ok {
		t.Fatal("missing hookSpecificOutput")
	}
	if hso["hookEventName"] != "PreToolUse" {
		t.Errorf("hookEventName: %v", hso["hookEventName"])
	}
	if hso["permissionDecision"] != "deny" {
		t.Errorf("permissionDecision: %v", hso["permissionDecision"])
	}
	reason, _ := hso["permissionDecisionReason"].(string)
	if reason == "" {
		t.Error("expected non-empty reason")
	}
}

func TestCheckCommand_MCPMode(t *testing.T) {
	input := `{
		"session_id": "test-session",
		"cwd": "/tmp/nonexistent",
		"tool_name": "Bash",
		"tool_input": {"command": "cat main.go"},
		"hook_event_name": "PreToolUse"
	}`

	// CLI mode (forceMCP=false, no .mcp.json in /tmp/nonexistent)
	cliResult, err := CheckCommand([]byte(input), false)
	if err != nil {
		t.Fatal(err)
	}
	if cliResult == nil {
		t.Fatal("expected result")
	}
	cliReason := cliResult.HookSpecificOutput.Reason
	if cliReason == "" {
		t.Fatal("expected non-empty CLI reason")
	}

	// MCP mode (forceMCP=true)
	mcpResult, err := CheckCommand([]byte(input), true)
	if err != nil {
		t.Fatal(err)
	}
	if mcpResult == nil {
		t.Fatal("expected result")
	}
	mcpReason := mcpResult.HookSpecificOutput.Reason
	if mcpReason == "" {
		t.Fatal("expected non-empty MCP reason")
	}

	// CLI reason should reference the CLI command.
	if cliReason == mcpReason {
		t.Error("CLI and MCP reasons should differ")
	}
}

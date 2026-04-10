// Copyright 2026 — see LICENSE file for terms.
package hookcmd

import (
	"reflect"
	"testing"
)

func TestParseShellCommand_Simple(t *testing.T) {
	cases := []struct {
		input    string
		wantName string
		wantArgs []string
	}{
		{"cat file.go", "cat", []string{"file.go"}},
		{"head -n 50 file.go", "head", []string{"-n", "50", "file.go"}},
		{`grep "hello world" .`, "grep", []string{"hello world", "."}},
		{`cat 'file with spaces.go'`, "cat", []string{"file with spaces.go"}},
		{"/usr/bin/cat file.go", "cat", []string{"file.go"}},
		{"ls -la src/", "ls", []string{"-la", "src/"}},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			parsed, mod := parseShellCommand(tc.input)
			if parsed == nil {
				t.Fatal("expected parsed command, got nil")
			}
			if parsed.Name != tc.wantName {
				t.Errorf("Name: got %q, want %q", parsed.Name, tc.wantName)
			}
			if !reflect.DeepEqual(parsed.Args, tc.wantArgs) {
				t.Errorf("Args: got %v, want %v", parsed.Args, tc.wantArgs)
			}
			if mod.IsSet() {
				t.Errorf("expected no modifier, got %+v", mod)
			}
		})
	}
}

func TestParseShellCommand_EnvVars(t *testing.T) {
	parsed, _ := parseShellCommand("LANG=C cat file.go")
	if parsed == nil {
		t.Fatal("expected parsed command, got nil")
	}
	if parsed.Name != "cat" {
		t.Errorf("Name: got %q, want %q", parsed.Name, "cat")
	}
	if !reflect.DeepEqual(parsed.Args, []string{"file.go"}) {
		t.Errorf("Args: got %v, want %v", parsed.Args, []string{"file.go"})
	}
}

func TestParseShellCommand_Redirections(t *testing.T) {
	// Redirections should not appear in Args (parser handles them separately).
	parsed, _ := parseShellCommand("cat file.go > out.txt")
	if parsed == nil {
		t.Fatal("expected parsed command, got nil")
	}
	if parsed.Name != "cat" {
		t.Errorf("Name: got %q, want %q", parsed.Name, "cat")
	}
	if !reflect.DeepEqual(parsed.Args, []string{"file.go"}) {
		t.Errorf("Args: got %v, want %v (redirections should be excluded)", parsed.Args, []string{"file.go"})
	}
}

func TestParseShellCommand_Pipeline(t *testing.T) {
	cases := []struct {
		input    string
		wantName string
		wantHead int
		wantTail int
	}{
		{"cat file.go | head -n 50", "cat", 50, 0},
		{"cat file.go | head -10", "cat", 10, 0},
		{"cat file.go | head", "cat", 10, 0}, // default 10
		{"cat file.go | tail -n 20", "cat", 0, 20},
		{"git log --oneline | head -n 10", "git", 10, 0},
		{"grep TODO . | head -5", "grep", 5, 0},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			parsed, mod := parseShellCommand(tc.input)
			if parsed == nil {
				t.Fatal("expected parsed command, got nil")
			}
			if parsed.Name != tc.wantName {
				t.Errorf("Name: got %q, want %q", parsed.Name, tc.wantName)
			}
			if mod.HeadLines != tc.wantHead {
				t.Errorf("HeadLines: got %d, want %d", mod.HeadLines, tc.wantHead)
			}
			if mod.TailLines != tc.wantTail {
				t.Errorf("TailLines: got %d, want %d", mod.TailLines, tc.wantTail)
			}
		})
	}
}

func TestParseShellCommand_PipelineSed(t *testing.T) {
	cases := []struct {
		input     string
		wantName  string
		wantHead  int
		wantStart int
		wantEnd   int
	}{
		// sed -n '1,Np' normalizes to HeadLines.
		{"cat file.go | sed -n '1,50p'", "cat", 50, 0, 0},
		{"cat file.go | sed -n '1p'", "cat", 1, 0, 0},
		// Arbitrary ranges set StartLine/EndLine.
		{"cat file.go | sed -n '5,10p'", "cat", 0, 5, 10},
		{"cat file.go | sed -n '100,200p'", "cat", 0, 100, 200},
		// Single line extraction.
		{"cat file.go | sed -n '42p'", "cat", 0, 42, 42},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			parsed, mod := parseShellCommand(tc.input)
			if parsed == nil {
				t.Fatal("expected parsed command, got nil")
			}
			if parsed.Name != tc.wantName {
				t.Errorf("Name: got %q, want %q", parsed.Name, tc.wantName)
			}
			if mod.HeadLines != tc.wantHead {
				t.Errorf("HeadLines: got %d, want %d", mod.HeadLines, tc.wantHead)
			}
			if mod.StartLine != tc.wantStart {
				t.Errorf("StartLine: got %d, want %d", mod.StartLine, tc.wantStart)
			}
			if mod.EndLine != tc.wantEnd {
				t.Errorf("EndLine: got %d, want %d", mod.EndLine, tc.wantEnd)
			}
		})
	}
}

func TestParseShellCommand_Complex(t *testing.T) {
	// All of these should return nil (too complex to analyze).
	cases := []struct {
		name    string
		command string
	}{
		{"empty", ""},
		{"three-stage pipeline", "cat file | grep pattern | head -5"},
		{"unknown pipe target", "cat file | sort"},
		{"double ampersand", "cd /tmp && ls"},
		{"logical or", "cat file || echo fallback"},
		{"semicolon", "echo hello; echo world"},
		{"background", "cat file &"},
		{"subshell", "(cat file)"},
		{"pipe in quotes OK", `grep "a|b" file`}, // single command, not nil
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			parsed, _ := parseShellCommand(tc.command)
			switch tc.name {
			case "pipe in quotes OK":
				if parsed == nil {
					t.Error("grep with | in pattern should parse as simple command")
				}
			default:
				if parsed != nil {
					t.Errorf("expected nil for complex command, got %+v", parsed)
				}
			}
		})
	}
}

func TestShellQuote(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"file.go", "file.go"},
		{"src/main.go", "src/main.go"},
		{"path with spaces", "'path with spaces'"},
		{"it's", "'it'\\''s'"},
		{"", "''"},
		{"*.go", "'*.go'"},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got := shellQuote(tc.input)
			if got != tc.want {
				t.Errorf("shellQuote(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestNonFlags(t *testing.T) {
	cases := []struct {
		input []string
		want  []string
	}{
		{[]string{"-n", "file.go"}, []string{"file.go"}},
		{[]string{"-la", "src/"}, []string{"src/"}},
		{[]string{"-l", "-w", "file.go"}, []string{"file.go"}},
	}
	for _, tc := range cases {
		t.Run("", func(t *testing.T) {
			got := nonFlags(tc.input)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("nonFlags(%v) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

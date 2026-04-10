// Copyright 2026 — see LICENSE file for terms.
package hookcmd

import (
	"reflect"
	"testing"
)

func TestTokenize(t *testing.T) {
	cases := []struct {
		input string
		want  []string
	}{
		{"cat file.go", []string{"cat", "file.go"}},
		{"head -n 50 file.go", []string{"head", "-n", "50", "file.go"}},
		{`grep "hello world" .`, []string{"grep", "hello world", "."}},
		{`cat 'file with spaces.go'`, []string{"cat", "file with spaces.go"}},
		{`echo foo\ bar`, []string{"echo", "foo bar"}},
		{"  head  -n  10  file.go  ", []string{"head", "-n", "10", "file.go"}},
		{"", nil},
		{"/usr/bin/cat file.go", []string{"/usr/bin/cat", "file.go"}},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got := tokenize(tc.input)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("tokenize(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestHasShellOperators(t *testing.T) {
	cases := []struct {
		input string
		want  bool
	}{
		{"cat file.go", false},
		{"cat file.go | head", true},
		{"cd /tmp && ls", true},
		{"echo hello; echo world", true},
		{"echo 'hello | world'", false},  // pipe inside quotes
		{`echo "hello && world"`, false}, // && inside quotes
		{"cat file.go &", true},          // backgrounding
		{"echo $(date)", true},           // subshell
		{"echo `date`", true},            // backtick subshell
		{"head -n 50 file.go", false},
		{`grep "a;b" file.go`, false}, // semicolon in quotes
		{`grep 'a|b' file.go`, false}, // pipe in quotes
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got := hasShellOperators(tc.input)
			if got != tc.want {
				t.Errorf("hasShellOperators(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestBaseName(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"cat", "cat"},
		{"/usr/bin/cat", "cat"},
		{"/bin/grep", "grep"},
		{"./local/bin/rg", "rg"},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got := baseName(tc.input)
			if got != tc.want {
				t.Errorf("baseName(%q) = %q, want %q", tc.input, got, tc.want)
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
		{[]string{"file.go", ">", "out.txt"}, []string{"file.go"}},
		{[]string{"file.go", ">>", "out.txt"}, []string{"file.go"}},
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

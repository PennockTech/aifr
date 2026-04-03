// Copyright 2026 — see LICENSE file for terms.
package engine

import "testing"

func TestSanitizeMessage(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "plain text unchanged",
			input: "feat: add logging support",
			want:  "feat: add logging support",
		},
		{
			name:  "newlines preserved",
			input: "subject\n\nbody line 1\nbody line 2",
			want:  "subject\n\nbody line 1\nbody line 2",
		},
		{
			name:  "tabs preserved",
			input: "subject\n\n\tindented body",
			want:  "subject\n\n\tindented body",
		},
		{
			name:  "carriage return escaped",
			input: "legit subject\rmalicious overlay",
			want:  `legit subject\rmalicious overlay`,
		},
		{
			name:  "CR at start of line",
			input: "line1\n\rline2",
			want:  `line1` + "\n" + `\rline2`,
		},
		{
			name:  "CRLF: CR escaped, LF preserved",
			input: "line1\r\nline2",
			want:  `line1\r` + "\n" + "line2",
		},
		{
			name:  "escape sequence",
			input: "normal\x1b[31mred text\x1b[0m",
			want:  `normal\e[31mred text\e[0m`,
		},
		{
			name:  "null byte",
			input: "before\x00after",
			want:  `before\x00after`,
		},
		{
			name:  "bell character",
			input: "ding\x07dong",
			want:  `ding\x07dong`,
		},
		{
			name:  "DEL character",
			input: "test\x7fvalue",
			want:  `test\x7fvalue`,
		},
		{
			name:  "multiple control chars",
			input: "\x01\x02\x03",
			want:  `\x01\x02\x03`,
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "no control chars fast path",
			input: "just a normal commit message with unicode: café 日本語",
			want:  "just a normal commit message with unicode: café 日本語",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeMessage(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeMessage(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

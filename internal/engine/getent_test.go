// Copyright 2026 — see LICENSE file for terms.
package engine

import "testing"

func TestGecosName(t *testing.T) {
	tests := []struct {
		name  string
		gecos string
		login string
		want  string
	}{
		{
			name:  "simple name no commas",
			gecos: "John Doe",
			login: "jdoe",
			want:  "John Doe",
		},
		{
			name:  "BSD four-field format",
			gecos: "John Doe,Room 101,555-1234,555-5678",
			login: "jdoe",
			want:  "John Doe",
		},
		{
			name:  "ampersand replacement",
			gecos: "&",
			login: "jdoe",
			want:  "Jdoe",
		},
		{
			name:  "ampersand in full name",
			gecos: "& Smith,Office,Phone",
			login: "john",
			want:  "John Smith",
		},
		{
			name:  "multiple ampersands",
			gecos: "& &,Office",
			login: "bob",
			want:  "Bob Bob",
		},
		{
			name:  "empty gecos",
			gecos: "",
			login: "nobody",
			want:  "",
		},
		{
			name:  "gecos is just commas",
			gecos: ",,,",
			login: "test",
			want:  "",
		},
		{
			name:  "no ampersand with commas",
			gecos: "Root User,,,",
			login: "root",
			want:  "Root User",
		},
		{
			name:  "empty login with ampersand",
			gecos: "&",
			login: "",
			want:  "",
		},
		{
			name:  "unicode login capitalization",
			gecos: "&",
			login: "ärni",
			want:  "Ärni",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := gecosName(tt.gecos, tt.login)
			if got != tt.want {
				t.Errorf("gecosName(%q, %q) = %q, want %q", tt.gecos, tt.login, got, tt.want)
			}
		})
	}
}

func TestPasswdMapperGecosName(t *testing.T) {
	// Simulate a passwd line: name:x:uid:gid:gecos:home:shell
	parts := []string{"jdoe", "x", "1000", "1000", "& Doe,Room 42,555-1234,555-5678", "/home/jdoe", "/bin/bash"}
	fields, keys := passwdMapper(parts)
	if fields == nil {
		t.Fatal("passwdMapper returned nil")
	}

	if got := fields["gecos_name"]; got != "Jdoe Doe" {
		t.Errorf("gecos_name = %q, want %q", got, "Jdoe Doe")
	}
	if got := fields["gecos"]; got != "& Doe,Room 42,555-1234,555-5678" {
		t.Errorf("gecos = %q, want original value", got)
	}
	if got := fields["name"]; got != "jdoe" {
		t.Errorf("name = %q, want %q", got, "jdoe")
	}
	if len(keys) != 2 || keys[0] != "jdoe" || keys[1] != "1000" {
		t.Errorf("keys = %v, want [jdoe 1000]", keys)
	}
}

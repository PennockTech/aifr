// Copyright 2026 — see LICENSE file for terms.
package output

import (
	"os"
	"testing"
)

func TestResolveFormat(t *testing.T) {
	supported := []string{"json", "text"}

	tests := []struct {
		name     string
		explicit string
		env      string
		want     string
		wantErr  bool
	}{
		{
			name: "no explicit no env",
			want: "json",
		},
		{
			name:     "explicit text",
			explicit: "text",
			want:     "text",
		},
		{
			name:     "explicit overrides env",
			explicit: "json",
			env:      "text",
			want:     "json",
		},
		{
			name: "env text",
			env:  "text",
			want: "text",
		},
		{
			name: "env preference list first match",
			env:  "yaml:text:json",
			want: "text",
		},
		{
			name:    "env preference list all unsupported",
			env:     "yaml:xml:csv",
			wantErr: true,
		},
		{
			name:     "explicit unsupported",
			explicit: "yaml",
			wantErr:  true,
		},
		{
			name: "env with spaces",
			env:  " text : json ",
			want: "text",
		},
		{
			name: "env empty entries ignored",
			env:  "::text::",
			want: "text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.env != "" {
				os.Setenv("AIFR_FORMAT", tt.env)
				defer os.Unsetenv("AIFR_FORMAT")
			} else {
				os.Unsetenv("AIFR_FORMAT")
			}

			got, err := ResolveFormat(tt.explicit, supported, "json")
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %q", got)
				}
				if _, ok := err.(*UnsupportedFormatError); !ok {
					t.Fatalf("expected UnsupportedFormatError, got %T: %v", err, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestUnsupportedFormatError(t *testing.T) {
	e := &UnsupportedFormatError{Requested: "yaml", Supported: []string{"json", "text"}}
	want := `unsupported format "yaml" (supported: json, text)`
	if got := e.Error(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	e.FromEnv = true
	want = `unsupported AIFR_FORMAT "yaml" (supported: json, text)`
	if got := e.Error(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

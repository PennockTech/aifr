// Copyright 2026 — see LICENSE file for terms.
package hookcmd

import "testing"

func TestAnalyzeCommand_NoSuggestion(t *testing.T) {
	cases := []struct {
		name    string
		command string
	}{
		{"empty", ""},
		{"aifr invocation", "aifr read file.go"},
		{"aifr bare", "aifr"},
		{"unrecognized command", "go build ./..."},
		{"make", "make test"},
		{"npm", "npm install"},
		{"pipe chain", "cat file.go | head -10"},
		{"double ampersand", "cd /tmp && ls"},
		{"semicolon", "echo hello; echo world"},
		{"subshell", "$(cat file.go)"},
		{"cat from stdin", "cat"},
		{"cat from stdin with flags", "cat -v"},
		{"head from stdin", "head -n 5"},
		{"tail -f", "tail -f server.log"},
		{"tail --follow", "tail --follow server.log"},
		{"grep from stdin", "grep pattern"},
		{"wc from stdin", "wc -l"},
		{"stat no args", "stat"},
		{"sed without -n", "sed 's/foo/bar/' file.go"},
		{"git status", "git status"},
		{"git push", "git push origin main"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := AnalyzeCommand(tc.command)
			if s != nil {
				t.Errorf("expected nil, got suggestion: %s", s.AifrCommand)
			}
		})
	}
}

func TestAnalyzeCommand_Cat(t *testing.T) {
	cases := []struct {
		command string
		want    string
	}{
		{"cat file.go", "aifr read file.go"},
		{"cat src/main.go", "aifr read src/main.go"},
		{"/usr/bin/cat file.go", "aifr read file.go"},
		{"cat file1.go file2.go", "aifr cat file1.go file2.go"},
		{"cat -n file.go", "aifr read file.go"},
	}
	for _, tc := range cases {
		t.Run(tc.command, func(t *testing.T) {
			s := AnalyzeCommand(tc.command)
			if s == nil {
				t.Fatal("expected suggestion, got nil")
			}
			if s.AifrCommand != tc.want {
				t.Errorf("got %q, want %q", s.AifrCommand, tc.want)
			}
			if s.Original != "cat" {
				t.Errorf("original: got %q, want %q", s.Original, "cat")
			}
		})
	}
}

func TestAnalyzeCommand_Head(t *testing.T) {
	cases := []struct {
		command string
		want    string
	}{
		{"head file.go", "aifr read --lines=1:10 file.go"},
		{"head -n 50 file.go", "aifr read --lines=1:50 file.go"},
		{"head -n50 file.go", "aifr read --lines=1:50 file.go"},
		{"head -20 file.go", "aifr read --lines=1:20 file.go"},
		{"head -n 100 src/main.go", "aifr read --lines=1:100 src/main.go"},
	}
	for _, tc := range cases {
		t.Run(tc.command, func(t *testing.T) {
			s := AnalyzeCommand(tc.command)
			if s == nil {
				t.Fatal("expected suggestion, got nil")
			}
			if s.AifrCommand != tc.want {
				t.Errorf("got %q, want %q", s.AifrCommand, tc.want)
			}
		})
	}
}

func TestAnalyzeCommand_Tail(t *testing.T) {
	cases := []struct {
		command string
		want    string
	}{
		{"tail file.go", "aifr read --lines=-10: file.go"},
		{"tail -n 20 file.go", "aifr read --lines=-20: file.go"},
		{"tail -n20 file.go", "aifr read --lines=-20: file.go"},
		{"tail -5 file.go", "aifr read --lines=-5: file.go"},
	}
	for _, tc := range cases {
		t.Run(tc.command, func(t *testing.T) {
			s := AnalyzeCommand(tc.command)
			if s == nil {
				t.Fatal("expected suggestion, got nil")
			}
			if s.AifrCommand != tc.want {
				t.Errorf("got %q, want %q", s.AifrCommand, tc.want)
			}
		})
	}
}

func TestAnalyzeCommand_Grep(t *testing.T) {
	cases := []struct {
		command string
		want    string
	}{
		{"grep TODO src/", "aifr search TODO src/"},
		{"grep -r pattern .", "aifr search pattern ."},
		{"grep -rn 'func main' .", "aifr search 'func main' ."},
		{"rg pattern src/", "aifr search pattern src/"},
		{"egrep 'foo|bar' dir/", "aifr search 'foo|bar' dir/"},
	}
	for _, tc := range cases {
		t.Run(tc.command, func(t *testing.T) {
			s := AnalyzeCommand(tc.command)
			if s == nil {
				t.Fatal("expected suggestion, got nil")
			}
			if s.AifrCommand != tc.want {
				t.Errorf("got %q, want %q", s.AifrCommand, tc.want)
			}
		})
	}
}

func TestAnalyzeCommand_Find(t *testing.T) {
	cases := []struct {
		command string
		want    string
	}{
		{"find .", "aifr find ."},
		{"find . -name '*.go'", "aifr find . --name='*.go'"},
		{"find . -name '*.go' -type f", "aifr find . --name='*.go' --type=f"},
		{"find src/ -type d", "aifr find src/ --type=d"},
	}
	for _, tc := range cases {
		t.Run(tc.command, func(t *testing.T) {
			s := AnalyzeCommand(tc.command)
			if s == nil {
				t.Fatal("expected suggestion, got nil")
			}
			if s.AifrCommand != tc.want {
				t.Errorf("got %q, want %q", s.AifrCommand, tc.want)
			}
		})
	}
}

func TestAnalyzeCommand_Ls(t *testing.T) {
	cases := []struct {
		command string
		want    string
	}{
		{"ls", "aifr list ."},
		{"ls src/", "aifr list src/"},
		{"ls -la src/", "aifr list src/"},
		{"ls -R src/", "aifr find src/"},
	}
	for _, tc := range cases {
		t.Run(tc.command, func(t *testing.T) {
			s := AnalyzeCommand(tc.command)
			if s == nil {
				t.Fatal("expected suggestion, got nil")
			}
			if s.AifrCommand != tc.want {
				t.Errorf("got %q, want %q", s.AifrCommand, tc.want)
			}
		})
	}
}

func TestAnalyzeCommand_Wc(t *testing.T) {
	cases := []struct {
		command string
		want    string
	}{
		{"wc file.go", "aifr wc file.go"},
		{"wc -l file.go", "aifr wc -l file.go"},
		{"wc -l -w file.go", "aifr wc -l -w file.go"},
		{"wc file1.go file2.go", "aifr wc file1.go file2.go"},
	}
	for _, tc := range cases {
		t.Run(tc.command, func(t *testing.T) {
			s := AnalyzeCommand(tc.command)
			if s == nil {
				t.Fatal("expected suggestion, got nil")
			}
			if s.AifrCommand != tc.want {
				t.Errorf("got %q, want %q", s.AifrCommand, tc.want)
			}
		})
	}
}

func TestAnalyzeCommand_Stat(t *testing.T) {
	s := AnalyzeCommand("stat file.go")
	if s == nil {
		t.Fatal("expected suggestion")
	}
	if s.AifrCommand != "aifr stat file.go" {
		t.Errorf("got %q", s.AifrCommand)
	}
}

func TestAnalyzeCommand_Diff(t *testing.T) {
	s := AnalyzeCommand("diff file1.go file2.go")
	if s == nil {
		t.Fatal("expected suggestion")
	}
	if s.AifrCommand != "aifr diff file1.go file2.go" {
		t.Errorf("got %q", s.AifrCommand)
	}
}

func TestAnalyzeCommand_Checksum(t *testing.T) {
	cases := []struct {
		command string
		want    string
	}{
		{"sha256sum file.go", "aifr checksum -a sha256 file.go"},
		{"md5sum file.go", "aifr checksum -a md5 file.go"},
		{"sha1sum file.go", "aifr checksum -a sha1 file.go"},
	}
	for _, tc := range cases {
		t.Run(tc.command, func(t *testing.T) {
			s := AnalyzeCommand(tc.command)
			if s == nil {
				t.Fatal("expected suggestion, got nil")
			}
			if s.AifrCommand != tc.want {
				t.Errorf("got %q, want %q", s.AifrCommand, tc.want)
			}
		})
	}
}

func TestAnalyzeCommand_Hexdump(t *testing.T) {
	cases := []struct {
		command string
		want    string
	}{
		{"hexdump file.bin", "aifr hexdump file.bin"},
		{"xxd file.bin", "aifr hexdump file.bin"},
	}
	for _, tc := range cases {
		t.Run(tc.command, func(t *testing.T) {
			s := AnalyzeCommand(tc.command)
			if s == nil {
				t.Fatal("expected suggestion, got nil")
			}
			if s.AifrCommand != tc.want {
				t.Errorf("got %q, want %q", s.AifrCommand, tc.want)
			}
		})
	}
}

func TestAnalyzeCommand_Sed(t *testing.T) {
	cases := []struct {
		command string
		want    string
	}{
		{"sed -n '5p' file.go", "aifr read --lines=5:5 file.go"},
		{"sed -n '10,20p' file.go", "aifr read --lines=10:20 file.go"},
	}
	for _, tc := range cases {
		t.Run(tc.command, func(t *testing.T) {
			s := AnalyzeCommand(tc.command)
			if s == nil {
				t.Fatal("expected suggestion, got nil")
			}
			if s.AifrCommand != tc.want {
				t.Errorf("got %q, want %q", s.AifrCommand, tc.want)
			}
		})
	}
}

func TestAnalyzeCommand_GitLog(t *testing.T) {
	s := AnalyzeCommand("git log")
	if s == nil {
		t.Fatal("expected suggestion")
	}
	if s.AifrCommand != "aifr log" {
		t.Errorf("got %q", s.AifrCommand)
	}
}

func TestAnalyzeCommand_EnvPrefix(t *testing.T) {
	s := AnalyzeCommand("LANG=C cat file.go")
	if s == nil {
		t.Fatal("expected suggestion")
	}
	if s.AifrCommand != "aifr read file.go" {
		t.Errorf("got %q", s.AifrCommand)
	}
}

func TestAnalyzeCommand_QuotedPaths(t *testing.T) {
	s := AnalyzeCommand(`cat "path with spaces/file.go"`)
	if s == nil {
		t.Fatal("expected suggestion")
	}
	if s.AifrCommand != "aifr read 'path with spaces/file.go'" {
		t.Errorf("got %q", s.AifrCommand)
	}
}

func TestAnalyzeCommand_AbsoluteCommandPath(t *testing.T) {
	s := AnalyzeCommand("/usr/bin/head -n 5 file.go")
	if s == nil {
		t.Fatal("expected suggestion")
	}
	if s.AifrCommand != "aifr read --lines=1:5 file.go" {
		t.Errorf("got %q", s.AifrCommand)
	}
}

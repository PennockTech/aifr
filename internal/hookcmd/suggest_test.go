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
		{"3-stage pipeline", "cat file | grep pattern | head -5"},
		{"unknown pipe target", "cat file | sort"},
		{"double ampersand", "cd /tmp && ls"},
		{"semicolon", "echo hello; echo world"},
		{"subshell", "$(cat file.go)"},
		{"cat from stdin", "cat"},
		{"cat from stdin with flags", "cat -v"},
		{"head from stdin", "head -n 5"},
		{"tail -f", "tail -f server.log"},
		{"tail --follow", "tail --follow server.log"},
		{"tail -f pipe", "tail -f server.log | head -5"},
		{"grep from stdin", "grep pattern"},
		{"wc from stdin", "wc -l"},
		{"stat no args", "stat"},
		{"sed without -n", "sed 's/foo/bar/' file.go"},
		{"git status", "git status"},
		{"git push", "git push origin main"},
		{"wc with head", "wc -l file.go | head -5"},
		{"stat with head", "stat file.go | head -5"},
		{"diff with head", "diff a.go b.go | head -5"},
		{"tail with head", "tail -20 file.go | head -5"},
		{"head with tail", "head -20 file.go | tail -5"},
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
		command  string
		want     string
		wantTool string
	}{
		{"cat file.go", "aifr read file.go", "aifr_read"},
		{"cat src/main.go", "aifr read src/main.go", "aifr_read"},
		{"/usr/bin/cat file.go", "aifr read file.go", "aifr_read"},
		{"cat file1.go file2.go", "aifr cat file1.go file2.go", "aifr_cat"},
		{"cat -n file.go", "aifr read file.go", "aifr_read"},
	}
	for _, tc := range cases {
		t.Run(tc.command, func(t *testing.T) {
			s := AnalyzeCommand(tc.command)
			if s == nil {
				t.Fatal("expected suggestion, got nil")
			}
			if s.AifrCommand != tc.want {
				t.Errorf("AifrCommand: got %q, want %q", s.AifrCommand, tc.want)
			}
			if s.ToolName != tc.wantTool {
				t.Errorf("ToolName: got %q, want %q", s.ToolName, tc.wantTool)
			}
			if s.Original != "cat" {
				t.Errorf("Original: got %q, want %q", s.Original, "cat")
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

// --- Pipeline tests ---

func TestAnalyzeCommand_PipelineHeadCat(t *testing.T) {
	cases := []struct {
		command string
		want    string
	}{
		{"cat file.go | head -n 50", "aifr read --lines=1:50 file.go"},
		{"cat file.go | head -10", "aifr read --lines=1:10 file.go"},
		{"cat file.go | head", "aifr read --lines=1:10 file.go"}, // default 10
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
			if s.ToolName != "aifr_read" {
				t.Errorf("ToolName: got %q, want %q", s.ToolName, "aifr_read")
			}
		})
	}
}

func TestAnalyzeCommand_PipelineTailCat(t *testing.T) {
	s := AnalyzeCommand("cat file.go | tail -n 20")
	if s == nil {
		t.Fatal("expected suggestion, got nil")
	}
	if s.AifrCommand != "aifr read --lines=-20: file.go" {
		t.Errorf("got %q", s.AifrCommand)
	}
}

func TestAnalyzeCommand_PipelineHeadGitLog(t *testing.T) {
	cases := []struct {
		command string
		want    string
	}{
		{"git log --oneline | head -n 10", "aifr log --oneline --max-count=10"},
		{"git log | head -n 5", "aifr log --max-count=5"},
		{"git log --oneline | head -20", "aifr log --oneline --max-count=20"},
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
			if s.ToolName != "aifr_log" {
				t.Errorf("ToolName: got %q, want %q", s.ToolName, "aifr_log")
			}
		})
	}
}

func TestAnalyzeCommand_PipelineHeadGrep(t *testing.T) {
	s := AnalyzeCommand("grep -rn TODO . | head -n 20")
	if s == nil {
		t.Fatal("expected suggestion, got nil")
	}
	if s.AifrCommand != "aifr search --max-matches=20 TODO ." {
		t.Errorf("got %q", s.AifrCommand)
	}
}

func TestAnalyzeCommand_PipelineHeadFind(t *testing.T) {
	s := AnalyzeCommand("find . -name '*.go' | head -n 30")
	if s == nil {
		t.Fatal("expected suggestion, got nil")
	}
	if s.AifrCommand != "aifr find . --name='*.go' --limit=30" {
		t.Errorf("got %q", s.AifrCommand)
	}
}

func TestAnalyzeCommand_PipelineHeadLs(t *testing.T) {
	s := AnalyzeCommand("ls -la src/ | head -n 20")
	if s == nil {
		t.Fatal("expected suggestion, got nil")
	}
	if s.AifrCommand != "aifr list src/ --limit=20" {
		t.Errorf("got %q", s.AifrCommand)
	}
}

func TestAnalyzeCommand_PipelineSedCat(t *testing.T) {
	cases := []struct {
		command string
		want    string
	}{
		{"cat file.go | sed -n '5,10p'", "aifr read --lines=5:10 file.go"},
		{"cat file.go | sed -n '42p'", "aifr read --lines=42:42 file.go"},
		{"cat file.go | sed -n '100,200p'", "aifr read --lines=100:200 file.go"},
		// start=1 normalizes to HeadLines, still produces the right suggestion.
		{"cat file.go | sed -n '1,50p'", "aifr read --lines=1:50 file.go"},
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
			if s.ToolName != "aifr_read" {
				t.Errorf("ToolName: got %q, want %q", s.ToolName, "aifr_read")
			}
		})
	}
}

func TestAnalyzeCommand_PipelineSedNoSuggestion(t *testing.T) {
	// sed line ranges on non-cat commands don't map to aifr parameters.
	cases := []struct {
		name    string
		command string
	}{
		{"grep with sed range", "grep TODO . | sed -n '5,10p'"},
		{"find with sed range", "find . -name '*.go' | sed -n '5,20p'"},
		{"git log with sed range", "git log | sed -n '5,10p'"},
		{"sed without -n", "cat file.go | sed '5,10p'"},
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

// --- MCP tool info tests ---

func TestAnalyzeCommand_MCPToolArgs(t *testing.T) {
	cases := []struct {
		name     string
		command  string
		wantTool string
		wantArg  string
		wantVal  any
	}{
		{"cat path", "cat main.go", "aifr_read", "path", "main.go"},
		{"grep pattern", "grep TODO src/", "aifr_search", "pattern", "TODO"},
		{"find name", "find . -name '*.go'", "aifr_find", "name", "*.go"},
		{"git log oneline", "git log --oneline", "aifr_log", "format", "oneline"},
		{"pipeline max_count", "git log | head -5", "aifr_log", "max_count", 5},
		{"pipeline limit", "find . | head -30", "aifr_find", "limit", 30},
		{"diff paths", "diff a.go b.go", "aifr_diff", "path_a", "a.go"},
		{"checksum algo", "sha256sum f.go", "aifr_checksum", "algorithm", "sha256"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := AnalyzeCommand(tc.command)
			if s == nil {
				t.Fatal("expected suggestion, got nil")
			}
			if s.ToolName != tc.wantTool {
				t.Errorf("ToolName: got %q, want %q", s.ToolName, tc.wantTool)
			}
			got, ok := s.ToolArgs[tc.wantArg]
			if !ok {
				t.Errorf("ToolArgs missing key %q; args=%v", tc.wantArg, s.ToolArgs)
				return
			}
			// Compare with type awareness: int vs float64 from JSON.
			switch want := tc.wantVal.(type) {
			case int:
				if got != want {
					t.Errorf("ToolArgs[%q]: got %v (%T), want %v", tc.wantArg, got, got, want)
				}
			default:
				if got != want {
					t.Errorf("ToolArgs[%q]: got %v, want %v", tc.wantArg, got, want)
				}
			}
		})
	}
}

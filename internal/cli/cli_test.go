package cli

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"testing"
)

// osReadError returns the operating system's message for reading a
// missing file ("no such file or directory" on Unix, a different text on
// Windows), so tests do not depend on the platform.
func osReadError(t *testing.T, path string) string {
	t.Helper()
	_, err := os.ReadFile(path)
	var pe *fs.PathError
	if !errors.As(err, &pe) {
		t.Fatalf("expected a path error reading %s, got %v", path, err)
	}
	return pe.Err.Error()
}

func TestRun(t *testing.T) {
	versionLine := "aura " + Version + "\n"
	missing := "error: cannot read does-not-exist.aura: " + osReadError(t, "does-not-exist.aura") + "\n"

	tests := []struct {
		name       string
		args       []string
		wantCode   int
		wantStdout string
		wantStderr string
	}{
		{
			name:       "version command",
			args:       []string{"version"},
			wantCode:   ExitOK,
			wantStdout: versionLine,
		},
		{
			name:       "version flag",
			args:       []string{"--version"},
			wantCode:   ExitOK,
			wantStdout: versionLine,
		},
		{
			name:       "help command",
			args:       []string{"help"},
			wantCode:   ExitOK,
			wantStdout: usage,
		},
		{
			name:       "short help flag",
			args:       []string{"-h"},
			wantCode:   ExitOK,
			wantStdout: usage,
		},
		{
			name:       "long help flag",
			args:       []string{"--help"},
			wantCode:   ExitOK,
			wantStdout: usage,
		},
		{
			name:       "no arguments",
			args:       nil,
			wantCode:   ExitUsage,
			wantStderr: usage,
		},
		{
			name:       "unknown command",
			args:       []string{"bogus"},
			wantCode:   ExitUsage,
			wantStderr: "error: unknown command \"bogus\"\n\n" + usage,
		},
		{
			name:       "run nonexistent file",
			args:       []string{"run", "does-not-exist.aura"},
			wantCode:   ExitUsage,
			wantStderr: missing,
		},
		{
			name:       "check nonexistent file",
			args:       []string{"check", "does-not-exist.aura"},
			wantCode:   ExitUsage,
			wantStderr: missing,
		},
		{
			name:       "run rejects the old .nova extension",
			args:       []string{"run", "hello.nova"},
			wantCode:   ExitUsage,
			wantStderr: "error: hello.nova is not an AURA source file: expected the .aura extension\n",
		},
		{
			name:       "check rejects other extensions",
			args:       []string{"check", "notes.txt"},
			wantCode:   ExitUsage,
			wantStderr: "error: notes.txt is not an AURA source file: expected the .aura extension\n",
		},
		{
			name:       "run rejects a missing extension",
			args:       []string{"run", "program"},
			wantCode:   ExitUsage,
			wantStderr: "error: program is not an AURA source file: expected the .aura extension\n",
		},
		{
			name:       "extension match is case-sensitive",
			args:       []string{"run", "HELLO.AURA"},
			wantCode:   ExitUsage,
			wantStderr: "error: HELLO.AURA is not an AURA source file: expected the .aura extension\n",
		},
		{
			name:       "run without file",
			args:       []string{"run"},
			wantCode:   ExitUsage,
			wantStderr: "error: run requires exactly one file argument\n\n" + usage,
		},
		{
			name:       "check without file",
			args:       []string{"check"},
			wantCode:   ExitUsage,
			wantStderr: "error: check requires exactly one file argument\n\n" + usage,
		},
		{
			name:       "run with two files",
			args:       []string{"run", "a.aura", "b.aura"},
			wantCode:   ExitUsage,
			wantStderr: "error: run requires exactly one file argument\n\n" + usage,
		},
		{
			name:       "build is not implemented",
			args:       []string{"build", "a.aura"},
			wantCode:   ExitUsage,
			wantStderr: "error: unknown command \"build\"\n\n" + usage,
		},
		{
			name:       "version with extra argument",
			args:       []string{"version", "extra"},
			wantCode:   ExitUsage,
			wantStderr: "error: version takes no arguments\n\n" + usage,
		},
		{
			name:       "help with extra argument",
			args:       []string{"help", "extra"},
			wantCode:   ExitUsage,
			wantStderr: "error: help takes no arguments\n\n" + usage,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := Run(tt.args, &stdout, &stderr)

			if code != tt.wantCode {
				t.Errorf("exit code = %d, want %d", code, tt.wantCode)
			}
			if got := stdout.String(); got != tt.wantStdout {
				t.Errorf("stdout = %q, want %q", got, tt.wantStdout)
			}
			if got := stderr.String(); got != tt.wantStderr {
				t.Errorf("stderr = %q, want %q", got, tt.wantStderr)
			}
		})
	}
}

func TestVersionIsSet(t *testing.T) {
	if Version == "" {
		t.Fatal("Version must not be empty")
	}
}

package main

import (
	"bytes"
	"errors"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// TestBinary builds the real tread executable and runs it as a subprocess,
// checking stdout, stderr and the process exit code (docs/10 §10.8).
func TestBinary(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the binary")
	}
	bin := filepath.Join(t.TempDir(), "tread")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	root := filepath.Join("..", "..")
	tests := []struct {
		args       []string
		exit       int
		stdout     string
		stderrHas  string
		stderrNone bool
	}{
		{args: []string{"run", filepath.Join(root, "examples", "hello.tr")}, exit: 0, stdout: "Hello, Tread!\n", stderrNone: true},
		{args: []string{"check", filepath.Join(root, "examples", "hello.tr")}, exit: 0, stdout: "", stderrNone: true},
		{args: []string{"run", filepath.Join(root, "testdata", "e2e", "D3.tr")}, exit: 1, stdout: "", stderrHas: "error[TR-T001]: expected Int but found String"},
		{args: []string{"run", filepath.Join(root, "testdata", "e2e", "D4.tr")}, exit: 1, stdout: "before\n", stderrHas: "error[TR-R001]"},
		{args: []string{"run", filepath.Join(root, "testdata", "e2e", "D6.tr")}, exit: 0, stdout: "B\n", stderrNone: true},
		{args: []string{"run", "missing.tr"}, exit: 2, stdout: "", stderrHas: "cannot read missing.tr"},
		{args: []string{"run", "hello.nova"}, exit: 2, stdout: "", stderrHas: "is not a Tread program: expected the .tr or .tbc extension"},
		{args: []string{"run"}, exit: 2, stdout: "", stderrHas: "run requires exactly one file argument"},
		{args: []string{"frobnicate"}, exit: 2, stdout: "", stderrHas: `unknown command "frobnicate"`},
		{args: []string{"version"}, exit: 0, stdout: "tread 0.4.0\n", stderrNone: true},
	}
	for _, tt := range tests {
		cmd := exec.Command(bin, tt.args...)
		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		err := cmd.Run()
		code := 0
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			code = ee.ExitCode()
		} else if err != nil {
			t.Fatalf("%v: %v", tt.args, err)
		}
		if code != tt.exit || stdout.String() != tt.stdout {
			t.Errorf("%v: exit %d stdout %q; want exit %d stdout %q\nstderr: %s", tt.args, code, stdout.String(), tt.exit, tt.stdout, stderr.String())
		}
		if tt.stderrNone && stderr.Len() > 0 {
			t.Errorf("%v: unexpected stderr %q", tt.args, stderr.String())
		}
		if tt.stderrHas != "" && !bytes.Contains(stderr.Bytes(), []byte(tt.stderrHas)) {
			t.Errorf("%v: stderr %q does not contain %q", tt.args, stderr.String(), tt.stderrHas)
		}
	}
}

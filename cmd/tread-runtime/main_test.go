package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestNoCompilerDependency enforces ADR-0011 §1: the runtime edition
// must not link the compiler frontend.
func TestNoCompilerDependency(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps", ".").Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}
	const internal = "github.com/sarvessveeriyah2312/Tread/internal/"
	forbidden := []string{"lexer", "parser", "ast", "token", "checker", "compiler", "pipeline"}
	deps := strings.Fields(string(out))
	for _, d := range deps {
		for _, f := range forbidden {
			if d == internal+f {
				t.Errorf("the runtime edition depends on %s", d)
			}
		}
	}
	for _, need := range []string{"vm", "bytecode", "cli"} {
		found := false
		for _, d := range deps {
			found = found || d == internal+need
		}
		if !found {
			t.Errorf("the runtime edition should depend on %s%s", internal, need)
		}
	}
}

// TestRuntimeBinary builds the SDK and the runtime, compiles a program
// with the SDK, and runs it with the runtime binary as a subprocess.
func TestRuntimeBinary(t *testing.T) {
	if testing.Short() {
		t.Skip("builds binaries")
	}
	dir := t.TempDir()
	exe := ""
	if runtime.GOOS == "windows" {
		exe = ".exe"
	}
	sdk, rt := filepath.Join(dir, "sdk", "tread"+exe), filepath.Join(dir, "rt", "tread"+exe)
	for bin, pkg := range map[string]string{sdk: "../tread", rt: "."} {
		if out, err := exec.Command("go", "build", "-o", bin, pkg).CombinedOutput(); err != nil {
			t.Fatalf("go build %s: %v\n%s", pkg, err, out)
		}
	}
	src := filepath.Join(dir, "hello.tr")
	hello, err := os.ReadFile(filepath.Join("..", "..", "examples", "hello.tr"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(src, hello, 0o644); err != nil {
		t.Fatal(err)
	}
	run := func(bin string, args ...string) (string, string, int) {
		cmd := exec.Command(bin, args...)
		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		err := cmd.Run()
		code := 0
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			code = ee.ExitCode()
		} else if err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		return stdout.String(), stderr.String(), code
	}
	if _, stderr, code := run(sdk, "build", src); code != 0 {
		t.Fatalf("sdk build: exit %d %s", code, stderr)
	}
	tbc := filepath.Join(dir, "hello.tbc")
	if stdout, stderr, code := run(rt, "run", tbc); code != 0 || stdout != "Hello, Tread!\n" || stderr != "" {
		t.Errorf("runtime run: exit %d stdout %q stderr %q", code, stdout, stderr)
	}
	if stdout, _, code := run(rt, "--version"); code != 0 || !strings.HasSuffix(stdout, " (runtime)\n") {
		t.Errorf("runtime --version: exit %d stdout %q", code, stdout)
	}
	if _, stderr, code := run(rt, "run", src); code != 2 || !strings.Contains(stderr, "needs the Tread SDK") {
		t.Errorf("runtime run .tr: exit %d stderr %q", code, stderr)
	}
	// The runtime binary is meaningfully smaller than the SDK.
	si, _ := os.Stat(sdk)
	ri, _ := os.Stat(rt)
	if ri.Size() >= si.Size() {
		t.Errorf("runtime binary (%d bytes) is not smaller than the SDK (%d bytes)", ri.Size(), si.Size())
	}
}

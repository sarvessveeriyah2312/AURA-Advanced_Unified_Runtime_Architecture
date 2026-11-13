package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFormatCommand(t *testing.T) {
	dir := t.TempDir()
	messy := filepath.Join(dir, "messy.tr")
	clean := filepath.Join(dir, "clean.tr")
	broken := filepath.Join(dir, "broken.tr")
	write := func(p, s string) {
		if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(messy, "fn main()->Unit{print(1+2)}\n")
	write(clean, "fn main() -> Unit { print(1) }\n")
	write(broken, "fn (\n")

	out, errOut, code := invoke("fmt", "--check", messy, clean)
	if code != ExitFailure || out != messy+"\n" || errOut != "" {
		t.Errorf("--check: exit %d stdout %q stderr %q", code, out, errOut)
	}
	if data, _ := os.ReadFile(messy); string(data) != "fn main()->Unit{print(1+2)}\n" {
		t.Error("--check changed a file")
	}
	out, errOut, code = invoke("fmt", messy, clean)
	if code != ExitOK || out != "" || errOut != "" {
		t.Errorf("fmt: exit %d stdout %q stderr %q", code, out, errOut)
	}
	if data, _ := os.ReadFile(messy); string(data) != "fn main() -> Unit { print(1 + 2) }\n" {
		t.Errorf("formatted = %q", data)
	}
	if _, _, code = invoke("fmt", "--check", messy, clean); code != ExitOK {
		t.Errorf("formatted files still listed: exit %d", code)
	}
	_, errOut, code = invoke("fmt", broken)
	if code != ExitFailure || !strings.Contains(errOut, "error[TR-P002]") {
		t.Errorf("syntax error: exit %d stderr %q", code, errOut)
	}
	if data, _ := os.ReadFile(broken); string(data) != "fn (\n" {
		t.Error("a file with syntax errors was changed")
	}
	inner := filepath.Join(dir, "inner.tr")
	write(inner, "fn main() -> Unit {\n    print(1 /* x */ + 2)\n}\n")
	_, errOut, code = invoke("fmt", inner)
	if code != ExitFailure || !strings.Contains(errOut, "comment is inside an expression") {
		t.Errorf("inner comment: exit %d stderr %q", code, errOut)
	}
	for _, args := range [][]string{{"fmt"}, {"fmt", "--x", messy}, {"test"}, {"test", "-v", messy}, {"lsp", "x"}} {
		if _, _, code := invoke(args...); code != ExitUsage {
			t.Errorf("%v: exit %d, want usage", args, code)
		}
	}
	for _, cmd := range []string{"fmt", "test", "lsp"} {
		if _, errOut, code := runtimeEdition(cmd, messy); code != ExitUsage || !strings.Contains(errOut, "needs the Tread SDK") {
			t.Errorf("runtime %s: exit %d stderr %q", cmd, code, errOut)
		}
	}
}

func TestTestCommandSeveralFiles(t *testing.T) {
	dir := t.TempDir()
	a, b := filepath.Join(dir, "a.tr"), filepath.Join(dir, "b.tr")
	_ = os.WriteFile(a, []byte("test \"one\" { assert(true) }\n"), 0o644)
	_ = os.WriteFile(b, []byte("fn f() -> Int {\n    return 1\n}\ntest \"two\" { assertEqual(f(), 1) }\n"), 0o644)
	out, errOut, code := invoke("test", a, b)
	want := a + "\nok    one\n" + b + "\nok    two\n2 tests: 2 passed, 0 failed\n"
	if code != ExitOK || out != want || errOut != "" {
		t.Errorf("exit %d\nstdout %q\nwant   %q\nstderr %q", code, out, want, errOut)
	}
}

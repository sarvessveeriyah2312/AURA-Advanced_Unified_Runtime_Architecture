package playground

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sarvessveeriyah2312/Tread/internal/cli"
	"github.com/sarvessveeriyah2312/Tread/internal/pipeline"
)

// The playground must print exactly what `tread run` and `tread check`
// print for the same program (with the file named main.tr).
func TestMatchesCLI(t *testing.T) {
	paths, _ := filepath.Glob("../../testdata/e2e/*.tr")
	if len(paths) < 10 {
		t.Fatalf("found only %d fixtures", len(paths))
	}
	dir := t.TempDir()
	for _, p := range paths {
		src, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(src, []byte("forever")) || strings.Contains(p, "trace_mutual") {
			continue // deep recursion: identical, but slow under the instruction limit
		}
		main := filepath.Join(dir, FileName)
		if err := os.WriteFile(main, src, 0o644); err != nil {
			t.Fatal(err)
		}
		for _, cmd := range []string{"run", "check"} {
			var out, errOut bytes.Buffer
			wd, _ := os.Getwd()
			_ = os.Chdir(dir)
			exit := cli.Run(pipeline.Compiler{}, []string{cmd, FileName}, &out, &errOut)
			_ = os.Chdir(wd)
			r := Run(string(src), cmd == "check")
			if r.Stdout != out.String() || r.Stderr != errOut.String() || r.Exit != exit {
				t.Errorf("%s %s: playground differs from the CLI\n got  exit %d %q %q\n want exit %d %q %q",
					cmd, filepath.Base(p), r.Exit, r.Stdout, r.Stderr, exit, out.String(), errOut.String())
			}
		}
	}
}

func TestStages(t *testing.T) {
	r := Run("fn main() -> Unit {\n    for i in 0..2 {\n        print(\"(${i})\")\n    }\n}\n", false)
	if r.Exit != 0 || r.Stdout != "(0)\n(1)\n" || r.Stderr != "" {
		t.Fatalf("run: %+v", r)
	}
	for _, want := range []string{"   1:1    fn               fn", "   2:15   ..               ..", `value "("`} {
		if !strings.Contains(r.Tokens, want) {
			t.Errorf("tokens missing %q:\n%s", want, r.Tokens)
		}
	}
	if !strings.HasPrefix(r.AST, "(program\n  (fn main () -> Unit\n    (block\n") || !strings.Contains(r.AST, "(for i 0 .. 2") {
		t.Errorf("ast:\n%s", r.AST)
	}
	if !strings.Contains(r.Bytecode, "fn 0 main") || !strings.Contains(r.Bytecode, "LT_INT") {
		t.Errorf("bytecode:\n%s", r.Bytecode)
	}

	// A type error: tokens and AST, no bytecode.
	r = Run("fn main() -> Unit {\n    let x: Int = \"s\"\n}\n", false)
	if r.Exit != 1 || !strings.Contains(r.Stderr, "error[TR-T001]") || r.Tokens == "" || r.AST == "" || r.Bytecode != "" {
		t.Errorf("type error: %+v", r)
	}
	// A syntax error: tokens only.
	r = Run("fn main() -> Unit {\n    let = 1\n}\n", true)
	if r.Exit != 1 || r.Tokens == "" || r.AST != "" || r.Bytecode != "" {
		t.Errorf("syntax error: %+v", r)
	}
}

func TestLimits(t *testing.T) {
	r := Run("fn main() -> Unit {\n    while true {\n    }\n}\n", false)
	if r.Exit != 1 || !strings.Contains(r.Stderr, "error[TR-R004]: instruction limit of 50000000 exceeded") {
		t.Errorf("infinite loop: exit %d %q", r.Exit, r.Stderr)
	}
	r = Run("fn main() -> Unit {\n    while true {\n        print(\"spam spam spam\")\n    }\n}\n", false)
	if r.Exit != cli.ExitInternal || len(r.Stdout) > MaxOutput || !strings.Contains(r.Stderr, "output limit") {
		t.Errorf("endless output: exit %d, %d bytes, %q", r.Exit, len(r.Stdout), r.Stderr)
	}
	r = Run(strings.Repeat(" ", MaxSource+1), false)
	if r.Exit != cli.ExitUsage {
		t.Errorf("huge source: exit %d", r.Exit)
	}
}

func TestPrettyAST(t *testing.T) {
	short := "(fn main () (block))"
	if got := PrettyAST(short); got != short+"\n" {
		t.Errorf("short: %q", got)
	}
	long := `(program (fn main () -> Unit (block (call print "a (not a list) b") (call print "x"))))`
	want := "(program\n  (fn main () -> Unit\n    (block (call print \"a (not a list) b\") (call print \"x\"))))\n"
	if got := PrettyAST(long); got != want {
		t.Errorf("long:\n%s\nwant:\n%s", got, want)
	}
}

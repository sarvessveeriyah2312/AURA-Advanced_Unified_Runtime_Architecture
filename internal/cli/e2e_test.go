package cli

import (
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sarvessveeriyah2312/AURA-Advanced_Unified_Runtime_Architecture/internal/bytecode"
	"github.com/sarvessveeriyah2312/AURA-Advanced_Unified_Runtime_Architecture/internal/diagnostic"
	"github.com/sarvessveeriyah2312/AURA-Advanced_Unified_Runtime_Architecture/internal/source"
)

// update rewrites golden files. Golden changes must be reviewed before
// they are committed (docs/08 §8.2).
var update = flag.Bool("update", false, "rewrite testdata/e2e golden files")

const fixtures = "../../testdata/e2e"

type e2eCase struct {
	name    string // fixture base name
	cmd     string // "run" or "check"
	exit    int
	stdout  *string  // exact stdout when non-nil
	stderrs []string // substrings that must appear in stderr, in order
}

func str(s string) *string { return &s }

// e2eCases implements docs/10 §10.8 D1-D10 (D8 is TestD8ExitCodeMapping)
// plus regression cases. Every case is also compared against golden
// files <name>.<cmd>.stdout and <name>.<cmd>.stderr.
var e2eCases = []e2eCase{
	{name: "D1", cmd: "run", exit: 1, stdout: str(""), stderrs: []string{"error[AURA-L001]", "D1.aura:3:13"}},
	{name: "D2", cmd: "run", exit: 1, stdout: str(""), stderrs: []string{"error[AURA-P002]", "D2.aura:3:9"}},
	{name: "D3", cmd: "run", exit: 1, stdout: str(""), stderrs: []string{"error[AURA-T001]: expected Int but found String\n" +
		"  --> D3.aura:4:18\n" +
		"   |\n" +
		" 4 | let total: Int = \"five\"\n" +
		"   |                  ^^^^^^\n"}},
	{name: "D4", cmd: "run", exit: 1, stdout: str("before\n"), stderrs: []string{"error[AURA-R001]", "D4.aura:5:11", "1 / z"}},
	{name: "D5", cmd: "run", exit: 1, stdout: str(""), stderrs: []string{"error[AURA-T011]", "D5.aura:5:11"}},
	{name: "D6", cmd: "run", exit: 0, stdout: str("B\n"), stderrs: nil},
	{name: "D7", cmd: "run", exit: 0, stdout: str("-9223372036854775808\n")},
	{name: "D9", cmd: "run", exit: 1, stdout: str(""), stderrs: []string{"error[AURA-R005]", "D9.aura:3:11"}},
	{name: "D10", cmd: "run", exit: 0, stdout: str("1.0\n0.1\n-0.0\n")},

	// aura check: same frontend diagnostics, never executes.
	{name: "D1", cmd: "check", exit: 1, stdout: str(""), stderrs: []string{"error[AURA-L001]"}},
	{name: "D2", cmd: "check", exit: 1, stdout: str(""), stderrs: []string{"error[AURA-P002]"}},
	{name: "D3", cmd: "check", exit: 1, stdout: str(""), stderrs: []string{"error[AURA-T001]"}},
	{name: "D5", cmd: "check", exit: 1, stdout: str(""), stderrs: []string{"error[AURA-T011]"}},
	{name: "D4", cmd: "check", exit: 0, stdout: str("")}, // runtime errors are not detected by check
	{name: "D6", cmd: "check", exit: 0, stdout: str("")}, // does not execute: no "B"
	{name: "D9", cmd: "check", exit: 0, stdout: str("")},

	// Regression and coverage cases.
	{name: "multiple", cmd: "run", exit: 1, stdout: str(""), stderrs: []string{"error[AURA-T001]", "\n\nerror[AURA-T002]", "\n\nerror[AURA-T007]"}},
	{name: "multiple", cmd: "check", exit: 1, stdout: str(""), stderrs: []string{"error[AURA-T001]", "error[AURA-T002]", "error[AURA-T007]"}},
	{name: "syntax_multiple", cmd: "check", exit: 1, stdout: str(""), stderrs: []string{"error[AURA-L001]", "error[AURA-P002]"}},
	{name: "no_main", cmd: "check", exit: 1, stdout: str(""), stderrs: []string{"error[AURA-T013]", "no_main.aura:1:1"}},
	{name: "calls", cmd: "run", exit: 0, stdout: str("fact(5) = 120\nfact(20) = 2432902008176640000\ntrue\n")},
	{name: "runtime_in_callee", cmd: "run", exit: 1, stdout: str("5\n"), stderrs: []string{"error[AURA-R001]: division by zero (in function 'div')", "runtime_in_callee.aura:3:12", "a / b"}},
	{name: "recursion", cmd: "run", exit: 1, stdout: str(""), stderrs: []string{"error[AURA-R003]", "forever(n + 1)"}},
}

func TestEndToEnd(t *testing.T) {
	for _, tc := range e2eCases {
		t.Run(tc.name+"/"+tc.cmd, func(t *testing.T) {
			path := filepath.Join(fixtures, tc.name+".aura")
			stdout, stderr, code := invoke(tc.cmd, path)
			stderr = strings.ReplaceAll(stderr, path, tc.name+".aura")

			if code != tc.exit {
				t.Errorf("exit code = %d, want %d\nstderr:\n%s", code, tc.exit, stderr)
			}
			if tc.stdout != nil && stdout != *tc.stdout {
				t.Errorf("stdout = %q, want %q", stdout, *tc.stdout)
			}
			if tc.exit == 0 && stderr != "" {
				t.Errorf("successful run wrote to stderr:\n%s", stderr)
			}
			rest := stderr
			for _, want := range tc.stderrs {
				i := strings.Index(rest, want)
				if i < 0 {
					t.Errorf("stderr missing %q (in order)\nstderr:\n%s", want, stderr)
					break
				}
				rest = rest[i+len(want):]
			}
			golden(t, tc.name+"."+tc.cmd+".stdout", stdout)
			golden(t, tc.name+"."+tc.cmd+".stderr", stderr)
		})
	}
}

func invoke(args ...string) (stdout, stderr string, code int) {
	var out, errOut strings.Builder
	code = Run(args, &out, &errOut)
	return out.String(), errOut.String(), code
}

func golden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join(fixtures, name)
	if *update {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("missing golden file %s (run: go test ./internal/cli -run TestEndToEnd -update, then review)", path)
	}
	if got != string(want) {
		t.Errorf("%s differs from golden:\n--- got\n%s\n--- want\n%s", name, got, want)
	}
}

// TestHelloExample is the §10.8 milestone: aura run examples/hello.aura.
func TestHelloExample(t *testing.T) {
	stdout, stderr, code := invoke("run", "../../examples/hello.aura")
	if code != ExitOK || stdout != "Hello, AURA!\n" || stderr != "" {
		t.Errorf("run: exit %d stdout %q stderr %q", code, stdout, stderr)
	}
	stdout, stderr, code = invoke("check", "../../examples/hello.aura")
	if code != ExitOK || stdout != "" || stderr != "" {
		t.Errorf("check: exit %d stdout %q stderr %q", code, stdout, stderr)
	}
}

// TestD8ExitCodeMapping checks every registry code against the Exit
// column of docs/04 §4.4.3.
func TestD8ExitCodeMapping(t *testing.T) {
	for _, c := range diagnostic.Registry() {
		want := ExitFailure
		if c == diagnostic.InternalCompilerError || c == diagnostic.InvalidBytecode {
			want = ExitInternal
		}
		if got := exitCodeFor(c); got != want {
			t.Errorf("%s exits %d, want %d", c, got, want)
		}
	}
	mixed := []diagnostic.Diagnostic{{Code: diagnostic.TypeMismatch}, {Code: diagnostic.InternalCompilerError}}
	if exitCodeForAll(mixed) != ExitInternal || exitCodeForAll(nil) != ExitOK {
		t.Error("exitCodeForAll must return the most severe code")
	}
}

// TestInvalidBytecodeIsInternal: bytecode validation failures cannot come
// from valid source; they are rendered without a source excerpt and map
// to exit code 3.
func TestInvalidBytecodeIsInternal(t *testing.T) {
	bad := &bytecode.Module{Main: 5}
	failure, err := execute(bad, &strings.Builder{})
	if err != nil || failure == nil || failure.withSource || failure.diag.Code != diagnostic.InvalidBytecode {
		t.Fatalf("got %+v, %v", failure, err)
	}
	var b strings.Builder
	renderAll(&b, nil, []diagnostic.Diagnostic{failure.diag})
	if !strings.HasPrefix(b.String(), "error[AURA-R002]: invalid module: main index 5 out of range") || strings.Contains(b.String(), "-->") {
		t.Errorf("rendering = %q", b.String())
	}
	if exitCodeFor(failure.diag.Code) != ExitInternal {
		t.Error("R002 must exit 3")
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("disk full") }

func TestOutputWriteFailureIsInternal(t *testing.T) {
	var errOut strings.Builder
	code := Run([]string{"run", "../../examples/hello.aura"}, failingWriter{}, &errOut)
	if code != ExitInternal || errOut.String() != "error: cannot write program output: disk full\n" {
		t.Errorf("exit %d stderr %q", code, errOut.String())
	}
}

func TestDirectoryArgument(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "looks-like-source.aura")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	_, stderr, code := invoke("run", dir)
	// The reason text is OS-specific ("is a directory" on Unix).
	if code != ExitUsage || !strings.HasPrefix(stderr, "error: cannot read "+dir+": ") {
		t.Errorf("exit %d stderr %q", code, stderr)
	}
}

func TestDeterministicDiagnostics(t *testing.T) {
	path := filepath.Join(fixtures, "multiple.aura")
	_, a, _ := invoke("check", path)
	_, b, _ := invoke("check", path)
	if a != b || a == "" {
		t.Error("diagnostics must be identical across runs")
	}
}

func TestRenderAllSeparatesWithBlankLine(t *testing.T) {
	f := source.NewFile(1, "t.aura", []byte("ab"))
	var b strings.Builder
	renderAll(&b, f, []diagnostic.Diagnostic{
		diagnostic.Errorf(diagnostic.InvalidCharacter, f.Span(0, 1), "one"),
		diagnostic.Errorf(diagnostic.InvalidCharacter, f.Span(1, 2), "two"),
	})
	if !strings.Contains(b.String(), "^\n\nerror[AURA-L001]: two") {
		t.Errorf("rendering:\n%s", b.String())
	}
}

// TestDeepNestingDoesNotCrash is the regression test for the Phase 5
// stack-overflow defect (ADR-0006): before the nesting limit, each of
// these inputs crashed the process with a Go "stack overflow" trace and
// exit status 2. Each must now give one AURA-P014 diagnostic and exit 1.
func TestDeepNestingDoesNotCrash(t *testing.T) {
	const n = 1_000_000
	wrap := func(expr string) string { return "fn main() -> Unit {\n    print(" + expr + ")\n}\n" }
	programs := map[string]string{
		"parentheses": wrap(strings.Repeat("(", n) + "1" + strings.Repeat(")", n)),
		"unary":       wrap(strings.Repeat("-", n) + "1"),
		"chain":       wrap(strings.Repeat("1+", n) + "1"),
		"blocks":      "fn main() -> Unit {\n" + strings.Repeat("{", n) + strings.Repeat("}", n) + "\n}\n",
		"else-if":     "fn main() -> Unit {\n    let x = 1\n    if x == 0 { }" + strings.Repeat(" else if x == 0 { }", n) + "\n}\n",
	}
	for name, src := range programs {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "deep.aura")
			if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
				t.Fatal(err)
			}
			for _, cmd := range []string{"check", "run"} {
				stdout, stderr, code := invoke(cmd, path)
				if code != ExitFailure || stdout != "" ||
					strings.Count(stderr, "error[") != 1 || !strings.Contains(stderr, "error[AURA-P014]: nesting too deep") ||
					strings.Contains(stderr, "goroutine") {
					t.Errorf("%s: exit %d, stdout %d bytes, stderr %.300q", cmd, code, len(stdout), stderr)
				}
			}
		})
	}
}

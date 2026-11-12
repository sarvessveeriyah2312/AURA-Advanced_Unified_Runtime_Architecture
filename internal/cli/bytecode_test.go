package cli

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sarvessveeriyah2312/Tread/internal/bytecode"
	"github.com/sarvessveeriyah2312/Tread/internal/pipeline"
)

// runtimeEdition invokes the CLI as the runtime edition (no compiler).
func runtimeEdition(args ...string) (stdout, stderr string, code int) {
	var out, errOut strings.Builder
	code = Run(nil, args, &out, &errOut)
	return out.String(), errOut.String(), code
}

// copyFixture copies a fixture into a temporary directory, so builds
// never write into testdata.
func copyFixture(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(t.TempDir(), filepath.Base(path))
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return dst
}

// TestBytecodeRoundTrip is the ADR-0011 phase 2 acceptance test: for every
// fixture and example, `tread build` then `tread run <file.tbc>` (in both
// editions) produces exactly the stdout, stderr and exit code of
// `tread run <file.tr>`, including runtime error excerpts and stack
// traces. A program with compile errors makes build fail like run and
// writes no file.
func TestBytecodeRoundTrip(t *testing.T) {
	paths, _ := filepath.Glob(filepath.Join(fixtures, "*.tr"))
	paths = append(paths, "../../examples/hello.tr")
	if len(paths) < 10 {
		t.Fatalf("found only %d fixtures", len(paths))
	}
	for _, p := range paths {
		t.Run(filepath.Base(p), func(t *testing.T) {
			src := copyFixture(t, p)
			wantOut, wantErr, wantCode := invoke("run", src)

			_, buildErr, buildCode := invoke("build", src)
			tbc := strings.TrimSuffix(src, ".tr") + ".tbc"
			if _, err := os.Stat(tbc); err != nil {
				// Compile error: build reports exactly what run reports.
				if buildCode != wantCode || buildErr != wantErr || buildCode == ExitOK {
					t.Fatalf("build without output: exit %d stderr %q; run: exit %d stderr %q", buildCode, buildErr, wantCode, wantErr)
				}
				return
			}
			if buildCode != ExitOK || buildErr != "" {
				t.Fatalf("build: exit %d stderr %q", buildCode, buildErr)
			}
			for name, runTBC := range map[string]func(...string) (string, string, int){"sdk": invoke, "runtime": runtimeEdition} {
				out, errOut, code := runTBC("run", tbc)
				if out != wantOut || errOut != wantErr || code != wantCode {
					t.Errorf("%s edition, run %s:\nexit %d (want %d)\nstdout %q\n  want %q\nstderr %q\n  want %q",
						name, filepath.Base(tbc), code, wantCode, out, wantOut, errOut, wantErr)
				}
			}
		})
	}
}

func TestBuild(t *testing.T) {
	src := copyFixture(t, "../../examples/hello.tr")
	dir := filepath.Dir(src)

	t.Run("custom output", func(t *testing.T) {
		out := filepath.Join(dir, "custom.tbc")
		if _, stderr, code := invoke("build", "-o", out, src); code != ExitOK || stderr != "" {
			t.Fatalf("exit %d stderr %q", code, stderr)
		}
		stdout, _, code := runtimeEdition("run", out)
		if code != ExitOK || stdout != "Hello, Tread!\n" {
			t.Errorf("exit %d stdout %q", code, stdout)
		}
		data, _ := os.ReadFile(out)
		f, err := bytecode.Decode(data)
		if err != nil || f.Producer != "tread "+Version || f.SourceName != src {
			t.Errorf("decoded %+v, %v", f, err)
		}
	})
	t.Run("flag after file", func(t *testing.T) {
		out := filepath.Join(dir, "after.tbc")
		if _, stderr, code := invoke("build", src, "-o", out); code != ExitOK {
			t.Fatalf("exit %d stderr %q", code, stderr)
		}
	})
	t.Run("deterministic", func(t *testing.T) {
		a, b := filepath.Join(dir, "a.tbc"), filepath.Join(dir, "b.tbc")
		invoke("build", "-o", a, src)
		invoke("build", "-o", b, src)
		da, _ := os.ReadFile(a)
		db, _ := os.ReadFile(b)
		if len(da) == 0 || string(da) != string(db) {
			t.Error("two builds of the same source differ")
		}
	})

	tests := []struct {
		name   string
		args   []string
		code   int
		stderr string
	}{
		{"no file", []string{"build"}, ExitUsage, "error: build requires exactly one file argument\n\n" + sdkUsage},
		{"two files", []string{"build", "a.tr", "b.tr"}, ExitUsage, "error: build requires exactly one file argument\n\n" + sdkUsage},
		{"-o without value", []string{"build", src, "-o"}, ExitUsage, "error: -o requires exactly one output file\n\n" + sdkUsage},
		{"-o twice", []string{"build", "-o", "a.tbc", "-o", "b.tbc", src}, ExitUsage, "error: -o requires exactly one output file\n\n" + sdkUsage},
		{"unknown flag", []string{"build", "--fast", src}, ExitUsage, "error: unknown flag \"--fast\" for build\n\n" + sdkUsage},
		{"output must be .tbc", []string{"build", "-o", "x.tr", src}, ExitUsage, "error: output x.tr must have the .tbc extension\n"},
		{"source must be .tr", []string{"build", "x.txt"}, ExitUsage, "error: x.txt is not a Tread source file: expected the .tr extension\n"},
		{"unwritable output", []string{"build", "-o", filepath.Join(dir, "no", "such", "dir.tbc"), src}, ExitFailure, "error: cannot write " + filepath.Join(dir, "no", "such", "dir.tbc") + ": "},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			stdout, stderr, code := invoke(tc.args...)
			if code != tc.code || stdout != "" || !strings.HasPrefix(stderr, tc.stderr) {
				t.Errorf("exit %d stdout %q stderr %q; want exit %d stderr prefix %q", code, stdout, stderr, tc.code, tc.stderr)
			}
		})
	}
}

func TestRunBytecodeRejects(t *testing.T) {
	src := copyFixture(t, "../../examples/hello.tr")
	tbc := strings.TrimSuffix(src, ".tr") + ".tbc"
	if _, stderr, code := invoke("build", src); code != ExitOK {
		t.Fatal(stderr)
	}
	good, _ := os.ReadFile(tbc)
	write := func(name string, data []byte) string {
		p := filepath.Join(filepath.Dir(src), name)
		if err := os.WriteFile(p, data, 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	future := append([]byte(nil), good...)
	binary.LittleEndian.PutUint16(future[4:], bytecode.FormatVersion+1)
	corrupt := append([]byte(nil), good...)
	corrupt[len(corrupt)/2] ^= 0xFF

	tests := []struct {
		name, path, want string
	}{
		{"source renamed", write("source.tbc", []byte("fn main() -> Unit {}\n")), "error[TR-R006]: cannot load %s: not a Tread bytecode file"},
		{"future version", write("future.tbc", future), "error[TR-R006]: cannot load %s: unsupported bytecode format version 2: this runtime supports version 1; rebuild the program with `tread build` from the same Tread release\n"},
		{"corrupted", write("corrupt.tbc", corrupt), "error[TR-R006]: cannot load %s: bytecode file is corrupted or truncated (checksum mismatch)\n"},
		{"truncated", write("short.tbc", good[:len(good)-10]), "error[TR-R006]: cannot load %s: bytecode file is corrupted or truncated"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			for _, run := range []func(...string) (string, string, int){invoke, runtimeEdition} {
				stdout, stderr, code := run("run", tc.path)
				want := strings.Replace(tc.want, "%s", tc.path, 1)
				if code != ExitFailure || stdout != "" || !strings.HasPrefix(stderr, want) {
					t.Errorf("exit %d stdout %q stderr %q; want prefix %q", code, stdout, stderr, want)
				}
			}
		})
	}
	if _, stderr, code := runtimeEdition("run", "missing.tbc"); code != ExitUsage || !strings.HasPrefix(stderr, "error: cannot read missing.tbc: ") {
		t.Errorf("missing file: exit %d stderr %q", code, stderr)
	}
}

func TestRuntimeEdition(t *testing.T) {
	needSDK := func(cmd string) string {
		return "error: " + cmd + " needs the Tread SDK; this is the Tread Runtime, which only runs compiled .tbc files\n"
	}
	tests := []struct {
		args   []string
		code   int
		stdout string
		stderr string
	}{
		{[]string{"--version"}, ExitOK, "tread " + Version + " (runtime)\n", ""},
		{[]string{"version"}, ExitOK, "tread " + Version + " (runtime)\n", ""},
		{[]string{"help"}, ExitOK, runtimeUsage, ""},
		{nil, ExitUsage, "", runtimeUsage},
		{[]string{"runtime", "--version"}, ExitOK, "Tread Runtime " + Version + " (bytecode format 1)\n", ""},
		{[]string{"sdk", "--version"}, ExitFailure, "", "error: the Tread SDK is not installed: this is the Tread Runtime " + Version + "\n"},
		{[]string{"check", "a.tr"}, ExitUsage, "", needSDK("check")},
		{[]string{"build", "a.tr"}, ExitUsage, "", needSDK("build")},
		{[]string{"new", "App"}, ExitUsage, "", needSDK("new")},
		{[]string{"run", "a.tr"}, ExitUsage, "", "error: a.tr is Tread source; running source needs the Tread SDK. This is the Tread Runtime, which only runs compiled .tbc files\n"},
		{[]string{"run", "a.txt"}, ExitUsage, "", "error: a.txt is not a Tread bytecode file: expected the .tbc extension\n"},
		{[]string{"bogus"}, ExitUsage, "", "error: unknown command \"bogus\"\n\n" + runtimeUsage},
	}
	for _, tc := range tests {
		stdout, stderr, code := runtimeEdition(tc.args...)
		if code != tc.code || stdout != tc.stdout || stderr != tc.stderr {
			t.Errorf("%v: exit %d stdout %q stderr %q\nwant exit %d stdout %q stderr %q", tc.args, code, stdout, stderr, tc.code, tc.stdout, tc.stderr)
		}
	}
	if strings.Contains(runtimeUsage, "  check <") || strings.Contains(runtimeUsage, "  build <") || strings.Contains(runtimeUsage, "  new <") {
		t.Error("runtime usage must not list SDK commands")
	}
}

func TestEditionVersions(t *testing.T) {
	tests := []struct {
		args   []string
		code   int
		stdout string
		stderr string
	}{
		{[]string{"sdk", "--version"}, ExitOK, "Tread SDK " + Version + "\n", ""},
		{[]string{"runtime", "--version"}, ExitOK, "Tread Runtime " + Version + " (bytecode format 1)\n", ""},
		{[]string{"sdk"}, ExitUsage, "", "error: usage: tread sdk --version\n\n" + sdkUsage},
		{[]string{"runtime", "-v"}, ExitUsage, "", "error: usage: tread runtime --version\n\n" + sdkUsage},
		{[]string{"doctor", "x"}, ExitUsage, "", "error: doctor takes no arguments\n\n" + sdkUsage},
	}
	for _, tc := range tests {
		stdout, stderr, code := invoke(tc.args...)
		if code != tc.code || stdout != tc.stdout || stderr != tc.stderr {
			t.Errorf("%v: exit %d stdout %q stderr %q", tc.args, code, stdout, stderr)
		}
	}
}

func TestNewProject(t *testing.T) {
	t.Chdir(t.TempDir())
	stdout, stderr, code := invoke("new", "HelloWorld")
	sep := string(filepath.Separator)
	if code != ExitOK || stderr != "" || stdout != "Created project HelloWorld in HelloWorld"+sep+"\n\nNext steps:\n  cd HelloWorld\n  tread run src/main.tr\n" {
		t.Fatalf("exit %d stdout %q stderr %q", code, stdout, stderr)
	}
	main := filepath.Join("HelloWorld", "src", "main.tr")
	for _, args := range [][]string{{"check", main}, {"run", main}, {"build", main}} {
		out, errOut, c := invoke(args...)
		if c != ExitOK || errOut != "" {
			t.Errorf("%v: exit %d stderr %q", args, c, errOut)
		}
		if args[0] == "run" && out != "Hello from HelloWorld!\n" {
			t.Errorf("run printed %q", out)
		}
	}
	if out, _, c := runtimeEdition("run", filepath.Join("HelloWorld", "src", "main.tbc")); c != ExitOK || out != "Hello from HelloWorld!\n" {
		t.Errorf("runtime run: exit %d stdout %q", c, out)
	}

	// Never overwrites: an existing directory is an error.
	_, stderr, code = invoke("new", "HelloWorld")
	if code != ExitFailure || stderr != "error: cannot create project: HelloWorld already exists\n" {
		t.Errorf("existing: exit %d stderr %q", code, stderr)
	}
	for _, bad := range []string{"1app", "../escape", "a b", "-x", "a/b", ""} {
		_, stderr, code := invoke("new", bad)
		if code != ExitUsage || !strings.HasPrefix(stderr, "error: invalid project name") {
			t.Errorf("new %q: exit %d stderr %q", bad, code, stderr)
		}
	}
	if _, stderr, code := invoke("new"); code != ExitUsage || !strings.HasPrefix(stderr, "error: new requires exactly one project name") {
		t.Errorf("no name: exit %d stderr %q", code, stderr)
	}
}

// TestDoctorDevelopmentBuild runs the real doctor in the test binary,
// which is not an installation: it must warn about that, run every
// self-test, and not fail.
func TestDoctorDevelopmentBuild(t *testing.T) {
	for name, compiler := range map[string]Compiler{"sdk": pipeline.Compiler{}, "runtime": nil} {
		var out, errOut strings.Builder
		code := Run(compiler, []string{"doctor"}, &out, &errOut)
		s := out.String()
		if code != ExitOK || errOut.Len() != 0 || !strings.HasPrefix(s, "Tread Installation Diagnostics\n\n") {
			t.Errorf("%s: exit %d stderr %q\n%s", name, code, errOut.String(), s)
		}
		for _, want := range []string{"[OK]   Runtime:", "[OK]   Bytecode compatibility:", "[OK]   Standard library:", "[WARN] Installation:"} {
			if !strings.Contains(s, want) {
				t.Errorf("%s: output lacks %q:\n%s", name, want, s)
			}
		}
		if hasSDK := strings.Contains(s, "[OK]   SDK compiler:"); hasSDK != (compiler != nil) {
			t.Errorf("%s: SDK compiler line present = %v\n%s", name, hasSDK, s)
		}
	}
}

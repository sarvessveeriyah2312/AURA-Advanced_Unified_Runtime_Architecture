// Package doctor implements `tread doctor`: installation diagnostics
// (ADR-0011 §5). Every reported result comes from a check that was
// actually performed; nothing is assumed.
//
// It depends on the bytecode format and the VM, never on the compiler:
// the SDK passes a compile function in Info.
package doctor

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/sarvessveeriyah2312/Tread/internal/bytecode"
	"github.com/sarvessveeriyah2312/Tread/internal/source"
	"github.com/sarvessveeriyah2312/Tread/internal/vm"
)

// Editions of tread (ADR-0011 §1).
const (
	SDK     = "sdk"
	Runtime = "runtime"
)

// ReleaseFile is the installation metadata file at the root of every
// distribution (ADR-0011 §4).
const ReleaseFile = "release"

// Info describes the running tread.
type Info struct {
	Version string
	Edition string // SDK or Runtime
	// Compile compiles source to a module. Nil in the runtime edition.
	Compile func(name string, src []byte) (*bytecode.Module, error)
}

// Env is the environment the checks inspect. Tests substitute it.
type Env struct {
	Executable string // absolute path of the running binary, or "" if unknown
	GOOS       string
	GOARCH     string
	Getenv     func(string) string
	LookPath   func(string) (string, error)
	ReadFile   func(string) ([]byte, error)
	// SameFile reports whether two paths name the same file.
	SameFile func(a, b string) bool
}

// SystemEnv returns the real environment.
func SystemEnv() Env {
	exe, err := os.Executable()
	if err == nil {
		if resolved, rerr := filepath.EvalSymlinks(exe); rerr == nil {
			exe = resolved
		}
	} else {
		exe = ""
	}
	return Env{
		Executable: exe,
		GOOS:       runtime.GOOS,
		GOARCH:     runtime.GOARCH,
		Getenv:     os.Getenv,
		LookPath:   exec.LookPath,
		ReadFile:   os.ReadFile,
		SameFile: func(a, b string) bool {
			fa, err1 := os.Stat(a)
			fb, err2 := os.Stat(b)
			return err1 == nil && err2 == nil && os.SameFile(fa, fb)
		},
	}
}

// Status is the result of one check.
type Status int

// Check results, in increasing severity.
const (
	OK Status = iota
	Warn
	Fail
)

func (s Status) String() string {
	switch s {
	case OK:
		return "[OK]  "
	case Warn:
		return "[WARN]"
	}
	return "[FAIL]"
}

// Check is one diagnostic line.
type Check struct {
	Status Status
	Name   string
	Detail string
}

// Report is the result of Diagnose.
type Report struct {
	Checks []Check
}

// Failed reports whether any check failed.
func (r *Report) Failed() bool { return r.count(Fail) > 0 }

func (r *Report) count(s Status) int {
	n := 0
	for _, c := range r.Checks {
		if c.Status == s {
			n++
		}
	}
	return n
}

// Write renders the report.
func (r *Report) Write(w io.Writer) error {
	var b strings.Builder
	b.WriteString("Tread Installation Diagnostics\n\n")
	for _, c := range r.Checks {
		fmt.Fprintf(&b, "%s %s: %s\n", c.Status, c.Name, c.Detail)
	}
	b.WriteString("\n")
	fails, warns := r.count(Fail), r.count(Warn)
	switch {
	case fails > 0:
		fmt.Fprintf(&b, "Installation has problems: %s failed.\n", plural(fails, "check"))
	case warns > 0:
		fmt.Fprintf(&b, "Installation is healthy, with %s.\n", plural(warns, "warning"))
	default:
		b.WriteString("Installation is healthy.\n")
	}
	_, err := io.WriteString(w, b.String())
	return err
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return fmt.Sprintf("%d %ss", n, word)
}

// Platform returns the release platform name for a Go OS and
// architecture, e.g. "macos-arm64" (ADR-0011 §6).
func Platform(goos, goarch string) string {
	os := map[string]string{"darwin": "macos"}[goos]
	if os == "" {
		os = goos
	}
	arch := map[string]string{"amd64": "x64"}[goarch]
	if arch == "" {
		arch = goarch
	}
	return os + "-" + arch
}

// ParseRelease parses a release file: KEY="value" lines; blank lines and
// lines starting with # are ignored.
func ParseRelease(data []byte) (map[string]string, error) {
	fields := map[string]string{}
	sc := bufio.NewScanner(bytes.NewReader(data))
	for n := 1; sc.Scan(); n++ {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		unq, err := strconv.Unquote(val)
		if !ok || err != nil || key == "" {
			return nil, fmt.Errorf("line %d is not KEY=\"value\"", n)
		}
		fields[key] = unq
	}
	return fields, sc.Err()
}

// Diagnose runs every check.
func Diagnose(env Env, info Info) *Report {
	r := &Report{}
	add := func(s Status, name, format string, args ...any) {
		r.Checks = append(r.Checks, Check{Status: s, Name: name, Detail: fmt.Sprintf(format, args...)})
	}
	edition := "SDK"
	if info.Edition == Runtime {
		edition = "Runtime"
	}

	// 1. The CLI itself.
	exe := env.Executable
	if exe == "" {
		add(Warn, "Tread CLI", "tread %s (%s); cannot determine the executable path", info.Version, edition)
	} else {
		add(OK, "Tread CLI", "tread %s (%s) at %s", info.Version, edition, exe)
	}

	// 2. Installation: <home>/release next to bin/.
	home := ""
	if exe != "" && filepath.Base(filepath.Dir(exe)) == "bin" {
		home = filepath.Dir(filepath.Dir(exe))
	}
	installed := false
	if home == "" {
		add(Warn, "Installation", "not running from an installation (the executable is not in a bin directory); this is normal for development builds")
	} else if data, err := env.ReadFile(filepath.Join(home, ReleaseFile)); err != nil {
		add(Warn, "Installation", "no %s file in %s; this is normal for development builds", ReleaseFile, home)
	} else if fields, err := ParseRelease(data); err != nil {
		add(Fail, "Installation", "%s is malformed: %v; reinstall Tread", filepath.Join(home, ReleaseFile), err)
	} else {
		want := map[string]string{
			"TREAD_VERSION":         info.Version,
			"TREAD_EDITION":         info.Edition,
			"TREAD_BYTECODE_FORMAT": strconv.Itoa(bytecode.FormatVersion),
			"TREAD_PLATFORM":        Platform(env.GOOS, env.GOARCH),
		}
		var bad []string
		for _, k := range []string{"TREAD_VERSION", "TREAD_EDITION", "TREAD_BYTECODE_FORMAT", "TREAD_PLATFORM"} {
			if fields[k] != want[k] {
				bad = append(bad, fmt.Sprintf("%s is %q, the executable is %q", k, fields[k], want[k]))
			}
		}
		if _, err := env.ReadFile(filepath.Join(home, "docs", "LICENSE")); err != nil {
			bad = append(bad, "docs/LICENSE is missing")
		}
		if len(bad) > 0 {
			add(Fail, "Installation", "%s does not match this executable: %s; reinstall Tread", home, strings.Join(bad, "; "))
		} else {
			installed = true
			add(OK, "Installation", "%s (Tread %s %s, %s)", home, edition, info.Version, fields["TREAD_PLATFORM"])
		}
	}

	// 3. TREAD_HOME (optional).
	switch th := env.Getenv("TREAD_HOME"); {
	case th == "":
		add(OK, "TREAD_HOME", "not set (optional: tread finds its installation from its own location)")
	case installed && env.SameFile(th, home):
		add(OK, "TREAD_HOME", "%s", th)
	case installed:
		add(Warn, "TREAD_HOME", "%s is not the installation running this command (%s); update or unset it", th, home)
	default:
		add(Warn, "TREAD_HOME", "set to %s, but this tread is not running from an installation", th)
	}

	// 4. PATH.
	if found, err := env.LookPath("tread"); err != nil {
		hint := ""
		if home != "" {
			hint = "; add " + filepath.Join(home, "bin") + " to PATH"
		}
		add(Warn, "PATH", "tread is not on PATH%s", hint)
	} else if exe != "" && env.SameFile(found, exe) {
		add(OK, "PATH", "tread resolves to this executable")
	} else {
		add(Warn, "PATH", "the first tread on PATH is %s, not this executable", found)
	}

	// 5-6. Runtime and bytecode compatibility, with a module built
	// without the compiler.
	runtimeOut, err := roundTripAndRun(selfTestModule(), nil)
	switch {
	case err != nil:
		add(Fail, "Runtime", "self-test failed: %v", err)
	case runtimeOut != selfTestOutput:
		add(Fail, "Runtime", "self-test printed %q, want %q", runtimeOut, selfTestOutput)
	default:
		add(OK, "Runtime", "encoded, verified and ran a test program (bytecode format %d)", bytecode.FormatVersion)
	}
	if err := checkCompatibility(); err != nil {
		add(Fail, "Bytecode compatibility", "%v", err)
	} else {
		add(OK, "Bytecode compatibility", "format %d loads; format %d is rejected with a rebuild hint", bytecode.FormatVersion, bytecode.FormatVersion+1)
	}

	// 7. The SDK compiler.
	stdlibOK := err == nil && runtimeOut == selfTestOutput
	if info.Compile != nil {
		var out string
		m, cerr := info.Compile("doctor.tr", []byte(selfTestSource))
		if cerr == nil {
			out, cerr = roundTripAndRun(m, []byte(selfTestSource))
		}
		switch {
		case cerr != nil:
			add(Fail, "SDK compiler", "self-test failed: %v", cerr)
			stdlibOK = false
		case out != selfTestOutput:
			add(Fail, "SDK compiler", "self-test printed %q, want %q", out, selfTestOutput)
			stdlibOK = false
		default:
			add(OK, "SDK compiler", "compiled a test program to bytecode and ran it")
		}
	}

	// 8. Standard library: the self-tests call print and toString.
	if stdlibOK {
		add(OK, "Standard library", "built-in print and toString work (no library modules in this release)")
	} else {
		add(Fail, "Standard library", "built-in print and toString did not produce the expected output")
	}
	return r
}

// selfTestSource is the SDK self-test; selfTestModule is the same
// program built by hand, so the runtime edition can test without a
// compiler.
const (
	selfTestSource = "fn main() -> Unit {\n    print(toString(40 + 2))\n}\n"
	selfTestOutput = "42\n"
)

func selfTestModule() *bytecode.Module {
	code := []bytecode.Instr{
		{Op: bytecode.OpConst, A: 0}, {Op: bytecode.OpConst, A: 1}, {Op: bytecode.OpAddInt},
		{Op: bytecode.OpToStringInt}, {Op: bytecode.OpPrintString}, {Op: bytecode.OpPop},
		{Op: bytecode.OpUnit}, {Op: bytecode.OpReturn},
	}
	return &bytecode.Module{
		Constants: []bytecode.Value{bytecode.IntValue(40), bytecode.IntValue(2)},
		Functions: []bytecode.Function{{Name: "main", Result: bytecode.KindUnit, Code: code, Spans: make([]source.Span, len(code))}},
	}
}

// roundTripAndRun encodes m, decodes it, and runs the decoded module.
func roundTripAndRun(m *bytecode.Module, src []byte) (string, error) {
	data, err := bytecode.Encode(&bytecode.File{Producer: "tread doctor", SourceName: "doctor.tr", Source: src, Module: m})
	if err != nil {
		return "", err
	}
	f, err := bytecode.Decode(data)
	if err != nil {
		return "", err
	}
	var out strings.Builder
	if err := vm.Run(f.Module, &out, vm.Config{MaxInstructions: 1000}); err != nil {
		return "", err
	}
	return out.String(), nil
}

// checkCompatibility confirms that the supported format loads and that
// a file claiming the next format version is rejected, not executed.
func checkCompatibility() error {
	data, err := bytecode.Encode(&bytecode.File{Module: selfTestModule()})
	if err != nil {
		return err
	}
	if _, err := bytecode.Decode(data); err != nil {
		return fmt.Errorf("format %d does not load: %v", bytecode.FormatVersion, err)
	}
	binary.LittleEndian.PutUint16(data[4:], bytecode.FormatVersion+1)
	_, err = bytecode.Decode(data)
	if err == nil || !strings.Contains(err.Error(), "rebuild") {
		return fmt.Errorf("a format %d file was not rejected with a rebuild hint (got %v)", bytecode.FormatVersion+1, err)
	}
	return nil
}

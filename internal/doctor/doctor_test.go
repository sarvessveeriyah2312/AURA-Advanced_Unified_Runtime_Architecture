package doctor

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sarvessveeriyah2312/Tread/internal/bytecode"
)

// fakeInstall builds an installation tree and an Env that sees it.
func fakeInstall(t *testing.T, release string) (Env, string) {
	t.Helper()
	home := filepath.Join(t.TempDir(), "tread")
	for _, d := range []string{"bin", "docs"} {
		if err := os.MkdirAll(filepath.Join(home, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	exe := filepath.Join(home, "bin", "tread")
	files := map[string]string{exe: "binary", filepath.Join(home, "docs", "LICENSE"): "MIT"}
	if release != "" {
		files[filepath.Join(home, ReleaseFile)] = release
	}
	for p, c := range files {
		if err := os.WriteFile(p, []byte(c), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	env := Env{
		Executable: exe,
		GOOS:       "linux",
		GOARCH:     "amd64",
		Getenv:     func(string) string { return "" },
		LookPath:   func(string) (string, error) { return exe, nil },
		ReadFile:   os.ReadFile,
		SameFile:   func(a, b string) bool { return filepath.Clean(a) == filepath.Clean(b) },
	}
	return env, home
}

const goodRelease = `# Tread installation metadata
TREAD_VERSION="1.2.3"
TREAD_EDITION="runtime"
TREAD_BYTECODE_FORMAT="4"
TREAD_PLATFORM="linux-x64"
`

func render(t *testing.T, r *Report) string {
	t.Helper()
	var b strings.Builder
	if err := r.Write(&b); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

func TestHealthyRuntimeInstall(t *testing.T) {
	env, home := fakeInstall(t, goodRelease)
	env.Getenv = func(k string) string {
		if k == "TREAD_HOME" {
			return home
		}
		return ""
	}
	r := Diagnose(env, Info{Version: "1.2.3", Edition: Runtime})
	got := render(t, r)
	want := "Tread Installation Diagnostics\n\n" +
		"[OK]   Tread CLI: tread 1.2.3 (Runtime) at " + env.Executable + "\n" +
		"[OK]   Installation: " + home + " (Tread Runtime 1.2.3, linux-x64)\n" +
		"[OK]   TREAD_HOME: " + home + "\n" +
		"[OK]   PATH: tread resolves to this executable\n" +
		"[OK]   Runtime: encoded, verified and ran a test program (bytecode format 4)\n" +
		"[OK]   Bytecode compatibility: format 4 loads; format 5 is rejected with a rebuild hint\n" +
		"[OK]   Standard library: built-in print and toString work (no library modules in this release)\n" +
		"\nInstallation is healthy.\n"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
	if r.Failed() {
		t.Error("healthy install reported as failed")
	}
}

func TestSDKCompilerCheck(t *testing.T) {
	env, _ := fakeInstall(t, strings.Replace(goodRelease, `"runtime"`, `"sdk"`, 1))
	// A compile function that returns the hand-built module stands in for
	// the real compiler (the CLI tests run the real one).
	info := Info{Version: "1.2.3", Edition: SDK, Compile: func(string, []byte) (*bytecode.Module, error) { return selfTestModule(), nil }}
	got := render(t, Diagnose(env, info))
	if !strings.Contains(got, "[OK]   SDK compiler: compiled a test program to bytecode and ran it\n") || !strings.HasSuffix(got, "Installation is healthy.\n") {
		t.Errorf("got:\n%s", got)
	}

	info.Compile = func(string, []byte) (*bytecode.Module, error) { return nil, errors.New("TR-C001: broken") }
	r := Diagnose(env, info)
	got = render(t, r)
	if !r.Failed() || !strings.Contains(got, "[FAIL] SDK compiler: self-test failed: TR-C001: broken\n") ||
		!strings.Contains(got, "[FAIL] Standard library:") || !strings.HasSuffix(got, "Installation has problems: 2 checks failed.\n") {
		t.Errorf("got:\n%s", got)
	}
}

func TestInstallationProblems(t *testing.T) {
	tests := []struct {
		name    string
		release string
		edit    func(env *Env, home string)
		want    []string
		failed  bool
	}{
		{"version mismatch", strings.Replace(goodRelease, "1.2.3", "1.0.0", 1), nil,
			[]string{`[FAIL] Installation: `, `TREAD_VERSION is "1.0.0", the executable is "1.2.3"; reinstall Tread`}, true},
		{"wrong platform", strings.Replace(goodRelease, "linux-x64", "macos-arm64", 1), nil,
			[]string{`TREAD_PLATFORM is "macos-arm64", the executable is "linux-x64"`}, true},
		{"malformed release", "TREAD_VERSION=1.2.3\n", nil, []string{"[FAIL] Installation: ", "is malformed: line 1 is not KEY=\"value\""}, true},
		{"missing license", goodRelease, func(_ *Env, home string) { os.Remove(filepath.Join(home, "docs", "LICENSE")) },
			[]string{"docs/LICENSE is missing"}, true},
		{"no release file", "", nil, []string{"[WARN] Installation: no release file in "}, false},
		{"development build", goodRelease, func(env *Env, _ string) { env.Executable = "/tmp/go-build/tread" },
			[]string{"[WARN] Installation: not running from an installation", "Installation is healthy, with 2 warnings."}, false},
		{"not on PATH", goodRelease, func(env *Env, _ string) {
			env.LookPath = func(string) (string, error) { return "", errors.New("not found") }
		}, []string{"[WARN] PATH: tread is not on PATH; add ", "Installation is healthy, with 1 warning."}, false},
		{"other tread first", goodRelease, func(env *Env, _ string) {
			env.LookPath = func(string) (string, error) { return "/usr/local/bin/tread", nil }
		}, []string{"[WARN] PATH: the first tread on PATH is /usr/local/bin/tread, not this executable"}, false},
		{"stale TREAD_HOME", goodRelease, func(env *Env, _ string) {
			env.Getenv = func(string) string { return "/opt/old-tread" }
		}, []string{"[WARN] TREAD_HOME: /opt/old-tread is not the installation running this command"}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env, home := fakeInstall(t, tc.release)
			if tc.edit != nil {
				tc.edit(&env, home)
			}
			r := Diagnose(env, Info{Version: "1.2.3", Edition: Runtime})
			got := render(t, r)
			for _, w := range tc.want {
				if !strings.Contains(got, w) {
					t.Errorf("output lacks %q:\n%s", w, got)
				}
			}
			if r.Failed() != tc.failed {
				t.Errorf("Failed() = %v, want %v", r.Failed(), tc.failed)
			}
		})
	}
}

func TestPlatform(t *testing.T) {
	for in, want := range map[[2]string]string{
		{"darwin", "arm64"}: "macos-arm64", {"darwin", "amd64"}: "macos-x64",
		{"linux", "amd64"}: "linux-x64", {"linux", "arm64"}: "linux-arm64", {"windows", "amd64"}: "windows-x64",
	} {
		if got := Platform(in[0], in[1]); got != want {
			t.Errorf("Platform(%s, %s) = %s, want %s", in[0], in[1], got, want)
		}
	}
}

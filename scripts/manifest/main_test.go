package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sarvessveeriyah2312/Tread/internal/bytecode"
)

func TestManifest(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"tread-sdk-1.2.3-linux-x64.tar.gz":    "a",
		"tread-runtime-1.2.3-windows-x64.zip": "b",
		"tread-sdk-1.2.3-macos-arm64.pkg":     "c",
		"tread-sdk-1.2.3-linux-arm64.deb":     "d",
		"install.sh":                          "e",
		"BUILD-INFO.txt":                      "ignored",
	}
	for n, c := range files {
		if err := os.WriteFile(filepath.Join(dir, n), []byte(c), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	os.Mkdir(filepath.Join(dir, ".work"), 0o755)
	if err := run([]string{"-version", "1.2.3", dir}); err != nil {
		t.Fatal(err)
	}
	var m Manifest
	data, _ := os.ReadFile(filepath.Join(dir, "release-manifest.json"))
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	if m.Version != "1.2.3" || m.BytecodeFormat != bytecode.FormatVersion || len(m.Artifacts) != 5 || m.ReleaseURL != Repository+"/releases/tag/v1.2.3" {
		t.Fatalf("manifest %+v", m)
	}
	sum := sha256.Sum256([]byte("a"))
	var found bool
	for _, a := range m.Artifacts {
		if a.File == "tread-sdk-1.2.3-linux-x64.tar.gz" {
			found = a.Edition == "sdk" && a.Platform == "linux-x64" && a.Kind == "archive" && a.Size == 1 &&
				a.SHA256 == hex.EncodeToString(sum[:]) && a.URL == Repository+"/releases/download/v1.2.3/"+a.File
		}
	}
	if !found {
		t.Errorf("linux archive entry wrong: %+v", m.Artifacts)
	}
	sums, _ := os.ReadFile(filepath.Join(dir, "SHA256SUMS"))
	lines := strings.Split(strings.TrimSpace(string(sums)), "\n")
	if len(lines) != 5 || !strings.HasSuffix(lines[0], "  install.sh") {
		t.Errorf("SHA256SUMS:\n%s", sums)
	}
	// Re-running (with the generated files present) is stable.
	before := string(data)
	if err := run([]string{"-version", "1.2.3", dir}); err != nil {
		t.Fatal(err)
	}
	if after, _ := os.ReadFile(filepath.Join(dir, "release-manifest.json")); string(after) != before {
		t.Error("manifest changed on re-run")
	}
}

func TestManifestRejectsStrayFiles(t *testing.T) {
	for _, name := range []string{"notes.txt", "tread-sdk-9.9.9-linux-x64.tar.gz", "tread-sdk-1.2.3-plan9-x64.tar.gz", "tread-sdk-1.2.3-linux-x64.zip", "tread-sdk-1.2.3-windows-x64.tar.gz"} {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644)
		if err := run([]string{"-version", "1.2.3", dir}); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
	if err := run([]string{"-version", "1.2.3", t.TempDir()}); err == nil {
		t.Error("empty directory accepted")
	}
}

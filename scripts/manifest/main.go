// Command manifest writes SHA256SUMS and release-manifest.json for a
// directory of Tread release artifacts (ADR-0011 §6). It is release
// tooling, run by scripts/build-release.sh and the release workflow; it
// is not part of the Tread toolchain.
//
// Usage:
//
//	go run ./scripts/manifest -version 0.4.0 DIST_DIR
//
// Every file in DIST_DIR must be a recognized artifact (or one of the
// files this command writes, or BUILD-INFO.txt), so nothing is published
// without being listed and checksummed.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/sarvessveeriyah2312/Tread/internal/bytecode"
)

// Repository is the GitHub repository that hosts releases.
const Repository = "https://github.com/sarvessveeriyah2312/Tread"

// Artifact is one downloadable file.
type Artifact struct {
	File     string `json:"file"`
	Edition  string `json:"edition"`  // "sdk", "runtime", or "" for install scripts
	Platform string `json:"platform"` // e.g. "linux-x64"; "unix" or "windows" for scripts
	Kind     string `json:"kind"`     // archive, deb, rpm, pkg, msi, script
	Size     int64  `json:"size"`
	SHA256   string `json:"sha256"`
	URL      string `json:"url"`
}

// Manifest is release-manifest.json.
type Manifest struct {
	Name           string     `json:"name"`
	Version        string     `json:"version"`
	BytecodeFormat int        `json:"bytecodeFormat"`
	Repository     string     `json:"repository"`
	ReleaseURL     string     `json:"releaseUrl"`
	Checksums      string     `json:"checksums"`
	Artifacts      []Artifact `json:"artifacts"`
}

// Generated and informational files that are not artifacts.
var skip = map[string]bool{"SHA256SUMS": true, "release-manifest.json": true, "BUILD-INFO.txt": true}

var platforms = map[string]bool{"windows-x64": true, "macos-arm64": true, "macos-x64": true, "linux-x64": true, "linux-arm64": true}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "manifest:", err)
		os.Exit(1)
	}
}

// classify returns the artifact described by a file name, or an error
// if the name is not a recognized artifact of this version.
func classify(name, version string) (Artifact, error) {
	switch name {
	case "install.sh":
		return Artifact{File: name, Platform: "unix", Kind: "script"}, nil
	case "install.ps1":
		return Artifact{File: name, Platform: "windows", Kind: "script"}, nil
	}
	re := regexp.MustCompile(`^tread-(sdk|runtime)-` + regexp.QuoteMeta(version) + `-([a-z]+-[a-z0-9]+)\.(tar\.gz|zip|deb|rpm|pkg|msi)$`)
	m := re.FindStringSubmatch(name)
	if m == nil || !platforms[m[2]] {
		return Artifact{}, fmt.Errorf("unrecognized file %q (not a Tread %s artifact)", name, version)
	}
	kind := m[3]
	if kind == "tar.gz" || kind == "zip" {
		kind = "archive"
	}
	wantZip := strings.HasPrefix(m[2], "windows-")
	if (m[3] == "zip") != wantZip && kind == "archive" {
		return Artifact{}, fmt.Errorf("%q: Windows archives are .zip, others .tar.gz", name)
	}
	return Artifact{File: name, Edition: m[1], Platform: m[2], Kind: kind}, nil
}

func run(args []string) error {
	fs := flag.NewFlagSet("manifest", flag.ContinueOnError)
	version := fs.String("version", "", "release version")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *version == "" || fs.NArg() != 1 {
		return errors.New("usage: manifest -version VERSION DIST_DIR")
	}
	dir := fs.Arg(0)
	ents, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	tag := "v" + *version
	m := Manifest{
		Name:           "Tread",
		Version:        *version,
		BytecodeFormat: bytecode.FormatVersion,
		Repository:     Repository,
		ReleaseURL:     Repository + "/releases/tag/" + tag,
		Checksums:      Repository + "/releases/download/" + tag + "/SHA256SUMS",
		Artifacts:      []Artifact{},
	}
	for _, e := range ents {
		name := e.Name()
		if skip[name] || strings.HasPrefix(name, ".") {
			continue
		}
		if !e.Type().IsRegular() {
			return fmt.Errorf("%s is not a regular file", name)
		}
		a, err := classify(name, *version)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		a.Size, a.SHA256 = int64(len(data)), hex.EncodeToString(sum[:])
		a.URL = Repository + "/releases/download/" + tag + "/" + name
		m.Artifacts = append(m.Artifacts, a)
	}
	if len(m.Artifacts) == 0 {
		return fmt.Errorf("no artifacts in %s", dir)
	}
	sort.Slice(m.Artifacts, func(i, j int) bool { return m.Artifacts[i].File < m.Artifacts[j].File })

	var sums strings.Builder
	for _, a := range m.Artifacts {
		fmt.Fprintf(&sums, "%s  %s\n", a.SHA256, a.File)
	}
	js, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "SHA256SUMS"), []byte(sums.String()), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "release-manifest.json"), append(js, '\n'), 0o644)
}

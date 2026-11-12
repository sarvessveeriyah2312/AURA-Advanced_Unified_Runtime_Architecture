// Package project creates new Tread projects (`tread new`, ADR-0011 §3).
//
// It only writes files. It never overwrites or deletes anything: an
// existing path is an error.
package project

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
)

// ConfigFile is the project configuration file name. No Tread 0.4
// command reads it; it records the entry point for future tooling.
const ConfigFile = "tread.toml"

// Entry is the entry point of a new project, relative to its root.
const Entry = "src/main.tr"

var validName = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]*$`)

// NameError reports an invalid project name.
type NameError struct{ Name string }

func (e *NameError) Error() string {
	return fmt.Sprintf("invalid project name %q: use a letter followed by letters, digits, '_' or '-'", e.Name)
}

// Files returns the files of a new project, keyed by slash-separated path
// relative to the project root. treadVersion is the creating SDK's
// version.
func Files(name, treadVersion string) map[string]string {
	return map[string]string{
		ConfigFile: `# Tread project configuration.
#
# Tread ` + treadVersion + ` does not read this file yet: run the entry point
# directly with the commands in README.md. It records the project's
# metadata for future tooling.

[project]
name = "` + name + `"
version = "0.1.0"
entry = "` + Entry + `"
tread = "` + treadVersion + `"
`,
		Entry: `module main

fn main() -> Unit {
    print("Hello from ` + name + `!")
}
`,
		"README.md": `# ` + name + `

A Tread program.

` + "```sh" + `
tread check src/main.tr    # check for errors
tread run src/main.tr      # run from source
tread build src/main.tr    # compile to src/main.tbc
tread run src/main.tbc     # run the compiled program
` + "```" + `
`,
	}
}

// Create makes the directory dir and writes a new project named name into
// it. dir must not exist.
func Create(dir, name, treadVersion string) error {
	if !validName.MatchString(name) {
		return &NameError{Name: name}
	}
	if err := os.Mkdir(dir, 0o755); err != nil {
		if errors.Is(err, fs.ErrExist) {
			return fmt.Errorf("cannot create project: %s already exists", dir)
		}
		return fmt.Errorf("cannot create project directory %s: %v", dir, unwrap(err))
	}
	files := Files(name, treadVersion)
	for _, rel := range sortedKeys(files) {
		path := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return fmt.Errorf("cannot create %s: %v", filepath.Dir(path), unwrap(err))
		}
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err != nil {
			return fmt.Errorf("cannot create %s: %v", path, unwrap(err))
		}
		_, werr := f.WriteString(files[rel])
		if cerr := f.Close(); werr == nil {
			werr = cerr
		}
		if werr != nil {
			return fmt.Errorf("cannot write %s: %v", path, unwrap(werr))
		}
	}
	return nil
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func unwrap(err error) error {
	var pe *fs.PathError
	if errors.As(err, &pe) {
		return pe.Err
	}
	return err
}

package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeInputs creates the files used by the tests and returns their specs.
func writeInputs(t *testing.T) (dir string, specs []string) {
	t.Helper()
	dir = t.TempDir()
	files := map[string]string{"aura": "binary\x00data", "LICENSE": "MIT\n", "README.md": "# AURA\n"}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	// Deliberately unsorted: the archive must sort entries itself.
	specs = []string{
		"644:README.md=" + filepath.Join(dir, "README.md"),
		"755:aura=" + filepath.Join(dir, "aura"),
		"644:LICENSE=" + filepath.Join(dir, "LICENSE"),
	}
	return dir, specs
}

func build(t *testing.T, out string, specs []string) []byte {
	t.Helper()
	args := append([]string{"-o", out, "-dir", "aura_1.2.3_os_arch"}, specs...)
	if err := run(args); err != nil {
		t.Fatalf("run: %v", err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestTarGzIsReproducibleAndCorrect(t *testing.T) {
	dir, specs := writeInputs(t)
	a := build(t, filepath.Join(dir, "a.tar.gz"), specs)
	// Different input order and a later build must give identical bytes.
	b := build(t, filepath.Join(dir, "b.tar.gz"), []string{specs[2], specs[0], specs[1]})
	if !bytes.Equal(a, b) {
		t.Fatal("two builds of the same inputs differ")
	}
	gz, err := gzip.NewReader(bytes.NewReader(a))
	if err != nil {
		t.Fatal(err)
	}
	if gz.Name != "" || !gz.ModTime.IsZero() {
		t.Errorf("gzip header records name %q / time %v", gz.Name, gz.ModTime)
	}
	tr := tar.NewReader(gz)
	var got []string
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if !h.ModTime.Equal(fixedTime) || h.Uid != 0 || h.Gid != 0 || h.Uname != "" {
			t.Errorf("%s: non-fixed metadata %v uid=%d gid=%d uname=%q", h.Name, h.ModTime, h.Uid, h.Gid, h.Uname)
		}
		body, _ := io.ReadAll(tr)
		got = append(got, h.Name+" "+os.FileMode(h.Mode).String()+" "+string(body))
	}
	want := []string{
		"aura_1.2.3_os_arch/ -rwxr-xr-x ",
		"aura_1.2.3_os_arch/LICENSE -rw-r--r-- MIT\n",
		"aura_1.2.3_os_arch/README.md -rw-r--r-- # AURA\n",
		"aura_1.2.3_os_arch/aura -rwxr-xr-x binary\x00data",
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("entries:\n got %q\nwant %q", got, want)
	}
}

func TestZipIsReproducibleAndCorrect(t *testing.T) {
	dir, specs := writeInputs(t)
	a := build(t, filepath.Join(dir, "a.zip"), specs)
	b := build(t, filepath.Join(dir, "b.zip"), []string{specs[1], specs[2], specs[0]})
	if !bytes.Equal(a, b) {
		t.Fatal("two builds of the same inputs differ")
	}
	zr, err := zip.NewReader(bytes.NewReader(a), int64(len(a)))
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(rc)
		rc.Close()
		got = append(got, f.Name+" "+f.Mode().String()+" "+string(body))
	}
	want := []string{
		"aura_1.2.3_os_arch/ drwxr-xr-x ",
		"aura_1.2.3_os_arch/LICENSE -rw-r--r-- MIT\n",
		"aura_1.2.3_os_arch/README.md -rw-r--r-- # AURA\n",
		"aura_1.2.3_os_arch/aura -rwxr-xr-x binary\x00data",
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("entries:\n got %q\nwant %q", got, want)
	}
}

func TestRejectsInvalidArguments(t *testing.T) {
	dir, specs := writeInputs(t)
	out := filepath.Join(dir, "x.tar.gz")
	cases := map[string][]string{
		"no entries":     {"-o", out, "-dir", "top"},
		"no output":      {"-dir", "top", specs[0]},
		"bad extension":  {"-o", filepath.Join(dir, "x.rar"), "-dir", "top", specs[0]},
		"nested dir":     {"-o", out, "-dir", "a/b", specs[0]},
		"missing mode":   {"-o", out, "-dir", "top", "LICENSE=" + filepath.Join(dir, "LICENSE")},
		"bad mode":       {"-o", out, "-dir", "top", "999:LICENSE=" + filepath.Join(dir, "LICENSE")},
		"escaping name":  {"-o", out, "-dir", "top", "644:../LICENSE=" + filepath.Join(dir, "LICENSE")},
		"absolute name":  {"-o", out, "-dir", "top", "644:/LICENSE=" + filepath.Join(dir, "LICENSE")},
		"duplicate name": {"-o", out, "-dir", "top", specs[0], specs[0]},
		"missing source": {"-o", out, "-dir", "top", "644:x=" + filepath.Join(dir, "missing")},
	}
	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			if err := run(args); err == nil {
				t.Errorf("run(%q) succeeded, want an error", args)
			}
		})
	}
}

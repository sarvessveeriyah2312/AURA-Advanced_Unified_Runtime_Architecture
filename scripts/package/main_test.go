package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// writeInputs creates the files used by the tests and returns their specs.
func writeInputs(t *testing.T) (dir string, specs []string) {
	t.Helper()
	dir = t.TempDir()
	files := map[string]string{"tread": "binary\x00data", "LICENSE": "MIT\n", "README.md": "# Tread\n"}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	// Deliberately unsorted: the archive must sort entries itself.
	specs = []string{
		"644:README.md=" + filepath.Join(dir, "README.md"),
		"755:tread=" + filepath.Join(dir, "tread"),
		"644:LICENSE=" + filepath.Join(dir, "LICENSE"),
	}
	return dir, specs
}

func build(t *testing.T, out string, specs []string) []byte {
	t.Helper()
	args := append([]string{"-o", out, "-dir", "tread_1.2.3_os_arch"}, specs...)
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
		"tread_1.2.3_os_arch/ -rwxr-xr-x ",
		"tread_1.2.3_os_arch/LICENSE -rw-r--r-- MIT\n",
		"tread_1.2.3_os_arch/README.md -rw-r--r-- # Tread\n",
		"tread_1.2.3_os_arch/tread -rwxr-xr-x binary\x00data",
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
		"tread_1.2.3_os_arch/ drwxr-xr-x ",
		"tread_1.2.3_os_arch/LICENSE -rw-r--r-- MIT\n",
		"tread_1.2.3_os_arch/README.md -rw-r--r-- # Tread\n",
		"tread_1.2.3_os_arch/tread -rwxr-xr-x binary\x00data",
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

func TestNestedEntriesGetDirectories(t *testing.T) {
	dir, _ := writeInputs(t)
	out := filepath.Join(dir, "n.tar.gz")
	if err := run([]string{"-o", out, "-dir", "top", "755:bin/tread=" + filepath.Join(dir, "tread"), "644:docs/LICENSE=" + filepath.Join(dir, "LICENSE")}); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(out)
	gz, _ := gzip.NewReader(bytes.NewReader(data))
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
		got = append(got, h.Name+" "+string(h.Typeflag))
	}
	want := "top/ 5|top/bin/ 5|top/bin/tread 0|top/docs/ 5|top/docs/LICENSE 0"
	if strings.Join(got, "|") != want {
		t.Errorf("entries %q, want %q", strings.Join(got, "|"), want)
	}
}

// readAr splits an ar archive into its members.
func readAr(t *testing.T, data []byte) map[string][]byte {
	t.Helper()
	if !bytes.HasPrefix(data, []byte("!<arch>\n")) {
		t.Fatal("missing ar magic")
	}
	members := map[string][]byte{}
	var order []string
	for off := 8; off < len(data); {
		h := string(data[off : off+60])
		if h[58:60] != "`\n" {
			t.Fatalf("bad ar header %q", h)
		}
		name := strings.TrimSpace(h[:16])
		size, err := strconv.Atoi(strings.TrimSpace(h[48:58]))
		if err != nil {
			t.Fatal(err)
		}
		off += 60
		members[name] = data[off : off+size]
		order = append(order, name)
		off += size + size%2
	}
	if strings.Join(order, ",") != "debian-binary,control.tar.gz,data.tar.gz" {
		t.Errorf("ar member order %v", order)
	}
	return members
}

func untar(t *testing.T, data []byte) map[string]*tar.Header {
	t.Helper()
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	tr := tar.NewReader(gz)
	out := map[string]*tar.Header{}
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return out
		}
		if err != nil {
			t.Fatal(err)
		}
		if h.Typeflag == tar.TypeReg {
			body, _ := io.ReadAll(tr)
			h.PAXRecords = map[string]string{"body": string(body)}
		}
		out[h.Name] = h
	}
}

func TestDeb(t *testing.T) {
	dir, _ := writeInputs(t)
	ctl := filepath.Join(dir, "control")
	if err := os.WriteFile(ctl, []byte("Package: tread\nVersion: 1.2.3\nArchitecture: amd64\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	args := func(out string) []string {
		return []string{"-o", out, "-dir", "opt/tread", "-control", ctl, "-link", "usr/bin/tread=/opt/tread/bin/tread",
			"755:bin/tread=" + filepath.Join(dir, "tread"), "644:docs/LICENSE=" + filepath.Join(dir, "LICENSE")}
	}
	a, b := filepath.Join(dir, "a.deb"), filepath.Join(dir, "b.deb")
	for _, out := range []string{a, b} {
		if err := run(args(out)); err != nil {
			t.Fatal(err)
		}
	}
	da, _ := os.ReadFile(a)
	db, _ := os.ReadFile(b)
	if !bytes.Equal(da, db) {
		t.Fatal("two .deb builds differ")
	}
	m := readAr(t, da)
	if string(m["debian-binary"]) != "2.0\n" {
		t.Errorf("debian-binary = %q", m["debian-binary"])
	}
	control := untar(t, m["control.tar.gz"])
	if c := control["./control"]; c == nil || !strings.HasSuffix(c.PAXRecords["body"], "Architecture: amd64\nInstalled-Size: 1\n") {
		t.Errorf("control = %+v", c)
	}
	if s := control["./md5sums"]; s == nil || !strings.Contains(s.PAXRecords["body"], "  opt/tread/bin/tread\n") {
		t.Errorf("md5sums = %+v", s)
	}
	data := untar(t, m["data.tar.gz"])
	for _, d := range []string{"./", "./opt/", "./opt/tread/", "./opt/tread/bin/", "./opt/tread/docs/", "./usr/", "./usr/bin/"} {
		if h := data[d]; h == nil || h.Typeflag != tar.TypeDir {
			t.Errorf("missing directory %s", d)
		}
	}
	if h := data["./opt/tread/bin/tread"]; h == nil || h.Mode != 0o755 || h.PAXRecords["body"] != "binary\x00data" {
		t.Errorf("binary entry %+v", h)
	}
	if h := data["./usr/bin/tread"]; h == nil || h.Typeflag != tar.TypeSymlink || h.Linkname != "/opt/tread/bin/tread" {
		t.Errorf("symlink entry %+v", h)
	}
	if len(data) != 10 {
		t.Errorf("%d data entries", len(data))
	}

	for name, bad := range map[string][]string{
		"no control":     {"-o", a, "-dir", "opt/tread", "755:bin/tread=" + filepath.Join(dir, "tread")},
		"absolute root":  {"-o", a, "-dir", "/opt/tread", "-control", ctl, "755:bin/tread=" + filepath.Join(dir, "tread")},
		"link in tar.gz": {"-o", filepath.Join(dir, "x.tar.gz"), "-dir", "top", "-link", "a=b", "755:bin/tread=" + filepath.Join(dir, "tread")},
		"bad link":       {"-o", a, "-dir", "opt/tread", "-control", ctl, "-link", "/usr/bin/tread", "755:bin/tread=" + filepath.Join(dir, "tread")},
	} {
		if err := run(bad); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

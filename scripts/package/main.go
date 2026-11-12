// Command package writes one reproducible Tread release archive or
// Debian package (ADR-0008, ADR-0011). It is release tooling, run by
// scripts/build-release.sh; it is not part of the Tread toolchain.
//
// Usage:
//
//	go run ./scripts/package -o OUT -dir TOP MODE:NAME=SRC...
//	go run ./scripts/package -o OUT.deb -dir opt/tread -control FILE \
//	    [-link PATH=TARGET]... MODE:NAME=SRC...
//
// OUT ends in .tar.gz, .zip or .deb, which selects the format. Every
// entry is stored as TOP/NAME with the octal permission MODE, the
// contents of the file SRC, and a fixed modification time. Directory
// entries are added for every parent directory. Entries are sorted by
// name, and owner, group and compression settings are fixed, so the same
// inputs always produce byte-identical output on every OS.
//
// For .deb, TOP is the install path without a leading slash (it may
// contain slashes), -control names the control file (an Installed-Size
// field is appended), and each -link adds a symbolic link PATH (relative
// to /) pointing at TARGET.
package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/md5"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// fixedTime is the modification time of every archive entry. It is the
// earliest time a zip file can record, and matches Go module zips.
var fixedTime = time.Date(1980, time.January, 1, 0, 0, 0, 0, time.UTC)

// entry is one file to store in the archive.
type entry struct {
	name string      // path inside TOP, using forward slashes
	mode os.FileMode // permission bits
	data []byte
}

// link is a symbolic link in a .deb.
type link struct{ name, target string }

type linkFlags []link

func (l *linkFlags) String() string { return fmt.Sprint(*l) }
func (l *linkFlags) Set(v string) error {
	name, target, ok := strings.Cut(v, "=")
	if !ok || !validName(name) || target == "" {
		return fmt.Errorf("invalid -link %q: want PATH=TARGET", v)
	}
	*l = append(*l, link{name, target})
	return nil
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "package:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("package", flag.ContinueOnError)
	out := fs.String("o", "", "output file (.tar.gz, .zip or .deb)")
	top := fs.String("dir", "", "top-level directory inside the archive")
	control := fs.String("control", "", "Debian control file (.deb only)")
	var links linkFlags
	fs.Var(&links, "link", "symbolic link PATH=TARGET (.deb only)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *out == "" || *top == "" || fs.NArg() == 0 {
		return errors.New("usage: package -o OUT.tar.gz|OUT.zip|OUT.deb -dir TOP MODE:NAME=SRC...")
	}
	isDeb := strings.HasSuffix(*out, ".deb")
	if isDeb {
		if !validName(*top) {
			return fmt.Errorf("invalid -dir %q: must be a relative path", *top)
		}
		if *control == "" {
			return errors.New(".deb output requires -control")
		}
	} else {
		if strings.ContainsAny(*top, `/\`) || *top == "." || *top == ".." {
			return fmt.Errorf("invalid -dir %q: must be a single directory name", *top)
		}
		if *control != "" || len(links) > 0 {
			return errors.New("-control and -link are only valid for .deb output")
		}
	}
	entries, err := readEntries(fs.Args())
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	switch {
	case isDeb:
		ctl, rerr := os.ReadFile(*control)
		if rerr != nil {
			return rerr
		}
		err = writeDeb(&buf, *top, ctl, entries, links)
	case strings.HasSuffix(*out, ".tar.gz"):
		err = writeTarGz(&buf, *top, entries)
	case strings.HasSuffix(*out, ".zip"):
		err = writeZip(&buf, *top, entries)
	default:
		return fmt.Errorf("unsupported output type %q: use .tar.gz, .zip or .deb", *out)
	}
	if err != nil {
		return err
	}
	return os.WriteFile(*out, buf.Bytes(), 0o644)
}

// validName reports whether name is a clean, relative, slash-separated
// path that stays inside its root.
func validName(name string) bool {
	return name != "" && !strings.Contains(name, `\`) && !strings.HasPrefix(name, "/") &&
		name == path.Clean(name) && name != "." && name != ".." && !strings.HasPrefix(name, "../")
}

// readEntries parses MODE:NAME=SRC arguments and reads each SRC.
func readEntries(specs []string) ([]entry, error) {
	seen := map[string]bool{}
	var entries []entry
	for _, spec := range specs {
		modeText, rest, ok1 := strings.Cut(spec, ":")
		name, src, ok2 := strings.Cut(rest, "=")
		if !ok1 || !ok2 || name == "" || src == "" {
			return nil, fmt.Errorf("invalid entry %q: want MODE:NAME=SRC", spec)
		}
		mode, err := strconv.ParseUint(modeText, 8, 32)
		if err != nil || mode > 0o777 {
			return nil, fmt.Errorf("invalid mode %q in %q", modeText, spec)
		}
		if !validName(name) || name != filepath.ToSlash(name) {
			return nil, fmt.Errorf("invalid name %q in %q", name, spec)
		}
		if seen[name] {
			return nil, fmt.Errorf("duplicate name %q", name)
		}
		seen[name] = true
		data, err := os.ReadFile(src)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry{name: name, mode: os.FileMode(mode), data: data})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].name < entries[j].name })
	return entries, nil
}

// item is a file, directory or symbolic link in an archive.
type item struct {
	name   string // full path; directories end in "/"
	mode   os.FileMode
	data   []byte
	target string // symbolic links only
}

func (it item) isDir() bool { return it.name == "" || strings.HasSuffix(it.name, "/") }

// layout returns the sorted items of an archive: a directory entry for
// every parent directory of root/NAME (including root itself), then the
// files and links. root may be "" (no prefix) or end without a slash.
func layout(root string, entries []entry, links []link, dirsFromRoot bool) []item {
	prefix := ""
	if root != "" {
		prefix = root + "/"
	}
	dirs := map[string]bool{}
	addParents := func(p string) {
		for d := path.Dir(p); d != "." && d != "/"; d = path.Dir(d) {
			if !dirsFromRoot && !strings.HasPrefix(d+"/", prefix) {
				break
			}
			dirs[d+"/"] = true
		}
	}
	var items []item
	for _, e := range entries {
		items = append(items, item{name: prefix + e.name, mode: e.mode, data: e.data})
		addParents(prefix + e.name)
	}
	for _, l := range links {
		items = append(items, item{name: l.name, mode: 0o777, target: l.target})
		addParents(l.name)
	}
	if root != "" {
		dirs[prefix] = true
	}
	for d := range dirs {
		items = append(items, item{name: d, mode: 0o755})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].name < items[j].name })
	return items
}

// writeTar writes items as a USTAR tar with fixed metadata. Names get
// namePrefix (for example "./" in Debian packages).
func writeTar(w io.Writer, items []item, namePrefix string) error {
	tw := tar.NewWriter(w)
	for _, it := range items {
		h := &tar.Header{
			Typeflag: tar.TypeReg,
			Name:     namePrefix + it.name,
			Mode:     int64(it.mode),
			Size:     int64(len(it.data)),
			ModTime:  fixedTime,
			Format:   tar.FormatUSTAR,
		}
		switch {
		case it.isDir():
			h.Typeflag = tar.TypeDir
		case it.target != "":
			h.Typeflag, h.Linkname, h.Size = tar.TypeSymlink, it.target, 0
		}
		if err := tw.WriteHeader(h); err != nil {
			return err
		}
		if h.Typeflag == tar.TypeReg {
			if _, err := tw.Write(it.data); err != nil {
				return err
			}
		}
	}
	return tw.Close()
}

// gzipped compresses with fixed settings; the gzip header keeps its zero
// values: no name, no time, OS unknown.
func gzipped(fill func(io.Writer) error) ([]byte, error) {
	var buf bytes.Buffer
	gz, err := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	if err != nil {
		return nil, err
	}
	if err := fill(gz); err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func writeTarGz(w io.Writer, top string, entries []entry) error {
	data, err := gzipped(func(gz io.Writer) error { return writeTar(gz, layout(top, entries, nil, false), "") })
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

func writeZip(w io.Writer, top string, entries []entry) error {
	zw := zip.NewWriter(w)
	for _, it := range layout(top, entries, nil, false) {
		fh := &zip.FileHeader{Name: it.name, Method: zip.Deflate, Modified: fixedTime}
		if it.isDir() {
			fh.Method = zip.Store
			fh.SetMode(os.ModeDir | it.mode)
		} else {
			fh.SetMode(it.mode)
		}
		fw, err := zw.CreateHeader(fh)
		if err != nil {
			return err
		}
		if _, err := fw.Write(it.data); err != nil {
			return err
		}
	}
	return zw.Close()
}

// writeDeb writes a Debian binary package: an ar archive holding
// debian-binary, control.tar.gz and data.tar.gz (deb(5)).
func writeDeb(w io.Writer, root string, control []byte, entries []entry, links []link) error {
	items := layout(root, entries, links, true)
	var size int64
	var sums strings.Builder
	for _, it := range items {
		if !it.isDir() && it.target == "" {
			size += int64(len(it.data))
			fmt.Fprintf(&sums, "%x  %s\n", md5.Sum(it.data), it.name)
		}
	}
	ctl := strings.TrimRight(string(control), "\n") + fmt.Sprintf("\nInstalled-Size: %d\n", (size+1023)/1024)

	data, err := gzipped(func(gz io.Writer) error {
		return writeTar(gz, append([]item{{name: "", mode: 0o755}}, items...), "./")
	})
	if err != nil {
		return err
	}
	ctlTar, err := gzipped(func(gz io.Writer) error {
		return writeTar(gz, []item{
			{name: "", mode: 0o755},
			{name: "control", mode: 0o644, data: []byte(ctl)},
			{name: "md5sums", mode: 0o644, data: []byte(sums.String())},
		}, "./")
	})
	if err != nil {
		return err
	}

	var b bytes.Buffer
	b.WriteString("!<arch>\n")
	for _, m := range []struct {
		name string
		data []byte
	}{{"debian-binary", []byte("2.0\n")}, {"control.tar.gz", ctlTar}, {"data.tar.gz", data}} {
		// ar member header: name/16 mtime/12 uid/6 gid/6 mode/8 size/10 "`\n".
		fmt.Fprintf(&b, "%-16s%-12d%-6d%-6d%-8s%-10d`\n", m.name, fixedTime.Unix(), 0, 0, "100644", len(m.data))
		b.Write(m.data)
		if len(m.data)%2 == 1 {
			b.WriteByte('\n')
		}
	}
	_, err = w.Write(b.Bytes())
	return err
}

// Command package writes one reproducible Tread release archive
// (ADR-0008). It is release tooling, run by scripts/build-release.sh;
// it is not part of the Tread toolchain.
//
// Usage:
//
//	go run ./scripts/package -o OUT -dir TOP MODE:NAME=SRC...
//
// OUT ends in .tar.gz or .zip, which selects the format. Every entry
// is stored as TOP/NAME with the octal permission MODE, the contents of
// the file SRC, and a fixed modification time. Entries are sorted by
// name, and owner, group and compression settings are fixed, so the
// same inputs always produce byte-identical archives on every OS.
package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
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

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "package:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("package", flag.ContinueOnError)
	out := fs.String("o", "", "output archive (.tar.gz or .zip)")
	top := fs.String("dir", "", "top-level directory inside the archive")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *out == "" || *top == "" || fs.NArg() == 0 {
		return errors.New("usage: package -o OUT.tar.gz|OUT.zip -dir TOP MODE:NAME=SRC...")
	}
	if strings.ContainsAny(*top, `/\`) || *top == "." || *top == ".." {
		return fmt.Errorf("invalid -dir %q: must be a single directory name", *top)
	}
	entries, err := readEntries(fs.Args())
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	switch {
	case strings.HasSuffix(*out, ".tar.gz"):
		err = writeTarGz(&buf, *top, entries)
	case strings.HasSuffix(*out, ".zip"):
		err = writeZip(&buf, *top, entries)
	default:
		return fmt.Errorf("unsupported archive type %q: use .tar.gz or .zip", *out)
	}
	if err != nil {
		return err
	}
	return os.WriteFile(*out, buf.Bytes(), 0o644)
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
		if strings.Contains(name, `\`) || strings.HasPrefix(name, "/") ||
			name != filepath.ToSlash(filepath.Clean(name)) || strings.HasPrefix(name, "..") {
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

func writeTarGz(w io.Writer, top string, entries []entry) error {
	gz, err := gzip.NewWriterLevel(w, gzip.BestCompression)
	if err != nil {
		return err
	}
	// The gzip header keeps its zero values: no name, no time, OS unknown.
	tw := tar.NewWriter(gz)
	hdr := func(name string, mode os.FileMode, size int64, typ byte) *tar.Header {
		return &tar.Header{
			Typeflag: typ,
			Name:     name,
			Mode:     int64(mode),
			Size:     size,
			ModTime:  fixedTime,
			Format:   tar.FormatUSTAR,
		}
	}
	if err := tw.WriteHeader(hdr(top+"/", 0o755, 0, tar.TypeDir)); err != nil {
		return err
	}
	for _, e := range entries {
		if err := tw.WriteHeader(hdr(top+"/"+e.name, e.mode, int64(len(e.data)), tar.TypeReg)); err != nil {
			return err
		}
		if _, err := tw.Write(e.data); err != nil {
			return err
		}
	}
	if err := tw.Close(); err != nil {
		return err
	}
	return gz.Close()
}

func writeZip(w io.Writer, top string, entries []entry) error {
	zw := zip.NewWriter(w)
	dir := &zip.FileHeader{Name: top + "/", Method: zip.Store, Modified: fixedTime}
	dir.SetMode(os.ModeDir | 0o755)
	if _, err := zw.CreateHeader(dir); err != nil {
		return err
	}
	for _, e := range entries {
		fh := &zip.FileHeader{Name: top + "/" + e.name, Method: zip.Deflate, Modified: fixedTime}
		fh.SetMode(e.mode)
		fw, err := zw.CreateHeader(fh)
		if err != nil {
			return err
		}
		if _, err := fw.Write(e.data); err != nil {
			return err
		}
	}
	return zw.Close()
}

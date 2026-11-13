package lsp

import (
	"net/url"
	"path/filepath"
	"runtime"
	"strings"
	"unicode/utf8"

	"github.com/sarvessveeriyah2312/Tread/internal/source"
)

// LSP positions count UTF-16 code units within a line (the default
// position encoding). These helpers convert between them and byte
// offsets.

// offsetOf returns the byte offset of an LSP position in text, clamped to
// the text.
func offsetOf(text []byte, p position) int {
	off := 0
	for line := 0; line < p.Line; line++ {
		i := indexNewline(text[off:])
		if i < 0 {
			return len(text)
		}
		off += i
	}
	units := 0
	for off < len(text) && units < p.Character {
		r, size := utf8.DecodeRune(text[off:])
		if r == '\n' || r == '\r' {
			break
		}
		units += utf16Len(r)
		off += size
	}
	return off
}

// indexNewline returns the offset just after the first line terminator
// (LF, CRLF or a lone CR), or -1.
func indexNewline(b []byte) int {
	for i := 0; i < len(b); i++ {
		switch b[i] {
		case '\n':
			return i + 1
		case '\r':
			if i+1 < len(b) && b[i+1] == '\n' {
				return i + 2
			}
			return i + 1
		}
	}
	return -1
}

// positionOf returns the LSP position of a byte offset.
func positionOf(text []byte, off int) position {
	if off > len(text) {
		off = len(text)
	}
	line, start := 0, 0
	for {
		i := indexNewline(text[start:])
		if i < 0 || start+i > off {
			break
		}
		start += i
		line++
	}
	units := 0
	for i := start; i < off; {
		r, size := utf8.DecodeRune(text[i:])
		units += utf16Len(r)
		i += size
	}
	return position{Line: line, Character: units}
}

func utf16Len(r rune) int {
	if r >= 0x10000 {
		return 2
	}
	return 1
}

func rangeOf(text []byte, s source.Span) lspRange {
	return lspRange{Start: positionOf(text, s.Start), End: positionOf(text, s.End)}
}

// uriToPath converts a file:// URI to a file system path.
func uriToPath(uri string) string {
	u, err := url.Parse(uri)
	if err != nil || u.Scheme != "file" {
		return uri
	}
	p := u.Path
	if runtime.GOOS == "windows" {
		p = strings.TrimPrefix(p, "/") // /C:/x -> C:/x
	}
	return filepath.FromSlash(p)
}

// pathToURI converts an absolute path to a file:// URI.
func pathToURI(path string) string {
	p := filepath.ToSlash(path)
	if !strings.HasPrefix(p, "/") {
		p = "/" + p // C:/x -> /C:/x
	}
	return (&url.URL{Scheme: "file", Path: p}).String()
}

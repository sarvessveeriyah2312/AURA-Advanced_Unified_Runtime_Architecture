package source

import (
	"sort"
	"unicode/utf8"
)

// FileID identifies a source file within one compilation.
type FileID int

// Span is a half-open UTF-8 byte range [Start, End) within one file.
type Span struct {
	File  FileID
	Start int
	End   int
}

// Len returns the number of bytes covered by the span.
func (s Span) Len() int { return s.End - s.Start }

// Join returns the smallest span covering both a and b. Both spans must
// belong to the same file.
func Join(a, b Span) Span {
	out := a
	if b.Start < out.Start {
		out.Start = b.Start
	}
	if b.End > out.End {
		out.End = b.End
	}
	return out
}

// Position is a user-facing, 1-based line and column. Columns count
// Unicode code points (ADR-0002 D2.2).
type Position struct {
	Line   int
	Column int
}

// bomLen is the length of the UTF-8 byte order mark EF BB BF.
const bomLen = 3

// File is an immutable source file with a precomputed line table.
type File struct {
	id         FileID
	name       string
	content    []byte
	lineStarts []int
	hasBOM     bool
}

// NewFile creates a file. The content slice must not be modified after
// the call.
func NewFile(id FileID, name string, content []byte) *File {
	f := &File{id: id, name: name, content: content}
	f.hasBOM = len(content) >= bomLen &&
		content[0] == 0xEF && content[1] == 0xBB && content[2] == 0xBF
	f.lineStarts = computeLineStarts(content)
	return f
}

// computeLineStarts returns the byte offset of the start of every line.
// LF, CRLF and lone CR each terminate one line (ADR-0003 L2).
func computeLineStarts(content []byte) []int {
	starts := []int{0}
	for i := 0; i < len(content); i++ {
		switch content[i] {
		case '\n':
			starts = append(starts, i+1)
		case '\r':
			if i+1 < len(content) && content[i+1] == '\n' {
				i++
			}
			starts = append(starts, i+1)
		}
	}
	return starts
}

// ID returns the file identifier.
func (f *File) ID() FileID { return f.id }

// Name returns the file name used in diagnostics.
func (f *File) Name() string { return f.name }

// Content returns the file bytes. Callers must not modify them.
func (f *File) Content() []byte { return f.content }

// Len returns the size of the file in bytes.
func (f *File) Len() int { return len(f.content) }

// HasBOM reports whether the file starts with a UTF-8 byte order mark.
func (f *File) HasBOM() bool { return f.hasBOM }

// LineCount returns the number of lines. An empty file has one line, and
// a file ending in a terminator has an empty final line.
func (f *File) LineCount() int { return len(f.lineStarts) }

// Span returns a span in this file.
func (f *File) Span(start, end int) Span { return Span{File: f.id, Start: start, End: end} }

// lineIndex returns the 0-based line containing offset.
func (f *File) lineIndex(offset int) int {
	return sort.Search(len(f.lineStarts), func(i int) bool {
		return f.lineStarts[i] > offset
	}) - 1
}

// LineBounds returns the byte range of the 1-based line, excluding its
// terminator. The first line excludes a leading BOM.
func (f *File) LineBounds(line int) (start, end int) {
	idx := line - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(f.lineStarts) {
		idx = len(f.lineStarts) - 1
	}
	start = f.lineStarts[idx]
	if idx+1 < len(f.lineStarts) {
		end = f.lineStarts[idx+1]
		if end > start && f.content[end-1] == '\n' {
			end--
		}
		if end > start && f.content[end-1] == '\r' {
			end--
		}
	} else {
		end = len(f.content)
	}
	if idx == 0 && f.hasBOM {
		start = bomLen
		if end < start {
			end = start
		}
	}
	return start, end
}

// Position converts a byte offset to a 1-based line and code-point column.
// Offsets are clamped to [0, Len()]; the end-of-file offset is valid.
func (f *File) Position(offset int) Position {
	if offset < 0 {
		offset = 0
	}
	if offset > len(f.content) {
		offset = len(f.content)
	}
	idx := f.lineIndex(offset)
	start, _ := f.LineBounds(idx + 1)
	col := 1
	for i := start; i < offset; {
		_, w := utf8.DecodeRune(f.content[i:])
		i += w
		col++
	}
	return Position{Line: idx + 1, Column: col}
}

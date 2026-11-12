package source

import (
	"strings"
	"testing"
)

// TestPosition implements the required position vectors P1-P14
// (docs/04 §4.3.4).
func TestPosition(t *testing.T) {
	const bom = "\xef\xbb\xbf"
	tests := []struct {
		id     string
		src    string
		target string // last occurrence of target; empty means end of file
		offset int
		line   int
		col    int
	}{
		{"P1", "let x = 1\nlet y = z", "z", 18, 2, 9},
		{"P2", `let s = "héllo" + 1`, "1", 19, 1, 19},
		{"P3", `let s = "日本" + 1`, "1", 19, 1, 16},
		{"P4", `let s = "🚀" + 1`, "1", 17, 1, 15},
		{"P5", "\tlet x = y", "y", 9, 1, 10},
		{"P6", "\tlet s = \"é\" + 1", "1", 16, 1, 16},
		{"P7", "let s = \"é\" + 1", "1", 16, 1, 16},
		{"P8", "let x = 1\r\nlet y = z", "z", 19, 2, 9},
		{"P9", "let s = \"\xff\" + 1", "1", 14, 1, 15},
		{"P10", bom + "let x = y", "y", 11, 1, 9},
		{"P11", "let x =", "", 7, 1, 8},
		{"P12", "fn main() {\n", "", 12, 2, 1},
		{"P13", "", "", 0, 1, 1},
		{"P14", "let x = 1\rlet y = z", "z", 18, 2, 9},
	}
	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			f := NewFile(1, "test.tr", []byte(tt.src))
			offset := len(tt.src)
			if tt.target != "" {
				offset = strings.LastIndex(tt.src, tt.target)
			}
			if offset != tt.offset {
				t.Fatalf("offset = %d, want %d", offset, tt.offset)
			}
			got := f.Position(offset)
			if got != (Position{tt.line, tt.col}) {
				t.Errorf("Position(%d) = %d:%d, want %d:%d", offset, got.Line, got.Column, tt.line, tt.col)
			}
		})
	}
}

func TestPrecomposedVersusDecomposed(t *testing.T) {
	// docs/04 §4.3.4 note on P7: precomposed é gives 1:15.
	src := "let s = \"é\" + 1"
	f := NewFile(1, "t", []byte(src))
	off := strings.LastIndex(src, "1")
	if off != 15 {
		t.Fatalf("offset = %d, want 15", off)
	}
	if got := f.Position(off); got != (Position{1, 15}) {
		t.Errorf("got %d:%d, want 1:15", got.Line, got.Column)
	}
}

func TestLineBounds(t *testing.T) {
	tests := []struct {
		src   string
		line  int
		start int
		end   int
	}{
		{"ab\ncd", 1, 0, 2},
		{"ab\ncd", 2, 3, 5},
		{"ab\r\ncd", 1, 0, 2},
		{"ab\r\ncd", 2, 4, 6},
		{"ab\rcd", 2, 3, 5},
		{"ab\n", 2, 3, 3},
		{"\xef\xbb\xbfab\ncd", 1, 3, 5},
		{"\xef\xbb\xbf", 1, 3, 3},
		{"", 1, 0, 0},
	}
	for _, tt := range tests {
		f := NewFile(1, "t", []byte(tt.src))
		s, e := f.LineBounds(tt.line)
		if s != tt.start || e != tt.end {
			t.Errorf("%q line %d = [%d,%d), want [%d,%d)", tt.src, tt.line, s, e, tt.start, tt.end)
		}
	}
}

func TestLineCount(t *testing.T) {
	tests := map[string]int{"": 1, "a": 1, "a\n": 2, "a\r\nb": 2, "a\rb\nc": 3, "\r\r\n": 3}
	for src, want := range tests {
		if got := NewFile(1, "t", []byte(src)).LineCount(); got != want {
			t.Errorf("%q: LineCount = %d, want %d", src, got, want)
		}
	}
}

func TestPositionClampsOutOfRange(t *testing.T) {
	f := NewFile(1, "t", []byte("ab"))
	if got := f.Position(-5); got != (Position{1, 1}) {
		t.Errorf("negative offset: got %v", got)
	}
	if got := f.Position(99); got != (Position{1, 3}) {
		t.Errorf("past end: got %v", got)
	}
}

func TestJoin(t *testing.T) {
	a := Span{File: 1, Start: 4, End: 6}
	b := Span{File: 1, Start: 1, End: 5}
	if got := Join(a, b); got != (Span{File: 1, Start: 1, End: 6}) {
		t.Errorf("Join = %v", got)
	}
}

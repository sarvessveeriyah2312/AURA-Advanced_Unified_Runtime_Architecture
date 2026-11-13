package diagnostic

import (
	"regexp"
	"strings"
	"testing"

	"github.com/sarvessveeriyah2312/Tread/internal/source"
)

func TestRegistryWellFormedAndUnique(t *testing.T) {
	re := regexp.MustCompile(`^TR-[LPTCR][0-9]{3}$`)
	seen := map[Code]bool{}
	counts := map[byte]int{}
	for _, c := range Registry() {
		if !re.MatchString(string(c)) {
			t.Errorf("malformed code %q", c)
		}
		if seen[c] {
			t.Errorf("duplicate code %q", c)
		}
		seen[c] = true
		counts[c.Phase()]++
	}
	// docs/04 §4.4.3 registers 9 L, 14 P, 30 T, 1 C and 8 R codes.
	want := map[byte]int{'L': 9, 'P': 14, 'T': 30, 'C': 1, 'R': 8}
	for phase, n := range want {
		if counts[phase] != n {
			t.Errorf("phase %c: %d codes, want %d", phase, counts[phase], n)
		}
	}
}

func TestRegistryIsSequentialPerPhase(t *testing.T) {
	next := map[byte]int{}
	for _, c := range Registry() {
		p := c.Phase()
		next[p]++
		want := Code("TR-" + string(p) + pad3(next[p]))
		if c != want {
			t.Errorf("got %s, want %s (codes must be numbered from 001 without gaps)", c, want)
		}
	}
}

func pad3(n int) string {
	s := "00" + string(rune('0'+n%10))
	if n >= 10 {
		s = "0" + string(rune('0'+n/10)) + string(rune('0'+n%10))
	}
	return s
}

func TestPhaseOfMalformedCode(t *testing.T) {
	if Code("bad").Phase() != 0 {
		t.Error("malformed code should have phase 0")
	}
}

// caretLine returns the reported "line:col", the caret prefix (spaces
// after the gutter) and the caret count of a rendered diagnostic.
func caretInfo(t *testing.T, out string) (loc string, prefix, carets int) {
	t.Helper()
	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	if len(lines) < 5 {
		t.Fatalf("unexpected rendering:\n%s", out)
	}
	m := regexp.MustCompile(`--> [^:]+:(\d+:\d+)$`).FindStringSubmatch(lines[1])
	if m == nil {
		t.Fatalf("no location line in:\n%s", out)
	}
	caret := lines[4]
	i := strings.Index(caret, "| ")
	if i < 0 {
		t.Fatalf("no gutter in caret line %q", caret)
	}
	rest := caret[i+2:]
	prefix = len(rest) - len(strings.TrimLeft(rest, " "))
	carets = strings.Count(rest, "^")
	if strings.Trim(rest, " ^") != "" {
		t.Fatalf("unexpected characters in caret line %q", caret)
	}
	return m[1], prefix, carets
}

// TestRenderingVectors implements R1-R6 (docs/04 §4.4.2).
func TestRenderingVectors(t *testing.T) {
	tests := []struct {
		id          string
		src         string
		start, end  int
		loc         string
		prefix      int
		carets      int
		printedLine string
	}{
		{"R1", `let x: Int = "five"`, 13, 19, "1:14", 13, 6, `let x: Int = "five"`},
		{"R2", "\tlet x = y", 9, 10, "1:10", 12, 1, "    let x = y"},
		{"R3", `let s = "日本" + 1`, 19, 20, "1:16", 17, 1, `let s = "日本" + 1`},
		{"R4", "let s = \"é\" + true", 16, 20, "1:16", 14, 4, "let s = \"é\" + true"},
		{"R5", `let s = "日本"`, 8, 16, "1:9", 8, 6, `let s = "日本"`},
		{"R6", "let s = \"\xff\" + 1", 9, 10, "1:10", 9, 1, "let s = \"�\" + 1"},
	}
	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			f := source.NewFile(1, "t.tr", []byte(tt.src))
			var b strings.Builder
			if err := Render(&b, f, Errorf(TypeMismatch, f.Span(tt.start, tt.end), "msg")); err != nil {
				t.Fatal(err)
			}
			out := b.String()
			loc, prefix, carets := caretInfo(t, out)
			if loc != tt.loc || prefix != tt.prefix || carets != tt.carets {
				t.Errorf("got %s prefix=%d carets=%d, want %s prefix=%d carets=%d\n%s",
					loc, prefix, carets, tt.loc, tt.prefix, tt.carets, out)
			}
			if got := strings.Split(out, "\n")[3]; got != " 1 | "+tt.printedLine {
				t.Errorf("printed line = %q, want %q", got, " 1 | "+tt.printedLine)
			}
		})
	}
}

// TestRenderingR7 is the exact golden output of the docs/04 §4.4 example.
func TestRenderingR7(t *testing.T) {
	src := "fn main() {\n    // a\n    // b\nlet total: Int = \"five\"\n}\n"
	f := source.NewFile(1, "examples/main.tr", []byte(src))
	start := strings.Index(src, `"five"`)
	d := Errorf(TypeMismatch, f.Span(start, start+len(`"five"`)), "expected Int but found String")
	var b strings.Builder
	if err := Render(&b, f, d); err != nil {
		t.Fatal(err)
	}
	want := "error[TR-T001]: expected Int but found String\n" +
		"  --> examples/main.tr:4:18\n" +
		"   |\n" +
		" 4 | let total: Int = \"five\"\n" +
		"   |                  ^^^^^^\n"
	if b.String() != want {
		t.Errorf("got:\n%s\nwant:\n%s", b.String(), want)
	}
}

func TestRenderHelpRelatedAndWideGutter(t *testing.T) {
	src := strings.Repeat("\n", 9) + "module b\n"
	f := source.NewFile(1, "m.tr", []byte(src))
	d := Errorf(DuplicateModule, f.Span(9, 17), "duplicate module declaration").
		WithHelp("remove one").
		WithRelated(f.Span(0, 0), "first module declaration is here")
	var b strings.Builder
	if err := Render(&b, f, d); err != nil {
		t.Fatal(err)
	}
	want := "error[TR-P008]: duplicate module declaration\n" +
		"   --> m.tr:10:1\n" +
		"    |\n" +
		" 10 | module b\n" +
		"    | ^^^^^^^^\n" +
		"    = help: remove one\n" +
		"    = note: first module declaration is here (m.tr:1:1)\n"
	if b.String() != want {
		t.Errorf("got:\n%s\nwant:\n%s", b.String(), want)
	}
}

func TestRenderMultilineSpanAndEOF(t *testing.T) {
	src := "let x = (a\n+ b)"
	f := source.NewFile(1, "t", []byte(src))
	var b strings.Builder
	_ = Render(&b, f, Errorf(ExpectedExpression, f.Span(8, len(src)), "m"))
	if _, prefix, carets := caretInfo(t, b.String()); prefix != 8 || carets != 2 {
		t.Errorf("multi-line span: prefix=%d carets=%d, want 8, 2 (underline to end of first line)", prefix, carets)
	}
	b.Reset()
	_ = Render(&b, f, Errorf(ExpectedExpression, f.Span(len(src), len(src)), "m"))
	if loc, prefix, carets := caretInfo(t, b.String()); loc != "2:5" || prefix != 4 || carets != 1 {
		t.Errorf("EOF span: %s prefix=%d carets=%d", loc, prefix, carets)
	}
}

func TestRuneWidth(t *testing.T) {
	tests := []struct {
		r    rune
		want int
	}{
		{'a', 1}, {'\t', 4}, {0x0301, 0}, {0xFEFF, 0}, {'日', 2}, {'🚀', 2}, {'é', 1}, {0xAC00, 2},
	}
	for _, tt := range tests {
		if got := runeWidth(tt.r, false); got != tt.want {
			t.Errorf("runeWidth(%U) = %d, want %d", tt.r, got, tt.want)
		}
	}
	if runeWidth(0, true) != 1 {
		t.Error("invalid byte must have width 1")
	}
}

func TestWideRangesSorted(t *testing.T) {
	for i := 1; i < len(wideRanges); i++ {
		if wideRanges[i].lo <= wideRanges[i-1].hi || wideRanges[i].lo > wideRanges[i].hi {
			t.Fatalf("wideRanges not sorted/disjoint at %d", i)
		}
	}
}

func TestWithRelatedDoesNotAlias(t *testing.T) {
	base := Errorf(DuplicateModule, source.Span{}, "m").WithRelated(source.Span{}, "a")
	x := base.WithRelated(source.Span{Start: 1}, "x")
	y := base.WithRelated(source.Span{Start: 2}, "y")
	if x.Related[1].Message != "x" || y.Related[1].Message != "y" || len(base.Related) != 1 {
		t.Error("WithRelated must not share backing arrays")
	}
}

func TestRenderWithoutSource(t *testing.T) {
	var b strings.Builder
	d := Errorf(InvalidBytecode, source.Span{}, "invalid module: main index 3 out of range").WithHelp("h")
	if err := Render(&b, nil, d); err != nil {
		t.Fatal(err)
	}
	want := "error[TR-R002]: invalid module: main index 3 out of range\n  = help: h\n"
	if b.String() != want {
		t.Errorf("got %q, want %q", b.String(), want)
	}
}

// Regression: Phase once read a fixed byte offset that only matched the
// old five-character prefix. It must follow Prefix, whatever its length.
func TestPhaseFollowsPrefix(t *testing.T) {
	cases := map[Code]byte{
		Code(Prefix + "T001"): 'T',
		Code(Prefix + "R005"): 'R',
		"XX-T001":             0, // wrong prefix
		Code(Prefix + "T01"):  0, // too short
		"":                    0,
	}
	for c, want := range cases {
		if got := c.Phase(); got != want {
			t.Errorf("Code(%q).Phase() = %q, want %q", c, got, want)
		}
	}
	for _, c := range Registry() {
		if !strings.HasPrefix(string(c), Prefix) || c.Phase() == 0 {
			t.Errorf("registered code %q does not start with %q or has no phase", c, Prefix)
		}
	}
}

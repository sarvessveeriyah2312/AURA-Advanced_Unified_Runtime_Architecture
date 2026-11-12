package diagnostic

import (
	"fmt"
	"strings"
	"testing"

	"github.com/sarvessveeriyah2312/Tread/internal/source"
)

// Stack trace rendering (ADR-0009).

// frameAt returns a frame for fn located at the first occurrence of
// text in file.
func frameAt(t *testing.T, f *source.File, fn, text string) TraceFrame {
	t.Helper()
	i := strings.Index(string(f.Content()), text)
	if i < 0 {
		t.Fatalf("%q not in source", text)
	}
	return TraceFrame{Function: fn, Span: f.Span(i, i+len(text))}
}

func renderTrace(t *testing.T, f *source.File, frames []TraceFrame) string {
	t.Helper()
	var b strings.Builder
	if err := RenderTrace(&b, f, frames); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

const traceSrc = "fn main() -> Unit {\n    print(average(10, 0))\n}\n\n" +
	"fn average(total: Int, count: Int) -> Int {\n    return divide(total, count)\n}\n\n" +
	"fn divide(a: Int, b: Int) -> Int {\n    return a / b\n}\n"

func TestRenderTraceAlignedColumns(t *testing.T) {
	f := source.NewFile(1, "trace.tr", []byte(traceSrc))
	got := renderTrace(t, f, []TraceFrame{
		frameAt(t, f, "divide", "a / b"),
		frameAt(t, f, "average", "divide(total, count)"),
		frameAt(t, f, "main", "average(10, 0)"),
	})
	want := "stack trace:\n" +
		"  0: divide   trace.tr:10:12  return a / b\n" +
		"  1: average  trace.tr:6:12   return divide(total, count)\n" +
		"  2: main     trace.tr:2:11   print(average(10, 0))\n"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestRenderTraceNothingForMainOrNoFile(t *testing.T) {
	f := source.NewFile(1, "trace.tr", []byte(traceSrc))
	one := []TraceFrame{frameAt(t, f, "main", "average(10, 0)")}
	if got := renderTrace(t, f, one); got != "" {
		t.Errorf("one frame rendered %q, want nothing", got)
	}
	if got := renderTrace(t, f, nil); got != "" {
		t.Errorf("no frames rendered %q", got)
	}
	two := []TraceFrame{frameAt(t, f, "divide", "a / b"), frameAt(t, f, "main", "average(10, 0)")}
	if got := renderTrace(t, nil, two); got != "" {
		t.Errorf("nil file rendered %q", got)
	}
}

func TestRenderTraceMergesIdenticalFrames(t *testing.T) {
	src := "fn deep(n: Int) -> Unit {\n    deep(n + 1)\n}\n\nfn main() -> Unit {\n    deep(1)\n}\n"
	f := source.NewFile(1, "deep.tr", []byte(src))
	frames := make([]TraceFrame, 0, 10000)
	for range 9999 {
		frames = append(frames, frameAt(t, f, "deep", "deep(n + 1)"))
	}
	frames = append(frames, frameAt(t, f, "main", "deep(1)"))
	want := "stack trace:\n" +
		"  0-9998: deep  deep.tr:2:5  deep(n + 1)\n" +
		"  9999:   main  deep.tr:6:5  deep(1)\n"
	if got := renderTrace(t, f, frames); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

// numbered builds a source with functions f0..f(n-1), each line calling
// the next, and returns frames f0 (innermost) ... f(n-1), main.
func numbered(t *testing.T, n int) (*source.File, []TraceFrame) {
	t.Helper()
	var b strings.Builder
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "call_%03d()\n", i)
	}
	b.WriteString("main_call()\n")
	f := source.NewFile(1, "m.tr", []byte(b.String()))
	frames := make([]TraceFrame, 0, n+1)
	for i := 0; i < n; i++ {
		frames = append(frames, frameAt(t, f, fmt.Sprintf("f%d", i), fmt.Sprintf("call_%03d()", i)))
	}
	frames = append(frames, frameAt(t, f, "main", "main_call()"))
	return f, frames
}

func TestRenderTraceCapBoundary(t *testing.T) {
	// 20 distinct lines: printed in full, no gap line.
	f, frames := numbered(t, 19)
	got := renderTrace(t, f, frames)
	if lines := strings.Count(got, "\n"); lines != 21 || strings.Contains(got, "not shown") {
		t.Errorf("20 frames: %d lines, gap=%v:\n%s", lines, strings.Contains(got, "not shown"), got)
	}
	// 21 distinct lines: 10 + gap + 10.
	f, frames = numbered(t, 20)
	got = renderTrace(t, f, frames)
	lines := strings.Split(strings.TrimSuffix(got, "\n"), "\n")
	if len(lines) != 22 { // header + 10 + gap + 10
		t.Fatalf("21 frames: %d lines:\n%s", len(lines), got)
	}
	if lines[11] != "  ... 1 more frames not shown" {
		t.Errorf("gap line = %q", lines[11])
	}
	if !strings.HasPrefix(lines[10], "  9:  f9 ") || !strings.HasPrefix(lines[12], "  11: f11") ||
		!strings.HasPrefix(lines[21], "  20: main") {
		t.Errorf("kept the wrong frames:\n%s", got)
	}
}

func TestRenderTraceMutualRecursionIsCapped(t *testing.T) {
	src := "fn ping(n: Int) -> Int {\n    return pong(n)\n}\n\nfn pong(n: Int) -> Int {\n    return ping(n)\n}\n\nfn main() -> Unit {\n    print(ping(1))\n}\n"
	f := source.NewFile(1, "mutual.tr", []byte(src))
	var frames []TraceFrame
	for i := 0; i < 9998; i++ { // alternating frames never merge
		if i%2 == 0 {
			frames = append(frames, frameAt(t, f, "pong", "ping(n)"))
		} else {
			frames = append(frames, frameAt(t, f, "ping", "pong(n)"))
		}
	}
	frames = append(frames, frameAt(t, f, "main", "ping(1)"))
	got := renderTrace(t, f, frames)
	lines := strings.Split(strings.TrimSuffix(got, "\n"), "\n")
	if len(lines) != 22 {
		t.Fatalf("%d lines, want 22:\n%s", len(lines), got)
	}
	if want := "  ... 9979 more frames not shown"; lines[11] != want { // 9999 - 20
		t.Errorf("gap = %q, want %q", lines[11], want)
	}
	if !strings.HasPrefix(lines[21], "  9998: main") {
		t.Errorf("last line = %q", lines[21])
	}
}

func TestRenderTraceOmittedCountsMergedFrames(t *testing.T) {
	// 22 groups where hidden group 10 merges 5 frames: omitted counts
	// original frames (5 + 1), not lines (2).
	f, base := numbered(t, 21)
	var frames []TraceFrame
	for i, fr := range base {
		frames = append(frames, fr)
		if i == 10 {
			for range 4 {
				frames = append(frames, fr)
			}
		}
	}
	got := renderTrace(t, f, frames)
	if !strings.Contains(got, "  ... 6 more frames not shown\n") {
		t.Errorf("want 6 omitted frames:\n%s", got)
	}
}

func TestRenderTraceExcerptRules(t *testing.T) {
	long := strings.Repeat("é", 70) // 70 code points, 140 bytes
	src := "fn main() -> Unit {\n\t\tlet\tx = f()   \n" + "    g(\"" + long + "\")\n}\n"
	f := source.NewFile(1, "e.tr", []byte(src))
	got := renderTrace(t, f, []TraceFrame{frameAt(t, f, "f", "f()"), frameAt(t, f, "main", "g(")})
	lines := strings.Split(strings.TrimSuffix(got, "\n"), "\n")
	if !strings.HasSuffix(lines[1], "  let x = f()") {
		t.Errorf("tabs and surrounding space not normalised: %q", lines[1])
	}
	excerpt := lines[2][strings.Index(lines[2], "g(\""):]
	if n := len([]rune(excerpt)); n != 60 || !strings.HasSuffix(excerpt, "...") {
		t.Errorf("long excerpt has %d code points (want 60, ending in ...): %q", n, excerpt)
	}
	for i, l := range lines {
		if strings.HasSuffix(l, " ") {
			t.Errorf("line %d has trailing spaces: %q", i, l)
		}
	}
}

func TestRenderTraceCRLFAndWideFileName(t *testing.T) {
	src := "fn main() -> Unit {\r\n    print(f())\r\n}\r\n\r\nfn f() -> Int {\r\n    return 1 / 0\r\n}\r\n"
	f := source.NewFile(1, "計算.tr", []byte(src))
	got := renderTrace(t, f, []TraceFrame{frameAt(t, f, "f", "1 / 0"), frameAt(t, f, "main", "f()")})
	want := "stack trace:\n" +
		"  0: f     計算.tr:6:12  return 1 / 0\n" +
		"  1: main  計算.tr:2:11  print(f())\n"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestRenderTraceDeterministic(t *testing.T) {
	f, frames := numbered(t, 40)
	first := renderTrace(t, f, frames)
	for range 5 {
		if renderTrace(t, f, frames) != first {
			t.Fatal("output differs between runs")
		}
	}
}

func TestRenderTraceEmptyExcerptHasNoTrailingSpaces(t *testing.T) {
	// A frame on a whitespace-only line has an empty excerpt; the padded
	// columns before it must not leave trailing spaces.
	src := "fn main() -> Unit {\n    f()\n        \n}\n"
	f := source.NewFile(1, "w.tr", []byte(src))
	blank := strings.Index(src, "        \n")
	frames := []TraceFrame{
		{Function: "f", Span: f.Span(blank, blank+1)},
		frameAt(t, f, "main", "f()"),
	}
	want := "stack trace:\n" +
		"  0: f     w.tr:3:1\n" +
		"  1: main  w.tr:2:5  f()\n"
	if got := renderTrace(t, f, frames); got != want {
		t.Errorf("got:\n%q\nwant:\n%q", got, want)
	}
}

func TestRenderTraceInvalidUTF8Excerpt(t *testing.T) {
	src := "fn main() -> Unit {\n    f()\n}\n\nfn f() -> Unit {\n    x\xff()\n}\n"
	f := source.NewFile(1, "u.tr", []byte(src))
	got := renderTrace(t, f, []TraceFrame{frameAt(t, f, "f", "x\xff()"), frameAt(t, f, "main", "f()")})
	if !strings.Contains(got, "  x�()\n") {
		t.Errorf("invalid UTF-8 not replaced with U+FFFD:\n%q", got)
	}
}

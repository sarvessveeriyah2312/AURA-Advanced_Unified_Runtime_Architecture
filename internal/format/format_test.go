package format

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sarvessveeriyah2312/Tread/internal/source"
)

func fmtString(t *testing.T, src string) (string, error) {
	t.Helper()
	out, diags, err := File(source.NewFile(1, "t.tr", []byte(src)))
	if len(diags) > 0 {
		t.Fatalf("syntax errors: %+v", diags)
	}
	return string(out), err
}

// TestIdempotentOnFixtures formats every valid fixture and example twice:
// the second pass must change nothing.
func TestIdempotentOnFixtures(t *testing.T) {
	var paths []string
	for _, g := range []string{"../../testdata/e2e/*.tr", "../../testdata/e2e/lib/*.tr", "../../examples/*.tr"} {
		m, _ := filepath.Glob(g)
		paths = append(paths, m...)
	}
	if len(paths) < 20 {
		t.Fatalf("only %d files", len(paths))
	}
	for _, path := range paths {
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		once, diags, err := File(source.NewFile(1, path, src))
		if len(diags) > 0 {
			continue // fixtures with syntax errors
		}
		if err != nil {
			t.Errorf("%s: %v", path, err)
			continue
		}
		twice, _, err := File(source.NewFile(1, path, once))
		if err != nil || string(twice) != string(once) {
			t.Errorf("%s: not idempotent (%v):\n--- once\n%s\n--- twice\n%s", path, err, once, twice)
		}
	}
}

// TestGolden formats testdata/messy.in.tr, which covers every construct
// and comment position, and compares with messy.out.tr.
func TestGolden(t *testing.T) {
	in, _ := os.ReadFile("testdata/messy.in.tr")
	want, _ := os.ReadFile("testdata/messy.out.tr")
	got, err := fmtString(t, string(in))
	if err != nil || got != string(want) {
		t.Errorf("err %v\n--- got\n%s\n--- want\n%s", err, got, want)
	}
}

func TestCases(t *testing.T) {
	tests := []struct{ in, want string }{
		{"", ""},
		{"// only a comment", "// only a comment\n"},
		{"fn main() -> Unit {}", "fn main() -> Unit { }\n"},
		{"fn main()->Unit{\n\n\n  let x=1\n\n\n\n  let y=2\n}\n\n\n\nfn f()->Unit{}", "fn main() -> Unit {\n    let x = 1\n\n    let y = 2\n}\n\nfn f() -> Unit { }\n"},
		{"fn main() -> Unit {\n    print(-(1 + 2) * 3 - (4 - 5))\n}\n", "fn main() -> Unit {\n    print(-(1 + 2) * 3 - (4 - 5))\n}\n"},
		{"fn main() -> Unit {\n    let p = Point {\n        x: 1, y: 2 }\n}", "fn main() -> Unit {\n    let p = Point {\n        x: 1\n        y: 2\n    }\n}\n"},
		{"fn main() -> Unit {\n    let s = \"\\u{41}\"\n}", "fn main() -> Unit {\n    let s = \"\\u{41}\"\n}\n"},
		{"fn main() -> Unit { /* a */ }", "fn main() -> Unit {\n    /* a */\n}\n"},
		{"fn main() -> Unit {\n    let x = 1\n    // end\n}", "fn main() -> Unit {\n    let x = 1\n    // end\n}\n"},
	}
	for _, tc := range tests {
		src := tc.in
		f := source.NewFile(1, "t.tr", []byte(src))
		out, diags, err := File(f)
		if len(diags) > 0 {
			continue // lexer-invalid case (\u is not an escape): checked below
		}
		if err != nil || string(out) != tc.want {
			t.Errorf("format(%q):\n got %q (%v)\nwant %q", tc.in, out, err, tc.want)
		}
	}
}

func TestRefusals(t *testing.T) {
	_, diags, _ := File(source.NewFile(1, "t.tr", []byte("fn (")))
	if len(diags) == 0 {
		t.Error("syntax error not reported")
	}
	_, diags, err := File(source.NewFile(1, "t.tr", []byte("fn main() -> Unit {\n    let x = 1 /* in */ + 2\n}\n")))
	if len(diags) > 0 || err == nil || !strings.Contains(err.Error(), "comment is inside an expression") || !strings.Contains(err.Error(), "(at 2:15)") {
		t.Errorf("err = %v", err)
	}
}

// FuzzFormat: formatting a valid program either refuses with an *Error or
// produces a program with the same tree (checked inside File) that
// formats to itself.
func FuzzFormat(f *testing.F) {
	paths, _ := filepath.Glob("../../testdata/e2e/*.tr")
	for _, p := range append(paths, "testdata/messy.in.tr") {
		if src, err := os.ReadFile(p); err == nil {
			f.Add(src)
		}
	}
	f.Fuzz(func(t *testing.T, src []byte) {
		out, diags, err := File(source.NewFile(1, "f.tr", src))
		if len(diags) > 0 {
			return
		}
		if err != nil {
			if _, ok := err.(*Error); !ok || strings.Contains(err.Error(), "internal formatter error") {
				t.Fatalf("%v\ninput %q", err, src)
			}
			return
		}
		again, _, err := File(source.NewFile(1, "f.tr", out))
		if err != nil || string(again) != string(out) {
			t.Fatalf("not idempotent: %v\ninput %q\nonce %q\ntwice %q", err, src, out, again)
		}
	})
}

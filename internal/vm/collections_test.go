package vm_test

import (
	"strings"
	"testing"
	"time"

	"github.com/sarvessveeriyah2312/Tread/internal/vm"
)

// Collections, enums, pattern matching and generics (ADR-0013).

func TestV26ValueSemantics(t *testing.T) {
	cases := []struct{ name, decls, body, want string }{
		{"assignment copies", "", "let mut a = [1, 2]\nlet b = a\na[0] = 9\nprint(a)\nprint(b)", "[9, 2]\n[1, 2]\n"},
		{"copy then change the copy", "", "let a = [1, 2]\nlet mut b = a\nb.push(3)\nprint(a)\nprint(b)", "[1, 2]\n[1, 2, 3]\n"},
		{"nested copies are independent", "", "let mut g = [[1], [2]]\nlet row = g[0]\ng[0].push(5)\ng[1][0] = 7\nprint(row)\nprint(g)", "[1]\n[[1, 5], [7]]\n"},
		{"argument is a copy", "fn f(xs: Array<Int>) -> Array<Int> {\n    let mut ys = xs\n    ys.push(9)\n    return ys\n}\n",
			"let a = [1]\nlet b = f(a)\nprint(a)\nprint(b)", "[1]\n[1, 9]\n"},
		{"a returned array is independent", "fn make() -> Array<Int> {\n    let xs = [1]\n    return xs\n}\n",
			"let mut a = make()\nlet b = a\na.push(2)\nprint(b)\nprint(a)", "[1]\n[1, 2]\n"},
		{"maps copy too", "", "let mut m = [\"a\": 1]\nlet n = m\nm[\"a\"] = 2\nm[\"b\"] = 3\nprint(n)\nprint(m)", "[\"a\": 1]\n[\"a\": 2, \"b\": 3]\n"},
		{"array inside a map", "", "let mut m = [\"xs\": [1]]\nlet before = m\nm[\"xs\"].push(2)\nm[\"xs\"][0] = 0\nprint(before)\nprint(m)", "[\"xs\": [1]]\n[\"xs\": [0, 2]]\n"},
		{"enum payload is a snapshot", "", "let mut xs = [1]\nlet o = Option.Some(xs)\nxs.push(2)\nprint(o)\nprint(xs)", "Option.Some([1])\n[1, 2]\n"},
		{"for-in iterates a snapshot", "", "let mut xs = [1, 2]\nfor x in xs {\n    xs.push(x * 10)\n}\nprint(xs)", "[1, 2, 10, 20]\n"},
		{"match binding is a snapshot", "", "let mut xs = [1]\nmatch xs {\n    all => {\n        xs.push(2)\n        print(all)\n    }\n}\nprint(xs)", "[1]\n[1, 2]\n"},
		{"receiver read before arguments change it", "", "let mut xs = [1, 2]\nprint(xs.contains(xs.len()))\nlet mut ys = [1, 2]\nprint(ys.contains(len2(ys)))", "true\ntrue\n"},
		{"argument evaluated before a mutation in a later argument", "fn pair(a: Array<Int>, b: Option<Int>) -> Unit {\n    print(a)\n    print(b)\n}\n",
			"let mut xs = [1, 2]\npair(xs, xs.pop())\nprint(xs)", "[1, 2]\nOption.Some(2)\n[1]\n"},
		{"element read before the row changes", "fn pair(a: Array<Int>, b: Option<Int>) -> Unit {\n    print(a)\n    print(b)\n}\n",
			"let mut g = [[1, 2]]\npair(g[0], g[0].pop())\nprint(g)", "[1, 2]\nOption.Some(2)\n[[1]]\n"},
	}
	helper := "fn len2(xs: Array<Int>) -> Int {\n    return xs.len()\n}\n"
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := runSrc(t, helper+tc.decls+mainOf(tc.body), vm.Config{})
			if err != nil || out != tc.want {
				t.Errorf("got %q %+v\nwant %q", out, err, tc.want)
			}
		})
	}
}

func TestV27CollectionsAndDisplay(t *testing.T) {
	cases := []struct{ name, decls, body, want string }{
		{"array methods", "", "let mut xs = [3]\nxs.push(4)\nprint(xs.len())\nprint(xs.contains(4))\nprint(xs.pop())\nprint(xs.pop())\nprint(xs.pop())\nprint(xs)", "2\ntrue\nOption.Some(4)\nOption.Some(3)\nOption.None\n[]\n"},
		{"map methods keep insertion order", "", "let mut m = [\"b\": 2, \"a\": 1]\nm[\"c\"] = 3\nm[\"b\"] = 20\nprint(m.keys())\nprint(m.get(\"b\"))\nprint(m.get(\"z\"))\nprint(m.remove(\"a\"))\nprint(m.remove(\"a\"))\nprint(m.len())\nprint(m.contains(\"c\"))\nprint(m)",
			"[\"b\", \"a\", \"c\"]\nOption.Some(20)\nOption.None\nOption.Some(1)\nOption.None\n2\ntrue\n[\"b\": 20, \"c\": 3]\n"},
		{"for over map keys", "", "let m = [1: \"one\", 2: \"two\"]\nfor k in m {\n    print(\"${k}=${m[k]}\")\n}", "1=one\n2=two\n"},
		{"display", "enum Shape {\n    Circle(Float)\n    Empty\n}\n",
			"print([\"a\", \"b \\\"q\\\" ${1}\", \"\\${x}\"])\nprint([1.0, 2.5])\nprint([true])\nlet e: Map<Int, Int> = [:]\nprint(e)\nprint(Shape.Circle(2.0))\nprint([Shape.Empty])\nprint(\"${[1]} and ${Option.Some(\"s\")}\")",
			"[\"a\", \"b \\\"q\\\" 1\", \"\\${x}\"]\n[1.0, 2.5]\n[true]\n[:]\nShape.Circle(2.0)\n[Shape.Empty]\n[1] and Option.Some(\"s\")\n"},
		{"structural equality", "", "print([1, 2] == [1, 2])\nprint([\"a\": 1, \"b\": 2] == [\"b\": 2, \"a\": 1])\nprint(Option.Some([1]) == Option.Some([1]))\nprint(Option.Some(1) == Option.None)\nprint([0.0 / 0.0] == [0.0 / 0.0])",
			"true\ntrue\ntrue\nfalse\nfalse\n"},
		{"match and generics", "enum Tree<T> {\n    Leaf\n    Node(Tree<T>, T, Tree<T>)\n}\nfn size<T>(t: Tree<T>) -> Int {\n    match t {\n        Tree.Leaf => { return 0 }\n        Tree.Node(l, _, r) => { return size(l) + 1 + size(r) }\n    }\n}\n",
			"let t = Tree.Node(Tree.Leaf, \"x\", Tree.Node(Tree.Leaf, \"y\", Tree.Leaf))\nprint(size(t))\nmatch Option.Some(Option.Some(3)) {\n    Option.Some(Option.Some(3)) => { print(\"three\") }\n    Option.Some(_) => { print(\"other\") }\n    Option.None => { print(\"none\") }\n}",
			"2\nthree\n"},
		{"literal patterns", "fn name(n: Int) -> String {\n    match n {\n        0 => { return \"zero\" }\n        -1 => { return \"minus one\" }\n        _ => { return \"many\" }\n    }\n}\n",
			"print(name(0))\nprint(name(-1))\nprint(name(5))\nmatch \"b\" {\n    \"a\" => { print(1) }\n    \"b\" => { print(2) }\n    _ => { print(3) }\n}", "zero\nminus one\nmany\n2\n"},
		{"break and continue inside match", "", "for i in 0..5 {\n    match i % 2 {\n        0 => { continue }\n        _ => { }\n    }\n    if i > 2 {\n        break\n    }\n    print(i)\n}", "1\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := runSrc(t, tc.decls+mainOf(tc.body), vm.Config{})
			if err != nil || out != tc.want {
				t.Errorf("got %q %+v\nwant %q", out, err, tc.want)
			}
		})
	}
}

func TestV28CollectionRuntimeErrors(t *testing.T) {
	cases := []struct{ body, code, msg, span string }{
		{"let xs = [1, 2]\nprint(xs[2])", vm.CodeIndexOutOfRange, "index 2 is out of range for an array of length 2", "xs[2]"},
		{"let xs = [1]\nlet i = -1\nprint(xs[i])", vm.CodeIndexOutOfRange, "index -1 is out of range for an array of length 1", "xs[i]"},
		{"let mut xs = [1]\nxs[5] = 1", vm.CodeIndexOutOfRange, "index 5 is out of range", "xs[5] = 1"},
		{"let mut g = [[1]]\ng[3].push(1)", vm.CodeIndexOutOfRange, "index 3 is out of range", "g[3].push(1)"},
		{"let m = [\"a\": 1]\nprint(m[\"zed\"])", vm.CodeKeyNotFound, `key "zed" is not in the map`, `m["zed"]`},
		{"let mut m = [1: [1]]\nm[2][0] = 5", vm.CodeKeyNotFound, "key 2 is not in the map", "m[2][0] = 5"},
		{"print([[1]][0][1])", vm.CodeIndexOutOfRange, "index 1 is out of range", "[[1]][0][1]"},
	}
	for _, tc := range cases {
		src := mainOf(tc.body)
		_, err := runSrc(t, src, vm.Config{})
		if err == nil || err.Code != tc.code || !strings.Contains(err.Message, tc.msg) || spanText(src, err) != tc.span {
			t.Errorf("%q: got %+v (span %q)", tc.body, err, spanText(src, err))
		}
	}
}

// Copy-on-write keeps repeated changes to one variable in place: these
// would take minutes if every change copied the array.
func TestV29ChangesAreInPlace(t *testing.T) {
	start := time.Now()
	src := mainOf("let mut xs: Array<Int> = []\nfor i in 0..200000 {\n    xs.push(i)\n}\nfor i in 1..200000 {\n    xs[i] = xs[i - 1] + xs[i]\n}\nprint(xs.len())\nprint(xs[199999])\n" +
		"let mut grid: Array<Array<Int>> = []\nfor r in 0..300 {\n    let mut row: Array<Int> = []\n    for c in 0..300 {\n        row.push(r * c)\n    }\n    grid.push(row)\n}\nfor r in 0..300 {\n    for c in 0..300 {\n        grid[r][c] = grid[r][c] + 1\n    }\n}\nprint(grid[299][299])\n" +
		"let mut m: Map<Int, Int> = [:]\nfor i in 0..50000 {\n    m[i] = i\n}\nprint(m.len())")
	out, err := runSrc(t, src, vm.Config{})
	if err != nil || out != "200000\n19999900000\n89402\n50000\n" {
		t.Fatalf("got %q %+v", out, err)
	}
	if d := time.Since(start); d > 20*time.Second {
		t.Errorf("took %v; changes are being copied", d)
	}
}

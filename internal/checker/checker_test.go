package checker

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"testing"

	"github.com/sarvessveeriyah2312/Tread/internal/ast"
	"github.com/sarvessveeriyah2312/Tread/internal/diagnostic"
	"github.com/sarvessveeriyah2312/Tread/internal/lexer"
	"github.com/sarvessveeriyah2312/Tread/internal/parser"
	"github.com/sarvessveeriyah2312/Tread/internal/source"
)

// prelude is available to every non-whole-program case: `u` is a Unit
// function (docs/10 §10.5.1 notation).
const prelude = "fn u() -> Unit { }\n"

type checkCase struct {
	id string
	// decls are extra top-level declarations; body is the body of
	// `fn main() -> Unit`. If whole is true, src is the entire program.
	decls string
	body  string
	whole bool
	src   string
	// want lists diagnostics as `T001 "text"`, where text is the source
	// covered by the primary span, in reported order.
	want []string
	// types maps a let-bound name in main to the expected type of its
	// initializer.
	types map[string]string
	// lexOK allows lexer diagnostics (K73).
	lexOK bool
	// files are other modules of the program, by import path (ADR-0014).
	// A diagnostic in one of them is written `T034 "text"@path`.
	files map[string]string
}

func (tc checkCase) source() string {
	if tc.whole {
		return tc.src
	}
	if tc.files != nil {
		return tc.decls + prelude + "fn main() -> Unit {\n" + tc.body + "\n}\n" // imports first
	}
	return prelude + tc.decls + "fn main() -> Unit {\n" + tc.body + "\n}\n"
}

var checkCases = []checkCase{
	// Integer literals (ADR-0004 item 1).
	{id: "K1", body: "let a = 9223372036854775807", types: map[string]string{"a": "Int"}},
	{id: "K2", body: "let a = 9223372036854775808", want: []string{`T010 "9223372036854775808"`}},
	{id: "K3", body: "let a = -9223372036854775808", types: map[string]string{"a": "Int"}},
	{id: "K4a", body: "let a = - 9223372036854775808", types: map[string]string{"a": "Int"}},
	{id: "K4b", body: "let a = -\n9223372036854775808", types: map[string]string{"a": "Int"}},
	{id: "K5", body: "let a = -9223372036854775809", want: []string{`T010 "9223372036854775809"`}},
	{id: "K6", body: "let a = -(9223372036854775808)", want: []string{`T010 "9223372036854775808"`}},
	{id: "K7", body: "let a = 1 - 9223372036854775808", want: []string{`T010 "9223372036854775808"`}},
	{id: "K8", body: "let a = 99999999999999999999999", want: []string{`T010 "99999999999999999999999"`}},
	{id: "K9", body: "let a = 0\nlet b = -0", types: map[string]string{"a": "Int", "b": "Int"}},

	// Float literals (ADR-0004 item 14).
	{id: "K10", body: "let f: Float = 0.5\nlet g: Int = 0.5\nlet h: Float = 1",
		want: []string{`T001 "0.5"`, `T001 "1"`}},
	{id: "K37", body: "let a = 1" + strings.Repeat("0", 309) + ".0", want: []string{`T017 "1` + strings.Repeat("0", 309) + `.0"`}},
	{id: "K38", body: "let a = 1" + strings.Repeat("0", 308) + ".0", types: map[string]string{"a": "Float"}},
	{id: "K39", body: "let a = 0." + strings.Repeat("0", 323) + "5", types: map[string]string{"a": "Float"}},
	{id: "K40", body: "let a = 0." + strings.Repeat("0", 400) + "1", want: []string{`T017 "0.` + strings.Repeat("0", 400) + `1"`}},
	{id: "K41", body: "let a = 0.0\nlet b = 0.000\nlet c = 0.1", types: map[string]string{"a": "Float", "b": "Float", "c": "Float"}},

	// Namespaces and built-ins (ADR-0004 items 3, 9).
	{id: "K11", body: "let Int = 5\nlet x: Int = Int", types: map[string]string{"x": "Int"}},
	{id: "K12", body: "let String = 1\nlet s: String = \"a\"", types: map[string]string{"s": "String"}},
	{id: "K13", body: "let Bool = \"x\"\nlet c: Bool = Bool", want: []string{`T001 "Bool"`}},
	{id: "K14", decls: "fn Float() -> Int { return 1 }\n", body: "let f: Float = 1.5\nlet g: Int = Float()",
		types: map[string]string{"f": "Float", "g": "Int"}},
	{id: "K15", decls: "fn f(Unit: Int) -> Unit { }\n"},
	{id: "K16", body: "let x: Foo = 1", want: []string{`T003 "Foo"`}},
	{id: "K17", body: "let y = Int", want: []string{`T002 "Int"`}},
	{id: "K18", body: "let v = 1\nlet w: v = 2", want: []string{`T003 "v"`}},
	{id: "K19", body: "let Int = 5\nlet Int = 6", want: []string{`T005 "Int"`}},
	{id: "K56", decls: "fn print(x: Int) -> Unit { }\n", want: []string{`T016 "print"`}},
	{id: "K57a", body: "let print = 1", want: []string{`T016 "print"`}},
	{id: "K57b", decls: "fn f(print: Int) -> Unit { }\n", want: []string{`T016 "print"`}},
	{id: "K58", body: "let p = print\nlet g = u", want: []string{`T018 "print"`, `T018 "u"`}},

	// print, interpolation, and Unit (ADR-0004 item 7).
	{id: "K20", body: "print(1)\nprint(1.5)\nprint(true)\nprint(\"s\")"},
	{id: "K21", body: "print(u())", want: []string{`T011 "u()"`}},
	{id: "K22", body: "print(print(1))", want: []string{`T011 "print(1)"`}},
	{id: "K23", body: "let s = \"${u()}\"", want: []string{`T011 "u()"`}},
	{id: "K24", body: "print()\nprint(1, 2)", want: []string{`T006 "print()"`, `T006 "print(1, 2)"`}},
	{id: "K25", decls: "fn f(x: Int) -> Int { return x }\n",
		body: "let a = 1\nlet b = 2\nlet s = \"${1} ${1.5} ${true} ${\"s\"} ${a + f(b)}\"", types: map[string]string{"s": "String"}},
	{id: "K74", body: "let s: String = \"${1}\"\nlet n: Int = \"${1}\"", want: []string{`T001 "\"${1}\""`}},

	// Conditionals (ADR-0004 item 5).
	{id: "K26", body: "let x = 1\nif 1 { } else if x { }", want: []string{`T001 "1"`, `T001 "x"`}},
	{id: "K27", body: "let a = true\nlet b = false\nif a { } else if b { let z = 1 }\nprint(z)", want: []string{`T002 "z"`}},
	{id: "K28", decls: "fn f(a: Bool, b: Bool) -> Int { if a { return 1 } else if b { return 2 } else { return 3 } }\n"},
	{id: "K29", decls: "fn f(a: Bool, b: Bool) -> Int { if a { return 1 } else if b { return 2 } }\n", want: []string{`T008 "f"`}},

	// Operators (ADR-0004 item 6).
	{id: "K31", body: "let a = -true\nlet b = -\"s\"\nlet c = -1\nlet d = -1.5",
		want: []string{`T007 "-true"`, `T007 "-\"s\""`}, types: map[string]string{"c": "Int", "d": "Float"}},
	{id: "K42", body: "let a = 1 + 2\nlet b = 1.5 + 2.5\nlet c = \"a\" + \"b\"\nlet d = 5 - 3\nlet e = 2.0 * 3.0\nlet f = 7 / 2\nlet g = 7.0 / 2.0\nlet h = 7 % 2",
		types: map[string]string{"a": "Int", "b": "Float", "c": "String", "d": "Int", "e": "Float", "f": "Int", "g": "Float", "h": "Int"}},
	{id: "K43", body: "let a = 1 + 1.5\nlet b = \"a\" + 1\nlet c = \"a\" - \"b\"\nlet d = true + true\nlet e = 7.5 % 2.0\nlet f = \"a\" * 2",
		want: []string{`T007 "1 + 1.5"`, `T007 "\"a\" + 1"`, `T007 "\"a\" - \"b\""`, `T007 "true + true"`, `T007 "7.5 % 2.0"`, `T007 "\"a\" * 2"`}},
	{id: "K44", body: "let a = 1 == 1\nlet b = 1.5 != 2.5\nlet c = \"a\" == \"b\"\nlet d = true == false\nlet e = 1 == 1.5\nlet f = u() == u()",
		want: []string{`T007 "1 == 1.5"`, `T007 "u() == u()"`}, types: map[string]string{"a": "Bool", "b": "Bool", "c": "Bool", "d": "Bool"}},
	{id: "K45", body: "let a = 1 < 2\nlet b = 1.5 >= 2.5\nlet c = \"a\" < \"b\"\nlet d = true < false\nlet e = 1 < 2.0",
		want: []string{`T007 "\"a\" < \"b\""`, `T007 "true < false"`, `T007 "1 < 2.0"`}, types: map[string]string{"a": "Bool", "b": "Bool"}},
	{id: "K46", body: "let a = true && false\nlet b = !true\nlet c = 1 && true\nlet d = !1",
		want: []string{`T007 "1 && true"`, `T007 "!1"`}, types: map[string]string{"a": "Bool", "b": "Bool"}},
	{id: "K47", body: "let a = u() + 1", want: []string{`T007 "u() + 1"`}},
	{id: "K48", body: "let mut f: Float = 1.0\nf = 1", want: []string{`T001 "1"`}},

	// main and function signatures (ADR-0004 item 8).
	{id: "K49", whole: true, src: "fn f() -> Unit { }\n", want: []string{`T013 ""@0`}},
	{id: "K50", whole: true, src: "fn main() { }\n", want: []string{`T014 "main"`}},
	{id: "K51", whole: true, src: "fn main(a: Int) -> Unit { }\n", want: []string{`T014 "a: Int"`}},
	{id: "K52", whole: true, src: "fn main() -> Int { return 0 }\n", want: []string{`T014 "Int"`}},
	{id: "K53", whole: true, src: "fn main() -> Unit { }\nfn main() -> Unit { }\n", want: []string{`T005 "main"@25`}},
	{id: "K54", whole: true, src: "fn main() -> Unit { }\n"},
	{id: "K55", decls: "fn f() { }\n", want: []string{`T015 "f"`}},

	// Scopes and shadowing (docs/02 §2.13).
	{id: "K59", body: "let x = 1\nif true { let x = \"s\"\nprint(x) }\nprint(x)"},
	{id: "K60", body: "if true { let y = 1\nlet y = 2 }", want: []string{`T005 "y"@63`}},
	{id: "K61", body: "print(y)\nlet y = 1", want: []string{`T002 "y"`}},
	{id: "K62", body: "let x = x", want: []string{`T002 "x"`}},
	{id: "K63", decls: "fn f(a: Int) -> Int { let a = 2\nreturn a }\n"},
	{id: "K64", decls: "fn f(a: Int, a: Int) -> Unit { }\n", want: []string{`T005 "a"`}},
	{id: "K65", decls: "fn g() -> Int { return 1 }\n", body: "let g = 2\ng()", want: []string{`T012 "g"`}},

	// Functions, calls, returns, and assignment.
	{id: "K30", decls: "fn f() -> Int { return 1 }\n", body: "f()()\nlet x = 1\nx()", want: []string{`T012 "f()"`, `T012 "x"`}},
	{id: "K32", body: "let x = 1\nx = 2", want: []string{`T004 "x = 2"`}},
	{id: "K33", body: "let x = 1\nlet x = 2", want: []string{`T005 "x"`}},
	{id: "K34", decls: "fn g(a: Int) -> Unit { }\n", body: "g()", want: []string{`T006 "g()"`}},
	{id: "K35", decls: "fn h() -> Unit { return 1 }\nfn k() -> Int { return }\n", want: []string{`T009 "return 1"`, `T009 "return"`}},
	{id: "K36", body: "print(zz)", want: []string{`T002 "zz"`}},
	{id: "K66", whole: true, src: "fn main() -> Unit {\n    later()\n}\nfn later() -> Unit { }\n"},
	{id: "K67", decls: "fn f(a: Int, b: String) -> Unit { }\n", body: "f(\"x\", 1)", want: []string{`T001 "\"x\""`, `T001 "1"`}},
	{id: "K68", decls: "fn f(a: Foo) -> Bar { }\n", want: []string{`T003 "Foo"`, `T003 "Bar"`}},
	{id: "K69", decls: "fn f() -> Int { return \"s\" }\n", want: []string{`T001 "\"s\""`}},
	{id: "K70a", body: "let mut c = 0\nc = c + 1"},
	{id: "K70b", body: "let mut c = 0\nc = \"s\"", want: []string{`T001 "\"s\""`}},
	{id: "K70c", decls: "fn f(a: Int) -> Unit { a = 1 }\n", want: []string{`T004 "a = 1"`}},
	{id: "K70d", body: "u = 1", want: []string{`T004 "u = 1"`}},
	{id: "K70e", body: "z = 1", want: []string{`T002 "z"`}},

	// Recovery.
	{id: "K71", body: "let a: Int = \"s\"\nlet b = undefinedName\nprint(u())",
		want: []string{`T001 "\"s\""`, `T002 "undefinedName"`, `T011 "u()"`}},
	{id: "K72", body: "let a = undefinedName\nlet b = a + 1\nprint(a)", want: []string{`T002 "undefinedName"`}},
	{id: "K73", body: "let x = 9abc + 1", lexOK: true},

	// toString (ADR-0004 item 13).
	{id: "K75", body: "let a = toString(1)\nlet b = toString(1.5)\nlet c = toString(true)\nlet d = toString(\"s\")",
		types: map[string]string{"a": "String", "b": "String", "c": "String", "d": "String"}},
	{id: "K76", body: "let a = toString(u())", want: []string{`T011 "u()"`}},
	{id: "K77", body: "let a = toString()\nlet b = toString(1, 2)", want: []string{`T006 "toString()"`, `T006 "toString(1, 2)"`}},
	{id: "K78", decls: "fn toString(x: Int) -> String { return \"\" }\n", body: "let t = toString",
		want: []string{`T016 "toString"`, `T018 "toString"`}},

	// Loops (ADR-0012).
	{id: "K79", body: "let mut i = 0\nwhile i < 3 {\ni = i + 1\n}"},
	{id: "K80", body: "while 1 { }\nwhile u() { }", want: []string{`T001 "1"`, `T001 "u()"`}},
	{id: "K81", body: "for i in 0..10 {\nlet sq: Int = i * i\nprint(sq)\n}"},
	{id: "K82", body: "for i in 0.5..\"s\" { }", want: []string{`T001 "0.5"`, `T001 "\"s\""`}},
	{id: "K83", body: "for i in 0..3 {\ni = 1\n}", want: []string{`T004 "i = 1"`}},
	{id: "K84", body: "for i in 0..3 { }\nprint(i)", want: []string{`T002 "i"`}},
	{id: "K85", body: "let n = 3\nfor n in 0..n {\nlet n = \"shadow\"\nprint(n)\n}"},
	{id: "K86", body: "for print in 0..3 { }", want: []string{`T016 "print"`}},
	{id: "K87", body: "break\ncontinue\nif true {\nbreak\n}", want: []string{`T019 "break"`, `T019 "continue"`, `T019 "break"`}},
	{id: "K88", body: "while true {\nbreak\n}\nfor i in 0..1 {\nif i == 0 {\ncontinue\n}\nwhile false { break }\n}"},
	{id: "K89", decls: "fn g() -> Unit {\nwhile true { }\n}\n", body: "g()\nbreak", want: []string{`T019 "break"`}},
	// Return paths (docs/12 §12.7): only `while true` without its own
	// `break` counts as never completing.
	{id: "K90", decls: "fn f() -> Int {\nwhile true {\nif u2() { return 1 }\n}\n}\nfn u2() -> Bool { return true }\n", body: "print(f())"},
	{id: "K91", decls: "fn f() -> Int {\nwhile (true) { }\n}\n", body: "print(f())"},
	{id: "K92", decls: "fn f() -> Int {\nwhile true {\nbreak\n}\n}\n", body: "print(f())", want: []string{`T008 "f"`}},
	{id: "K93", decls: "fn f() -> Int {\nwhile true {\nif true { break }\n}\n}\n", body: "print(f())", want: []string{`T008 "f"`}},
	{id: "K94", decls: "fn f() -> Int {\nwhile true {\nwhile true { break }\nfor i in 0..1 { break }\n}\n}\n", body: "print(f())"},
	{id: "K95", decls: "fn f(b: Bool) -> Int {\nwhile b { return 1 }\n}\nfn g() -> Int {\nfor i in 0..1 { return i }\n}\n", body: "print(f(true) + g())",
		want: []string{`T008 "f"`, `T008 "g"`}},
	{id: "K96", decls: "fn f() -> Int {\nwhile !false { }\n}\n", body: "print(f())", want: []string{`T008 "f"`}},

	// Arrays (ADR-0013).
	{id: "K97", body: "let a = [1, 2]\nlet b: Array<String> = []\nlet c = [[1], []]\nlet d = a[0]\nlet e = c[1]",
		types: map[string]string{"a": "Array<Int>", "b": "Array<String>", "c": "Array<Array<Int>>", "d": "Int", "e": "Array<Int>"}},
	{id: "K98", body: "let a = []\nlet b = [[], [1]]\nlet c = [[], []]", types: map[string]string{"b": "Array<Array<Int>>"}, want: []string{`T020 "[]"`, `T020 "[]"`}},
	{id: "K99", body: "let a = [1, \"x\"]\nlet b: Array<Int> = [true]", want: []string{`T001 "\"x\""`, `T001 "true"`}},
	{id: "K100", body: "let a = [1]\nlet b = a[\"0\"]\nlet c = 5[0]", want: []string{`T001 "\"0\""`, `T023 "5"`}},
	{id: "K101", body: "let a = [1]\na[0] = 2\na.push(3)\nlet mut b = [1]\nb[0] = \"x\"",
		want: []string{`T004 "a[0] = 2"`, `T004 "a.push(3)"`, `T001 "\"x\""`}},
	{id: "K102", decls: "fn f(xs: Array<Int>) -> Unit {\nxs.push(1)\n}\n", body: "let mut g = [[1]]\ng[0][0] = 2\ng[0].push(3)\nf(g[0])\nfor x in g[0] {\nprint(x)\n}",
		want: []string{`T004 "xs.push(1)"`}},
	{id: "K103", body: "let mut a = [1]\nlet n: Int = a.len()\nlet b: Bool = a.contains(1)\nlet c: Option<Int> = a.pop()\na.push(\"s\")\na.size()\nlet d = a.len",
		want: []string{`T001 "\"s\""`, `T024 "size"`, `T024 "a.len"`}},
	{id: "K104", body: "let a = [u()]\nlet b: Array<Unit> = []", want: []string{`T030 "u()"`, `T030 "Unit"`}},
	{id: "K105", body: "for x in 5 {\n}", want: []string{`T023 "5"`}},
	{id: "K106", body: "let a = [1] == [1]\nlet b = [1] == [\"a\"]\nprint([1, 2])\nlet s = \"${[1]}\"", types: map[string]string{"a": "Bool"},
		want: []string{`T001 "\"a\""`}}, // the right side is checked against the left's type

	// Maps.
	{id: "K107", body: "let m = [\"a\": 1]\nlet n: Map<Int, String> = [:]\nlet v = m[\"a\"]\nlet k = m.keys()\nlet g = m.get(\"a\")",
		types: map[string]string{"m": "Map<String, Int>", "n": "Map<Int, String>", "v": "Int", "k": "Array<String>", "g": "Option<Int>"}},
	{id: "K108", body: "let m = [:]\nlet f: Map<Float, Int> = [:]\nlet g = [1.5: 1]", want: []string{`T020 "[:]"`, `T022 "Float"`, `T022 "1.5"`}},
	{id: "K109", body: "let mut m = [\"a\": 1]\nm[\"b\"] = 2\nm[1] = 2\nlet r: Option<Int> = m.remove(\"a\")\nlet x = [\"a\": 1]\nx.remove(\"a\")",
		want: []string{`T001 "1"`, `T004 "x.remove(\"a\")"`}},
	{id: "K110", body: "let m = [\"a\": [1]]\nfor k in m {\nlet s: String = k\n}", types: map[string]string{"m": "Map<String, Array<Int>>"}},

	// Enums and variants.
	{id: "K111", decls: "enum Shape {\nCircle(Float)\nRect(Float, Float)\nEmpty\n}\n", body: "let a = Shape.Circle(1.0)\nlet b = Shape.Empty\nlet c: Shape = Shape.Rect(1.0, 2.0)",
		types: map[string]string{"a": "Shape", "b": "Shape"}},
	{id: "K112", decls: "enum Shape {\nCircle(Float)\nEmpty\n}\n", body: "let a = Shape.Square\nlet b = Shape.Circle\nlet c = Shape.Empty()\nlet d = Shape.Circle(1)\nlet e = Shape.Circle(1.0, 2.0)\nlet f = Shape",
		want: []string{`T026 "Square"`, `T006 "Shape.Circle"`, `T006 "Shape.Empty()"`, `T001 "1"`, `T006 "Shape.Circle(1.0, 2.0)"`, `T025 "Shape"`}},
	{id: "K113", body: "let a = Option.Some(5)\nlet b: Option<String> = Option.None\nlet c = Option.None\nlet d: Option<Int> = Option.Some(\"x\")\nlet e = Option.Some(u())",
		types: map[string]string{"a": "Option<Int>", "b": "Option<String>"},
		want:  []string{`T020 "Option.None"`, `T001 "\"x\""`, `T030 "Option.Some(u())"`}},
	{id: "K114", decls: "enum Int { A }\nenum E { A\nA }\nenum E { B }\nenum Option { X }\nenum G<T, T> { V(T) }\nenum H { V(Unit) }\nenum K<Int> { V }\n", body: "",
		want: []string{`T005 "Int"`, `T005 "A"`, `T005 "E"`, `T005 "Option"`, `T005 "T"`, `T030 "Unit"`, `T005 "Int"`}},
	{id: "K115", decls: "enum Tree {\nLeaf\nNode(Tree, Int, Tree)\n}\n", body: "let t = Tree.Node(Tree.Leaf, 1, Tree.Leaf)", types: map[string]string{"t": "Tree"}},
	{id: "K116", body: "let a: Array<Int, Int> = []\nlet b: Int<String> = 1\nlet c: Option = Option.None\nlet d: Array<Nope> = []",
		want: []string{`T021 "Array<Int, Int>"`, `T021 "Int<String>"`, `T021 "Option"`, `T003 "Nope"`}},

	// Generic functions.
	{id: "K117", decls: "fn first<T>(xs: Array<T>) -> Option<T> {\nif xs.len() == 0 {\nreturn Option.None\n}\nreturn Option.Some(xs[0])\n}\nfn id<T>(x: T) -> T {\nreturn x\n}\n",
		body:  "let a = first([1, 2])\nlet b = id(\"s\")\nlet c: Option<String> = first([])\nlet d = id([Option.Some(1)])",
		types: map[string]string{"a": "Option<Int>", "b": "String", "c": "Option<String>", "d": "Array<Option<Int>>"}},
	{id: "K118", decls: "fn empty<T>() -> Array<T> {\nreturn []\n}\nfn pair<A>(x: A, y: A) -> Unit { }\n",
		body: "let a = empty()\nlet b: Array<Int> = empty()\npair(1, \"s\")\npair(u(), u())",
		want: []string{`T020 "empty()"`, `T001 "\"s\""`, `T030 "pair(u(), u())"`}},
	{id: "K119", decls: "fn bad<T>(x: T) -> Int {\nreturn x + 1\n}\nfn show<T>(x: T) -> String {\nprint(x)\nlet same = x == x\nreturn \"${x}\"\n}\nfn twice<T, T>(x: T) -> Unit { }\nfn hide<Int>(x: Int) -> Unit { }\n", body: "",
		want: []string{`T007 "x + 1"`, `T005 "T"`, `T005 "Int"`}},
	{id: "K120", decls: "enum Pair<A, B> { Of(A, B) }\nfn swap<A, B>(p: Pair<A, B>) -> Pair<B, A> {\nmatch p {\nPair.Of(a, b) => { return Pair.Of(b, a) }\n}\n}\n",
		body: "let s = swap(Pair.Of(1, \"x\"))", types: map[string]string{"s": "Pair<String, Int>"}},

	// Match: patterns, exhaustiveness, unreachable arms.
	{id: "K121", decls: "enum Shape {\nCircle(Float)\nRect(Float, Float)\nEmpty\n}\nfn area(s: Shape) -> Float {\nmatch s {\nShape.Circle(r) => { return r * r }\nShape.Rect(w, h) => { return w * h }\nShape.Empty => { return 0.0 }\n}\n}\n",
		body: "print(area(Shape.Empty))"},
	{id: "K122", decls: "enum Shape {\nCircle(Float)\nEmpty\n}\n", body: "let s = Shape.Empty\nmatch s {\nShape.Circle(_) => { }\n}",
		want: []string{`T027 "s"`}},
	{id: "K123", body: "let o = Option.Some(Option.Some(1))\nmatch o {\nOption.Some(Option.Some(_)) => { }\nOption.None => { }\n}",
		want: []string{`T027 "o"`}},
	{id: "K124", body: "let n = 3\nmatch n {\n0 => { }\n-1 => { }\n}\nmatch n {\n0 => { }\nother => { print(other) }\n}\nlet b = true\nmatch b {\ntrue => { }\nfalse => { }\n}",
		want: []string{`T027 "n"`}},
	{id: "K125", body: "let n = 3\nmatch n {\n_ => { }\n1 => { }\n}\nmatch Option.Some(1) {\nOption.Some(_) => { }\nOption.None => { }\nOption.Some(2) => { }\n}",
		want: []string{`T028 "1"`, `T028 "Option.Some(2)"`}},
	{id: "K126", decls: "enum Shape {\nCircle(Float)\nEmpty\n}\n", body: "let s = Shape.Empty\nmatch s {\nShape.Circle => { }\nShape.Empty(x) => { }\nShape.Circle(1.5) => { }\nShape.Circle(1, 2) => { }\nShape.Square => { }\nNope.A => { }\n\"x\" => { }\nOption.None => { }\n_ => { }\n}",
		want: []string{`T029 "Shape.Circle"`, `T029 "Shape.Empty(x)"`, `T029 "1.5"`, `T029 "Shape.Circle(1, 2)"`, `T026 "Square"`, `T026 "Nope"`, `T001 "\"x\""`, `T001 "Option.None"`}},
	{id: "K127", body: "let o = Option.Some(1)\nmatch o {\nOption.Some(x) => { x = 2 }\nOption.None => { }\n}\nmatch o {\nOption.Some(v) => { }\nv => { }\n}",
		want: []string{`T004 "x = 2"`}},
	{id: "K128", decls: "fn f(o: Option<Int>) -> Int {\nmatch o {\nOption.Some(n) => { return n }\nOption.None => { }\n}\n}\nfn g(o: Option<Int>) -> Int {\nmatch o {\nOption.Some(n) => { return n }\n_ => { return 0 }\n}\n}\n",
		body: "print(f(Option.None) + g(Option.None))", want: []string{`T008 "f"`}},
	{id: "K129", body: "let s = \"a\"\nmatch s {\n\"a\" => { }\n\"b\" => { }\n_ => { }\n}\nlet xs = [1]\nmatch xs {\nall => { print(all) }\n}"},
	{id: "K130", body: "let o = Option.Some(1)\nmatch o {\nOption.Some(a) => { }\nOption.None => { }\n}\nmatch o {\nOption.Some(x) => { let x = 1 }\n_ => { }\n}\nmatch [1] {\nprint => { }\n}",
		want: []string{`T016 "print"`}},
	// Arguments that need an expected type are checked after the others.
	{id: "K131", decls: "enum Tree<T> {\nLeaf\nNode(Tree<T>, T, Tree<T>)\n}\nfn pick<T>(a: Array<T>, b: Array<T>) -> T {\nreturn b[0]\n}\n",
		body:  "let t = Tree.Node(Tree.Leaf, 1, Tree.Leaf)\nlet p = pick([], [\"s\"])\nlet q = Option.Some([Option.None, Option.Some(2)])",
		types: map[string]string{"t": "Tree<Int>", "p": "String", "q": "Option<Array<Option<Int>>>"}},

	// Structs and methods (ADR-0014).
	{id: "K132", decls: pointDecl,
		body:  "let mut p = Point { y: 2, x: 1 }\np.x = 5\np.shift(1)\nlet n = p.len2()\nlet o = Point.origin()\nlet s = \"${p}\"\nlet e = p == o",
		types: map[string]string{"p": "Point", "n": "Int", "o": "Point", "s": "String", "e": "Bool"}},
	{id: "K133", decls: pointDecl,
		body: "let p = Point { x: 1 }\nlet q = Point { x: 1, y: 2, z: 3 }\nlet r = Point { x: 1, x: 2, y: 3 }",
		want: []string{`T031 "Point"`, `T031 "z"`, `T031 "x"`}},
	{id: "K134", decls: pointDecl,
		body: "let p = Point { x: 1, y: 2 }\np.x = 3\np.shift(1)\nprint(p.z)\nprint(p.len2)\np.nope()\nlet q = Point.len2()",
		want: []string{`T004 "p.x = 3"`, `T004 "p.shift(1)"`, `T024 "z"`, `T024 "p.len2"`, `T024 "nope"`, `T024 "Point.len2"`}},
	{id: "K135", decls: "struct Box<T> { v: T }\nimpl Box {\nfn get(self) -> T {\nreturn self.v\n}\nfn put(mut self, v: T) -> Unit {\nself.v = v\n}\nfn wrap(v: T) -> Box<T> {\nreturn Box { v: v }\n}\nfn bad(self) -> Unit {\nself.v = self.v\n}\n}\n",
		body:  "let mut b = Box { v: \"s\" }\nb.put(\"t\")\nlet g = b.get()\nlet w = Box.wrap(1)\nlet e: Box<Array<Int>> = Box { v: [] }\nb.put(1)",
		types: map[string]string{"b": "Box<String>", "g": "String", "w": "Box<Int>", "e": "Box<Array<Int>>"},
		want:  []string{`T004 "self.v = self.v"`, `T001 "1"`}},
	{id: "K136", decls: "struct A { b: B }\nstruct B { a: A }\nstruct List { next: Option<List> }\nstruct Wrap<T> { v: T }\nstruct C { w: Wrap<C> }\n",
		want: []string{`T032 "b"`, `T032 "a"`, `T032 "w"`}},
	{id: "K137", decls: "struct P { x: Int }\nimpl P {\nfn x(self) -> Int {\nreturn 1\n}\nfn f(self) -> Unit { }\nfn f(self) -> Unit { }\n}\nimpl Q { }\nenum E { A }\nimpl E { }\nstruct E { }\n",
		body: "let a = P\nlet b = P { x: 1 }\nif b == (P { x: 1 }) { }\nmatch b {\nP.x => { }\n}",
		want: []string{`T005 "x"`, `T005 "f"`, `T003 "Q"`, `T003 "E"`, `T005 "E"`, `T025 "P"`, `T029 "P"`}},

	// Modules (ADR-0014).
	{id: "K138", files: map[string]string{
		"geometry":      "pub struct Point {\npub x: Int\npub y: Int\n}\nimpl Point {\npub fn sum(self) -> Int {\nreturn self.x + self.y\n}\n}\npub fn origin() -> Point {\nreturn Point { x: 0, y: 0 }\n}\npub enum Shape {\nDot(Point)\nNone\n}\n",
		"shapes.circle": "module circle\npub fn area(r: Float) -> Float {\nreturn r * r * 3.0\n}\nfn main() -> Unit { }\n",
	},
		decls: "import geometry\nimport shapes.circle\nfn f(p: geometry.Point) -> geometry.Shape {\nreturn geometry.Shape.Dot(p)\n}\n",
		body:  "let p = geometry.Point { x: 1, y: 2 }\nlet o = geometry.origin()\nlet s = p.sum()\nlet a = circle.area(2.0)\nlet sh = f(o)\nmatch sh {\ngeometry.Shape.Dot(q) => { print(q.x) }\ngeometry.Shape.None => { }\n}",
		types: map[string]string{"p": "geometry.Point", "o": "geometry.Point", "s": "Int", "a": "Float", "sh": "geometry.Shape"}},
	{id: "K139", files: map[string]string{
		"lib": "module other\npub struct Pub {\npub a: Int\nb: Int\n}\nimpl Pub {\nfn hidden(self) -> Int {\nreturn self.b\n}\n}\nstruct Priv { }\nfn helper() -> Int {\nreturn 1\n}\npub fn make() -> Pub {\nreturn Pub { a: 1, b: 2 }\n}\n",
	},
		decls: "import lib\nfn g(x: lib.Priv) -> Unit { }\n",
		body:  "let p = lib.make()\nprint(p.b)\nprint(p.hidden())\nprint(lib.helper())\nlet q = lib.Pub { a: 1, b: 2 }\nlet lib = 1\nprint(lib)",
		want:  []string{`T034 "lib.Priv"`, `T034 "b"`, `T034 "hidden"`, `T034 "helper"`, `T034 "Pub"`, `T033 "module other"@lib`}},
	{id: "K140", files: map[string]string{"lib": "pub fn f() -> Int {\nreturn 1\n}\n"},
		decls: "import lib\nfn lib() -> Unit { }\n",
		body:  "print(lib)\nlib.g()\nlet x: lib = 1\nlet y = lib.f",
		want:  []string{`T005 "lib"`, `T025 "lib"`, `T002 "g"`, `T003 "lib"`, `T018 "lib.f"`}},

	// Tests and assertions (ADR-0015).
	{id: "K141", decls: "test \"ok\" {\nassert(1 == 1)\nassertEqual([1], [1])\nassertEqual(Option.None, Option.Some(1))\nassertEqual(Option.Some(\"a\"), Option.None)\n}\n"},
	{id: "K142", decls: "test \"a\" { }\ntest \"a\" { }\ntest \"b\" {\nassert(1)\nassertEqual(1, \"x\")\nassertEqual(1)\nreturn 1\nassertEqual(u(), u())\n}\n",
		want: []string{`T005 "\"a\""`, `T001 "1"`, `T001 "\"x\""`, `T006 "assertEqual(1)"`, `T009 "return 1"`, `T011 "u()"`}},
	{id: "K143", decls: "fn assert(x: Int) -> Unit { }\n", want: []string{`T016 "assert"`}},
}

const pointDecl = "struct Point {\nx: Int\ny: Int\n}\nimpl Point {\nfn origin() -> Point {\nreturn Point { x: 0, y: 0 }\n}\nfn len2(self) -> Int {\nreturn self.x * self.x + self.y * self.y\n}\nfn shift(mut self, d: Int) -> Unit {\nself.x = self.x + d\nself.shift2(d)\n}\nfn shift2(mut self, d: Int) -> Unit {\nself.y = self.y + d\n}\n}\n"

// runCheck lexes, parses and checks src. It fails the test on unexpected
// lexer or parser diagnostics.
func runCheck(t *testing.T, src string, lexOK bool) (*CheckedProgram, []diagnostic.Diagnostic) {
	t.Helper()
	f := source.NewFile(1, "t.tr", []byte(src))
	toks, lexDiags := lexer.Lex(f)
	if len(lexDiags) > 0 && !lexOK {
		t.Fatalf("unexpected lexer diagnostics: %+v", lexDiags)
	}
	prog, parseDiags := parser.Parse(toks)
	if len(parseDiags) > 0 {
		t.Fatalf("unexpected parser diagnostics: %+v", parseDiags)
	}
	return Check(prog)
}

// runCheckFiles checks a program of several modules (ADR-0014). It
// returns the source text of every file by file ID.
func runCheckFiles(t *testing.T, src string, files map[string]string) (*CheckedProgram, []diagnostic.Diagnostic, map[source.FileID]string, map[source.FileID]string) {
	t.Helper()
	texts := map[source.FileID]string{1: src}
	names := map[source.FileID]string{1: ""}
	parse := func(id source.FileID, text string) *ast.Program {
		toks, lexDiags := lexer.Lex(source.NewFile(id, "f.tr", []byte(text)))
		prog, parseDiags := parser.Parse(toks)
		if len(lexDiags)+len(parseDiags) > 0 {
			t.Fatalf("unexpected syntax diagnostics: %+v %+v", lexDiags, parseDiags)
		}
		return prog
	}
	mods := []SourceModule{{Program: parse(1, src)}}
	index := map[string]int{"": 0}
	var paths []string
	for p := range files {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		id := source.FileID(len(mods) + 1)
		texts[id], names[id] = files[p], p
		index[p] = len(mods)
		mods = append(mods, SourceModule{Path: p, Program: parse(id, files[p])})
	}
	for i := range mods {
		mods[i].Imports = map[*ast.ImportDecl]int{}
		for _, d := range mods[i].Program.Imports {
			if j, ok := index[d.PathString()]; ok {
				mods[i].Imports[d] = j
			}
		}
	}
	out, diags := CheckModules(mods)
	return out, diags, texts, names
}

// describe renders a diagnostic as `T001 "text"`; when the span text is
// empty or ambiguous, tests append @offset.
func describe(src string, d diagnostic.Diagnostic, withOffset bool) string {
	s := fmt.Sprintf("%s %q", strings.TrimPrefix(string(d.Code), "TR-"), src[d.Span.Start:d.Span.End])
	if withOffset {
		s += fmt.Sprintf("@%d", d.Span.Start)
	}
	return s
}

func TestCheckRequiredCases(t *testing.T) {
	for _, tc := range checkCases {
		t.Run(tc.id, func(t *testing.T) {
			src := tc.source()
			var out *CheckedProgram
			var diags []diagnostic.Diagnostic
			texts := map[source.FileID]string{1: src}
			names := map[source.FileID]string{1: ""}
			if tc.files != nil {
				out, diags, texts, names = runCheckFiles(t, src, tc.files)
			} else {
				out, diags = runCheck(t, src, tc.lexOK)
			}
			var got []string
			for i, d := range diags {
				if d.Code.Phase() != 'T' {
					t.Fatalf("checker produced non-checker code %s", d.Code)
				}
				withOffset := i < len(tc.want) && strings.Contains(tc.want[i], `"@`) && !strings.HasSuffix(tc.want[i], `"@`+names[d.Span.File])
				text := describe(texts[d.Span.File], d, withOffset)
				if names[d.Span.File] != "" {
					text += "@" + names[d.Span.File]
				}
				got = append(got, text)
			}
			if strings.Join(got, " | ") != strings.Join(tc.want, " | ") {
				t.Errorf("diagnostics:\n got  %v\n want %v", got, tc.want)
			}
			for name, want := range tc.types {
				let := findLet(out.Program, name)
				if let == nil {
					t.Fatalf("no let %q in main", name)
				}
				if got := out.Types[let.Value].String(); got != want {
					t.Errorf("type of %s = %s, want %s", name, got, want)
				}
			}
		})
	}
}

// findLet returns the first `let name` in main's body (searching nested
// blocks).
func findLet(prog *ast.Program, name string) *ast.LetStmt {
	for _, fn := range prog.Funcs {
		if fn.Name.Name != "main" {
			continue
		}
		var found *ast.LetStmt
		var walk func(s ast.Stmt)
		walk = func(s ast.Stmt) {
			switch s := s.(type) {
			case *ast.LetStmt:
				if found == nil && s.Name.Name == name {
					found = s
				}
			case *ast.Block:
				for _, x := range s.Stmts {
					walk(x)
				}
			case *ast.IfStmt:
				walk(s.Then)
				if s.Else != nil {
					walk(s.Else)
				}
			}
		}
		walk(fn.Body)
		return found
	}
	return nil
}

// TestEveryCheckerCodeIsTested enforces docs/08 §8.1 for checker codes.
func TestEveryCheckerCodeIsTested(t *testing.T) {
	seen := map[string]bool{}
	for _, tc := range checkCases {
		for _, w := range tc.want {
			seen["TR-"+strings.SplitN(w, " ", 2)[0]] = true
		}
	}
	var missing []string
	for _, c := range diagnostic.Registry() {
		// Import cycles and unreadable files are found while loading the
		// files (internal/pipeline tests).
		if c == diagnostic.ImportCycle || c == diagnostic.ImportNotFound {
			continue
		}
		if c.Phase() == 'T' && !seen[string(c)] {
			missing = append(missing, string(c))
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("checker codes never produced by a required case: %v", missing)
	}
}

func TestIntValues(t *testing.T) {
	src := prelude + "fn main() -> Unit {\nlet a = -9223372036854775808\nlet b = 9223372036854775807\nlet c = -(5)\n}\n"
	out, diags := runCheck(t, src, false)
	if len(diags) != 0 {
		t.Fatalf("diags: %+v", diags)
	}
	a := findLet(out.Program, "a").Value.(*ast.UnaryExpr)
	if v, ok := out.IntValues[a]; !ok || v != math.MinInt64 {
		t.Errorf("minimum Int not recorded on the unary node: %v %v", v, ok)
	}
	if _, ok := out.IntValues[a.Operand]; ok {
		t.Error("the magnitude literal must not have its own IntValue")
	}
	b := findLet(out.Program, "b").Value
	if out.IntValues[b] != math.MaxInt64 {
		t.Errorf("max Int = %d", out.IntValues[b])
	}
	c := findLet(out.Program, "c").Value.(*ast.UnaryExpr)
	if _, ok := out.IntValues[c]; ok {
		t.Error("-(5) is an ordinary negation, not a constant")
	}
}

func TestFloatValues(t *testing.T) {
	src := prelude + "fn main() -> Unit {\nlet a = 0." + strings.Repeat("0", 323) + "5\nlet b = 0.1\n}\n"
	out, _ := runCheck(t, src, false)
	a := findLet(out.Program, "a").Value.(*ast.FloatLit)
	if v := out.FloatValues[a]; v <= 0 || v >= 2.3e-308 {
		t.Errorf("expected a positive subnormal, got %g", v)
	}
	b := findLet(out.Program, "b").Value.(*ast.FloatLit)
	if out.FloatValues[b] != 0.1 || b.Lexeme != "0.1" {
		t.Errorf("0.1 = %v, lexeme %q", out.FloatValues[b], b.Lexeme)
	}
}

func TestResolutionTables(t *testing.T) {
	src := "fn add(a: Int, b: Int) -> Int {\n    return a + b\n}\nfn main() -> Unit {\n    let mut x = add(1, 2)\n    x = x + 1\n    print(x)\n}\n"
	out, diags := runCheck(t, src, false)
	if len(diags) != 0 {
		t.Fatalf("diags: %+v", diags)
	}
	if out.Main == nil || out.Main.Name != "main" || out.Main.Result != Unit {
		t.Fatalf("Main = %+v", out.Main)
	}
	if len(out.Functions) != 2 || out.Functions[0].Name != "add" || out.Functions[0].Result != Int || len(out.Functions[0].Params) != 2 {
		t.Fatalf("Functions = %+v", out.Functions)
	}
	let := findLet(out.Program, "x")
	call := let.Value.(*ast.CallExpr)
	if sym := out.Uses[call.Callee.(*ast.Ident)]; sym == nil || sym.Kind != FuncSymbol || sym.Func != out.Functions[0] {
		t.Errorf("callee not resolved to add: %+v", sym)
	}
	if def := out.Defs[let.Name]; def == nil || def.Kind != VarSymbol || !def.Mutable || def.Type != Int {
		t.Errorf("let x def = %+v", def)
	}
	assign := out.Main.Decl.Body.Stmts[1].(*ast.AssignStmt)
	if out.Uses[assign.Target.(*ast.Ident)] != out.Defs[let.Name] {
		t.Error("assignment target not resolved to the let symbol")
	}
	printCall := out.Main.Decl.Body.Stmts[2].(*ast.ExprStmt).Call
	if sym := out.Uses[printCall.Callee.(*ast.Ident)]; sym == nil || sym.Kind != BuiltinSymbol {
		t.Errorf("print not resolved to the built-in: %+v", sym)
	}
	// Every expression in main has a recorded type.
	for _, e := range []ast.Expr{let.Value, assign.Value, printCall} {
		if _, ok := out.Types[e]; !ok {
			t.Errorf("missing type for %s", ast.Format(e))
		}
	}
}

func TestInvalidMainIsNotRecorded(t *testing.T) {
	out, _ := runCheck(t, "fn main(a: Int) -> Unit { }\n", false)
	if out.Main != nil {
		t.Error("an invalid main must not be recorded as the entry point")
	}
}

func TestHelpAndRelated(t *testing.T) {
	src := prelude + "fn main() -> Unit {\nlet x = 1\nx = 2\nlet x = 3\n}\n"
	_, diags := runCheck(t, src, false)
	if len(diags) != 2 {
		t.Fatalf("diags: %+v", diags)
	}
	if diags[0].Help == "" || len(diags[0].Related) != 1 {
		t.Errorf("T004 should carry help and the declaration span: %+v", diags[0])
	}
	if len(diags[1].Related) != 1 {
		t.Errorf("T005 should carry the previous declaration span: %+v", diags[1])
	}
}

func TestDeterministic(t *testing.T) {
	src := prelude + "fn f() { }\nfn main() -> Unit {\nlet a: Int = \"s\"\nprint(u())\nlet b = q + r\n}\n"
	_, a := runCheck(t, src, false)
	_, b := runCheck(t, src, false)
	if fmt.Sprint(a) != fmt.Sprint(b) {
		t.Error("checker output is not deterministic")
	}
	for i := 1; i < len(a); i++ {
		if a[i].Span.Start < a[i-1].Span.Start {
			t.Error("diagnostics are not sorted by position")
		}
	}
}

// TestFullProgram checks a realistic program end to end through the
// frontend and checker.
func TestFullProgram(t *testing.T) {
	src := `module main

fn add(a: Int, b: Int) -> Int {
    return a + b
}

fn grade(score: Int) -> String {
    if score >= 80 {
        return "A"
    } else if score >= 60 {
        return "B"
    } else {
        return "C"
    }
}

fn main() -> Unit {
    let name = "Tread"
    let mut total: Int = add(2,
                             3)
    total = total +
        1
    let ratio = 1.5 * 2.0
    let ok = total > 5 && !(ratio < 1.0)
    print("Hello, " + name + "! total=${total} ratio=${ratio} ok=${ok}")
    print(grade(total * 10))
    print(-9223372036854775808)
}
`
	_, diags := runCheck(t, src, false)
	if len(diags) != 0 {
		t.Errorf("unexpected diagnostics: %+v", diags)
	}
}

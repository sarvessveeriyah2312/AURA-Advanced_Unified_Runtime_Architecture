package checker

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"testing"

	"github.com/sarvessveeriyah2312/AURA-Advanced_Unified_Runtime_Architecture/internal/ast"
	"github.com/sarvessveeriyah2312/AURA-Advanced_Unified_Runtime_Architecture/internal/diagnostic"
	"github.com/sarvessveeriyah2312/AURA-Advanced_Unified_Runtime_Architecture/internal/lexer"
	"github.com/sarvessveeriyah2312/AURA-Advanced_Unified_Runtime_Architecture/internal/parser"
	"github.com/sarvessveeriyah2312/AURA-Advanced_Unified_Runtime_Architecture/internal/source"
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
}

func (tc checkCase) source() string {
	if tc.whole {
		return tc.src
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
}

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
			out, diags := runCheck(t, src, tc.lexOK)
			var got []string
			for i, d := range diags {
				if d.Code.Phase() != 'T' {
					t.Fatalf("checker produced non-checker code %s", d.Code)
				}
				withOffset := i < len(tc.want) && strings.Contains(tc.want[i], `"@`)
				got = append(got, describe(src, d, withOffset))
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
	if out.Uses[assign.Target] != out.Defs[let.Name] {
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

package parser

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/sarvessveeriyah2312/Tread/internal/ast"
	"github.com/sarvessveeriyah2312/Tread/internal/diagnostic"
	"github.com/sarvessveeriyah2312/Tread/internal/lexer"
	"github.com/sarvessveeriyah2312/Tread/internal/source"
	"github.com/sarvessveeriyah2312/Tread/internal/token"
)

// mode selects the parser entry point for a test case.
type mode int

const (
	program mode = iota // whole file: Parse
	stmts               // statement list, as inside a block (line/col preserved)
	expr                // a single expression
)

type parseCase struct {
	id    string
	mode  mode
	src   string
	want  string   // ast.Format output (statements joined by a space)
	diags []string // exact parser diagnostics, "P004@1:11"
	// lexDiags lists expected lexer diagnostics; all others must be absent.
	lexDiags []string
}

const s68 = "if score >= 80 {\n    print(\"A\")\n} else if score >= 60 {\n    print(\"B\")\n} else {\n    print(\"C\")\n}"
const s68AST = `(if (>= score 80) (block (call print "A")) else (if (>= score 60) (block (call print "B")) else (block (call print "C"))))`

// parseCases implements the required parser test cases S1-S85 of docs/03
// §3.5 (S86 is the fuzz target). Cases with several inputs use suffixes.
var parseCases = []parseCase{
	// Newline and continuation (N1-N7).
	{id: "S1", mode: stmts, src: "let t = a +\n    b", want: "(let t (+ a b))"},
	{id: "S2", mode: stmts, src: "let x =\n    1", want: "(let x 1)"},
	{id: "S3", mode: stmts, src: "let x:\n    Int = 1", want: "(let x:Int 1)"},
	{id: "S4", mode: program, src: "fn f() ->\n    Int { return 1 }", want: "(program (fn f () -> Int (block (return 1))))"},
	{id: "S5a", mode: stmts, src: "let b = !\n    c", want: "(let b (! c))"},
	{id: "S5b", mode: stmts, src: "let n = -\n    1", want: "(let n (- 1))"},
	{id: "S6", mode: stmts, src: "let c = a &&\n b ||\n d", want: "(let c (|| (&& a b) d))"},
	{id: "S7", mode: stmts, src: "f(\n    a,\n    b\n)", want: "(call f a b)"},
	{id: "S8", mode: program, src: "fn f(\n    a: Int,\n    b: Int\n) { }", want: "(program (fn f (a:Int b:Int) (block)))"},
	{id: "S9", mode: stmts, src: "let x = (a\n    + b)", want: "(let x (paren (+ a b)))"},
	{id: "S10", mode: stmts, src: "let x = (f\n    (1))", want: "(let x (paren (call f 1)))"},
	{id: "S11", mode: stmts, src: "let t = a\n    - b", want: "(let t a)", diags: []string{"P006@2:5"}},
	{id: "S12", mode: stmts, src: "let t = a\n    + b", want: "(let t a)", diags: []string{"P001@2:5"}},
	{id: "S14", mode: program, src: "fn main() { print(1) }", want: "(program (fn main () (block (call print 1))))"},
	{id: "S15a", mode: program, src: "fn main() { }", want: "(program (fn main () (block)))"},
	{id: "S15b", mode: program, src: "fn main() {}", want: "(program (fn main () (block)))"},
	{id: "S16", mode: stmts, src: "let a = 1 let b = 2", want: "(let a 1)", diags: []string{"P004@1:11"}},
	{id: "S17", mode: program, src: "fn a() {} fn b() {}", want: "(program (fn a () (block)) (fn b () (block)))", diags: []string{"P004@1:11"}},
	{id: "S18", mode: stmts, src: "return\nf()", want: "(return) (call f)"},
	{id: "S19", mode: stmts, src: "{ return }", want: "(block (return))"},
	{id: "S20", mode: stmts, src: "return 1 + 2", want: "(return (+ 1 2))"},
	{id: "S21", mode: program, src: "fn main()\n{ print(1) }", want: "(program (fn main () (block (call print 1))))", diags: []string{"P005@2:1"}},
	{id: "S22", mode: stmts, src: "if c\n{ print(1) }", want: "(if c (block (call print 1)))", diags: []string{"P005@2:1"}},
	{id: "S23", mode: stmts, src: "if c { print(1) }\nelse { print(2) }", want: "(if c (block (call print 1)) else (block (call print 2)))", diags: []string{"P005@2:1"}},
	{id: "S24", mode: stmts, src: "if c { print(1) } else\n{ print(2) }", want: "(if c (block (call print 1)) else (block (call print 2)))", diags: []string{"P005@2:1"}},
	{id: "S25", mode: stmts, src: "f\n(1)", want: "", diags: []string{"P006@1:1", "P006@2:1"}},

	// Statements (N8).
	{id: "S27a", mode: stmts, src: "f()", want: "(call f)"},
	{id: "S27b", mode: stmts, src: "f(1, 2)", want: "(call f 1 2)"},
	{id: "S27c", mode: stmts, src: "f()()", want: "(call (call f))"},
	{id: "S28a", mode: stmts, src: "x", diags: []string{"P006@1:1"}},
	{id: "S28b", mode: stmts, src: "1", diags: []string{"P006@1:1"}},
	{id: "S28c", mode: stmts, src: `"s"`, diags: []string{"P006@1:1"}},
	{id: "S28d", mode: stmts, src: "true", diags: []string{"P006@1:1"}},
	{id: "S29a", mode: stmts, src: "a + b", diags: []string{"P006@1:1"}},
	{id: "S29b", mode: stmts, src: "-x", diags: []string{"P006@1:1"}},
	{id: "S29c", mode: stmts, src: "!x", diags: []string{"P006@1:1"}},
	{id: "S29d", mode: stmts, src: "(f())", diags: []string{"P006@1:1"}},
	{id: "S30", mode: stmts, src: "x = 1", want: "(assign x 1)"},
	{id: "S31", mode: stmts, src: "x = \n 1", want: "(assign x 1)"},

	// Commas (ADR-0003 Q5).
	{id: "S32", mode: stmts, src: "f(a, b,)", want: "(call f a b)", diags: []string{"P007@1:7"}},
	{id: "S33", mode: program, src: "fn f(a: Int,) { }", want: "(program (fn f (a:Int) (block)))", diags: []string{"P007@1:12"}},
	{id: "S34a", mode: stmts, src: "f(,)", want: "(call f <error>)", diags: []string{"P001@1:3"}},
	{id: "S34b", mode: stmts, src: "f(a,,b)", want: "(call f a <error> b)", diags: []string{"P001@1:5"}},
	{id: "S35", mode: stmts, src: "f(a\n,\nb)", want: "(call f a b)"},

	// Modules (N9).
	{id: "S36", mode: program, src: "fn main() { }", want: "(program (fn main () (block)))"},
	{id: "S37", mode: program, src: "module app\nfn main() { }", want: "(program (module app) (fn main () (block)))"},
	{id: "S38", mode: program, src: "// c\n\n/* d */\nmodule app\nfn main() { }", want: "(program (module app) (fn main () (block)))"},
	{id: "S39", mode: program, src: "\xef\xbb\xbfmodule app", want: "(program (module app))"},
	{id: "S40", mode: program, src: "module a\nmodule b", want: "(program (module a))", diags: []string{"P008@2:1"}},
	{id: "S41", mode: program, src: "fn main() { }\nmodule a", want: "(program (fn main () (block)))", diags: []string{"P009@2:1"}},
	{id: "S42", mode: program, src: "module a fn main() { }", want: "(program (module a) (fn main () (block)))", diags: []string{"P004@1:10"}},
	{id: "S43", mode: program, src: "module", want: "(program)", diags: []string{"P002@1:7"}},
	{id: "S44", mode: program, src: "module fn", want: "(program (module fn))", diags: []string{"P002@1:8"}},

	// Identifiers, keywords, and type names (ADR-0004 item 3).
	{id: "S45", mode: stmts, src: "let match = 1", want: "(let match 1)", diags: []string{"P003@1:5"}},
	{id: "S46", mode: stmts, src: "let _ = f()", want: "(let _ (call f))"},
	{id: "S47", mode: program, src: "fn if() { }", want: "(program (fn if () (block)))", diags: []string{"P002@1:4"}},
	{id: "S48", mode: stmts, src: "let Int: Int = 1", want: "(let Int:Int 1)"},
	{id: "S49", mode: program, src: "fn String(Bool: Float) -> Unit { }", want: "(program (fn String (Bool:Float) -> Unit (block)))"},
	{id: "S50", mode: stmts, src: "let x: = 1", want: "(let x 1)", diags: []string{"P013@1:8"}},
	{id: "S51", mode: stmts, src: "let x: 5 = 1", diags: []string{"P013@1:8"}},
	{id: "S52", mode: stmts, src: "let x: fn = 1", diags: []string{"P013@1:8"}},
	{id: "S53", mode: program, src: "fn f() -> { }", want: "(program (fn f () (block)))", diags: []string{"P013@1:11"}},

	// Precedence and associativity.
	{id: "S54", mode: expr, src: "a - b - c", want: "(- (- a b) c)"},
	{id: "S55", mode: expr, src: "a / b * c % d", want: "(% (* (/ a b) c) d)"},
	{id: "S56", mode: expr, src: "a + b * c", want: "(+ a (* b c))"},
	{id: "S57a", mode: expr, src: "-a * b", want: "(* (- a) b)"},
	{id: "S57b", mode: expr, src: "!a == b", want: "(== (! a) b)"},
	{id: "S58a", mode: expr, src: "- - a", want: "(- (- a))"},
	{id: "S58b", mode: expr, src: "!!a", want: "(! (! a))"},
	{id: "S59", mode: expr, src: "a < b == c < d", want: "(== (< a b) (< c d))"},
	{id: "S60", mode: expr, src: "a || b && c", want: "(|| a (&& b c))"},
	{id: "S61", mode: expr, src: "(a + b) * c", want: "(* (paren (+ a b)) c)"},

	// Literals (ADR-0004 item 1).
	{id: "S62", mode: stmts, src: "let m = -9223372036854775808", want: "(let m (- 9223372036854775808))"},
	{id: "S63", mode: stmts, src: "let p = -(5)", want: "(let p (- (paren 5)))"},
	{id: "S64", mode: stmts, src: "let x = 9abc + 1", want: "(let x (+ <error> 1))", lexDiags: []string{"L008@1:9"}},

	// Interpolation (ADR-0003 Q1).
	{id: "S65", mode: expr, src: `"a${x + 1}b"`, want: `(interp "a" (+ x 1) "b")`},
	{id: "S66", mode: expr, src: `"${f("x")} ${y}"`, want: `(interp (call f "x") " " y)`},
	{id: "S67", mode: expr, src: `"\${x} ${y}"`, want: `(interp "${x} " y)`},

	// Conditionals and else-if (ADR-0004 item 5).
	{id: "S68", mode: stmts, src: s68, want: s68AST},
	{id: "S69", mode: stmts, src: "if a { } else if b { }", want: "(if a (block) else (if b (block)))"},
	{id: "S70", mode: stmts, src: "if a { } else if b { } else if c { } else { }", want: "(if a (block) else (if b (block) else (if c (block) else (block))))"},
	{id: "S71", mode: stmts, src: "if a { if b { } else { } } else if c { if d { } }", want: "(if a (block (if b (block) else (block))) else (if c (block (if d (block)))))"},
	{id: "S72", mode: stmts, src: "if a { } else if b { } else { } else { }", want: "(if a (block) else (if b (block) else (block)))", diags: []string{"P004@1:33"}},
	{id: "S73", mode: stmts, src: "if a { }\nelse if b { }", want: "(if a (block) else (if b (block)))", diags: []string{"P005@2:1"}},
	{id: "S74", mode: stmts, src: "if a { } else\nif b { }", want: "(if a (block) else (if b (block)))", diags: []string{"P005@2:1"}},
	{id: "S75", mode: stmts, src: "if a { } else if\nb { }", want: "(if a (block) else (if <error> (block)))", diags: []string{"P001@1:17"}},
	{id: "S76", mode: stmts, src: "if a { } else if b\n{ }", want: "(if a (block) else (if b (block)))", diags: []string{"P005@2:1"}},
	{id: "S77", mode: stmts, src: "if a { } else if { }", want: "(if a (block) else (if <error> (block)))", diags: []string{"P001@1:18"}},
	{id: "S78", mode: stmts, src: "if a { } else b { }", want: "(if a (block))", diags: []string{"P012@1:15"}},
	{id: "S79", mode: stmts, src: "else { }", diags: []string{"P012@1:1"}},
	{id: "S80", mode: stmts, src: "if a { f() } else if b >= { g() } else { h() }\nk()",
		want:  "(if a (block (call f)) else (if (>= b <error>) (block (call g)) else (block (call h)))) (call k)",
		diags: []string{"P001@1:27"}},

	// Recovery (N10).
	{id: "S82", mode: stmts, src: "f(1, 2\nlet x = 3\nprint(x)", want: "(call f 1 2) (let x 3) (call print x)", diags: []string{"P010@1:2"}},
	{id: "S83", mode: stmts, src: "let = 1\nlet y 2\nprint(y)", want: "(call print y)", diags: []string{"P002@1:5", "P012@2:7"}},
	{id: "S84", mode: program, src: "fn main() {\nprint(1)", want: "(program (fn main () (block (call print 1))))", diags: []string{"P011@1:11"}},
	{id: "S85", mode: program, src: "fn main() { let x = }\nfn g() { }", want: "(program (fn main () (block (let x <error>))) (fn g () (block)))", diags: []string{"P001@1:21"}},

	// Nesting limit (ADR-0006). n nested constructs plus the innermost
	// operand use n+1 levels; the limit is MaxNestingDepth (10,000).
	{id: "S87", mode: stmts, src: "let x = " + strings.Repeat("(", 9999) + "1" + strings.Repeat(")", 9999),
		want: "(let x " + strings.Repeat("(paren ", 9999) + "1" + strings.Repeat(")", 9999) + ")"},
	{id: "S88", mode: stmts, src: "let x = " + strings.Repeat("(", 10000) + "1" + strings.Repeat(")", 10000),
		want: "(let x " + strings.Repeat("(paren ", 10000) + "<error>" + strings.Repeat(")", 10000) + ")", diags: []string{"P014@1:10009"}},
	{id: "S89", mode: stmts, src: strings.Repeat("{", 10000) + strings.Repeat("}", 10000),
		want: strings.Repeat("(block ", 9999) + "(block)" + strings.Repeat(")", 9999)},
	{id: "S90", mode: stmts, src: strings.Repeat("{", 10001) + strings.Repeat("}", 10001),
		want: strings.Repeat("(block ", 9999) + "(block)" + strings.Repeat(")", 9999), diags: []string{"P014@1:10001"}},
	{id: "S91", mode: stmts, src: "let x = " + strings.Repeat("1+", 9999) + "1",
		want: "(let x " + strings.Repeat("(+ ", 9999) + "1" + strings.Repeat(" 1)", 9999) + ")"},
	{id: "S92", mode: stmts, src: "let x = " + strings.Repeat("1+", 10000) + "1",
		want: "(let x " + strings.Repeat("(+ ", 10000) + "1" + strings.Repeat(" 1)", 9999) + " <error>))", diags: []string{"P014@1:20009"}},
	{id: "S93", mode: stmts, src: "let x = " + strings.Repeat("-", 10000) + "1",
		want: "(let x " + strings.Repeat("(- ", 10000) + "<error>" + strings.Repeat(")", 10000) + ")", diags: []string{"P014@1:10009"}},
	{id: "S94", mode: stmts, src: "if a { }" + strings.Repeat(" else if a { }", 9999),
		// The last if's condition `a` would open level 10,001.
		want: strings.Repeat("(if a (block) else ", 9999) + "(if <error> <nil>)" + strings.Repeat(")", 9999), diags: []string{"P014@1:139990"}},
}

// run lexes and parses src in the given mode and returns the formatted
// AST, the parser diagnostics and the lexer diagnostics.
func run(t *testing.T, m mode, src string) (string, []string, []string, *ast.Program) {
	t.Helper()
	f := source.NewFile(1, "t.tr", []byte(src))
	toks, lexDiags := lexer.Lex(f)
	var out string
	var prog *ast.Program
	var diags []diagnostic.Diagnostic
	switch m {
	case program:
		prog, diags = Parse(toks)
		out = ast.Format(prog)
	case stmts:
		p := newParser(toks)
		var parts []string
		for _, s := range p.parseStatements(token.EOF) {
			parts = append(parts, ast.Format(s))
		}
		out, diags = strings.Join(parts, " "), p.diags
	case expr:
		p := newParser(toks)
		out = ast.Format(p.parseExpr())
		if !p.at(token.EOF) {
			t.Fatalf("expression did not consume all input; stopped at %v", p.cur().Kind)
		}
		diags = p.diags
	}
	checkSpans(t, f, diags)
	return out, describeAll(f, diags), describeAll(f, lexDiags), prog
}

func describeAll(f *source.File, diags []diagnostic.Diagnostic) []string {
	var out []string
	for _, d := range diags {
		p := f.Position(d.Span.Start)
		out = append(out, fmt.Sprintf("%s@%d:%d", strings.TrimPrefix(string(d.Code), "TR-"), p.Line, p.Column))
	}
	return out
}

func checkSpans(t *testing.T, f *source.File, diags []diagnostic.Diagnostic) {
	t.Helper()
	for _, d := range diags {
		if d.Span.Start < 0 || d.Span.End > f.Len() || d.Span.Start > d.Span.End {
			t.Fatalf("diagnostic %s span %v out of bounds", d.Code, d.Span)
		}
		if d.Code.Phase() != 'P' {
			t.Fatalf("parser produced non-parser code %s", d.Code)
		}
	}
}

func TestParseRequiredCases(t *testing.T) {
	for _, tc := range parseCases {
		t.Run(tc.id, func(t *testing.T) {
			got, diags, lexDiags, _ := run(t, tc.mode, tc.src)
			if strings.Join(lexDiags, " ") != strings.Join(tc.lexDiags, " ") {
				t.Errorf("lexer diagnostics = %v, want %v", lexDiags, tc.lexDiags)
			}
			if got != tc.want {
				t.Errorf("AST:\n got  %s\n want %s", got, tc.want)
			}
			if strings.Join(diags, " ") != strings.Join(tc.diags, " ") {
				t.Errorf("diagnostics:\n got  %v\n want %v", diags, tc.diags)
			}
		})
	}
}

// TestS11Help checks the N8 help text for a leading operator.
func TestS11Help(t *testing.T) {
	f := source.NewFile(1, "t", []byte("let t = a\n    - b"))
	toks, _ := lexer.Lex(f)
	p := newParser(toks)
	p.parseStatements(token.EOF)
	if len(p.diags) != 1 || !strings.Contains(p.diags[0].Help, "end of that line") {
		t.Fatalf("expected P006 with help text, got %+v", p.diags)
	}
	// A non-operator statement start gets no help text.
	f = source.NewFile(1, "t", []byte("x"))
	toks, _ = lexer.Lex(f)
	p = newParser(toks)
	p.parseStatements(token.EOF)
	if len(p.diags) != 1 || p.diags[0].Help != "" {
		t.Fatalf("expected P006 without help, got %+v", p.diags)
	}
}

// TestS13BlankAndCommentLines: blank and comment-only lines anywhere give
// the same AST as without them.
func TestS13BlankAndCommentLines(t *testing.T) {
	plain := "module app\nfn main() {\n    let x = 1\n    print(x)\n}\n"
	noisy := "// header\n\n/* block\n   comment */\nmodule app\n\n// c\nfn main() {\n\n    // c\n    let x = 1\n\n    /* c */\n    print(x)\n\n}\n\n// end\n"
	a, da, _, _ := run(t, program, plain)
	b, db, _, _ := run(t, program, noisy)
	if a != b || len(da)+len(db) != 0 {
		t.Errorf("ASTs differ or diagnostics present:\n %s %v\n %s %v", a, da, b, db)
	}
}

// TestS26S81LineTerminators: CRLF and lone-CR versions of S1, S7, S16 and
// S68 give identical results to the LF versions.
func TestS26S81LineTerminators(t *testing.T) {
	ids := map[string]bool{"S1": true, "S7": true, "S16": true, "S68": true}
	for _, tc := range parseCases {
		if !ids[tc.id] {
			continue
		}
		for _, term := range []string{"\r\n", "\r"} {
			src := strings.ReplaceAll(tc.src, "\n", term)
			got, diags, _, _ := run(t, tc.mode, src)
			if got != tc.want || strings.Join(diags, " ") != strings.Join(tc.diags, " ") {
				t.Errorf("%s with %q: got %s %v, want %s %v", tc.id, term, got, diags, tc.want, tc.diags)
			}
		}
	}
}

func TestModuleDetails(t *testing.T) {
	// S36: implicit module.
	_, _, _, prog := run(t, program, "fn main() { }")
	if prog.Module != nil || prog.ModuleName() != "main" {
		t.Errorf("S36: want implicit module main, got %+v", prog.Module)
	}
	// S37: explicit module with span.
	_, _, _, prog = run(t, program, "module app\nfn main() { }")
	if prog.ModuleName() != "app" || prog.Module.Loc.Start != 0 || prog.Module.Loc.End != 10 {
		t.Errorf("S37: got %+v", prog.Module)
	}
	// S40: related span at the first declaration.
	f := source.NewFile(1, "t", []byte("module a\nmodule b"))
	toks, _ := lexer.Lex(f)
	_, diags := Parse(toks)
	if len(diags) != 1 || len(diags[0].Related) != 1 || f.Position(diags[0].Related[0].Span.Start) != (source.Position{Line: 1, Column: 1}) {
		t.Errorf("S40: want P008 with related span at 1:1, got %+v", diags)
	}
}

// TestElseIfIsNestedIf checks the AST shape of ADR-0004 item 5 directly.
func TestElseIfIsNestedIf(t *testing.T) {
	f := source.NewFile(1, "t", []byte(s68))
	toks, _ := lexer.Lex(f)
	p := newParser(toks)
	list := p.parseStatements(token.EOF)
	outer, ok := list[0].(*ast.IfStmt)
	if !ok {
		t.Fatalf("want IfStmt, got %T", list[0])
	}
	inner, ok := outer.Else.(*ast.IfStmt)
	if !ok {
		t.Fatalf("else-if must be a nested *ast.IfStmt, got %T", outer.Else)
	}
	if _, ok := inner.Else.(*ast.Block); !ok {
		t.Fatalf("final else must be a *ast.Block, got %T", inner.Else)
	}
}

func TestLiteralsPreserved(t *testing.T) {
	f := source.NewFile(1, "t", []byte("let a = 1.050 + 007"))
	toks, _ := lexer.Lex(f)
	p := newParser(toks)
	list := p.parseStatements(token.EOF)
	bin := list[0].(*ast.LetStmt).Value.(*ast.BinaryExpr)
	if fl, ok := bin.Left.(*ast.FloatLit); !ok || fl.Lexeme != "1.050" {
		t.Errorf("float lexeme not preserved: %#v", bin.Left)
	}
	if _, ok := bin.Right.(*ast.ErrorExpr); !ok {
		t.Errorf("malformed literal must be an ErrorExpr, got %T", bin.Right)
	}
}

func TestSpans(t *testing.T) {
	src := "let x = f(a, (b))"
	f := source.NewFile(1, "t", []byte(src))
	toks, _ := lexer.Lex(f)
	p := newParser(toks)
	let := p.parseStatements(token.EOF)[0].(*ast.LetStmt)
	if let.Loc.Start != 0 || let.Loc.End != len(src) {
		t.Errorf("let span = %v", let.Loc)
	}
	call := let.Value.(*ast.CallExpr)
	if got := src[call.Loc.Start:call.Loc.End]; got != "f(a, (b))" {
		t.Errorf("call span covers %q", got)
	}
	paren := call.Args[1].(*ast.ParenExpr)
	if got := src[paren.Loc.Start:paren.Loc.End]; got != "(b)" {
		t.Errorf("paren span covers %q", got)
	}
}

func TestFullProgram(t *testing.T) {
	src := `module main

fn add(a: Int, b: Int) -> Int {
    return a + b
}

fn main() {
    let name = "Tread"
    let mut total: Int = add(2,
                             3)
    total = total +
        1
    if total >= 6 {
        print("Hello, ${name}! total=${total}")
    } else if total < 0 {
        print("negative")
    } else {
        print(-9223372036854775808)
    }
}
`
	got, diags, lexDiags, _ := run(t, program, src)
	want := `(program (module main) (fn add (a:Int b:Int) -> Int (block (return (+ a b)))) ` +
		`(fn main () (block (let name "Tread") (let mut total:Int (call add 2 3)) (assign total (+ total 1)) ` +
		`(if (>= total 6) (block (call print (interp "Hello, " name "! total=" total))) ` +
		`else (if (< total 0) (block (call print "negative")) else (block (call print (- 9223372036854775808))))))))`
	if got != want || len(diags)+len(lexDiags) != 0 {
		t.Errorf("got  %s\nwant %s\ndiags %v %v", got, want, diags, lexDiags)
	}
}

// TestEveryParserCodeIsTested enforces docs/08 §8.1 for parser codes.
func TestEveryParserCodeIsTested(t *testing.T) {
	seen := map[string]bool{}
	for _, tc := range parseCases {
		for _, d := range tc.diags {
			seen["TR-"+strings.SplitN(d, "@", 2)[0]] = true
		}
	}
	var missing []string
	for _, c := range diagnostic.Registry() {
		if c.Phase() == 'P' && !seen[string(c)] {
			missing = append(missing, string(c))
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("parser codes never produced by a required case: %v", missing)
	}
}

func TestParseWithoutEOFToken(t *testing.T) {
	prog, diags := Parse(nil)
	if prog == nil || len(diags) != 0 {
		t.Errorf("empty token slice: got %v %v", prog, diags)
	}
}

package compiler_test

import (
	"math"
	"strings"
	"testing"

	"github.com/sarvessveeriyah2312/AURA-Advanced_Unified_Runtime_Architecture/internal/ast"
	"github.com/sarvessveeriyah2312/AURA-Advanced_Unified_Runtime_Architecture/internal/bytecode"
	"github.com/sarvessveeriyah2312/AURA-Advanced_Unified_Runtime_Architecture/internal/checker"
	"github.com/sarvessveeriyah2312/AURA-Advanced_Unified_Runtime_Architecture/internal/compiler"
	"github.com/sarvessveeriyah2312/AURA-Advanced_Unified_Runtime_Architecture/internal/diagnostic"
	"github.com/sarvessveeriyah2312/AURA-Advanced_Unified_Runtime_Architecture/internal/lexer"
	"github.com/sarvessveeriyah2312/AURA-Advanced_Unified_Runtime_Architecture/internal/parser"
	"github.com/sarvessveeriyah2312/AURA-Advanced_Unified_Runtime_Architecture/internal/source"
)

// frontend lexes, parses and checks src, failing on any diagnostic.
func frontend(t *testing.T, src string) *checker.CheckedProgram {
	t.Helper()
	f := source.NewFile(1, "t.tr", []byte(src))
	toks, ld := lexer.Lex(f)
	prog, pd := parser.Parse(toks)
	cp, cd := checker.Check(prog)
	if all := append(append(ld, pd...), cd...); len(all) > 0 {
		t.Fatalf("frontend diagnostics: %+v\n%s", all, src)
	}
	return cp
}

func compileSrc(t *testing.T, src string) *bytecode.Module {
	t.Helper()
	m, diags := compiler.Compile(frontend(t, src))
	if len(diags) > 0 {
		t.Fatalf("compile diagnostics: %+v", diags)
	}
	if err := m.Validate(); err != nil {
		t.Fatalf("compiled module is invalid: %v", err)
	}
	return m
}

func mainOf(src string) string { return "fn main() -> Unit {\n" + src + "\n}\n" }

// listing returns "OP" or "OP A" for each instruction of the named
// function.
func listing(t *testing.T, m *bytecode.Module, name string) []string {
	t.Helper()
	for _, fn := range m.Functions {
		if fn.Name != name {
			continue
		}
		var out []string
		for _, in := range fn.Code {
			s := in.Op.String()
			if in.Op.Operand() != bytecode.NoOperand {
				s += " " + itoa(in.A)
			}
			out = append(out, s)
		}
		return out
	}
	t.Fatalf("no function %s", name)
	return nil
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}

func opsOf(t *testing.T, m *bytecode.Module, name string) map[string]bool {
	set := map[string]bool{}
	for _, l := range listing(t, m, name) {
		set[strings.Fields(l)[0]] = true
	}
	return set
}

func requireOps(t *testing.T, got map[string]bool, want ...string) {
	t.Helper()
	for _, w := range want {
		if !got[w] {
			t.Errorf("missing opcode %s", w)
		}
	}
}

func TestG1Negation(t *testing.T) {
	m := compileSrc(t, mainOf("let x = 1\nlet y = -x\nlet f = 1.5\nlet g = -f"))
	requireOps(t, opsOf(t, m, "main"), "NEG_INT", "NEG_FLOAT")
}

func TestG2MinimumInt(t *testing.T) {
	m := compileSrc(t, mainOf("let a = -9223372036854775808"))
	got := strings.Join(listing(t, m, "main"), "; ")
	if got != "CONST 0; STORE_LOCAL 0; UNIT; RETURN" {
		t.Errorf("listing = %s", got)
	}
	if len(m.Constants) != 1 || m.Constants[0].Kind != bytecode.KindInt || m.Constants[0].Int != math.MinInt64 {
		t.Errorf("constants = %v", m.Constants)
	}
}

const s68 = "let score = 70\nif score >= 80 {\n    print(\"A\")\n} else if score >= 60 {\n    print(\"B\")\n} else {\n    print(\"C\")\n}"

func TestG3ElseIfLayoutAndDeterminism(t *testing.T) {
	m := compileSrc(t, mainOf(s68))
	want := []string{
		"CONST 0", "STORE_LOCAL 0", // let score = 70
		"LOAD_LOCAL 0", "CONST 1", "GE_INT", "JUMP_IF_FALSE 10",
		"CONST 2", "PRINT_STRING", "POP", "JUMP 21",
		"LOAD_LOCAL 0", "CONST 3", "GE_INT", "JUMP_IF_FALSE 18",
		"CONST 4", "PRINT_STRING", "POP", "JUMP 21",
		"CONST 5", "PRINT_STRING", "POP",
		"UNIT", "RETURN",
	}
	if got := listing(t, m, "main"); strings.Join(got, "; ") != strings.Join(want, "; ") {
		t.Errorf("listing:\n got  %v\n want %v", got, want)
	}
	if a, b := m.Disassemble(), compileSrc(t, mainOf(s68)).Disassemble(); a != b {
		t.Error("compilation is not deterministic")
	}
}

func TestG4InvariantViolations(t *testing.T) {
	const src = "fn main() -> Unit {\n    let x = 1 + 2\n    print(x)\n}\n"
	tests := map[string]func(cp *checker.CheckedProgram){
		"missing main": func(cp *checker.CheckedProgram) { cp.Main = nil },
		// The opcode for `1 + 2` is selected from the left operand's
		// checked type.
		"missing operand type": func(cp *checker.CheckedProgram) {
			for e := range cp.Types {
				if bin, ok := e.(*ast.BinaryExpr); ok {
					delete(cp.Types, bin.Left)
				}
			}
		},
		"missing symbol": func(cp *checker.CheckedProgram) {
			for id := range cp.Uses {
				if id.Name == "x" {
					delete(cp.Uses, id)
				}
			}
		},
		"missing int value": func(cp *checker.CheckedProgram) {
			for e := range cp.IntValues {
				delete(cp.IntValues, e)
			}
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			cp := frontend(t, src)
			mutate(cp)
			m, diags := compiler.Compile(cp)
			if m != nil || len(diags) == 0 || diags[0].Code != diagnostic.InternalCompilerError {
				t.Errorf("want TR-C001 and no module, got %v %+v", m, diags)
			}
		})
	}
	if m, diags := compiler.Compile(nil); m != nil || len(diags) != 1 || diags[0].Code != diagnostic.InternalCompilerError {
		t.Errorf("nil program: %v %+v", m, diags)
	}
}

func TestG5Interpolation(t *testing.T) {
	m := compileSrc(t, mainOf("let i = 1\nlet f = 1.5\nlet b = true\nlet s = \"x\"\nlet r = \"a${i}${f}${b}${s}\""))
	got := strings.Join(listing(t, m, "main")[8:18], "; ")
	want := "CONST 4; LOAD_LOCAL 0; TO_STRING_INT; LOAD_LOCAL 1; TO_STRING_FLOAT; LOAD_LOCAL 2; TO_STRING_BOOL; LOAD_LOCAL 3; BUILD_STRING 5; STORE_LOCAL 4"
	if got != want {
		t.Errorf("listing:\n got  %s\n want %s", got, want)
	}
}

func TestG6Arithmetic(t *testing.T) {
	m := compileSrc(t, mainOf(`let a = 7
let b = 2
let x1 = a + b
let x2 = a - b
let x3 = a * b
let x4 = a / b
let x5 = a % b
let f = 1.5
let y1 = f + f
let y2 = f - f
let y3 = f * f
let y4 = f / f
let s = "a" + "b"`))
	requireOps(t, opsOf(t, m, "main"), "ADD_INT", "SUB_INT", "MUL_INT", "DIV_INT", "MOD_INT",
		"ADD_FLOAT", "SUB_FLOAT", "MUL_FLOAT", "DIV_FLOAT", "CONCAT_STRING")
}

func TestG7Comparisons(t *testing.T) {
	m := compileSrc(t, mainOf(`let i = 1
let f = 1.5
let s = "s"
let b = true
let c1 = i == i
let c2 = f == f
let c3 = s == s
let c4 = b == b
let c5 = i != i
let c6 = f != f
let c7 = s != s
let c8 = b != b
let c9 = i < i
let c10 = f < f
let c11 = i <= i
let c12 = f <= f
let c13 = i > i
let c14 = f > f
let c15 = i >= i
let c16 = f >= f`))
	requireOps(t, opsOf(t, m, "main"), "EQ_INT", "EQ_FLOAT", "EQ_STRING", "EQ_BOOL", "NE_INT", "NE_FLOAT",
		"NE_STRING", "NE_BOOL", "LT_INT", "LT_FLOAT", "LE_INT", "LE_FLOAT", "GT_INT", "GT_FLOAT", "GE_INT", "GE_FLOAT")
}

func TestG8PrintAndToString(t *testing.T) {
	m := compileSrc(t, mainOf(`print(1)
print(1.5)
print("s")
print(true)
let a = toString(1)
let b = toString(1.5)
let c = toString(true)
let d = toString("s")`))
	ops := listing(t, m, "main")
	requireOps(t, opsOf(t, m, "main"), "PRINT_INT", "PRINT_FLOAT", "PRINT_STRING", "PRINT_BOOL",
		"TO_STRING_INT", "TO_STRING_FLOAT", "TO_STRING_BOOL")
	// toString("s") is just the constant: CONST, STORE_LOCAL.
	tail := strings.Join(ops[len(ops)-4:], "; ")
	if !strings.HasPrefix(tail, "CONST") || !strings.Contains(tail, "STORE_LOCAL 3") {
		t.Errorf("toString(String) should emit no conversion; tail = %s", tail)
	}
}

func TestG9ShortCircuitAndNot(t *testing.T) {
	m := compileSrc(t, mainOf("let a = true\nlet b = false\nlet x = a && b\nlet y = a || b\nlet z = !a"))
	got := strings.Join(listing(t, m, "main")[4:], "; ")
	want := "LOAD_LOCAL 0; JUMP_IF_FALSE 8; LOAD_LOCAL 1; JUMP 9; CONST 1; STORE_LOCAL 2; " +
		"LOAD_LOCAL 0; JUMP_IF_FALSE 14; CONST 0; JUMP 15; LOAD_LOCAL 1; STORE_LOCAL 3; " +
		"LOAD_LOCAL 0; NOT; STORE_LOCAL 4; UNIT; RETURN"
	if got != want {
		t.Errorf("listing:\n got  %s\n want %s", got, want)
	}
}

func TestG10Slots(t *testing.T) {
	m := compileSrc(t, "fn f(a: Int, b: Int) -> Int {\n    let c = a\n    if true {\n        let c = b\n        return c\n    }\n    return c\n}\n"+mainOf("print(f(1, 2))"))
	for _, fn := range m.Functions {
		if fn.Name == "f" {
			if fn.NumParams != 2 || fn.NumLocals != 4 {
				t.Errorf("params=%d locals=%d, want 2, 4", fn.NumParams, fn.NumLocals)
			}
		}
	}
	got := strings.Join(listing(t, m, "f"), "; ")
	want := "LOAD_LOCAL 0; STORE_LOCAL 2; CONST 0; JUMP_IF_FALSE 8; LOAD_LOCAL 1; STORE_LOCAL 3; LOAD_LOCAL 3; RETURN; LOAD_LOCAL 2; RETURN"
	if got != want {
		t.Errorf("listing:\n got  %s\n want %s", got, want)
	}
}

func TestG11Constants(t *testing.T) {
	m := compileSrc(t, mainOf("print(1)\nprint(2)\nprint(1)\nprint(1.0)\nprint(\"a\")\nprint(\"a\")\nprint(true)\nprint(1.0)"))
	var got []string
	for _, c := range m.Constants {
		got = append(got, c.Kind.String()+":"+c.String())
	}
	if want := "Int:1 Int:2 Float:1 String:\"a\" Bool:true"; strings.Join(got, " ") != want {
		t.Errorf("constants = %v, want %s", got, want)
	}
}

func TestG12UnitReturnsAndCallStatements(t *testing.T) {
	m := compileSrc(t, "fn u(b: Bool) -> Unit {\n    if b {\n        return\n    }\n    print(1)\n}\n"+mainOf("u(true)"))
	if got := strings.Join(listing(t, m, "u"), "; "); got != "LOAD_LOCAL 0; JUMP_IF_FALSE 4; UNIT; RETURN; CONST 0; PRINT_INT; POP; UNIT; RETURN" {
		t.Errorf("u: %s", got)
	}
	if got := strings.Join(listing(t, m, "main"), "; "); got != "CONST 1; CALL 0; POP; UNIT; RETURN" {
		t.Errorf("main: %s", got)
	}
	// A non-Unit function whose if/else both return gets an unreachable
	// epilogue so the jump to the end has a valid target.
	m = compileSrc(t, "fn g(b: Bool) -> Int {\n    if b {\n        return 1\n    } else {\n        return 2\n    }\n}\n"+mainOf("print(g(true))"))
	if l := listing(t, m, "g"); l[len(l)-2] != "UNIT" || l[len(l)-1] != "RETURN" {
		t.Errorf("g: %v", l)
	}
	m = compileSrc(t, "fn h() -> Int {\n    return 1\n}\n"+mainOf("print(h())"))
	if got := strings.Join(listing(t, m, "h"), "; "); got != "CONST 0; RETURN" {
		t.Errorf("h should have no epilogue: %s", got)
	}
}

func TestG13Spans(t *testing.T) {
	src := "fn f(a: Int, b: Int) -> Int {\n    return a / b\n}\n" + mainOf("print(f(1, 2))")
	m := compileSrc(t, src)
	text := func(fnName string, op bytecode.Opcode) string {
		for _, fn := range m.Functions {
			if fn.Name != fnName {
				continue
			}
			for i, in := range fn.Code {
				if in.Op == op {
					return src[fn.Spans[i].Start:fn.Spans[i].End]
				}
			}
		}
		return "<none>"
	}
	if got := text("f", bytecode.OpDivInt); got != "a / b" {
		t.Errorf("DIV_INT span = %q", got)
	}
	if got := text("main", bytecode.OpCall); got != "f(1, 2)" {
		t.Errorf("CALL span = %q", got)
	}
	if got := text("main", bytecode.OpPrintInt); got != "print(f(1, 2))" {
		t.Errorf("PRINT_INT span = %q", got)
	}
}

func TestG14FunctionIndexes(t *testing.T) {
	m := compileSrc(t, "fn a() -> Int {\n    return 1\n}\n"+mainOf("print(a() + b())")+"fn b() -> Int {\n    return 2\n}\n")
	var names []string
	for _, fn := range m.Functions {
		names = append(names, fn.Name)
	}
	if strings.Join(names, ",") != "a,main,b" || m.Main != 1 {
		t.Errorf("functions %v main=%d", names, m.Main)
	}
	if l := listing(t, m, "main"); l[0] != "CALL 0" || l[1] != "CALL 2" {
		t.Errorf("calls: %v", l)
	}
}

func TestG15LetAndAssign(t *testing.T) {
	m := compileSrc(t, mainOf("let mut x = 1\nx = x + 2"))
	if got := strings.Join(listing(t, m, "main"), "; "); got != "CONST 0; STORE_LOCAL 0; LOAD_LOCAL 0; CONST 1; ADD_INT; STORE_LOCAL 0; UNIT; RETURN" {
		t.Errorf("listing = %s", got)
	}
}

// TestEveryOpcodeIsEmitted: across representative programs, the compiler
// emits all 46 opcodes (docs/10 §10.6).
func TestEveryOpcodeIsEmitted(t *testing.T) {
	programs := []string{
		mainOf(s68),
		mainOf("let x = 1\nlet y = -x\nlet f = 1.5\nlet g = -f\nlet a = x + x - x * x / x % x\nlet b = f + f - f * f / f"),
		mainOf("let s = \"a\" + \"b\"\nlet i = \"${1}${1.5}${true}${s}\"\nlet n = !true && false || true"),
		mainOf("let i = 1\nlet f = 1.0\nlet s = \"\"\nlet b = true\nlet r = i == i && f == f && s == s && b == b && i != i && f != f && s != s && b != b\nlet q = i < i && f < f && i <= i && f <= f && i > i && f > f && i >= i && f >= f"),
		mainOf("print(1)\nprint(1.5)\nprint(\"s\")\nprint(true)\nlet t = toString(1) + toString(1.5) + toString(false)"),
		"fn f() -> Int {\n    return 1\n}\n" + mainOf("let x = f()\nf()"),
	}
	seen := map[bytecode.Opcode]bool{}
	for _, src := range programs {
		m := compileSrc(t, src)
		for _, fn := range m.Functions {
			for _, in := range fn.Code {
				seen[in.Op] = true
			}
		}
	}
	for _, op := range bytecode.AllOpcodes() {
		if !seen[op] {
			t.Errorf("opcode %s is never emitted", op)
		}
	}
	if len(bytecode.AllOpcodes()) != 46 {
		t.Errorf("instruction set has %d opcodes, docs/05 §5.3 specifies 46", len(bytecode.AllOpcodes()))
	}
}

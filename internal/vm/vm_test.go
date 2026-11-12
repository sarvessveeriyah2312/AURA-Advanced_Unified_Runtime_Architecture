package vm_test

import (
	"errors"
	"math/rand"
	"strings"
	"testing"

	"github.com/sarvessveeriyah2312/AURA-Advanced_Unified_Runtime_Architecture/internal/bytecode"
	"github.com/sarvessveeriyah2312/AURA-Advanced_Unified_Runtime_Architecture/internal/checker"
	"github.com/sarvessveeriyah2312/AURA-Advanced_Unified_Runtime_Architecture/internal/compiler"
	"github.com/sarvessveeriyah2312/AURA-Advanced_Unified_Runtime_Architecture/internal/diagnostic"
	"github.com/sarvessveeriyah2312/AURA-Advanced_Unified_Runtime_Architecture/internal/lexer"
	"github.com/sarvessveeriyah2312/AURA-Advanced_Unified_Runtime_Architecture/internal/parser"
	"github.com/sarvessveeriyah2312/AURA-Advanced_Unified_Runtime_Architecture/internal/source"
	"github.com/sarvessveeriyah2312/AURA-Advanced_Unified_Runtime_Architecture/internal/vm"
)

// ---------------------------------------------------------------------------
// Helpers

func in(op bytecode.Opcode, a ...int) bytecode.Instr {
	i := bytecode.Instr{Op: op}
	if len(a) > 0 {
		i.A = a[0]
	}
	return i
}

// fn builds a function whose instruction i has the span [i, i+1).
func fn(name string, params, locals int, code ...bytecode.Instr) bytecode.Function {
	spans := make([]source.Span, len(code))
	for i := range spans {
		spans[i] = source.Span{File: 1, Start: i, End: i + 1}
	}
	return bytecode.Function{Name: name, NumParams: params, NumLocals: locals, Code: code, Spans: spans}
}

func exec(t *testing.T, m *bytecode.Module, cfg vm.Config) (string, *vm.Error) {
	t.Helper()
	var out strings.Builder
	err := vm.Run(m, &out, cfg)
	if err == nil {
		return out.String(), nil
	}
	var ve *vm.Error
	if !errors.As(err, &ve) {
		t.Fatalf("unexpected non-runtime error: %v", err)
	}
	return out.String(), ve
}

// runSrc runs Tread source through lex, parse, check, compile and execute.
func runSrc(t *testing.T, src string, cfg vm.Config) (string, *vm.Error) {
	t.Helper()
	f := source.NewFile(1, "t.tr", []byte(src))
	toks, ld := lexer.Lex(f)
	prog, pd := parser.Parse(toks)
	cp, cd := checker.Check(prog)
	if all := append(append(ld, pd...), cd...); len(all) > 0 {
		t.Fatalf("frontend diagnostics: %+v\n%s", all, src)
	}
	m, diags := compiler.Compile(cp)
	if len(diags) > 0 {
		t.Fatalf("compile diagnostics: %+v", diags)
	}
	return exec(t, m, cfg)
}

func mainOf(body string) string { return "fn main() -> Unit {\n" + body + "\n}\n" }

func spanText(src string, e *vm.Error) string {
	if e == nil || !e.HasSpan {
		return "<no span>"
	}
	return src[e.Span.Start:e.Span.End]
}

var (
	iv = bytecode.IntValue
	fv = bytecode.FloatValue
	sv = bytecode.StringValue
	bv = bytecode.BoolValue
)

// ---------------------------------------------------------------------------
// V20: stack effect of every opcode (docs/05 §5.3)

type opCase struct {
	name   string
	consts []bytecode.Value
	locals int
	code   []bytecode.Instr // main body; UNIT RETURN is appended
	extra  []bytecode.Function
	want   string
}

func binCase(op, print bytecode.Opcode, a, b bytecode.Value, want string) opCase {
	return opCase{name: op.String(), consts: []bytecode.Value{a, b}, want: want,
		code: []bytecode.Instr{in(bytecode.OpConst, 0), in(bytecode.OpConst, 1), in(op), in(print), in(bytecode.OpPop)}}
}

func unCase(op, print bytecode.Opcode, a bytecode.Value, want string) opCase {
	return opCase{name: op.String(), consts: []bytecode.Value{a}, want: want,
		code: []bytecode.Instr{in(bytecode.OpConst, 0), in(op), in(print), in(bytecode.OpPop)}}
}

var (
	pInt    = bytecode.OpPrintInt
	pFloat  = bytecode.OpPrintFloat
	pString = bytecode.OpPrintString
	pBool   = bytecode.OpPrintBool
)

var opCases = []opCase{
	{name: "CONST+PRINT_INT", consts: []bytecode.Value{iv(42)}, code: []bytecode.Instr{in(bytecode.OpConst, 0), in(pInt), in(bytecode.OpPop)}, want: "42\n"},
	{name: "UNIT+POP", code: []bytecode.Instr{in(bytecode.OpUnit), in(bytecode.OpPop)}, want: ""},
	{name: "POP removes only the top", consts: []bytecode.Value{iv(1), iv(2)},
		code: []bytecode.Instr{in(bytecode.OpConst, 0), in(bytecode.OpConst, 1), in(bytecode.OpPop), in(pInt), in(bytecode.OpPop)}, want: "1\n"},
	{name: "STORE_LOCAL+LOAD_LOCAL", locals: 1, consts: []bytecode.Value{iv(7)},
		code: []bytecode.Instr{in(bytecode.OpConst, 0), in(bytecode.OpStoreLocal, 0), in(bytecode.OpLoadLocal, 0), in(pInt), in(bytecode.OpPop)}, want: "7\n"},
	binCase(bytecode.OpAddInt, pInt, iv(2), iv(3), "5\n"),
	binCase(bytecode.OpSubInt, pInt, iv(2), iv(3), "-1\n"),
	binCase(bytecode.OpMulInt, pInt, iv(4), iv(-3), "-12\n"),
	binCase(bytecode.OpDivInt, pInt, iv(-7), iv(2), "-3\n"),
	binCase(bytecode.OpModInt, pInt, iv(-7), iv(3), "-1\n"),
	unCase(bytecode.OpNegInt, pInt, iv(5), "-5\n"),
	binCase(bytecode.OpAddFloat, pFloat, fv(1.5), fv(2.5), "4.0\n"),
	binCase(bytecode.OpSubFloat, pFloat, fv(1.5), fv(2), "-0.5\n"),
	binCase(bytecode.OpMulFloat, pFloat, fv(1.5), fv(2), "3.0\n"),
	binCase(bytecode.OpDivFloat, pFloat, fv(1), fv(4), "0.25\n"),
	unCase(bytecode.OpNegFloat, pFloat, fv(0), "-0.0\n"),
	binCase(bytecode.OpConcatString, pString, sv("ab"), sv("cd"), "abcd\n"),
	{name: "BUILD_STRING", consts: []bytecode.Value{sv("a"), sv("b"), sv("c")},
		code: []bytecode.Instr{in(bytecode.OpConst, 0), in(bytecode.OpConst, 1), in(bytecode.OpConst, 2), in(bytecode.OpBuildString, 3), in(pString), in(bytecode.OpPop)}, want: "abc\n"},
	unCase(bytecode.OpNot, pBool, bv(true), "false\n"),
	binCase(bytecode.OpEqInt, pBool, iv(1), iv(1), "true\n"),
	binCase(bytecode.OpEqFloat, pBool, fv(1), fv(2), "false\n"),
	binCase(bytecode.OpEqString, pBool, sv("a"), sv("a"), "true\n"),
	binCase(bytecode.OpEqBool, pBool, bv(true), bv(false), "false\n"),
	binCase(bytecode.OpNeInt, pBool, iv(1), iv(2), "true\n"),
	binCase(bytecode.OpNeFloat, pBool, fv(1), fv(1), "false\n"),
	binCase(bytecode.OpNeString, pBool, sv("a"), sv("b"), "true\n"),
	binCase(bytecode.OpNeBool, pBool, bv(true), bv(true), "false\n"),
	binCase(bytecode.OpLtInt, pBool, iv(1), iv(2), "true\n"),
	binCase(bytecode.OpLtFloat, pBool, fv(2), fv(1), "false\n"),
	binCase(bytecode.OpLeInt, pBool, iv(2), iv(2), "true\n"),
	binCase(bytecode.OpLeFloat, pBool, fv(3), fv(2), "false\n"),
	binCase(bytecode.OpGtInt, pBool, iv(3), iv(2), "true\n"),
	binCase(bytecode.OpGtFloat, pBool, fv(2), fv(2), "false\n"),
	binCase(bytecode.OpGeInt, pBool, iv(2), iv(2), "true\n"),
	binCase(bytecode.OpGeFloat, pBool, fv(1), fv(2), "false\n"),
	{name: "JUMP", consts: []bytecode.Value{iv(1), iv(2)},
		code: []bytecode.Instr{in(bytecode.OpJump, 3), in(bytecode.OpConst, 0), in(pInt), in(bytecode.OpConst, 1), in(pInt), in(bytecode.OpPop)}, want: "2\n"},
	{name: "JUMP_IF_FALSE taken", consts: []bytecode.Value{bv(false), iv(1)},
		code: []bytecode.Instr{in(bytecode.OpConst, 0), in(bytecode.OpJumpIfFalse, 5), in(bytecode.OpConst, 1), in(pInt), in(bytecode.OpPop)}, want: ""},
	{name: "JUMP_IF_FALSE not taken", consts: []bytecode.Value{bv(true), iv(1)},
		code: []bytecode.Instr{in(bytecode.OpConst, 0), in(bytecode.OpJumpIfFalse, 5), in(bytecode.OpConst, 1), in(pInt), in(bytecode.OpPop)}, want: "1\n"},
	{name: "CALL+RETURN", consts: []bytecode.Value{iv(10), iv(3)},
		// main: sub(10, 3); sub returns a - b from slots 0 and 1.
		code:  []bytecode.Instr{in(bytecode.OpConst, 0), in(bytecode.OpConst, 1), in(bytecode.OpCall, 1), in(pInt), in(bytecode.OpPop)},
		extra: []bytecode.Function{fn("sub", 2, 2, in(bytecode.OpLoadLocal, 0), in(bytecode.OpLoadLocal, 1), in(bytecode.OpSubInt), in(bytecode.OpReturn))},
		want:  "7\n"},
	{name: "PRINT_FLOAT", consts: []bytecode.Value{fv(2.5)}, code: []bytecode.Instr{in(bytecode.OpConst, 0), in(pFloat), in(bytecode.OpPop)}, want: "2.5\n"},
	{name: "PRINT_STRING", consts: []bytecode.Value{sv("hi")}, code: []bytecode.Instr{in(bytecode.OpConst, 0), in(pString), in(bytecode.OpPop)}, want: "hi\n"},
	{name: "PRINT_BOOL pushes Unit", consts: []bytecode.Value{bv(false)}, code: []bytecode.Instr{in(bytecode.OpConst, 0), in(pBool), in(bytecode.OpStoreLocal, 0)}, locals: 1, want: "false\n"},
	unCase(bytecode.OpToStringInt, pString, iv(-12), "-12\n"),
	unCase(bytecode.OpToStringFloat, pString, fv(1), "1.0\n"),
	unCase(bytecode.OpToStringBool, pString, bv(true), "true\n"),
}

func (c opCase) module() *bytecode.Module {
	code := append(append([]bytecode.Instr(nil), c.code...), in(bytecode.OpUnit), in(bytecode.OpReturn))
	fns := append([]bytecode.Function{fn("main", 0, c.locals, code...)}, c.extra...)
	return &bytecode.Module{Constants: c.consts, Functions: fns}
}

func TestV20OpcodeStackEffects(t *testing.T) {
	seen := map[bytecode.Opcode]bool{}
	for _, c := range opCases {
		t.Run(c.name, func(t *testing.T) {
			m := c.module()
			for _, f := range m.Functions {
				for _, i := range f.Code {
					seen[i.Op] = true
				}
			}
			out, err := exec(t, m, vm.Config{})
			if err != nil || out != c.want {
				t.Errorf("got %q, %v; want %q", out, err, c.want)
			}
		})
	}
	for _, op := range bytecode.AllOpcodes() {
		if !seen[op] {
			t.Errorf("opcode %s has no stack-effect test", op)
		}
	}
}

// Every value-consuming opcode reports stack underflow (R002) instead of
// panicking when its operands are missing.
func TestV3UnderflowForEveryOpcode(t *testing.T) {
	for _, op := range bytecode.AllOpcodes() {
		var code []bytecode.Instr
		switch op.Operand() {
		case bytecode.NoOperand:
			code = []bytecode.Instr{in(op)}
		case bytecode.CountOperand:
			code = []bytecode.Instr{in(op, 2)}
		case bytecode.SlotOperand, bytecode.TargetOperand:
			if op != bytecode.OpStoreLocal && op != bytecode.OpJumpIfFalse {
				continue // these push or jump without consuming
			}
			code = []bytecode.Instr{in(op, 0)}
		case bytecode.FuncOperand:
			code = []bytecode.Instr{in(op, 1)}
		default:
			continue
		}
		if op == bytecode.OpUnit {
			continue
		}
		code = append(code, in(bytecode.OpUnit), in(bytecode.OpReturn))
		m := &bytecode.Module{Functions: []bytecode.Function{fn("main", 0, 1, code...), fn("two", 2, 2, in(bytecode.OpUnit), in(bytecode.OpReturn))}}
		_, err := exec(t, m, vm.Config{})
		if err == nil || err.Code != vm.CodeInvalidBytecode || !err.HasSpan || err.Span.Start != 0 {
			t.Errorf("%s with an empty stack: got %+v, want R002 at instruction 0", op, err)
		}
	}
}

// ---------------------------------------------------------------------------
// V2, V3: invalid bytecode

func validModule() *bytecode.Module {
	return &bytecode.Module{
		Constants: []bytecode.Value{sv("ran")},
		Functions: []bytecode.Function{fn("main", 0, 1,
			in(bytecode.OpConst, 0), in(bytecode.OpPrintString), in(bytecode.OpPop), in(bytecode.OpUnit), in(bytecode.OpReturn))},
	}
}

func TestV2ValidationErrors(t *testing.T) {
	tests := map[string]func(m *bytecode.Module){
		"jump target":         func(m *bytecode.Module) { m.Functions[0].Code[3] = in(bytecode.OpJump, 99) },
		"constant index":      func(m *bytecode.Module) { m.Functions[0].Code[0].A = 5 },
		"local slot":          func(m *bytecode.Module) { m.Functions[0].Code[3] = in(bytecode.OpLoadLocal, 1) },
		"function index":      func(m *bytecode.Module) { m.Functions[0].Code[3] = in(bytecode.OpCall, 3) },
		"unknown opcode":      func(m *bytecode.Module) { m.Functions[0].Code[3] = bytecode.Instr{Op: 200} },
		"zero opcode":         func(m *bytecode.Module) { m.Functions[0].Code[3] = bytecode.Instr{} },
		"operand on no-op":    func(m *bytecode.Module) { m.Functions[0].Code[2].A = 1 },
		"BUILD_STRING 0":      func(m *bytecode.Module) { m.Functions[0].Code[3] = in(bytecode.OpBuildString, 0) },
		"main index":          func(m *bytecode.Module) { m.Main = 1 },
		"main has parameters": func(m *bytecode.Module) { m.Functions[0].NumParams = 1 },
		"spans length":        func(m *bytecode.Module) { m.Functions[0].Spans = m.Functions[0].Spans[:2] },
		"params above locals": func(m *bytecode.Module) {
			m.Functions = append(m.Functions, fn("f", 3, 1, in(bytecode.OpUnit), in(bytecode.OpReturn)))
		},
		"empty body":    func(m *bytecode.Module) { m.Functions = append(m.Functions, fn("f", 0, 0)) },
		"negative slot": func(m *bytecode.Module) { m.Functions[0].Code[3] = in(bytecode.OpStoreLocal, -1) },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			m := validModule()
			mutate(m)
			out, err := exec(t, m, vm.Config{})
			if err == nil || err.Code != vm.CodeInvalidBytecode || err.HasSpan {
				t.Errorf("want R002 without span, got %+v", err)
			}
			if out != "" {
				t.Errorf("an invalid module must not execute; output %q", out)
			}
		})
	}
	if out, err := exec(t, validModule(), vm.Config{}); err != nil || out != "ran\n" {
		t.Fatalf("the unmodified module must run: %q %v", out, err)
	}
	if err := vm.Execute(nil, &strings.Builder{}); err == nil {
		t.Error("nil module must be rejected")
	}
}

func TestV3DynamicChecks(t *testing.T) {
	tests := map[string]struct {
		consts []bytecode.Value
		code   []bytecode.Instr
		pc     int
	}{
		"stack underflow": {code: []bytecode.Instr{in(bytecode.OpAddInt), in(bytecode.OpReturn)}, pc: 0},
		"wrong tag":       {consts: []bytecode.Value{sv("x")}, code: []bytecode.Instr{in(bytecode.OpConst, 0), in(bytecode.OpNegInt), in(bytecode.OpReturn)}, pc: 1},
		"bool tag":        {consts: []bytecode.Value{iv(1)}, code: []bytecode.Instr{in(bytecode.OpConst, 0), in(bytecode.OpJumpIfFalse, 0)}, pc: 1},
		"build non-string": {consts: []bytecode.Value{iv(1)},
			code: []bytecode.Instr{in(bytecode.OpConst, 0), in(bytecode.OpBuildString, 1), in(bytecode.OpReturn)}, pc: 1},
		"ran off the end":  {consts: []bytecode.Value{iv(1)}, code: []bytecode.Instr{in(bytecode.OpConst, 0), in(bytecode.OpPop)}, pc: 1},
		"return underflow": {code: []bytecode.Instr{in(bytecode.OpReturn)}, pc: 0},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			m := &bytecode.Module{Constants: tt.consts, Functions: []bytecode.Function{fn("main", 0, 0, tt.code...)}}
			_, err := exec(t, m, vm.Config{})
			if err == nil || err.Code != vm.CodeInvalidBytecode || !err.HasSpan || err.Span.Start != tt.pc || err.Function != "main" {
				t.Errorf("want R002 at pc %d in main, got %+v", tt.pc, err)
			}
		})
	}
}

// TestRandomBytecodeNeverPanics feeds random modules to the VM.
func TestRandomBytecodeNeverPanics(t *testing.T) {
	rng := rand.New(rand.NewSource(4))
	ops := bytecode.AllOpcodes()
	consts := []bytecode.Value{iv(0), iv(-1), iv(1 << 62), fv(0), sv("s"), bv(true), bytecode.UnitValue()}
	for i := 0; i < 20000; i++ {
		var fns []bytecode.Function
		nf := 1 + rng.Intn(3)
		for f := 0; f < nf; f++ {
			n := 1 + rng.Intn(12)
			code := make([]bytecode.Instr, n)
			for j := range code {
				op := ops[rng.Intn(len(ops))]
				code[j] = bytecode.Instr{Op: op, A: rng.Intn(n + 2)}
				if op.Operand() == bytecode.NoOperand {
					code[j].A = 0
				}
			}
			params := 0
			if f > 0 {
				params = rng.Intn(3)
			}
			fns = append(fns, fn("f", params, params+rng.Intn(3), code...))
		}
		m := &bytecode.Module{Constants: consts, Functions: fns}
		var out strings.Builder
		_ = vm.Run(m, &out, vm.Config{MaxInstructions: 500, MaxCallDepth: 50})
	}
}

// ---------------------------------------------------------------------------
// Source-level behavior (V1, V4-V19)

func TestV1DivisionByZero(t *testing.T) {
	for _, expr := range []string{"1 / z", "1 % z"} {
		src := mainOf("let z = 0\nprint(" + expr + ")")
		_, err := runSrc(t, src, vm.Config{})
		if err == nil || err.Code != vm.CodeDivisionByZero || spanText(src, err) != expr {
			t.Errorf("%s: got %+v (span %q)", expr, err, spanText(src, err))
		}
	}
}

func TestV4CallDepth(t *testing.T) {
	src := "fn f(n: Int) -> Int {\n    return f(n + 1)\n}\n" + mainOf("print(f(0))")
	_, err := runSrc(t, src, vm.Config{})
	if err == nil || err.Code != vm.CodeCallDepth || spanText(src, err) != "f(n + 1)" || err.Function != "f" {
		t.Errorf("got %+v (span %q)", err, spanText(src, err))
	}
	_, err = runSrc(t, src, vm.Config{MaxCallDepth: 5})
	if err == nil || err.Code != vm.CodeCallDepth {
		t.Errorf("custom depth: %+v", err)
	}
}

func TestV5InstructionLimit(t *testing.T) {
	src := "fn f(n: Int) -> Int {\n    if n == 0 {\n        return 0\n    }\n    return f(n - 1)\n}\n" + mainOf("print(f(100))")
	if out, err := runSrc(t, src, vm.Config{}); err != nil || out != "0\n" {
		t.Fatalf("unlimited run: %q %+v", out, err)
	}
	_, err := runSrc(t, src, vm.Config{MaxInstructions: 50})
	if err == nil || err.Code != vm.CodeInstructionLimit {
		t.Errorf("got %+v", err)
	}
}

const minmax = "let max = 9223372036854775807\nlet min = -9223372036854775808\n"

func TestV6IntegerOverflow(t *testing.T) {
	for _, expr := range []string{"max + 1", "min - 1", "max * 2", "-min", "min / -1", "min * -1", "-1 * min"} {
		src := mainOf(minmax + "print(" + expr + ")")
		_, err := runSrc(t, src, vm.Config{})
		if err == nil || err.Code != vm.CodeIntegerOverflow || spanText(src, err) != expr {
			t.Errorf("%s: got %+v (span %q)", expr, err, spanText(src, err))
		}
	}
	// Results at the boundary are not overflow.
	out, err := runSrc(t, mainOf(minmax+"print(max - 1 + 1)\nprint(min + 1 - 1)\nprint(-max)\nprint(max * 1)\nprint(min / 1)"), vm.Config{})
	if err != nil || out != "9223372036854775807\n-9223372036854775808\n-9223372036854775807\n9223372036854775807\n-9223372036854775808\n" {
		t.Errorf("boundaries: %q %+v", out, err)
	}
}

func TestV7MinModMinusOne(t *testing.T) {
	out, err := runSrc(t, mainOf(minmax+"print(min % -1)"), vm.Config{})
	if err != nil || out != "0\n" {
		t.Errorf("got %q %+v", out, err)
	}
}

func TestV8DivisionAndRemainder(t *testing.T) {
	out, err := runSrc(t, mainOf("print(7 / 2)\nprint(-7 / 2)\nprint(7 / -2)\nprint(7 % 3)\nprint(-7 % 3)\nprint(7 % -3)"), vm.Config{})
	if err != nil || out != "3\n-3\n-3\n1\n-1\n1\n" {
		t.Errorf("got %q %+v", out, err)
	}
}

func TestV9FloatIEEE(t *testing.T) {
	big := "1" + strings.Repeat("0", 308) + ".0"
	out, err := runSrc(t, mainOf("print(1.0 / 0.0)\nprint(-1.0 / 0.0)\nprint(0.0 / 0.0)\nprint("+big+" * 10.0)"), vm.Config{})
	if err != nil || out != "+Inf\n-Inf\nNaN\n+Inf\n" {
		t.Errorf("got %q %+v", out, err)
	}
}

func TestV10NaNComparisons(t *testing.T) {
	out, err := runSrc(t, mainOf("let n = 0.0 / 0.0\nprint(n == n)\nprint(n != n)\nprint(n < 1.0)\nprint(n <= n)\nprint(n > 1.0)\nprint(n >= n)"), vm.Config{})
	if err != nil || out != "false\ntrue\nfalse\nfalse\nfalse\nfalse\n" {
		t.Errorf("got %q %+v", out, err)
	}
}

func TestV12Print(t *testing.T) {
	out, err := runSrc(t, mainOf("print(42)\nprint(-7)\nprint(1.5)\nprint(2.0)\nprint(true)\nprint(\"hi\")\nprint(\"\")"), vm.Config{})
	if err != nil || out != "42\n-7\n1.5\n2.0\ntrue\nhi\n\n" {
		t.Errorf("got %q %+v", out, err)
	}
}

// TestV13FormattingConsistency: print, toString and interpolation produce
// identical text for the same value (ADR-0004 items 12, 13).
func TestV13FormattingConsistency(t *testing.T) {
	for _, v := range []string{"42", "-7", "1.0", "1.25", "-0.0", "0.1", "1000000.0", "0.00001", "true", "false", "\"s\"", "0.0 / 0.0", "1.0 / 0.0", "-1.0 / 0.0"} {
		src := mainOf("print(" + v + ")\nprint(toString(" + v + "))\nprint(\"${" + v + "}\")")
		out, err := runSrc(t, src, vm.Config{})
		lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
		if err != nil || len(lines) != 3 || lines[0] != lines[1] || lines[1] != lines[2] {
			t.Errorf("%s: inconsistent output %q %+v", v, out, err)
		}
	}
	out, _ := runSrc(t, mainOf("print(1000000.0)\nprint(-0.0)"), vm.Config{})
	if out != "1e+06\n-0.0\n" {
		t.Errorf("got %q", out)
	}
}

func TestV14Calls(t *testing.T) {
	src := `fn fact(n: Int) -> Int {
    if n <= 1 {
        return 1
    }
    return n * fact(n - 1)
}

fn add(a: Int, b: Int) -> Int {
    return a + b
}

fn mul(a: Int, b: Int) -> Int {
    let r = a * b
    return r
}

fn shadow(a: Int) -> Int {
    let a = a + 100
    return a
}

fn countdown(n: Int) -> Unit {
    let local = n * 10
    if n > 0 {
        countdown(n - 1)
    }
    print(local)
}

fn main() -> Unit {
    print(fact(20))
    print(add(mul(2, 3), 4))
    print(shadow(1))
    countdown(2)
    let x = add(1, 2)
    add(5, 5)
    print(x)
}
`
	out, err := runSrc(t, src, vm.Config{})
	if err != nil || out != "2432902008176640000\n10\n101\n0\n10\n20\n3\n" {
		t.Errorf("got %q %+v", out, err)
	}
}

func TestV15Conditionals(t *testing.T) {
	grade := "fn grade(score: Int) -> Unit {\n    if score >= 80 {\n        print(\"A\")\n    } else if score >= 60 {\n        print(\"B\")\n    } else {\n        print(\"C\")\n    }\n}\n"
	out, err := runSrc(t, grade+mainOf("grade(90)\ngrade(70)\ngrade(10)\nif false { print(\"no\") }\nif true { print(\"yes\") }"), vm.Config{})
	if err != nil || out != "A\nB\nC\nyes\n" {
		t.Errorf("got %q %+v", out, err)
	}
}

func TestV16ShortCircuit(t *testing.T) {
	src := "fn f() -> Bool {\n    print(\"called\")\n    return true\n}\n" +
		mainOf("print(false && f())\nprint(true || f())\nprint(true && f())\nprint(false || f())")
	out, err := runSrc(t, src, vm.Config{})
	if err != nil || out != "false\ntrue\ncalled\ntrue\ncalled\ntrue\n" {
		t.Errorf("got %q %+v", out, err)
	}
}

func TestV17StringsAndInterpolation(t *testing.T) {
	src := mainOf("let name = \"Tread\"\nlet n = 3\nprint(\"Hello, \" + name + \"!\")\nprint(\"${name} has ${n} items, ratio ${1.5 * 2.0}, ok=${n > 2}\")\nprint(\"\\${literal} $5\")")
	out, err := runSrc(t, src, vm.Config{})
	if err != nil || out != "Hello, Tread!\nTread has 3 items, ratio 3.0, ok=true\n${literal} $5\n" {
		t.Errorf("got %q %+v", out, err)
	}
}

func TestV18ErrorInCallee(t *testing.T) {
	src := "fn div(a: Int, b: Int) -> Int {\n    return a / b\n}\n" + mainOf("print(div(1, 0))")
	_, err := runSrc(t, src, vm.Config{})
	if err == nil || err.Code != vm.CodeDivisionByZero || err.Function != "div" || spanText(src, err) != "a / b" {
		t.Errorf("got %+v (span %q)", err, spanText(src, err))
	}
	f := source.NewFile(1, "t", []byte(src))
	if p := f.Position(err.Span.Start); p.Line != 2 || p.Column != 12 {
		t.Errorf("position %d:%d, want 2:12", p.Line, p.Column)
	}
}

func TestV19OutputBeforeError(t *testing.T) {
	out, err := runSrc(t, mainOf("print(\"before\")\nlet z = 0\nprint(1 / z)\nprint(\"after\")"), vm.Config{})
	if err == nil || out != "before\n" {
		t.Errorf("got %q %+v", out, err)
	}
}

// ---------------------------------------------------------------------------
// Misc

func TestRuntimeCodesAreRegistered(t *testing.T) {
	registered := map[string]bool{}
	for _, c := range diagnostic.Registry() {
		registered[string(c)] = true
	}
	for _, c := range []string{vm.CodeDivisionByZero, vm.CodeInvalidBytecode, vm.CodeCallDepth, vm.CodeInstructionLimit, vm.CodeIntegerOverflow} {
		if !registered[c] {
			t.Errorf("%s is not in the diagnostic registry", c)
		}
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("disk full") }

func TestWriterErrorIsReturned(t *testing.T) {
	err := vm.Run(validModule(), failingWriter{}, vm.Config{})
	var ve *vm.Error
	if err == nil || errors.As(err, &ve) || err.Error() != "disk full" {
		t.Errorf("writer errors must be returned unchanged, got %v", err)
	}
}

func TestErrorString(t *testing.T) {
	e := &vm.Error{Code: vm.CodeDivisionByZero, Message: "division by zero", Function: "f"}
	if e.Error() != "TR-R001: division by zero (in f)" {
		t.Errorf("Error() = %q", e.Error())
	}
}

// TestRandomProgramsNeverPanic runs random well-typed-or-not source
// through the full pipeline; programs that pass the checker are compiled
// and executed with limits.
func TestRandomProgramsNeverPanic(t *testing.T) {
	frags := []string{"let a = 1", "let b = 2.5", "let s = \"x\"", "print(a)", "print(b)", "print(s + s)",
		"print(\"${a}${b}\")", "print(a / 0)", "print(a % 0)", "print(9223372036854775807 + a)",
		"print(b / 0.0)", "if a > 0 { print(a) } else { print(b) }", "print(toString(b))", "print(-a)",
		"print(a == a && b < b || s != s)", "print(!true)"}
	rng := rand.New(rand.NewSource(5))
	for i := 0; i < 2000; i++ {
		var body []string
		for j, n := 0, rng.Intn(8); j < n; j++ {
			body = append(body, frags[rng.Intn(len(frags))])
		}
		src := mainOf("let a = 1\nlet b = 2.5\nlet s = \"x\"\n" + strings.Join(body, "\n"))
		f := source.NewFile(1, "t", []byte(src))
		toks, _ := lexer.Lex(f)
		prog, _ := parser.Parse(toks)
		cp, cd := checker.Check(prog)
		if len(cd) > 0 {
			continue
		}
		m, diags := compiler.Compile(cp)
		if len(diags) > 0 {
			t.Fatalf("compile failed for a checked program: %+v\n%s", diags, src)
		}
		var out strings.Builder
		_ = vm.Run(m, &out, vm.Config{MaxInstructions: 10000})
	}
}

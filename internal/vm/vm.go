package vm

import (
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"

	"github.com/your-org/aura/internal/bytecode"
	"github.com/your-org/aura/internal/source"
)

// Runtime error codes (docs/05 §5.9). They are the registry strings of
// docs/04 §4.4.3; this package may not import internal/diagnostic
// (ADR-0001), so the CLI converts them.
const (
	CodeDivisionByZero   = "AURA-R001"
	CodeInvalidBytecode  = "AURA-R002"
	CodeCallDepth        = "AURA-R003"
	CodeInstructionLimit = "AURA-R004"
	CodeIntegerOverflow  = "AURA-R005"
)

// Error is a structured runtime error. HasSpan is false for errors found
// by validation before execution.
type Error struct {
	Code     string
	Message  string
	Function string
	Span     source.Span
	HasSpan  bool
}

func (e *Error) Error() string {
	if e.Function == "" {
		return e.Code + ": " + e.Message
	}
	return fmt.Sprintf("%s: %s (in %s)", e.Code, e.Message, e.Function)
}

// Config sets execution limits. Zero values mean: MaxCallDepth uses the
// default, MaxInstructions is unlimited.
type Config struct {
	MaxCallDepth    int
	MaxInstructions int64
}

// DefaultMaxCallDepth is the call depth limit when Config.MaxCallDepth is
// zero (docs/05 §5.5).
const DefaultMaxCallDepth = 10000

// Execute runs m with default limits, writing program output to out.
func Execute(m *bytecode.Module, out io.Writer) error {
	return Run(m, out, Config{})
}

// Run validates and executes m. It returns nil on success, a *Error for
// an AURA runtime error or invalid bytecode, or the writer's error if
// output fails. It never panics.
func Run(m *bytecode.Module, out io.Writer, cfg Config) error {
	if m == nil {
		return &Error{Code: CodeInvalidBytecode, Message: "no module"}
	}
	if err := m.Validate(); err != nil {
		return &Error{Code: CodeInvalidBytecode, Message: err.Error()}
	}
	if cfg.MaxCallDepth <= 0 {
		cfg.MaxCallDepth = DefaultMaxCallDepth
	}
	vm := &machine{mod: m, out: out, cfg: cfg}
	return vm.run()
}

type frame struct {
	fn        *bytecode.Function
	ip        int
	locals    []bytecode.Value
	stackBase int
}

type machine struct {
	mod    *bytecode.Module
	out    io.Writer
	cfg    Config
	stack  []bytecode.Value
	frames []frame
	// position of the instruction being executed, for errors
	fn   *bytecode.Function
	span source.Span
}

func (m *machine) fail(code, format string, args ...any) *Error {
	return &Error{Code: code, Message: fmt.Sprintf(format, args...), Function: m.fn.Name, Span: m.span, HasSpan: true}
}

func (m *machine) invalid(format string, args ...any) *Error {
	return m.fail(CodeInvalidBytecode, "invalid bytecode: "+format, args...)
}

func (m *machine) push(v bytecode.Value) { m.stack = append(m.stack, v) }

// pop removes the top value, which must belong to the current frame.
func (m *machine) pop() (bytecode.Value, *Error) {
	base := m.frames[len(m.frames)-1].stackBase
	if len(m.stack) <= base {
		return bytecode.Value{}, m.invalid("stack underflow")
	}
	v := m.stack[len(m.stack)-1]
	m.stack = m.stack[:len(m.stack)-1]
	return v, nil
}

func (m *machine) popKind(k bytecode.Kind) (bytecode.Value, *Error) {
	v, err := m.pop()
	if err != nil {
		return v, err
	}
	if v.Kind != k {
		return v, m.invalid("expected %s operand, found %s", k, v.Kind)
	}
	return v, nil
}

func (m *machine) pop2(k bytecode.Kind) (a, b bytecode.Value, err *Error) {
	if b, err = m.popKind(k); err != nil {
		return
	}
	a, err = m.popKind(k)
	return
}

func (m *machine) run() error {
	main := &m.mod.Functions[m.mod.Main]
	m.frames = []frame{{fn: main, locals: make([]bytecode.Value, main.NumLocals)}}
	var executed int64
	for {
		fr := &m.frames[len(m.frames)-1]
		m.fn = fr.fn
		if fr.ip >= len(fr.fn.Code) {
			m.span = fr.fn.Spans[len(fr.fn.Spans)-1]
			return m.invalid("execution ran past the end of function %s", fr.fn.Name)
		}
		in := fr.fn.Code[fr.ip]
		m.span = fr.fn.Spans[fr.ip]
		fr.ip++
		executed++
		if m.cfg.MaxInstructions > 0 && executed > m.cfg.MaxInstructions {
			return m.fail(CodeInstructionLimit, "instruction limit of %d exceeded", m.cfg.MaxInstructions)
		}
		done, err := m.step(fr, in)
		if err != nil {
			return err
		}
		if done {
			return nil
		}
	}
}

// step executes one instruction. It reports done when main returns.
func (m *machine) step(fr *frame, in bytecode.Instr) (done bool, err error) {
	var e *Error
	switch in.Op {
	case bytecode.OpConst:
		m.push(m.mod.Constants[in.A])
	case bytecode.OpUnit:
		m.push(bytecode.UnitValue())
	case bytecode.OpPop:
		_, e = m.pop()
	case bytecode.OpLoadLocal:
		m.push(fr.locals[in.A])
	case bytecode.OpStoreLocal:
		var v bytecode.Value
		if v, e = m.pop(); e == nil {
			fr.locals[in.A] = v
		}

	case bytecode.OpAddInt, bytecode.OpSubInt, bytecode.OpMulInt, bytecode.OpDivInt, bytecode.OpModInt:
		e = m.intBinary(in.Op)
	case bytecode.OpNegInt:
		var a bytecode.Value
		if a, e = m.popKind(bytecode.KindInt); e == nil {
			if a.Int == math.MinInt64 {
				e = m.fail(CodeIntegerOverflow, "integer overflow: cannot negate %d", a.Int)
			} else {
				m.push(bytecode.IntValue(-a.Int))
			}
		}

	case bytecode.OpAddFloat, bytecode.OpSubFloat, bytecode.OpMulFloat, bytecode.OpDivFloat:
		var a, b bytecode.Value
		if a, b, e = m.pop2(bytecode.KindFloat); e == nil {
			m.push(bytecode.FloatValue(floatBinary(in.Op, a.Float, b.Float)))
		}
	case bytecode.OpNegFloat:
		var a bytecode.Value
		if a, e = m.popKind(bytecode.KindFloat); e == nil {
			m.push(bytecode.FloatValue(-a.Float))
		}

	case bytecode.OpConcatString:
		var a, b bytecode.Value
		if a, b, e = m.pop2(bytecode.KindString); e == nil {
			m.push(bytecode.StringValue(a.Str + b.Str))
		}
	case bytecode.OpBuildString:
		e = m.buildString(in.A)
	case bytecode.OpNot:
		var a bytecode.Value
		if a, e = m.popKind(bytecode.KindBool); e == nil {
			m.push(bytecode.BoolValue(!a.Bool))
		}

	case bytecode.OpEqInt, bytecode.OpNeInt, bytecode.OpLtInt, bytecode.OpLeInt, bytecode.OpGtInt, bytecode.OpGeInt,
		bytecode.OpEqFloat, bytecode.OpNeFloat, bytecode.OpLtFloat, bytecode.OpLeFloat, bytecode.OpGtFloat, bytecode.OpGeFloat,
		bytecode.OpEqString, bytecode.OpNeString, bytecode.OpEqBool, bytecode.OpNeBool:
		e = m.compare(in.Op)

	case bytecode.OpJump:
		fr.ip = in.A
	case bytecode.OpJumpIfFalse:
		var c bytecode.Value
		if c, e = m.popKind(bytecode.KindBool); e == nil && !c.Bool {
			fr.ip = in.A
		}
	case bytecode.OpCall:
		e = m.call(in.A)
	case bytecode.OpReturn:
		var v bytecode.Value
		if v, e = m.pop(); e != nil {
			break
		}
		m.stack = m.stack[:fr.stackBase]
		m.frames = m.frames[:len(m.frames)-1]
		if len(m.frames) == 0 {
			return true, nil
		}
		m.push(v)

	case bytecode.OpPrintInt, bytecode.OpPrintFloat, bytecode.OpPrintString, bytecode.OpPrintBool:
		var v bytecode.Value
		if v, e = m.popKind(printKind[in.Op]); e != nil {
			break
		}
		if _, werr := io.WriteString(m.out, Format(v)+"\n"); werr != nil {
			return false, werr
		}
		m.push(bytecode.UnitValue())
	case bytecode.OpToStringInt, bytecode.OpToStringFloat, bytecode.OpToStringBool:
		var v bytecode.Value
		if v, e = m.popKind(toStringKind[in.Op]); e == nil {
			m.push(bytecode.StringValue(Format(v)))
		}

	default:
		e = m.invalid("unknown opcode %d", in.Op)
	}
	if e != nil {
		return false, e
	}
	return false, nil
}

var printKind = map[bytecode.Opcode]bytecode.Kind{
	bytecode.OpPrintInt: bytecode.KindInt, bytecode.OpPrintFloat: bytecode.KindFloat,
	bytecode.OpPrintString: bytecode.KindString, bytecode.OpPrintBool: bytecode.KindBool,
}

var toStringKind = map[bytecode.Opcode]bytecode.Kind{
	bytecode.OpToStringInt: bytecode.KindInt, bytecode.OpToStringFloat: bytecode.KindFloat,
	bytecode.OpToStringBool: bytecode.KindBool,
}

// intBinary implements checked Int arithmetic (ADR-0004 item 11).
func (m *machine) intBinary(op bytecode.Opcode) *Error {
	a, b, e := m.pop2(bytecode.KindInt)
	if e != nil {
		return e
	}
	x, y := a.Int, b.Int
	var r int64
	switch op {
	case bytecode.OpAddInt:
		r = x + y
		if (r > x) != (y > 0) {
			return m.fail(CodeIntegerOverflow, "integer overflow: %d + %d", x, y)
		}
	case bytecode.OpSubInt:
		r = x - y
		if (r < x) != (y > 0) {
			return m.fail(CodeIntegerOverflow, "integer overflow: %d - %d", x, y)
		}
	case bytecode.OpMulInt:
		if x != 0 && y != 0 {
			r = x * y
			if r/y != x || (x == -1 && y == math.MinInt64) || (y == -1 && x == math.MinInt64) {
				return m.fail(CodeIntegerOverflow, "integer overflow: %d * %d", x, y)
			}
		}
	case bytecode.OpDivInt:
		if y == 0 {
			return m.fail(CodeDivisionByZero, "division by zero")
		}
		if x == math.MinInt64 && y == -1 {
			return m.fail(CodeIntegerOverflow, "integer overflow: %d / -1", x)
		}
		r = x / y // truncates toward zero
	case bytecode.OpModInt:
		if y == 0 {
			return m.fail(CodeDivisionByZero, "remainder by zero")
		}
		if y == -1 {
			r = 0 // also covers the minimum Int % -1
		} else {
			r = x % y // sign of the dividend
		}
	}
	m.push(bytecode.IntValue(r))
	return nil
}

func floatBinary(op bytecode.Opcode, a, b float64) float64 {
	switch op {
	case bytecode.OpAddFloat:
		return a + b
	case bytecode.OpSubFloat:
		return a - b
	case bytecode.OpMulFloat:
		return a * b
	}
	return a / b // IEEE-754: division by zero gives ±Inf or NaN
}

func (m *machine) compare(op bytecode.Opcode) *Error {
	var kind bytecode.Kind
	switch op {
	case bytecode.OpEqInt, bytecode.OpNeInt, bytecode.OpLtInt, bytecode.OpLeInt, bytecode.OpGtInt, bytecode.OpGeInt:
		kind = bytecode.KindInt
	case bytecode.OpEqFloat, bytecode.OpNeFloat, bytecode.OpLtFloat, bytecode.OpLeFloat, bytecode.OpGtFloat, bytecode.OpGeFloat:
		kind = bytecode.KindFloat
	case bytecode.OpEqString, bytecode.OpNeString:
		kind = bytecode.KindString
	default:
		kind = bytecode.KindBool
	}
	a, b, e := m.pop2(kind)
	if e != nil {
		return e
	}
	var r bool
	switch op {
	case bytecode.OpEqInt:
		r = a.Int == b.Int
	case bytecode.OpNeInt:
		r = a.Int != b.Int
	case bytecode.OpLtInt:
		r = a.Int < b.Int
	case bytecode.OpLeInt:
		r = a.Int <= b.Int
	case bytecode.OpGtInt:
		r = a.Int > b.Int
	case bytecode.OpGeInt:
		r = a.Int >= b.Int
	// Go's float comparisons are IEEE-754: NaN is unordered.
	case bytecode.OpEqFloat:
		r = a.Float == b.Float
	case bytecode.OpNeFloat:
		r = a.Float != b.Float
	case bytecode.OpLtFloat:
		r = a.Float < b.Float
	case bytecode.OpLeFloat:
		r = a.Float <= b.Float
	case bytecode.OpGtFloat:
		r = a.Float > b.Float
	case bytecode.OpGeFloat:
		r = a.Float >= b.Float
	case bytecode.OpEqString:
		r = a.Str == b.Str
	case bytecode.OpNeString:
		r = a.Str != b.Str
	case bytecode.OpEqBool:
		r = a.Bool == b.Bool
	case bytecode.OpNeBool:
		r = a.Bool != b.Bool
	}
	m.push(bytecode.BoolValue(r))
	return nil
}

func (m *machine) buildString(n int) *Error {
	base := m.frames[len(m.frames)-1].stackBase
	if len(m.stack)-base < n {
		return m.invalid("stack underflow: BUILD_STRING %d", n)
	}
	parts := m.stack[len(m.stack)-n:]
	var b strings.Builder
	for _, p := range parts {
		if p.Kind != bytecode.KindString {
			return m.invalid("BUILD_STRING operand is %s, not String", p.Kind)
		}
		b.WriteString(p.Str)
	}
	m.stack = m.stack[:len(m.stack)-n]
	m.push(bytecode.StringValue(b.String()))
	return nil
}

func (m *machine) call(index int) *Error {
	callee := &m.mod.Functions[index]
	if len(m.frames) >= m.cfg.MaxCallDepth {
		return m.fail(CodeCallDepth, "call depth limit of %d exceeded calling %s", m.cfg.MaxCallDepth, callee.Name)
	}
	n := callee.NumParams
	if len(m.stack)-m.frames[len(m.frames)-1].stackBase < n {
		return m.invalid("stack underflow: %s needs %d arguments", callee.Name, n)
	}
	locals := make([]bytecode.Value, callee.NumLocals)
	copy(locals, m.stack[len(m.stack)-n:])
	m.stack = m.stack[:len(m.stack)-n]
	m.frames = append(m.frames, frame{fn: callee, locals: locals, stackBase: len(m.stack)})
	return nil
}

// Format returns the program-visible text of a value (docs/05 §5.8,
// ADR-0004 item 12). print, toString and interpolation all use it.
func Format(v bytecode.Value) string {
	switch v.Kind {
	case bytecode.KindInt:
		return strconv.FormatInt(v.Int, 10)
	case bytecode.KindFloat:
		return FormatFloat(v.Float)
	case bytecode.KindBool:
		return strconv.FormatBool(v.Bool)
	case bytecode.KindString:
		return v.Str
	}
	return "unit"
}

// FormatFloat renders f as the shortest round-trip representation, with
// ".0" appended to finite results that would otherwise look like an Int.
func FormatFloat(f float64) string {
	s := strconv.FormatFloat(f, 'g', -1, 64)
	if math.IsInf(f, 0) || math.IsNaN(f) || strings.ContainsAny(s, ".e") {
		return s
	}
	return s + ".0"
}

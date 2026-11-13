package bytecode

import (
	"fmt"
	"math"
	"strings"

	"github.com/sarvessveeriyah2312/Tread/internal/source"
)

// Kind is the runtime tag of a Value (docs/05 §5.2).
type Kind uint8

const (
	KindUnit Kind = iota
	KindInt
	KindFloat
	KindBool
	KindString
	// Heap values (ADR-0013). Constants are never heap values.
	KindArray
	KindMap
	KindEnum
	// KindAny is the result kind of a function returning a type
	// parameter. Values never have this kind.
	KindAny
)

func (k Kind) String() string {
	switch k {
	case KindUnit:
		return "Unit"
	case KindInt:
		return "Int"
	case KindFloat:
		return "Float"
	case KindBool:
		return "Bool"
	case KindString:
		return "String"
	case KindArray:
		return "Array"
	case KindMap:
		return "Map"
	case KindEnum:
		return "Enum"
	case KindAny:
		return "Any"
	}
	return "Kind(?)"
}

// Value is a tagged runtime value. Only the field matching Kind is
// meaningful.
type Value struct {
	Kind  Kind
	Int   int64
	Float float64
	Bool  bool
	Str   string
	// Ref is the payload of a heap value: *Array, *Map or *EnumValue.
	Ref any
}

// Constructors.
func UnitValue() Value           { return Value{Kind: KindUnit} }
func IntValue(v int64) Value     { return Value{Kind: KindInt, Int: v} }
func FloatValue(v float64) Value { return Value{Kind: KindFloat, Float: v} }
func BoolValue(v bool) Value     { return Value{Kind: KindBool, Bool: v} }
func StringValue(v string) Value { return Value{Kind: KindString, Str: v} }

// String renders the value for disassembly (not program output).
func (v Value) String() string {
	switch v.Kind {
	case KindInt:
		return fmt.Sprintf("%d", v.Int)
	case KindFloat:
		return fmt.Sprintf("%v", v.Float)
	case KindBool:
		return fmt.Sprintf("%t", v.Bool)
	case KindString:
		return fmt.Sprintf("%q", v.Str)
	}
	return "unit"
}

// Instr is one instruction: an opcode and its operands A and B (0 when
// unused). B is used only by path and variant instructions (ADR-0013).
type Instr struct {
	Op Opcode
	A  int
	B  int
}

// Function is a compiled function record (docs/05 §5.4).
type Function struct {
	Name      string
	NumParams int
	NumLocals int
	Result    Kind
	Code      []Instr
	// Spans is the source mapping table: Spans[i] is the source span of
	// Code[i].
	Spans []source.Span
}

// Module is a complete compiled program.
type Module struct {
	Constants []Value
	Functions []Function
	Main      int
	// Enums describes every enum type; Option is always enum 0
	// (ADR-0013).
	Enums []EnumInfo
}

// MaxPathDepth bounds the B operand of path instructions.
const MaxPathDepth = 1 << 16

// ValidationError describes why a module was rejected (docs/05 §5.5).
type ValidationError struct {
	Function string // empty for module-level errors
	PC       int    // -1 for function- or module-level errors
	Msg      string
}

func (e *ValidationError) Error() string {
	switch {
	case e.Function == "":
		return "invalid module: " + e.Msg
	case e.PC < 0:
		return fmt.Sprintf("invalid function %s: %s", e.Function, e.Msg)
	}
	return fmt.Sprintf("invalid instruction %s@%d: %s", e.Function, e.PC, e.Msg)
}

// Validate checks every static invariant of docs/05 §5.5. A valid module
// can still fail at run time (stack underflow, wrong operand tags), which
// the VM checks dynamically.
func (m *Module) Validate() error {
	if m.Main < 0 || m.Main >= len(m.Functions) {
		return &ValidationError{PC: -1, Msg: fmt.Sprintf("main index %d out of range", m.Main)}
	}
	if n := m.Functions[m.Main].NumParams; n != 0 {
		return &ValidationError{PC: -1, Msg: fmt.Sprintf("main must have no parameters, has %d", n)}
	}
	for fi := range m.Functions {
		fn := &m.Functions[fi]
		bad := func(pc int, format string, args ...any) error {
			return &ValidationError{Function: fn.Name, PC: pc, Msg: fmt.Sprintf(format, args...)}
		}
		switch {
		case len(fn.Code) == 0:
			return bad(-1, "empty body")
		case len(fn.Spans) != len(fn.Code):
			return bad(-1, "source mapping has %d entries for %d instructions", len(fn.Spans), len(fn.Code))
		case fn.NumParams < 0 || fn.NumParams > fn.NumLocals:
			return bad(-1, "%d parameters but %d locals", fn.NumParams, fn.NumLocals)
		}
		for pc, in := range fn.Code {
			if !in.Op.Valid() {
				return bad(pc, "unknown opcode %d", in.Op)
			}
			var limit int
			if k := in.Op.Operand(); k != PathOperand && k != VariantOperand && in.B != 0 {
				return bad(pc, "%s takes no second operand, has %d", in.Op, in.B)
			}
			switch in.Op.Operand() {
			case NoOperand:
				if in.A != 0 {
					return bad(pc, "%s takes no operand, has %d", in.Op, in.A)
				}
				continue
			case SizeOperand, FieldOperand:
				if in.A < 0 || in.A > maxCount {
					return bad(pc, "%s operand %d out of range", in.Op, in.A)
				}
				continue
			case PathOperand:
				if in.B < 0 || in.B > MaxPathDepth || (in.Op == OpStorePath && in.B < 1) {
					return bad(pc, "%s path depth %d out of range", in.Op, in.B)
				}
				limit = fn.NumLocals
			case VariantOperand:
				if in.A < 0 || in.A >= len(m.Enums) {
					return bad(pc, "%s enum %d out of range [0,%d)", in.Op, in.A, len(m.Enums))
				}
				if in.B < 0 || in.B >= len(m.Enums[in.A].Variants) {
					return bad(pc, "%s variant %d out of range for enum %s", in.Op, in.B, m.Enums[in.A].Name)
				}
				continue
			case ConstOperand:
				limit = len(m.Constants)
			case SlotOperand:
				limit = fn.NumLocals
			case TargetOperand:
				limit = len(fn.Code)
			case FuncOperand:
				limit = len(m.Functions)
			case CountOperand:
				if in.A < 1 {
					return bad(pc, "%s count must be at least 1, is %d", in.Op, in.A)
				}
				continue
			}
			if in.A < 0 || in.A >= limit {
				return bad(pc, "%s operand %d out of range [0,%d)", in.Op, in.A, limit)
			}
		}
	}
	return nil
}

// Disassemble renders the module deterministically, for tests and
// debugging.
func (m *Module) Disassemble() string {
	var b strings.Builder
	fmt.Fprintf(&b, "main=%d\n", m.Main)
	for i, e := range m.Enums {
		fmt.Fprintf(&b, "enum %d %s", i, e.Name)
		for _, v := range e.Variants {
			fmt.Fprintf(&b, " %s/%d", v.Name, v.Fields)
		}
		b.WriteByte('\n')
	}
	for i, c := range m.Constants {
		fmt.Fprintf(&b, "const %d %s %s\n", i, c.Kind, c)
	}
	for fi, fn := range m.Functions {
		fmt.Fprintf(&b, "fn %d %s params=%d locals=%d result=%s\n", fi, fn.Name, fn.NumParams, fn.NumLocals, fn.Result)
		for pc, in := range fn.Code {
			switch in.Op.Operand() {
			case NoOperand:
				fmt.Fprintf(&b, "  %3d %s\n", pc, in.Op)
			case PathOperand, VariantOperand:
				fmt.Fprintf(&b, "  %3d %s %d %d\n", pc, in.Op, in.A, in.B)
			default:
				fmt.Fprintf(&b, "  %3d %s %d\n", pc, in.Op, in.A)
			}
		}
	}
	return b.String()
}

// SameFloat reports whether two floats are identical bit patterns (used
// for constant deduplication: -0.0 and 0.0 are distinct).
func SameFloat(a, b float64) bool { return math.Float64bits(a) == math.Float64bits(b) }

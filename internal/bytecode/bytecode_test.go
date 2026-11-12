package bytecode

import (
	"strings"
	"testing"

	"github.com/your-org/aura/internal/source"
)

func TestOpcodeTable(t *testing.T) {
	ops := AllOpcodes()
	if len(ops) != 46 {
		t.Fatalf("%d opcodes, docs/05 §5.3 specifies 46", len(ops))
	}
	seen := map[string]bool{}
	for _, op := range ops {
		name := op.String()
		if name == "" || name == "INVALID" || seen[name] || strings.ToUpper(name) != name {
			t.Errorf("bad or duplicate mnemonic %q for %d", name, op)
		}
		seen[name] = true
	}
	if opInvalid.Valid() || Opcode(200).Valid() || Opcode(200).String() != "INVALID" || Opcode(200).Operand() != NoOperand {
		t.Error("invalid opcodes must be reported as invalid")
	}
	operands := map[Opcode]OperandKind{
		OpConst: ConstOperand, OpLoadLocal: SlotOperand, OpStoreLocal: SlotOperand,
		OpJump: TargetOperand, OpJumpIfFalse: TargetOperand, OpCall: FuncOperand, OpBuildString: CountOperand,
	}
	for _, op := range ops {
		want, ok := operands[op]
		if !ok {
			want = NoOperand
		}
		if op.Operand() != want {
			t.Errorf("%s operand kind = %d, want %d", op, op.Operand(), want)
		}
	}
}

func TestValuesAndKinds(t *testing.T) {
	tests := []struct {
		v    Value
		kind string
		text string
	}{
		{UnitValue(), "Unit", "unit"},
		{IntValue(-3), "Int", "-3"},
		{FloatValue(1.5), "Float", "1.5"},
		{BoolValue(true), "Bool", "true"},
		{StringValue("a\n"), "String", `"a\n"`},
	}
	for _, tt := range tests {
		if tt.v.Kind.String() != tt.kind || tt.v.String() != tt.text {
			t.Errorf("%v: kind %s text %s", tt.v, tt.v.Kind, tt.v.String())
		}
	}
	if Kind(99).String() != "Kind(?)" {
		t.Error("unknown kind name")
	}
	if !SameFloat(1, 1) {
		t.Error("equal floats must be the same constant")
	}
	if SameFloat(0, negZero()) {
		t.Error("0.0 and -0.0 must be distinct constants")
	}
}

func negZero() float64 {
	z := 0.0
	return -z
}

func sampleModule() *Module {
	return &Module{
		Constants: []Value{IntValue(1)},
		Functions: []Function{{
			Name: "main", NumLocals: 1,
			Code:  []Instr{{Op: OpConst}, {Op: OpStoreLocal}, {Op: OpUnit}, {Op: OpReturn}},
			Spans: make([]source.Span, 4),
		}},
	}
}

func TestValidateAndErrors(t *testing.T) {
	if err := sampleModule().Validate(); err != nil {
		t.Fatalf("valid module rejected: %v", err)
	}
	tests := map[string]struct {
		mutate func(m *Module)
		msg    string
	}{
		"module":      {func(m *Module) { m.Main = 3 }, "invalid module: main index 3 out of range"},
		"function":    {func(m *Module) { m.Functions[0].Spans = nil }, "invalid function main: source mapping has 0 entries for 4 instructions"},
		"instruction": {func(m *Module) { m.Functions[0].Code[1].A = 1 }, "invalid instruction main@1: STORE_LOCAL operand 1 out of range [0,1)"},
	}
	for name, tt := range tests {
		m := sampleModule()
		tt.mutate(m)
		err := m.Validate()
		if err == nil || err.Error() != tt.msg {
			t.Errorf("%s: got %v, want %q", name, err, tt.msg)
		}
	}
}

func TestDisassemble(t *testing.T) {
	want := "main=0\nconst 0 Int 1\nfn 0 main params=0 locals=1 result=Unit\n    0 CONST 0\n    1 STORE_LOCAL 0\n    2 UNIT\n    3 RETURN\n"
	if got := sampleModule().Disassemble(); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

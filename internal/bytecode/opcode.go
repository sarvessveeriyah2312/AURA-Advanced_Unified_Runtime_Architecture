package bytecode

// Opcode identifies an instruction. The set and stack effects are
// specified in docs/05 §5.3 (ADR-0004 item 10).
type Opcode uint8

// Opcodes. The zero value is deliberately invalid so that a zeroed
// instruction is rejected by validation.
const (
	opInvalid Opcode = iota

	// Stack, locals and constants.
	OpConst
	OpUnit
	OpPop
	OpLoadLocal
	OpStoreLocal

	// Integer arithmetic (checked).
	OpAddInt
	OpSubInt
	OpMulInt
	OpDivInt
	OpModInt
	OpNegInt

	// Float arithmetic (IEEE-754).
	OpAddFloat
	OpSubFloat
	OpMulFloat
	OpDivFloat
	OpNegFloat

	// Strings and booleans.
	OpConcatString
	OpBuildString
	OpNot

	// Comparison.
	OpEqInt
	OpEqFloat
	OpEqString
	OpEqBool
	OpNeInt
	OpNeFloat
	OpNeString
	OpNeBool
	OpLtInt
	OpLtFloat
	OpLeInt
	OpLeFloat
	OpGtInt
	OpGtFloat
	OpGeInt
	OpGeFloat

	// Control flow and calls.
	OpJump
	OpJumpIfFalse
	OpCall
	OpReturn

	// Output and conversion.
	OpPrintInt
	OpPrintFloat
	OpPrintString
	OpPrintBool
	OpToStringInt
	OpToStringFloat
	OpToStringBool

	// Collections, enums and generic values (ADR-0013).
	OpMakeArray     // A = element count
	OpMakeMap       // A = entry count
	OpMakeEnum      // A = enum, B = variant
	OpLoadPath      // A = slot, B = depth: read local[k1]...[kB]
	OpPeekPath      // like LOAD_PATH, but the value is only read
	OpStorePath     // A = slot, B = depth >= 1: local[k1]...[kB] = value
	OpIndex         // container, key -> element
	OpLen           // array or map -> Int
	OpContains      // container, value -> Bool
	OpMapGet        // map, key -> Option
	OpMapKeys       // map -> Array of keys
	OpPushPath      // A = slot, B = depth: push onto the array at the path
	OpPopPath       // A = slot, B = depth: pop from the array -> Option
	OpRemovePath    // A = slot, B = depth: remove a key -> Option
	OpIsVariant     // A = enum, B = variant: value -> Bool
	OpEnumField     // A = field: enum value -> field value
	OpEqValue       // structural ==
	OpNeValue       // structural !=
	OpPrintValue    // print any value
	OpToStringValue // text of any value

	opCount
)

// OperandKind describes what an instruction's operand A refers to.
type OperandKind uint8

const (
	NoOperand OperandKind = iota
	ConstOperand
	SlotOperand
	TargetOperand
	FuncOperand
	CountOperand
	// ADR-0013.
	SizeOperand    // A >= 0: an element or entry count
	PathOperand    // A = local slot, B = path depth
	VariantOperand // A = enum index, B = variant index
	FieldOperand   // A >= 0: a field index, checked at run time
)

type opInfo struct {
	name    string
	operand OperandKind
}

var opTable = [opCount]opInfo{
	OpConst: {"CONST", ConstOperand}, OpUnit: {"UNIT", NoOperand}, OpPop: {"POP", NoOperand},
	OpLoadLocal: {"LOAD_LOCAL", SlotOperand}, OpStoreLocal: {"STORE_LOCAL", SlotOperand},
	OpAddInt: {"ADD_INT", NoOperand}, OpSubInt: {"SUB_INT", NoOperand}, OpMulInt: {"MUL_INT", NoOperand},
	OpDivInt: {"DIV_INT", NoOperand}, OpModInt: {"MOD_INT", NoOperand}, OpNegInt: {"NEG_INT", NoOperand},
	OpAddFloat: {"ADD_FLOAT", NoOperand}, OpSubFloat: {"SUB_FLOAT", NoOperand}, OpMulFloat: {"MUL_FLOAT", NoOperand},
	OpDivFloat: {"DIV_FLOAT", NoOperand}, OpNegFloat: {"NEG_FLOAT", NoOperand},
	OpConcatString: {"CONCAT_STRING", NoOperand}, OpBuildString: {"BUILD_STRING", CountOperand}, OpNot: {"NOT", NoOperand},
	OpEqInt: {"EQ_INT", NoOperand}, OpEqFloat: {"EQ_FLOAT", NoOperand}, OpEqString: {"EQ_STRING", NoOperand}, OpEqBool: {"EQ_BOOL", NoOperand},
	OpNeInt: {"NE_INT", NoOperand}, OpNeFloat: {"NE_FLOAT", NoOperand}, OpNeString: {"NE_STRING", NoOperand}, OpNeBool: {"NE_BOOL", NoOperand},
	OpLtInt: {"LT_INT", NoOperand}, OpLtFloat: {"LT_FLOAT", NoOperand}, OpLeInt: {"LE_INT", NoOperand}, OpLeFloat: {"LE_FLOAT", NoOperand},
	OpGtInt: {"GT_INT", NoOperand}, OpGtFloat: {"GT_FLOAT", NoOperand}, OpGeInt: {"GE_INT", NoOperand}, OpGeFloat: {"GE_FLOAT", NoOperand},
	OpJump: {"JUMP", TargetOperand}, OpJumpIfFalse: {"JUMP_IF_FALSE", TargetOperand},
	OpCall: {"CALL", FuncOperand}, OpReturn: {"RETURN", NoOperand},
	OpPrintInt: {"PRINT_INT", NoOperand}, OpPrintFloat: {"PRINT_FLOAT", NoOperand},
	OpPrintString: {"PRINT_STRING", NoOperand}, OpPrintBool: {"PRINT_BOOL", NoOperand},
	OpToStringInt: {"TO_STRING_INT", NoOperand}, OpToStringFloat: {"TO_STRING_FLOAT", NoOperand},
	OpToStringBool: {"TO_STRING_BOOL", NoOperand},
	OpMakeArray:    {"MAKE_ARRAY", SizeOperand}, OpMakeMap: {"MAKE_MAP", SizeOperand}, OpMakeEnum: {"MAKE_ENUM", VariantOperand},
	OpLoadPath: {"LOAD_PATH", PathOperand}, OpPeekPath: {"PEEK_PATH", PathOperand}, OpStorePath: {"STORE_PATH", PathOperand},
	OpIndex: {"INDEX", NoOperand}, OpLen: {"LEN", NoOperand}, OpContains: {"CONTAINS", NoOperand},
	OpMapGet: {"MAP_GET", NoOperand}, OpMapKeys: {"MAP_KEYS", NoOperand},
	OpPushPath: {"PUSH_PATH", PathOperand}, OpPopPath: {"POP_PATH", PathOperand}, OpRemovePath: {"REMOVE_PATH", PathOperand},
	OpIsVariant: {"IS_VARIANT", VariantOperand}, OpEnumField: {"ENUM_FIELD", FieldOperand},
	OpEqValue: {"EQ_VALUE", NoOperand}, OpNeValue: {"NE_VALUE", NoOperand},
	OpPrintValue: {"PRINT_VALUE", NoOperand}, OpToStringValue: {"TO_STRING_VALUE", NoOperand},
}

// Valid reports whether o is a defined opcode.
func (o Opcode) Valid() bool { return o > opInvalid && o < opCount }

// String returns the opcode mnemonic, e.g. "ADD_INT".
func (o Opcode) String() string {
	if !o.Valid() {
		return "INVALID"
	}
	return opTable[o].name
}

// Operand returns the kind of operand the opcode takes.
func (o Opcode) Operand() OperandKind {
	if !o.Valid() {
		return NoOperand
	}
	return opTable[o].operand
}

// AllOpcodes returns every defined opcode in numeric order.
func AllOpcodes() []Opcode {
	ops := make([]Opcode, 0, opCount-1)
	for o := opInvalid + 1; o < opCount; o++ {
		ops = append(ops, o)
	}
	return ops
}

package diagnostic

// Code is a stable diagnostic identifier such as "TR-L001". Codes are
// never reused or renumbered (ADR-0004 item 4). The authoritative list is
// the registry in docs/04-compiler-architecture.md §4.4.3.
type Code string

// Lexer codes.
const (
	InvalidCharacter         Code = "TR-L001"
	InvalidUTF8              Code = "TR-L002"
	UnterminatedString       Code = "TR-L003"
	InvalidEscape            Code = "TR-L004"
	UnterminatedBlockComment Code = "TR-L005"
	EmptyInterpolation       Code = "TR-L006"
	UnterminatedInterp       Code = "TR-L007"
	MalformedNumber          Code = "TR-L008"
	LeadingZero              Code = "TR-L009"
)

// Parser codes.
const (
	ExpectedExpression Code = "TR-P001"
	ExpectedIdentifier Code = "TR-P002"
	ReservedWord       Code = "TR-P003"
	ExpectedNewline    Code = "TR-P004"
	SameLine           Code = "TR-P005"
	NonCallStatement   Code = "TR-P006"
	TrailingComma      Code = "TR-P007"
	DuplicateModule    Code = "TR-P008"
	ModuleNotFirst     Code = "TR-P009"
	MissingRParen      Code = "TR-P010"
	MissingRBrace      Code = "TR-P011"
	UnexpectedToken    Code = "TR-P012"
	ExpectedTypeName   Code = "TR-P013"
	NestingTooDeep     Code = "TR-P014"
	// ImportNotFirst: an import after another declaration (ADR-0014).
	ImportNotFirst Code = "TR-P015"
)

// Type checker codes (Phase 2).
const (
	TypeMismatch         Code = "TR-T001"
	UndefinedName        Code = "TR-T002"
	UnknownType          Code = "TR-T003"
	AssignToImmutable    Code = "TR-T004"
	DuplicateDeclaration Code = "TR-T005"
	WrongArgumentCount   Code = "TR-T006"
	InvalidOperandTypes  Code = "TR-T007"
	MissingReturn        Code = "TR-T008"
	InvalidReturn        Code = "TR-T009"
	IntLiteralOutOfRange Code = "TR-T010"
	NotDisplayable       Code = "TR-T011"
	NotCallable          Code = "TR-T012"
	MissingMain          Code = "TR-T013"
	InvalidMainSignature Code = "TR-T014"
	MissingReturnType    Code = "TR-T015"
	BuiltinRedeclared    Code = "TR-T016"
	FloatLiteralRange    Code = "TR-T017"
	FunctionAsValue      Code = "TR-T018"
	// LoopControlOutsideLoop: `break` or `continue` outside a loop
	// (ADR-0012).
	LoopControlOutsideLoop Code = "TR-T019"

	// Collections, enums, pattern matching and generics (ADR-0013).
	CannotInfer         Code = "TR-T020" // add a type annotation
	WrongTypeArguments  Code = "TR-T021" // wrong number of type arguments
	InvalidMapKey       Code = "TR-T022" // map keys must be Int, String or Bool
	NotIndexable        Code = "TR-T023" // cannot index or iterate this type
	UnknownMethod       Code = "TR-T024" // no such method, or a method not called
	EnumAsValue         Code = "TR-T025" // an enum type used as a value
	UnknownVariant      Code = "TR-T026" // no such enum or variant
	NonExhaustiveMatch  Code = "TR-T027" // match misses a case
	UnreachablePattern  Code = "TR-T028" // match arm can never run
	InvalidPattern      Code = "TR-T029" // Float pattern, wrong field count
	UnitInCompositeType Code = "TR-T030" // Unit inside a collection, payload or type argument

	// Structs, methods and imports (ADR-0014).
	InvalidStructLiteral Code = "TR-T031" // missing, unknown or repeated field
	RecursiveStruct      Code = "TR-T032" // struct contains itself directly
	ModuleNameMismatch   Code = "TR-T033" // imported file's module name differs
	PrivateItem          Code = "TR-T034" // not pub in its module
	ImportCycle          Code = "TR-T035" // modules import each other
	ImportNotFound       Code = "TR-T036" // imported file cannot be read
)

// Compiler and runtime codes (Phase 3). The VM cannot import this
// package (ADR-0001); it uses the same strings.
const (
	InternalCompilerError Code = "TR-C001"
	DivisionByZero        Code = "TR-R001"
	InvalidBytecode       Code = "TR-R002"
	CallDepthExceeded     Code = "TR-R003"
	InstructionLimit      Code = "TR-R004"
	IntegerOverflow       Code = "TR-R005"
	// BytecodeLoadError: a .tbc file is malformed, corrupted, or uses an
	// unsupported format version (ADR-0011).
	BytecodeLoadError Code = "TR-R006"
	// ADR-0013.
	IndexOutOfRange Code = "TR-R007"
	KeyNotFound     Code = "TR-R008"
)

// Registry returns every registered code in registry order. The result is
// a fresh slice; callers may modify it.
func Registry() []Code {
	return []Code{
		InvalidCharacter, InvalidUTF8, UnterminatedString, InvalidEscape,
		UnterminatedBlockComment, EmptyInterpolation, UnterminatedInterp,
		MalformedNumber, LeadingZero,
		ExpectedExpression, ExpectedIdentifier, ReservedWord, ExpectedNewline,
		SameLine, NonCallStatement, TrailingComma, DuplicateModule,
		ModuleNotFirst, MissingRParen, MissingRBrace, UnexpectedToken,
		ExpectedTypeName, NestingTooDeep, ImportNotFirst,
		TypeMismatch, UndefinedName, UnknownType, AssignToImmutable,
		DuplicateDeclaration, WrongArgumentCount, InvalidOperandTypes,
		MissingReturn, InvalidReturn, IntLiteralOutOfRange, NotDisplayable,
		NotCallable, MissingMain, InvalidMainSignature, MissingReturnType,
		BuiltinRedeclared, FloatLiteralRange, FunctionAsValue,
		LoopControlOutsideLoop,
		CannotInfer, WrongTypeArguments, InvalidMapKey, NotIndexable, UnknownMethod,
		EnumAsValue, UnknownVariant, NonExhaustiveMatch, UnreachablePattern,
		InvalidPattern, UnitInCompositeType,
		InvalidStructLiteral, RecursiveStruct, ModuleNameMismatch, PrivateItem,
		ImportCycle, ImportNotFound,
		InternalCompilerError,
		DivisionByZero, InvalidBytecode, CallDepthExceeded, InstructionLimit,
		IntegerOverflow, BytecodeLoadError, IndexOutOfRange, KeyNotFound,
	}
}

// Prefix starts every diagnostic code, as in TR-T001.
const Prefix = "TR-"

// Phase returns the phase letter of the code ('L', 'P', 'T', 'C' or 'R'),
// or 0 if the code is malformed. It reads the letter right after Prefix,
// so it does not depend on the prefix's length.
func (c Code) Phase() byte {
	s := string(c)
	if len(s) != len(Prefix)+4 || s[:len(Prefix)] != Prefix {
		return 0
	}
	return s[len(Prefix)]
}

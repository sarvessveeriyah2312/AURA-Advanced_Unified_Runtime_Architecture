package diagnostic

// Code is a stable diagnostic identifier such as "AURA-L001". Codes are
// never reused or renumbered (ADR-0004 item 4). The authoritative list is
// the registry in docs/04-compiler-architecture.md §4.4.3.
type Code string

// Lexer codes.
const (
	InvalidCharacter         Code = "AURA-L001"
	InvalidUTF8              Code = "AURA-L002"
	UnterminatedString       Code = "AURA-L003"
	InvalidEscape            Code = "AURA-L004"
	UnterminatedBlockComment Code = "AURA-L005"
	EmptyInterpolation       Code = "AURA-L006"
	UnterminatedInterp       Code = "AURA-L007"
	MalformedNumber          Code = "AURA-L008"
	LeadingZero              Code = "AURA-L009"
)

// Parser codes.
const (
	ExpectedExpression Code = "AURA-P001"
	ExpectedIdentifier Code = "AURA-P002"
	ReservedWord       Code = "AURA-P003"
	ExpectedNewline    Code = "AURA-P004"
	SameLine           Code = "AURA-P005"
	NonCallStatement   Code = "AURA-P006"
	TrailingComma      Code = "AURA-P007"
	DuplicateModule    Code = "AURA-P008"
	ModuleNotFirst     Code = "AURA-P009"
	MissingRParen      Code = "AURA-P010"
	MissingRBrace      Code = "AURA-P011"
	UnexpectedToken    Code = "AURA-P012"
	ExpectedTypeName   Code = "AURA-P013"
	NestingTooDeep     Code = "AURA-P014"
)

// Type checker codes (Phase 2).
const (
	TypeMismatch         Code = "AURA-T001"
	UndefinedName        Code = "AURA-T002"
	UnknownType          Code = "AURA-T003"
	AssignToImmutable    Code = "AURA-T004"
	DuplicateDeclaration Code = "AURA-T005"
	WrongArgumentCount   Code = "AURA-T006"
	InvalidOperandTypes  Code = "AURA-T007"
	MissingReturn        Code = "AURA-T008"
	InvalidReturn        Code = "AURA-T009"
	IntLiteralOutOfRange Code = "AURA-T010"
	NotDisplayable       Code = "AURA-T011"
	NotCallable          Code = "AURA-T012"
	MissingMain          Code = "AURA-T013"
	InvalidMainSignature Code = "AURA-T014"
	MissingReturnType    Code = "AURA-T015"
	BuiltinRedeclared    Code = "AURA-T016"
	FloatLiteralRange    Code = "AURA-T017"
	FunctionAsValue      Code = "AURA-T018"
)

// Compiler and runtime codes (Phase 3). The VM cannot import this
// package (ADR-0001); it uses the same strings.
const (
	InternalCompilerError Code = "AURA-C001"
	DivisionByZero        Code = "AURA-R001"
	InvalidBytecode       Code = "AURA-R002"
	CallDepthExceeded     Code = "AURA-R003"
	InstructionLimit      Code = "AURA-R004"
	IntegerOverflow       Code = "AURA-R005"
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
		ExpectedTypeName, NestingTooDeep,
		TypeMismatch, UndefinedName, UnknownType, AssignToImmutable,
		DuplicateDeclaration, WrongArgumentCount, InvalidOperandTypes,
		MissingReturn, InvalidReturn, IntLiteralOutOfRange, NotDisplayable,
		NotCallable, MissingMain, InvalidMainSignature, MissingReturnType,
		BuiltinRedeclared, FloatLiteralRange, FunctionAsValue,
		InternalCompilerError,
		DivisionByZero, InvalidBytecode, CallDepthExceeded, InstructionLimit,
		IntegerOverflow,
	}
}

// Phase returns the phase letter of the code ('L', 'P', 'T', 'C' or 'R'),
// or 0 if the code is malformed.
func (c Code) Phase() byte {
	if len(c) != len("AURA-X000") {
		return 0
	}
	return c[5]
}

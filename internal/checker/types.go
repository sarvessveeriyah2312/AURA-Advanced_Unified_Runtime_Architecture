package checker

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/sarvessveeriyah2312/Tread/internal/ast"
	"github.com/sarvessveeriyah2312/Tread/internal/source"
)

// TypeKind classifies a Type.
type TypeKind uint8

const (
	KInvalid TypeKind = iota
	KInt
	KFloat
	KBool
	KString
	KUnit
	KArray // Args[0] is the element type (ADR-0013)
	KMap   // Args[0] is the key type, Args[1] the value type
	KEnum  // Enum, with type arguments in Args
	KParam // a generic type parameter
	// KStruct is a struct, with type arguments in Args (ADR-0014).
	KStruct
)

// TypeInfo is a Tread type. Types are interned (see intern), so two
// types are identical exactly when their pointers are equal; == works
// for every type, including Array<Int> and Option<String>.
type TypeInfo struct {
	Kind   TypeKind
	Args   []Type
	Enum   *Enum
	Param  *TypeParam
	Struct *Struct
	key    string
}

// Type is an interned Tread type.
type Type = *TypeInfo

// The primitive types. Invalid is the error type: the type of an
// expression that already produced a diagnostic. It is compatible with
// everything and never causes further diagnostics (docs/12 §12.2).
var (
	Invalid = intern(&TypeInfo{Kind: KInvalid})
	Int     = intern(&TypeInfo{Kind: KInt})
	Float   = intern(&TypeInfo{Kind: KFloat})
	Bool    = intern(&TypeInfo{Kind: KBool})
	String  = intern(&TypeInfo{Kind: KString})
	Unit    = intern(&TypeInfo{Kind: KUnit})
)

var (
	internMu    sync.Mutex
	internTable = map[string]Type{}
	nextID      atomic.Int64
)

// intern returns the canonical Type equal to t.
func intern(t *TypeInfo) Type {
	var b strings.Builder
	fmt.Fprintf(&b, "%d", t.Kind)
	switch t.Kind {
	case KEnum:
		fmt.Fprintf(&b, "e%d", t.Enum.id)
	case KParam:
		fmt.Fprintf(&b, "p%d", t.Param.id)
	case KStruct:
		fmt.Fprintf(&b, "s%d", t.Struct.id)
	}
	for _, a := range t.Args {
		b.WriteString("(" + a.key + ")")
	}
	t.key = b.String()
	internMu.Lock()
	defer internMu.Unlock()
	if existing, ok := internTable[t.key]; ok {
		return existing
	}
	internTable[t.key] = t
	return t
}

// ArrayOf returns Array<elem>.
func ArrayOf(elem Type) Type { return intern(&TypeInfo{Kind: KArray, Args: []Type{elem}}) }

// MapOf returns Map<key, value>.
func MapOf(key, value Type) Type { return intern(&TypeInfo{Kind: KMap, Args: []Type{key, value}}) }

// EnumOf returns the enum type e<args...>.
func EnumOf(e *Enum, args []Type) Type {
	return intern(&TypeInfo{Kind: KEnum, Enum: e, Args: append([]Type(nil), args...)})
}

// StructOf returns the struct type s<args...> (ADR-0014).
func StructOf(s *Struct, args []Type) Type {
	return intern(&TypeInfo{Kind: KStruct, Struct: s, Args: append([]Type(nil), args...)})
}

// ParamType returns the type of a type parameter.
func ParamType(p *TypeParam) Type { return intern(&TypeInfo{Kind: KParam, Param: p}) }

func (t *TypeInfo) String() string {
	switch t.Kind {
	case KInt:
		return "Int"
	case KFloat:
		return "Float"
	case KBool:
		return "Bool"
	case KString:
		return "String"
	case KUnit:
		return "Unit"
	case KArray:
		return "Array<" + t.Args[0].String() + ">"
	case KMap:
		return "Map<" + t.Args[0].String() + ", " + t.Args[1].String() + ">"
	case KEnum, KStruct:
		name := ""
		if t.Kind == KEnum {
			name = qualified(t.Enum.Module, t.Enum.Name)
		} else {
			name = qualified(t.Struct.Module, t.Struct.Name)
		}
		if len(t.Args) == 0 {
			return name
		}
		parts := make([]string, len(t.Args))
		for i, a := range t.Args {
			parts[i] = a.String()
		}
		return name + "<" + strings.Join(parts, ", ") + ">"
	case KParam:
		return t.Param.Name
	}
	return "_" // unknown: an error was already reported
}

// qualified prefixes name with the module qualifier of a declaration
// outside the root file (ADR-0014).
func qualified(module, name string) string {
	if module == "" {
		return name
	}
	return module + "." + name
}

// isDisplayable reports whether values of t can be printed or
// interpolated: every type except Unit (ADR-0004 item 7, ADR-0013).
func (t *TypeInfo) isDisplayable() bool { return t != Unit }

func (t *TypeInfo) isNumeric() bool { return t == Int || t == Float }

// IsPrimitive reports whether t is Int, Float, Bool, String or Unit:
// the types with typed instructions (docs/05 §5.3).
func (t *TypeInfo) IsPrimitive() bool { return t.Kind >= KInt && t.Kind <= KUnit }

// containsUnit reports whether t is Unit or has Unit inside it.
func (t *TypeInfo) containsUnit() bool {
	if t == Unit {
		return true
	}
	for _, a := range t.Args {
		if a.containsUnit() {
			return true
		}
	}
	return false
}

// containsInvalid reports whether t is or contains Invalid.
func (t *TypeInfo) containsInvalid() bool {
	if t == Invalid {
		return true
	}
	for _, a := range t.Args {
		if a.containsInvalid() {
			return true
		}
	}
	return false
}

// isMapKey reports whether t may be a map key (ADR-0013).
func (t *TypeInfo) isMapKey() bool { return t == Int || t == String || t == Bool || t == Invalid }

// builtinTypes is the primitive part of the type namespace (ADR-0004
// item 3). It is separate from value scopes. Array, Map and Option are
// resolved by resolveType.
var builtinTypes = map[string]Type{
	"Int": Int, "Float": Float, "Bool": Bool, "String": String, "Unit": Unit,
}

// TypeParam is a generic type parameter of a function or an enum.
type TypeParam struct {
	Name string
	Decl source.Span
	id   int64
}

func newTypeParam(name string, decl source.Span) *TypeParam {
	return &TypeParam{Name: name, Decl: decl, id: nextID.Add(1)}
}

// Enum is an enum declaration, or the built-in Option<T>.
type Enum struct {
	Name string
	// Module is the qualifier of the declaring module, "" for the root
	// file and for Option (ADR-0014).
	Module   string
	Pub      bool
	mod      *moduleInfo
	Params   []*TypeParam
	Variants []*Variant
	Decl     *ast.EnumDecl // nil for Option
	// Index is the enum's position in the module; Option is always 0.
	Index int
	id    int64
}

// Variant is one variant of an enum. Field types may mention the enum's
// type parameters.
type Variant struct {
	Enum   *Enum
	Name   string
	Index  int
	Fields []Type
	Decl   source.Span // the variant's name; zero for Option's variants
}

// Lookup returns the variant named name, or nil.
func (e *Enum) Lookup(name string) *Variant {
	for _, v := range e.Variants {
		if v.Name == name {
			return v
		}
	}
	return nil
}

// newOption returns the built-in `enum Option<T> { Some(T) None }`.
func newOption() *Enum {
	t := newTypeParam("T", source.Span{})
	e := &Enum{Name: "Option", Params: []*TypeParam{t}, id: nextID.Add(1)}
	e.Variants = []*Variant{
		{Enum: e, Name: "Some", Index: 0, Fields: []Type{ParamType(t)}},
		{Enum: e, Name: "None", Index: 1},
	}
	return e
}

// subst replaces type parameters in t according to m.
func subst(t Type, m map[*TypeParam]Type) Type {
	if len(m) == 0 || len(t.Args) == 0 && t.Kind != KParam {
		return t
	}
	if t.Kind == KParam {
		if r, ok := m[t.Param]; ok {
			return r
		}
		return t
	}
	args := make([]Type, len(t.Args))
	for i, a := range t.Args {
		args[i] = subst(a, m)
	}
	return intern(&TypeInfo{Kind: t.Kind, Enum: t.Enum, Param: t.Param, Struct: t.Struct, Args: args})
}

// enumArgs maps an enum or struct type's parameters to its arguments.
func enumArgs(t Type) map[*TypeParam]Type {
	var params []*TypeParam
	switch t.Kind {
	case KEnum:
		params = t.Enum.Params
	case KStruct:
		params = t.Struct.Params
	default:
		return nil
	}
	m := make(map[*TypeParam]Type, len(t.Args))
	for i, p := range params {
		if i < len(t.Args) {
			m[p] = t.Args[i]
		}
	}
	return m
}

// Struct is a struct declaration (ADR-0014).
type Struct struct {
	Name   string
	Module string // qualifier of the declaring module; "" for the root file
	Pub    bool
	Params []*TypeParam
	Fields []*Field
	Decl   *ast.StructDecl
	// Methods holds the methods and associated functions of its impl
	// blocks, by name.
	Methods map[string]*Function
	// Index is the struct's position in CheckedProgram.Structs.
	Index int
	mod   *moduleInfo
	id    int64
}

// Field is one struct field. Its type may mention the struct's type
// parameters.
type Field struct {
	Name  string
	Type  Type
	Index int
	Pub   bool
	Decl  source.Span
}

// Field returns the field named name, or nil.
func (s *Struct) Field(name string) *Field {
	for _, f := range s.Fields {
		if f.Name == name {
			return f
		}
	}
	return nil
}

// SymbolKind classifies a value-namespace symbol.
type SymbolKind int

const (
	BuiltinSymbol SymbolKind = iota
	FuncSymbol
	ParamSymbol
	VarSymbol
	LoopVarSymbol // the variable of a `for` loop; immutable (ADR-0012)
	MatchVarSymbol
	ModuleSymbol // an imported module's qualifier (ADR-0014)
)

// Symbol is a named entity in the value namespace.
type Symbol struct {
	Name    string
	Kind    SymbolKind
	Type    Type      // ParamSymbol, VarSymbol, LoopVarSymbol and MatchVarSymbol
	Mutable bool      // VarSymbol declared with `let mut`
	Func    *Function // FuncSymbol and BuiltinSymbol
	Decl    source.Span
	mod     *moduleInfo // ModuleSymbol
}

// Function is a resolved function signature.
type Function struct {
	Name string
	// TypeParams are the function's own type parameters; a method's
	// struct's parameters are in Struct.Params (ADR-0014).
	TypeParams []*TypeParam
	// Params are the parameters; for a method with a receiver, Params[0]
	// is `self`.
	Params []*Symbol
	Result Type
	Decl   *ast.FuncDecl // nil for built-ins
	// ADR-0014.
	Pub     bool
	Module  string  // qualifier of the declaring module; "" for the root
	Struct  *Struct // the struct of an impl function; nil otherwise
	HasSelf bool    // a method: Params[0] is the receiver
	SelfMut bool    // the receiver is `mut self`
	// Test is set for a `test` block (ADR-0015); Name is its name.
	Test bool
	mod  *moduleInfo
}

// FullName is the function's name in bytecode and stack traces, such as
// "dist", "Point.shift" or "geometry.Point.shift".
func (f *Function) FullName() string {
	if f.Test {
		return "test " + strconvQuote(f.Name)
	}
	name := f.Name
	if f.Struct != nil {
		name = f.Struct.Name + "." + name
	}
	return qualified(f.Module, name)
}

// scope is one level of the value namespace. The map is used only for
// lookup, never iterated for output (docs/04 §4.6).
type scope struct {
	parent *scope
	names  map[string]*Symbol
}

func newScope(parent *scope) *scope {
	return &scope{parent: parent, names: map[string]*Symbol{}}
}

func (s *scope) lookup(name string) *Symbol {
	for ; s != nil; s = s.parent {
		if sym, ok := s.names[name]; ok {
			return sym
		}
	}
	return nil
}

// Method identifies a built-in collection method (ADR-0013).
type Method int

const (
	MethodNone Method = iota
	MethodLen
	MethodPush
	MethodPop
	MethodContains
	MethodGet
	MethodRemove
	MethodKeys
)

// CheckedProgram is the result of semantic analysis (docs/12 §12.8). The
// AST is not modified; all results are side tables keyed by AST nodes.
type CheckedProgram struct {
	Program   *ast.Program
	Functions []*Function // user functions, in source order
	// Enums lists every enum; Option is first (Index 0), then user enums
	// in source order.
	Enums []*Enum
	// Main is the validated entry point, or nil if it is missing or
	// invalid.
	Main *Function
	// Types holds the type of every checked expression.
	Types map[ast.Expr]Type
	// Uses resolves identifier expressions, callees and assignment
	// targets.
	Uses map[*ast.Ident]*Symbol
	// Defs maps declared names (functions, parameters, lets, loop
	// variables, pattern bindings) to symbols.
	Defs map[*ast.Ident]*Symbol
	// IntValues holds every in-range IntLit and every UnaryExpr that
	// denotes the minimum Int (ADR-0004 item 1).
	IntValues map[ast.Expr]int64
	// FloatValues holds every in-range FloatLit (ADR-0004 item 14).
	FloatValues map[*ast.FloatLit]float64
	// Variants maps variant constructions (a MemberExpr, or a CallExpr
	// whose callee is one) and variant patterns to their variant.
	Variants map[ast.Node]*Variant
	// Methods maps method calls to the built-in method.
	Methods map[*ast.CallExpr]Method
	// Exhaustive records the match statements that cover every case.
	Exhaustive map[*ast.MatchStmt]bool
	// ADR-0014.
	// Structs lists every struct of the program, in Index order.
	Structs []*Struct
	// Calls maps calls of user functions that are not plain names
	// (geometry.dist(p), p.len(), Point.new()) to their function. For a
	// method with a receiver, the receiver is the callee's X.
	Calls map[*ast.CallExpr]*Function
	// Fields maps field accesses to their field.
	Fields map[*ast.MemberExpr]*Field
	// Tests are the test blocks of the root file, in source order
	// (ADR-0015). Tests of imported files are checked but not listed.
	Tests []*Function
	// TypeNames holds the type of every resolved type annotation, for the
	// language server (ADR-0015).
	TypeNames map[*ast.TypeName]Type
}

// ModulePath returns the import path of a ModuleSymbol, or "".
func (s *Symbol) ModulePath() string {
	if s.mod == nil {
		return ""
	}
	return s.mod.path
}

// ModuleProgram returns the syntax tree of a ModuleSymbol's file, or nil.
func (s *Symbol) ModuleProgram() *ast.Program {
	if s.mod == nil {
		return nil
	}
	return s.mod.prog
}

// Exports lists the pub functions, structs and enums of a ModuleSymbol's
// module, sorted by name.
func (s *Symbol) Exports() (funcs []*Function, structs []*Struct, enums []*Enum) {
	if s.mod == nil {
		return nil, nil, nil
	}
	for _, sym := range s.mod.scope.names {
		if sym.Kind == FuncSymbol && sym.Func.Pub {
			funcs = append(funcs, sym.Func)
		}
	}
	for _, st := range s.mod.structs {
		if st.Pub {
			structs = append(structs, st)
		}
	}
	for _, e := range s.mod.enums {
		if e.Pub && e.Decl != nil {
			enums = append(enums, e)
		}
	}
	sort.Slice(funcs, func(i, j int) bool { return funcs[i].Name < funcs[j].Name })
	sort.Slice(structs, func(i, j int) bool { return structs[i].Name < structs[j].Name })
	sort.Slice(enums, func(i, j int) bool { return enums[i].Name < enums[j].Name })
	return funcs, structs, enums
}

// Options changes what CheckModules requires.
type Options struct {
	// Tests checks the program for `tread test`: main is not required.
	Tests bool
}

func strconvQuote(s string) string { return strconv.Quote(s) }

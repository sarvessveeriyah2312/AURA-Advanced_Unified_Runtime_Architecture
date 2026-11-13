package checker

import (
	"fmt"
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
)

// TypeInfo is a Tread type. Types are interned (see intern), so two
// types are identical exactly when their pointers are equal; == works
// for every type, including Array<Int> and Option<String>.
type TypeInfo struct {
	Kind  TypeKind
	Args  []Type
	Enum  *Enum
	Param *TypeParam
	key   string
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
	case KEnum:
		if len(t.Args) == 0 {
			return t.Enum.Name
		}
		parts := make([]string, len(t.Args))
		for i, a := range t.Args {
			parts[i] = a.String()
		}
		return t.Enum.Name + "<" + strings.Join(parts, ", ") + ">"
	case KParam:
		return t.Param.Name
	}
	return "_" // unknown: an error was already reported
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
	Name     string
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
	return intern(&TypeInfo{Kind: t.Kind, Enum: t.Enum, Param: t.Param, Args: args})
}

// enumArgs maps an enum type's parameters to its arguments.
func enumArgs(t Type) map[*TypeParam]Type {
	if t.Kind != KEnum {
		return nil
	}
	m := make(map[*TypeParam]Type, len(t.Args))
	for i, p := range t.Enum.Params {
		if i < len(t.Args) {
			m[p] = t.Args[i]
		}
	}
	return m
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
)

// Symbol is a named entity in the value namespace.
type Symbol struct {
	Name    string
	Kind    SymbolKind
	Type    Type      // ParamSymbol, VarSymbol, LoopVarSymbol and MatchVarSymbol
	Mutable bool      // VarSymbol declared with `let mut`
	Func    *Function // FuncSymbol and BuiltinSymbol
	Decl    source.Span
}

// Function is a resolved function signature.
type Function struct {
	Name       string
	TypeParams []*TypeParam
	Params     []*Symbol
	Result     Type
	Decl       *ast.FuncDecl // nil for built-ins
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
}

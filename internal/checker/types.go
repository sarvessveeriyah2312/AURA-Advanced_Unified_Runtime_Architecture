package checker

import (
	"github.com/sarvessveeriyah2312/Tread/internal/ast"
	"github.com/sarvessveeriyah2312/Tread/internal/source"
)

// Type is a Tread MVP type.
type Type int

const (
	// Invalid is the error type: the type of an expression that already
	// produced a diagnostic. It is compatible with everything and never
	// causes further diagnostics (docs/12 §12.2).
	Invalid Type = iota
	Int
	Float
	Bool
	String
	Unit
)

func (t Type) String() string {
	switch t {
	case Int:
		return "Int"
	case Float:
		return "Float"
	case Bool:
		return "Bool"
	case String:
		return "String"
	case Unit:
		return "Unit"
	}
	return "<invalid>"
}

// isDisplayable reports whether values of t can be printed or
// interpolated (ADR-0004 item 7).
func (t Type) isDisplayable() bool {
	return t == Int || t == Float || t == Bool || t == String
}

func (t Type) isNumeric() bool { return t == Int || t == Float }

// builtinTypes is the type namespace (ADR-0004 item 3). It is separate
// from value scopes.
var builtinTypes = map[string]Type{
	"Int": Int, "Float": Float, "Bool": Bool, "String": String, "Unit": Unit,
}

// SymbolKind classifies a value-namespace symbol.
type SymbolKind int

const (
	BuiltinSymbol SymbolKind = iota
	FuncSymbol
	ParamSymbol
	VarSymbol
)

// Symbol is a named entity in the value namespace.
type Symbol struct {
	Name    string
	Kind    SymbolKind
	Type    Type      // ParamSymbol and VarSymbol
	Mutable bool      // VarSymbol declared with `let mut`
	Func    *Function // FuncSymbol and BuiltinSymbol
	Decl    source.Span
}

// Function is a resolved function signature.
type Function struct {
	Name   string
	Params []*Symbol
	Result Type
	Decl   *ast.FuncDecl // nil for built-ins
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

// CheckedProgram is the result of semantic analysis (docs/12 §12.8). The
// AST is not modified; all results are side tables keyed by AST nodes.
type CheckedProgram struct {
	Program   *ast.Program
	Functions []*Function // user functions, in source order
	// Main is the validated entry point, or nil if it is missing or
	// invalid.
	Main *Function
	// Types holds the type of every checked expression.
	Types map[ast.Expr]Type
	// Uses resolves identifier expressions, callees and assignment
	// targets.
	Uses map[*ast.Ident]*Symbol
	// Defs maps declared names (functions, parameters, lets) to symbols.
	Defs map[*ast.Ident]*Symbol
	// IntValues holds every in-range IntLit and every UnaryExpr that
	// denotes the minimum Int (ADR-0004 item 1).
	IntValues map[ast.Expr]int64
	// FloatValues holds every in-range FloatLit (ADR-0004 item 14).
	FloatValues map[*ast.FloatLit]float64
}

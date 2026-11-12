package ast

import (
	"github.com/your-org/aura/internal/source"
	"github.com/your-org/aura/internal/token"
)

// ImplicitModuleName is the module name used when a file has no module
// declaration (ADR-0002 D3.1).
const ImplicitModuleName = "main"

// Node is implemented by every AST node. Every node carries a span
// (docs/04 §4.8).
type Node interface {
	Span() source.Span
}

// Expr is an expression node.
type Expr interface {
	Node
	exprNode()
}

// Stmt is a statement node.
type Stmt interface {
	Node
	stmtNode()
}

// Program is a parsed source file.
type Program struct {
	// Module is nil when the file has no module declaration; the module
	// is then implicitly named ImplicitModuleName.
	Module *ModuleDecl
	Funcs  []*FuncDecl
	Loc    source.Span
}

// ModuleName returns the declared or implicit module name.
func (p *Program) ModuleName() string {
	if p.Module == nil || p.Module.Name == nil {
		return ImplicitModuleName
	}
	return p.Module.Name.Name
}

// ModuleDecl is `module name`.
type ModuleDecl struct {
	Name *Ident
	Loc  source.Span
}

// FuncDecl is a top-level function declaration.
type FuncDecl struct {
	Name   *Ident
	Params []*Param
	Result *TypeName // nil when no return type is written
	Body   *Block
	Loc    source.Span
}

// Param is one function parameter.
type Param struct {
	Name *Ident
	Type *TypeName
	Loc  source.Span
}

// TypeName is a type annotation. It is resolved by the checker in the
// type namespace (ADR-0004 item 3).
type TypeName struct {
	Name string
	Loc  source.Span
}

// Block is `{ statements }`.
type Block struct {
	Stmts []Stmt
	Loc   source.Span
}

// LetStmt is `let [mut] name [: Type] = value`.
type LetStmt struct {
	Mutable bool
	Name    *Ident
	Type    *TypeName // nil when omitted (or after a reported error)
	Value   Expr
	Loc     source.Span
}

// AssignStmt is `name = value`.
type AssignStmt struct {
	Target *Ident
	Value  Expr
	Loc    source.Span
}

// IfStmt is `if cond { } [else ...]`. Else is nil, a *Block, or a nested
// *IfStmt for `else if` (ADR-0004 item 5).
type IfStmt struct {
	Cond Expr
	Then *Block
	Else Stmt
	Loc  source.Span
}

// ReturnStmt is `return [value]`. Value is nil for a bare return.
type ReturnStmt struct {
	Value Expr
	Loc   source.Span
}

// ExprStmt is a call used as a statement; the parser accepts no other
// expression statements (ADR-0003 Q4).
type ExprStmt struct {
	Call *CallExpr
	Loc  source.Span
}

// Ident is an identifier, used both as a name and as an expression.
type Ident struct {
	Name string
	Loc  source.Span
}

// IntLit is an integer literal. Digits holds the decimal digits exactly
// as written; the checker converts and range-checks them (ADR-0004 item 1).
type IntLit struct {
	Digits string
	Loc    source.Span
}

// FloatLit is a float literal; Lexeme is kept exactly as written.
type FloatLit struct {
	Lexeme string
	Loc    source.Span
}

// StringLit is a string literal without interpolation, with escapes
// decoded.
type StringLit struct {
	Value string
	Loc   source.Span
}

// Segment is one part of an interpolated string: literal text when Expr
// is nil, otherwise an interpolated expression.
type Segment struct {
	Text string
	Expr Expr
}

// InterpString is a string with ${...} interpolation. Empty literal
// segments are omitted.
type InterpString struct {
	Segments []Segment
	Loc      source.Span
}

// BoolLit is `true` or `false`.
type BoolLit struct {
	Value bool
	Loc   source.Span
}

// ParenExpr is a parenthesized expression, kept explicitly (docs/04 §4.8).
type ParenExpr struct {
	Inner Expr
	Loc   source.Span
}

// UnaryExpr is `-x` or `!x`.
type UnaryExpr struct {
	Op      token.Kind
	Operand Expr
	Loc     source.Span
}

// BinaryExpr is `left op right`.
type BinaryExpr struct {
	Op    token.Kind
	Left  Expr
	Right Expr
	Loc   source.Span
}

// CallExpr is `callee(args)`.
type CallExpr struct {
	Callee Expr
	Args   []Expr
	Loc    source.Span
}

// ErrorExpr stands in for an INVALID token or a failed sub-parse. The
// error has already been reported; later phases skip it silently.
type ErrorExpr struct {
	Loc source.Span
}

func (n *Program) Span() source.Span      { return n.Loc }
func (n *ModuleDecl) Span() source.Span   { return n.Loc }
func (n *FuncDecl) Span() source.Span     { return n.Loc }
func (n *Param) Span() source.Span        { return n.Loc }
func (n *TypeName) Span() source.Span     { return n.Loc }
func (n *Block) Span() source.Span        { return n.Loc }
func (n *LetStmt) Span() source.Span      { return n.Loc }
func (n *AssignStmt) Span() source.Span   { return n.Loc }
func (n *IfStmt) Span() source.Span       { return n.Loc }
func (n *ReturnStmt) Span() source.Span   { return n.Loc }
func (n *ExprStmt) Span() source.Span     { return n.Loc }
func (n *Ident) Span() source.Span        { return n.Loc }
func (n *IntLit) Span() source.Span       { return n.Loc }
func (n *FloatLit) Span() source.Span     { return n.Loc }
func (n *StringLit) Span() source.Span    { return n.Loc }
func (n *InterpString) Span() source.Span { return n.Loc }
func (n *BoolLit) Span() source.Span      { return n.Loc }
func (n *ParenExpr) Span() source.Span    { return n.Loc }
func (n *UnaryExpr) Span() source.Span    { return n.Loc }
func (n *BinaryExpr) Span() source.Span   { return n.Loc }
func (n *CallExpr) Span() source.Span     { return n.Loc }
func (n *ErrorExpr) Span() source.Span    { return n.Loc }

func (*Block) stmtNode()      {}
func (*LetStmt) stmtNode()    {}
func (*AssignStmt) stmtNode() {}
func (*IfStmt) stmtNode()     {}
func (*ReturnStmt) stmtNode() {}
func (*ExprStmt) stmtNode()   {}

func (*Ident) exprNode()        {}
func (*IntLit) exprNode()       {}
func (*FloatLit) exprNode()     {}
func (*StringLit) exprNode()    {}
func (*InterpString) exprNode() {}
func (*BoolLit) exprNode()      {}
func (*ParenExpr) exprNode()    {}
func (*UnaryExpr) exprNode()    {}
func (*BinaryExpr) exprNode()   {}
func (*CallExpr) exprNode()     {}
func (*ErrorExpr) exprNode()    {}

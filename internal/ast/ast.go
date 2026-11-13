package ast

import (
	"strings"

	"github.com/sarvessveeriyah2312/Tread/internal/source"
	"github.com/sarvessveeriyah2312/Tread/internal/token"
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
	Module  *ModuleDecl
	Imports []*ImportDecl // ADR-0014
	Funcs   []*FuncDecl
	Enums   []*EnumDecl   // ADR-0013
	Structs []*StructDecl // ADR-0014
	Impls   []*ImplDecl   // ADR-0014
	Loc     source.Span
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

// ImportDecl is `import a.b.c` (ADR-0014).
type ImportDecl struct {
	Path []*Ident
	Loc  source.Span
}

// Name returns the import's qualifier: the last segment of its path.
func (d *ImportDecl) Name() *Ident { return d.Path[len(d.Path)-1] }

// PathString returns the dotted path, as in "shapes.circle".
func (d *ImportDecl) PathString() string {
	parts := make([]string, len(d.Path))
	for i, p := range d.Path {
		parts[i] = p.Name
	}
	return strings.Join(parts, ".")
}

// StructDecl is `[pub] struct Name<T> { [pub] field: Type ... }`
// (ADR-0014).
type StructDecl struct {
	Pub        bool
	Name       *Ident
	TypeParams []*Ident
	Fields     []*FieldDecl
	Loc        source.Span
}

// FieldDecl is one struct field.
type FieldDecl struct {
	Pub  bool
	Name *Ident
	Type *TypeName
	Loc  source.Span
}

// ImplDecl is `impl Name { fn ... }` (ADR-0014).
type ImplDecl struct {
	Name  *Ident
	Funcs []*FuncDecl
	Loc   source.Span
}

// SelfParam is the receiver `self` or `mut self` of a method (ADR-0014).
type SelfParam struct {
	Mutable bool
	Loc     source.Span
}

// FuncDecl is a top-level function declaration, or a function in an
// impl block.
type FuncDecl struct {
	Pub        bool       // ADR-0014
	Self       *SelfParam // the receiver of a method; nil otherwise
	Name       *Ident
	TypeParams []*Ident // generic parameters `<T, U>` (ADR-0013)
	Params     []*Param
	Result     *TypeName // nil when no return type is written
	Body       *Block
	Loc        source.Span
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
	// Module is the qualifier of `geometry.Point`, or nil (ADR-0014).
	Module *Ident
	Name   string
	Args   []*TypeName // type arguments, as in Array<Int> (ADR-0013)
	Loc    source.Span
}

// EnumDecl is `enum Name<T> { Variant(Type, ...) ... }` (ADR-0013).
type EnumDecl struct {
	Pub        bool // ADR-0014
	Name       *Ident
	TypeParams []*Ident
	Variants   []*Variant
	Loc        source.Span
}

// Variant is one enum variant and the types of its payload fields.
type Variant struct {
	Name   *Ident
	Fields []*TypeName
	Loc    source.Span
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

// AssignStmt is `target = value`. Target is an *Ident or a path of
// *IndexExpr and *MemberExpr (field) nodes rooted in one, such as
// grid[r][c] (ADR-0013) or p.pos.x (ADR-0014).
type AssignStmt struct {
	Target Expr
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

// WhileStmt is `while cond { }` (ADR-0012).
type WhileStmt struct {
	Cond Expr
	Body *Block
	Loc  source.Span
}

// ForStmt is `for name in start..end { }` (ADR-0012). The range is
// half-open: name takes each Int from start up to, but not including,
// end.
type ForStmt struct {
	Var   *Ident
	Start Expr
	End   Expr
	// Iter is the collection of `for name in collection` (ADR-0013);
	// Start and End are nil then.
	Iter Expr
	Body *Block
	Loc  source.Span
}

// MatchStmt is `match subject { pattern => { } ... }` (ADR-0013).
type MatchStmt struct {
	Subject Expr
	Arms    []*MatchArm
	Loc     source.Span
}

// MatchArm is one `pattern => { body }`.
type MatchArm struct {
	Pattern Pattern
	Body    *Block
	Loc     source.Span
}

// Pattern is a match pattern.
type Pattern interface {
	Node
	patNode()
}

// WildcardPat is `_`.
type WildcardPat struct {
	Loc source.Span
}

// BindPat binds the matched value to a new name.
type BindPat struct {
	Name *Ident
}

// LitPat matches a literal: an IntLit (or a negated one), a StringLit, a
// BoolLit, or a FloatLit (rejected by the checker).
type LitPat struct {
	Value Expr
}

// VariantPat is `Enum.Variant` or `Enum.Variant(p, ...)`.
type VariantPat struct {
	Module  *Ident // qualifier of `geometry.Shape.Circle`, or nil (ADR-0014)
	Enum    *Ident
	Variant *Ident
	Args    []Pattern
	Loc     source.Span
}

// BreakStmt is `break`; it leaves the innermost loop (ADR-0012).
type BreakStmt struct {
	Loc source.Span
}

// ContinueStmt is `continue`; it starts the next iteration of the
// innermost loop (ADR-0012).
type ContinueStmt struct {
	Loc source.Span
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

// ArrayLit is `[a, b, c]` (ADR-0013).
type ArrayLit struct {
	Elems []Expr
	Loc   source.Span
}

// MapLit is `[k: v, ...]`, or `[:]` when empty (ADR-0013).
type MapLit struct {
	Keys   []Expr
	Values []Expr
	Loc    source.Span
}

// IndexExpr is `x[index]`.
type IndexExpr struct {
	X     Expr
	Index Expr
	Loc   source.Span
}

// MemberExpr is `x.name`: an enum variant (`Shape.Empty`) or, as the
// callee of a CallExpr, a method call or variant construction.
type MemberExpr struct {
	X    Expr
	Name *Ident
	Loc  source.Span
}

// StructLit is `[module.]Name { field: value, ... }` (ADR-0014).
type StructLit struct {
	Module *Ident
	Name   *Ident
	Fields []*FieldInit
	Loc    source.Span
}

// FieldInit is one `field: value` of a struct literal.
type FieldInit struct {
	Name  *Ident
	Value Expr
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
func (n *WhileStmt) Span() source.Span    { return n.Loc }
func (n *ForStmt) Span() source.Span      { return n.Loc }
func (n *BreakStmt) Span() source.Span    { return n.Loc }
func (n *ContinueStmt) Span() source.Span { return n.Loc }
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
func (n *EnumDecl) Span() source.Span     { return n.Loc }
func (n *Variant) Span() source.Span      { return n.Loc }
func (n *MatchStmt) Span() source.Span    { return n.Loc }
func (n *MatchArm) Span() source.Span     { return n.Loc }
func (n *WildcardPat) Span() source.Span  { return n.Loc }
func (n *BindPat) Span() source.Span      { return n.Name.Loc }
func (n *LitPat) Span() source.Span       { return n.Value.Span() }
func (n *VariantPat) Span() source.Span   { return n.Loc }
func (n *ArrayLit) Span() source.Span     { return n.Loc }
func (n *MapLit) Span() source.Span       { return n.Loc }
func (n *IndexExpr) Span() source.Span    { return n.Loc }
func (n *MemberExpr) Span() source.Span   { return n.Loc }
func (n *ImportDecl) Span() source.Span   { return n.Loc }
func (n *StructDecl) Span() source.Span   { return n.Loc }
func (n *FieldDecl) Span() source.Span    { return n.Loc }
func (n *ImplDecl) Span() source.Span     { return n.Loc }
func (n *StructLit) Span() source.Span    { return n.Loc }

func (*StructLit) exprNode() {}

func (*MatchStmt) stmtNode() {}

func (*WildcardPat) patNode() {}
func (*BindPat) patNode()     {}
func (*LitPat) patNode()      {}
func (*VariantPat) patNode()  {}

func (*ArrayLit) exprNode()   {}
func (*MapLit) exprNode()     {}
func (*IndexExpr) exprNode()  {}
func (*MemberExpr) exprNode() {}

func (*Block) stmtNode()        {}
func (*LetStmt) stmtNode()      {}
func (*AssignStmt) stmtNode()   {}
func (*IfStmt) stmtNode()       {}
func (*ReturnStmt) stmtNode()   {}
func (*WhileStmt) stmtNode()    {}
func (*ForStmt) stmtNode()      {}
func (*BreakStmt) stmtNode()    {}
func (*ContinueStmt) stmtNode() {}
func (*ExprStmt) stmtNode()     {}

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

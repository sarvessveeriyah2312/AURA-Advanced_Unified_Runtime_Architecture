package checker

import (
	"errors"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/sarvessveeriyah2312/Tread/internal/ast"
	"github.com/sarvessveeriyah2312/Tread/internal/diagnostic"
	"github.com/sarvessveeriyah2312/Tread/internal/source"
)

// minIntMagnitude is the digit string of |math.MinInt64|. It is valid only
// as the direct operand of unary minus (ADR-0004 item 1).
const minIntMagnitude = "9223372036854775808"

// Check performs name resolution and type checking (docs/12). It never
// panics on user programs and does not modify the AST. A program with any
// returned diagnostic must not be compiled.
func Check(prog *ast.Program) (*CheckedProgram, []diagnostic.Diagnostic) {
	c := &checker{
		out: &CheckedProgram{
			Program:     prog,
			Types:       map[ast.Expr]Type{},
			Uses:        map[*ast.Ident]*Symbol{},
			Defs:        map[*ast.Ident]*Symbol{},
			IntValues:   map[ast.Expr]int64{},
			FloatValues: map[*ast.FloatLit]float64{},
		},
		file: prog.Loc.File,
	}
	c.universe = newScope(nil)
	for _, b := range []*Function{{Name: "print", Result: Unit}, {Name: "toString", Result: String}} {
		c.universe.names[b.Name] = &Symbol{Name: b.Name, Kind: BuiltinSymbol, Func: b}
	}
	c.module = newScope(c.universe)

	c.collectSignatures(prog)
	c.validateMain()
	for _, fn := range c.out.Functions {
		c.checkFunction(fn)
	}
	sort.SliceStable(c.diags, func(i, j int) bool {
		return c.diags[i].Span.Start < c.diags[j].Span.Start
	})
	return c.out, c.diags
}

type checker struct {
	out      *CheckedProgram
	diags    []diagnostic.Diagnostic
	file     source.FileID
	universe *scope
	module   *scope
	// current function and scope while checking bodies
	fn    *Function
	scope *scope
	// loops is the number of loops enclosing the current statement
	// (ADR-0012).
	loops int
}

func (c *checker) errorf(code diagnostic.Code, span source.Span, format string, args ...any) {
	c.diags = append(c.diags, diagnostic.Errorf(code, span, format, args...))
}

// ---------------------------------------------------------------------------
// Declarations

// declare adds sym to s. Built-in names are protected (T016) and
// same-scope duplicates are T005. It reports whether sym was added.
func (c *checker) declare(s *scope, sym *Symbol, what string) bool {
	if b := c.universe.names[sym.Name]; b != nil {
		c.errorf(diagnostic.BuiltinRedeclared, sym.Decl,
			"cannot declare %s '%s': '%s' is a built-in function", what, sym.Name, sym.Name)
		return false
	}
	if prev, ok := s.names[sym.Name]; ok {
		c.diags = append(c.diags, diagnostic.Errorf(diagnostic.DuplicateDeclaration, sym.Decl,
			"'%s' is already declared in this scope", sym.Name).
			WithRelated(prev.Decl, "previous declaration is here"))
		return false
	}
	s.names[sym.Name] = sym
	return true
}

// resolveType resolves a type annotation in the type namespace.
func (c *checker) resolveType(tn *ast.TypeName) Type {
	if tn == nil {
		return Invalid // parser already reported
	}
	if t, ok := builtinTypes[tn.Name]; ok {
		return t
	}
	c.errorf(diagnostic.UnknownType, tn.Loc, "unknown type '%s'", tn.Name)
	return Invalid
}

func (c *checker) collectSignatures(prog *ast.Program) {
	for _, decl := range prog.Funcs {
		if decl == nil || decl.Name == nil {
			continue
		}
		fn := &Function{Name: decl.Name.Name, Decl: decl}
		params := newScope(nil) // only for duplicate-parameter detection
		for _, p := range decl.Params {
			if p == nil || p.Name == nil {
				continue
			}
			sym := &Symbol{Name: p.Name.Name, Kind: ParamSymbol, Type: c.resolveType(p.Type), Decl: p.Name.Loc}
			if c.declare(params, sym, "parameter") {
				c.out.Defs[p.Name] = sym
			}
			fn.Params = append(fn.Params, sym)
		}
		switch {
		case decl.Result != nil:
			fn.Result = c.resolveType(decl.Result)
		case fn.Name == "main":
			fn.Result = Invalid // reported as T014 by validateMain
		default:
			c.errorf(diagnostic.MissingReturnType, decl.Name.Loc,
				"function '%s' must declare its return type (for example '-> Unit')", fn.Name)
			fn.Result = Invalid
		}
		sym := &Symbol{Name: fn.Name, Kind: FuncSymbol, Func: fn, Decl: decl.Name.Loc}
		if c.declare(c.module, sym, "function") {
			c.out.Defs[decl.Name] = sym
		}
		// Duplicates are still checked so their bodies get diagnostics.
		c.out.Functions = append(c.out.Functions, fn)
	}
}

// validateMain enforces ADR-0004 item 8.
func (c *checker) validateMain() {
	sym := c.module.names["main"]
	if sym == nil || sym.Kind != FuncSymbol {
		c.errorf(diagnostic.MissingMain, source.Span{File: c.file},
			"program has no entry point: declare 'fn main() -> Unit'")
		return
	}
	fn := sym.Func
	decl := fn.Decl
	valid := true
	if len(decl.Params) > 0 {
		first, last := decl.Params[0], decl.Params[len(decl.Params)-1]
		c.errorf(diagnostic.InvalidMainSignature, source.Join(first.Loc, last.Loc),
			"'main' must not take parameters")
		valid = false
	}
	switch {
	case decl.Result == nil:
		c.errorf(diagnostic.InvalidMainSignature, decl.Name.Loc,
			"'main' must declare its return type: 'fn main() -> Unit'")
		valid = false
	case decl.Result.Name != "Unit":
		c.errorf(diagnostic.InvalidMainSignature, decl.Result.Loc,
			"'main' must return Unit, not '%s'", decl.Result.Name)
		valid = false
	}
	if valid {
		c.out.Main = fn
	}
}

// ---------------------------------------------------------------------------
// Function bodies and statements

func (c *checker) checkFunction(fn *Function) {
	decl := fn.Decl
	if decl.Body == nil {
		return
	}
	c.fn = fn
	params := newScope(c.module)
	for _, p := range fn.Params {
		if c.universe.names[p.Name] == nil {
			if _, dup := params.names[p.Name]; !dup {
				params.names[p.Name] = p
			}
		}
	}
	c.scope = params
	c.checkBlock(decl.Body)
	if fn.Result != Unit && fn.Result != Invalid && !alwaysReturns(decl.Body) {
		c.errorf(diagnostic.MissingReturn, decl.Name.Loc,
			"function '%s' does not return a value on every path; expected %s", fn.Name, fn.Result)
	}
	c.fn, c.scope, c.loops = nil, nil, 0
}

func (c *checker) checkBlock(b *ast.Block) {
	if b == nil {
		return
	}
	saved := c.scope
	c.scope = newScope(saved)
	for _, s := range b.Stmts {
		c.checkStmt(s)
	}
	c.scope = saved
}

func (c *checker) checkStmt(s ast.Stmt) {
	switch s := s.(type) {
	case *ast.Block:
		c.checkBlock(s)
	case *ast.LetStmt:
		c.checkLet(s)
	case *ast.AssignStmt:
		c.checkAssign(s)
	case *ast.IfStmt:
		c.checkIf(s)
	case *ast.ReturnStmt:
		c.checkReturn(s)
	case *ast.WhileStmt:
		c.checkWhile(s)
	case *ast.ForStmt:
		c.checkFor(s)
	case *ast.BreakStmt:
		c.checkLoopControl("break", s.Loc)
	case *ast.ContinueStmt:
		c.checkLoopControl("continue", s.Loc)
	case *ast.ExprStmt:
		if s.Call != nil {
			c.expr(s.Call)
		}
	}
}

func (c *checker) checkLet(s *ast.LetStmt) {
	valueType := Invalid
	if s.Value != nil {
		valueType = c.expr(s.Value)
	}
	varType := valueType
	if s.Type != nil {
		varType = c.resolveType(s.Type)
		if !compatible(varType, valueType) {
			c.errorf(diagnostic.TypeMismatch, s.Value.Span(),
				"expected %s but found %s", varType, valueType)
		}
	}
	if s.Name == nil {
		return
	}
	// Declared after the initializer is checked (docs/02 §2.13), and even
	// when it failed, so that later uses do not cascade.
	sym := &Symbol{Name: s.Name.Name, Kind: VarSymbol, Type: varType, Mutable: s.Mutable, Decl: s.Name.Loc}
	if c.declare(c.scope, sym, "variable") {
		c.out.Defs[s.Name] = sym
	}
}

func (c *checker) checkAssign(s *ast.AssignStmt) {
	valueType := c.expr(s.Value)
	if s.Target == nil {
		return
	}
	sym := c.scope.lookup(s.Target.Name)
	if sym == nil {
		c.errorf(diagnostic.UndefinedName, s.Target.Loc, "undefined name '%s'", s.Target.Name)
		return
	}
	c.out.Uses[s.Target] = sym
	switch {
	case sym.Kind == FuncSymbol || sym.Kind == BuiltinSymbol:
		c.errorf(diagnostic.AssignToImmutable, s.Loc, "cannot assign to function '%s'", sym.Name)
		return
	case sym.Kind == ParamSymbol:
		c.errorf(diagnostic.AssignToImmutable, s.Loc, "cannot assign to parameter '%s'", sym.Name)
		return
	case sym.Kind == LoopVarSymbol:
		c.diags = append(c.diags, diagnostic.Errorf(diagnostic.AssignToImmutable, s.Loc,
			"cannot assign to loop variable '%s'", sym.Name).
			WithRelated(sym.Decl, "loop variable declared here"))
		return
	case !sym.Mutable:
		c.diags = append(c.diags, diagnostic.Errorf(diagnostic.AssignToImmutable, s.Loc,
			"cannot assign twice to immutable variable '%s'", sym.Name).
			WithHelp("declare it with 'let mut' to allow assignment").
			WithRelated(sym.Decl, "variable declared here"))
		return
	}
	if !compatible(sym.Type, valueType) {
		c.errorf(diagnostic.TypeMismatch, s.Value.Span(), "expected %s but found %s", sym.Type, valueType)
	}
}

func (c *checker) checkIf(s *ast.IfStmt) {
	if s.Cond != nil {
		if t := c.expr(s.Cond); !compatible(Bool, t) {
			c.errorf(diagnostic.TypeMismatch, s.Cond.Span(), "condition must be Bool, found %s", t)
		}
	}
	c.checkBlock(s.Then)
	switch e := s.Else.(type) {
	case *ast.Block:
		c.checkBlock(e)
	case *ast.IfStmt:
		c.checkIf(e)
	}
}

// checkWhile checks `while cond { }` (ADR-0012).
func (c *checker) checkWhile(s *ast.WhileStmt) {
	if s.Cond != nil {
		if t := c.expr(s.Cond); !compatible(Bool, t) {
			c.errorf(diagnostic.TypeMismatch, s.Cond.Span(), "condition must be Bool, found %s", t)
		}
	}
	c.loops++
	c.checkBlock(s.Body)
	c.loops--
}

// checkFor checks `for name in start..end { }` (ADR-0012). The bounds
// are checked in the enclosing scope, before the loop variable exists.
// The loop variable lives in its own scope around the body, so the body
// may shadow it like a parameter.
func (c *checker) checkFor(s *ast.ForStmt) {
	for _, bound := range []ast.Expr{s.Start, s.End} {
		if bound == nil {
			continue // parser already reported
		}
		if t := c.expr(bound); !compatible(Int, t) {
			c.errorf(diagnostic.TypeMismatch, bound.Span(), "range bound must be Int, found %s", t)
		}
	}
	saved := c.scope
	c.scope = newScope(saved)
	if s.Var != nil {
		sym := &Symbol{Name: s.Var.Name, Kind: LoopVarSymbol, Type: Int, Decl: s.Var.Loc}
		if c.declare(c.scope, sym, "loop variable") {
			c.out.Defs[s.Var] = sym
		}
	}
	c.loops++
	c.checkBlock(s.Body)
	c.loops--
	c.scope = saved
}

// checkLoopControl reports `break` or `continue` outside a loop.
func (c *checker) checkLoopControl(word string, span source.Span) {
	if c.loops == 0 {
		c.errorf(diagnostic.LoopControlOutsideLoop, span, "'%s' outside of a loop", word)
	}
}

func (c *checker) checkReturn(s *ast.ReturnStmt) {
	want := c.fn.Result
	if s.Value == nil {
		if want != Unit && want != Invalid {
			c.errorf(diagnostic.InvalidReturn, s.Loc, "missing return value; expected %s", want)
		}
		return
	}
	got := c.expr(s.Value)
	switch {
	case want == Unit:
		c.errorf(diagnostic.InvalidReturn, s.Loc, "cannot return a value from a function returning Unit")
	case !compatible(want, got):
		c.errorf(diagnostic.TypeMismatch, s.Value.Span(), "expected %s but found %s", want, got)
	}
}

// alwaysReturns implements docs/12 §12.7.
func alwaysReturns(s ast.Stmt) bool {
	switch s := s.(type) {
	case *ast.ReturnStmt:
		return true
	case *ast.Block:
		if s == nil {
			return false
		}
		for _, st := range s.Stmts {
			if alwaysReturns(st) {
				return true
			}
		}
	case *ast.IfStmt:
		if s.Then == nil || s.Else == nil {
			return false
		}
		return alwaysReturns(s.Then) && alwaysReturns(s.Else)
	case *ast.WhileStmt:
		// ADR-0012: `while true` with no `break` of its own never
		// completes, so control cannot fall off the end after it. No
		// other loop counts: its body may run zero times.
		return isTrueLiteral(s.Cond) && !breaks(s.Body)
	}
	return false
}

// compatible reports whether a value of type got may be used where want
// is required. Invalid is compatible with everything (error recovery).
func compatible(want, got Type) bool {
	return want == Invalid || got == Invalid || want == got
}

// ---------------------------------------------------------------------------
// Expressions

// expr type-checks e, records its type, and returns it.
func (c *checker) expr(e ast.Expr) Type {
	t := c.exprType(e)
	if e != nil {
		c.out.Types[e] = t
	}
	return t
}

func (c *checker) exprType(e ast.Expr) Type {
	switch e := e.(type) {
	case nil, *ast.ErrorExpr:
		return Invalid
	case *ast.IntLit:
		return c.intLit(e)
	case *ast.FloatLit:
		return c.floatLit(e)
	case *ast.StringLit:
		return String
	case *ast.BoolLit:
		return Bool
	case *ast.InterpString:
		for _, seg := range e.Segments {
			if seg.Expr == nil {
				continue
			}
			if t := c.expr(seg.Expr); t == Unit {
				c.errorf(diagnostic.NotDisplayable, seg.Expr.Span(), "a value of type Unit cannot be interpolated")
			}
		}
		return String
	case *ast.Ident:
		return c.ident(e)
	case *ast.ParenExpr:
		return c.expr(e.Inner)
	case *ast.UnaryExpr:
		return c.unary(e)
	case *ast.BinaryExpr:
		return c.binary(e)
	case *ast.CallExpr:
		return c.call(e)
	}
	return Invalid
}

func (c *checker) intLit(e *ast.IntLit) Type {
	v, err := strconv.ParseInt(e.Digits, 10, 64)
	if err != nil {
		c.errorf(diagnostic.IntLiteralOutOfRange, e.Loc,
			"integer literal %s is out of range for Int (maximum 9223372036854775807)", e.Digits)
		return Invalid
	}
	c.out.IntValues[e] = v
	return Int
}

func (c *checker) floatLit(e *ast.FloatLit) Type {
	v, err := strconv.ParseFloat(e.Lexeme, 64)
	switch {
	case math.IsInf(v, 0) || (err != nil && !errors.Is(err, strconv.ErrRange)):
		c.errorf(diagnostic.FloatLiteralRange, e.Loc,
			"float literal %s is too large for Float", truncateLexeme(e.Lexeme))
		return Invalid
	case v == 0 && strings.ContainsAny(e.Lexeme, "123456789"):
		c.errorf(diagnostic.FloatLiteralRange, e.Loc,
			"float literal %s is too small for Float and would round to zero", truncateLexeme(e.Lexeme))
		return Invalid
	}
	c.out.FloatValues[e] = v
	return Float
}

// truncateLexeme shortens very long literals in messages; the span still
// covers the whole lexeme.
func truncateLexeme(s string) string {
	if len(s) <= 40 {
		return s
	}
	return s[:20] + "..." + s[len(s)-10:]
}

func (c *checker) ident(e *ast.Ident) Type {
	sym := c.scope.lookup(e.Name)
	if sym == nil {
		c.errorf(diagnostic.UndefinedName, e.Loc, "undefined name '%s'", e.Name)
		return Invalid
	}
	c.out.Uses[e] = sym
	if sym.Kind == FuncSymbol || sym.Kind == BuiltinSymbol {
		c.errorf(diagnostic.FunctionAsValue, e.Loc,
			"function '%s' cannot be used as a value; functions can only be called", e.Name)
		return Invalid
	}
	return sym.Type
}

func (c *checker) unary(e *ast.UnaryExpr) Type {
	op := e.Op.String()
	if lit, ok := e.Operand.(*ast.IntLit); ok && op == "-" && lit.Digits == minIntMagnitude {
		// The minimum Int: valid only as the direct operand of unary
		// minus. Recorded on the unary node; the literal has no value.
		c.out.Types[lit] = Int
		c.out.IntValues[e] = math.MinInt64
		return Int
	}
	t := c.expr(e.Operand)
	if t == Invalid {
		return Invalid
	}
	switch {
	case op == "-" && t.isNumeric():
		return t
	case op == "!" && t == Bool:
		return Bool
	}
	c.errorf(diagnostic.InvalidOperandTypes, e.Loc, "operator '%s' cannot be applied to %s", op, t)
	return Invalid
}

// binary implements the operator table of ADR-0004 item 6.
func (c *checker) binary(e *ast.BinaryExpr) Type {
	l := c.expr(e.Left)
	r := c.expr(e.Right)
	if l == Invalid || r == Invalid {
		return Invalid
	}
	op := e.Op.String()
	same := l == r
	switch op {
	case "+":
		if same && (l.isNumeric() || l == String) {
			return l
		}
	case "-", "*", "/":
		if same && l.isNumeric() {
			return l
		}
	case "%":
		if same && l == Int {
			return Int
		}
	case "==", "!=":
		if same && l != Unit {
			return Bool
		}
	case "<", "<=", ">", ">=":
		if same && l.isNumeric() {
			return Bool
		}
	case "&&", "||":
		if same && l == Bool {
			return Bool
		}
	}
	c.errorf(diagnostic.InvalidOperandTypes, e.Loc, "operator '%s' cannot be applied to %s and %s", op, l, r)
	return Invalid
}

func (c *checker) call(e *ast.CallExpr) Type {
	id, ok := e.Callee.(*ast.Ident)
	if !ok {
		// Not a name: e.g. f()() or (f)(1). The MVP has no function values.
		if t := c.expr(e.Callee); t != Invalid {
			c.errorf(diagnostic.NotCallable, e.Callee.Span(), "expression of type %s is not callable", t)
		}
		c.checkArgs(e, nil)
		return Invalid
	}
	sym := c.scope.lookup(id.Name)
	if sym == nil {
		c.errorf(diagnostic.UndefinedName, id.Loc, "undefined function '%s'", id.Name)
		c.checkArgs(e, nil)
		return Invalid
	}
	c.out.Uses[id] = sym
	c.out.Types[id] = Invalid // a callee name has no value type
	switch sym.Kind {
	case BuiltinSymbol:
		return c.callBuiltin(e, sym.Func)
	case FuncSymbol:
		c.checkArgs(e, sym.Func)
		return sym.Func.Result
	default:
		c.errorf(diagnostic.NotCallable, id.Loc, "'%s' is a %s, not a function", id.Name, sym.Type)
		c.checkArgs(e, nil)
		return Invalid
	}
}

// checkArgs checks every argument; with fn non-nil it also checks arity
// (T006) and argument types (T001).
func (c *checker) checkArgs(e *ast.CallExpr, fn *Function) {
	types := make([]Type, len(e.Args))
	for i, a := range e.Args {
		types[i] = c.expr(a)
	}
	if fn == nil {
		return
	}
	if len(e.Args) != len(fn.Params) {
		c.errorf(diagnostic.WrongArgumentCount, e.Loc,
			"function '%s' expects %d argument(s) but %d were given", fn.Name, len(fn.Params), len(e.Args))
		return
	}
	for i, p := range fn.Params {
		if !compatible(p.Type, types[i]) {
			c.errorf(diagnostic.TypeMismatch, e.Args[i].Span(),
				"argument %d of '%s': expected %s but found %s", i+1, fn.Name, p.Type, types[i])
		}
	}
}

// callBuiltin checks print and toString (docs/02 §2.14, ADR-0004 items 7
// and 13). Both take exactly one displayable argument.
func (c *checker) callBuiltin(e *ast.CallExpr, fn *Function) Type {
	types := make([]Type, len(e.Args))
	for i, a := range e.Args {
		types[i] = c.expr(a)
	}
	if len(e.Args) != 1 {
		c.errorf(diagnostic.WrongArgumentCount, e.Loc, "'%s' expects exactly 1 argument but %d were given", fn.Name, len(e.Args))
		return fn.Result
	}
	if types[0] == Unit {
		c.errorf(diagnostic.NotDisplayable, e.Args[0].Span(),
			"'%s' cannot take a value of type Unit; it accepts Int, Float, Bool or String", fn.Name)
	}
	return fn.Result
}

// isTrueLiteral reports whether e is the literal `true`, possibly in
// parentheses.
func isTrueLiteral(e ast.Expr) bool {
	for {
		switch x := e.(type) {
		case *ast.ParenExpr:
			e = x.Inner
		case *ast.BoolLit:
			return x.Value
		default:
			return false
		}
	}
}

// breaks reports whether s contains a `break` that leaves the loop whose
// body s is. A `break` inside a nested loop leaves only that loop.
func breaks(s ast.Stmt) bool {
	switch s := s.(type) {
	case *ast.BreakStmt:
		return true
	case *ast.Block:
		if s == nil {
			return false
		}
		for _, st := range s.Stmts {
			if breaks(st) {
				return true
			}
		}
	case *ast.IfStmt:
		return (s.Then != nil && breaks(s.Then)) || (s.Else != nil && breaks(s.Else))
	}
	return false
}

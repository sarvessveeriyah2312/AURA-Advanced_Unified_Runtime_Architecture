package checker

import (
	"sort"

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
			Variants:    map[ast.Node]*Variant{},
			Methods:     map[*ast.CallExpr]Method{},
			Exhaustive:  map[*ast.MatchStmt]bool{},
		},
		file:  prog.Loc.File,
		enums: map[string]*Enum{},
	}
	c.universe = newScope(nil)
	for _, b := range []*Function{{Name: "print", Result: Unit}, {Name: "toString", Result: String}} {
		c.universe.names[b.Name] = &Symbol{Name: b.Name, Kind: BuiltinSymbol, Func: b}
	}
	c.module = newScope(c.universe)

	option := newOption()
	c.enums[option.Name] = option
	c.out.Enums = append(c.out.Enums, option)

	c.collectEnums(prog)
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
	// enums is the type namespace's enums, including Option.
	enums map[string]*Enum
	// typeParams are the type parameters in scope (of the current
	// function or enum declaration).
	typeParams map[string]*TypeParam
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

// declareTypeParams creates the type parameters of a function or enum.
// Duplicates and names that hide a type are T005.
func (c *checker) declareTypeParams(ids []*ast.Ident) ([]*TypeParam, map[string]*TypeParam) {
	if len(ids) == 0 {
		return nil, nil
	}
	params := make([]*TypeParam, 0, len(ids))
	byName := map[string]*TypeParam{}
	for _, id := range ids {
		if id == nil {
			continue
		}
		p := newTypeParam(id.Name, id.Loc)
		switch {
		case byName[id.Name] != nil:
			c.errorf(diagnostic.DuplicateDeclaration, id.Loc, "type parameter '%s' is already declared", id.Name)
		case c.isTypeName(id.Name):
			c.errorf(diagnostic.DuplicateDeclaration, id.Loc, "type parameter '%s' would hide the type '%s'", id.Name, id.Name)
		default:
			byName[id.Name] = p
		}
		params = append(params, p)
	}
	return params, byName
}

// isTypeName reports whether name is a built-in type or a declared enum.
func (c *checker) isTypeName(name string) bool {
	switch name {
	case "Array", "Map":
		return true
	}
	return builtinTypes[name] != nil || c.enums[name] != nil
}

// resolveType resolves a type annotation in the type namespace: the
// primitives, Array<T>, Map<K, V>, enums (with type arguments when
// generic), and the type parameters in scope.
func (c *checker) resolveType(tn *ast.TypeName) Type {
	if tn == nil {
		return Invalid // parser already reported
	}
	args := make([]Type, len(tn.Args))
	for i, a := range tn.Args {
		args[i] = c.resolveType(a)
		if args[i] == Unit {
			c.errorf(diagnostic.UnitInCompositeType, a.Loc, "Unit cannot be used inside another type")
			args[i] = Invalid
		}
	}
	arity := func(want int) bool {
		if len(args) == want {
			return true
		}
		if want == 0 {
			c.errorf(diagnostic.WrongTypeArguments, tn.Loc, "type '%s' takes no type arguments", tn.Name)
		} else {
			c.errorf(diagnostic.WrongTypeArguments, tn.Loc, "type '%s' takes %d type argument(s) but %d were given", tn.Name, want, len(args))
		}
		return false
	}
	if p := c.typeParams[tn.Name]; p != nil {
		if !arity(0) {
			return Invalid
		}
		return ParamType(p)
	}
	if t, ok := builtinTypes[tn.Name]; ok {
		if !arity(0) {
			return Invalid
		}
		return t
	}
	switch tn.Name {
	case "Array":
		if !arity(1) {
			return Invalid
		}
		return ArrayOf(args[0])
	case "Map":
		if !arity(2) {
			return Invalid
		}
		if !args[0].isMapKey() {
			c.errorf(diagnostic.InvalidMapKey, tn.Args[0].Loc, "map keys must be Int, String or Bool, not %s", args[0])
			return Invalid
		}
		return MapOf(args[0], args[1])
	}
	if e := c.enums[tn.Name]; e != nil {
		if !arity(len(e.Params)) {
			return Invalid
		}
		return EnumOf(e, args)
	}
	c.errorf(diagnostic.UnknownType, tn.Loc, "unknown type '%s'", tn.Name)
	return Invalid
}

// collectEnums declares every enum, then resolves variant fields, so
// enums may refer to each other (and to themselves) in any order.
func (c *checker) collectEnums(prog *ast.Program) {
	var decls []*Enum
	for _, decl := range prog.Enums {
		if decl == nil || decl.Name == nil {
			continue
		}
		e := &Enum{Name: decl.Name.Name, Decl: decl, Index: len(c.out.Enums), id: nextID.Add(1)}
		switch {
		case c.enums[e.Name] != nil && c.enums[e.Name].Decl != nil:
			c.diags = append(c.diags, diagnostic.Errorf(diagnostic.DuplicateDeclaration, decl.Name.Loc,
				"enum '%s' is already declared", e.Name).
				WithRelated(c.enums[e.Name].Decl.Name.Loc, "previous declaration is here"))
			continue
		case c.isTypeName(e.Name):
			c.errorf(diagnostic.DuplicateDeclaration, decl.Name.Loc, "cannot declare enum '%s': '%s' is a built-in type", e.Name, e.Name)
			continue
		}
		c.enums[e.Name] = e
		c.out.Enums = append(c.out.Enums, e)
		decls = append(decls, e)
	}
	for _, e := range decls {
		var byName map[string]*TypeParam
		e.Params, byName = c.declareTypeParams(e.Decl.TypeParams)
		c.typeParams = byName
		seen := map[string]*ast.Variant{}
		for _, v := range e.Decl.Variants {
			if v == nil || v.Name == nil {
				continue
			}
			if prev := seen[v.Name.Name]; prev != nil {
				c.diags = append(c.diags, diagnostic.Errorf(diagnostic.DuplicateDeclaration, v.Name.Loc,
					"variant '%s' is already declared in enum '%s'", v.Name.Name, e.Name).
					WithRelated(prev.Name.Loc, "previous declaration is here"))
				continue
			}
			seen[v.Name.Name] = v
			variant := &Variant{Enum: e, Name: v.Name.Name, Index: len(e.Variants)}
			for _, f := range v.Fields {
				t := c.resolveType(f)
				if t == Unit {
					c.errorf(diagnostic.UnitInCompositeType, f.Loc, "a variant cannot carry a Unit value")
					t = Invalid
				}
				variant.Fields = append(variant.Fields, t)
			}
			e.Variants = append(e.Variants, variant)
		}
		c.typeParams = nil
	}
}

func (c *checker) collectSignatures(prog *ast.Program) {
	for _, decl := range prog.Funcs {
		if decl == nil || decl.Name == nil {
			continue
		}
		fn := &Function{Name: decl.Name.Name, Decl: decl}
		var byName map[string]*TypeParam
		fn.TypeParams, byName = c.declareTypeParams(decl.TypeParams)
		c.typeParams = byName
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
		c.typeParams = nil
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
	if len(decl.TypeParams) > 0 {
		c.errorf(diagnostic.InvalidMainSignature, decl.Name.Loc, "'main' cannot have type parameters")
		valid = false
	}
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
	case decl.Result.Name != "Unit" || len(decl.Result.Args) > 0:
		c.errorf(diagnostic.InvalidMainSignature, decl.Result.Loc,
			"'main' must return Unit, not '%s'", ast.TypeString(decl.Result))
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
	c.typeParams = map[string]*TypeParam{}
	for _, p := range fn.TypeParams {
		if c.typeParams[p.Name] == nil {
			c.typeParams[p.Name] = p
		}
	}
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
	if fn.Result != Unit && fn.Result != Invalid && !c.alwaysReturns(decl.Body) {
		c.errorf(diagnostic.MissingReturn, decl.Name.Loc,
			"function '%s' does not return a value on every path; expected %s", fn.Name, fn.Result)
	}
	c.fn, c.scope, c.loops, c.typeParams = nil, nil, 0, nil
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
	case *ast.MatchStmt:
		c.checkMatch(s)
	case *ast.ExprStmt:
		if s.Call != nil {
			c.expr(s.Call)
		}
	}
}

func (c *checker) checkLet(s *ast.LetStmt) {
	var varType Type
	if s.Type != nil {
		varType = c.resolveType(s.Type)
		if s.Value != nil {
			c.exprAs(s.Value, varType)
		}
	} else {
		varType = Invalid
		if s.Value != nil {
			varType = c.expr(s.Value)
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

// exprAs checks e where a value of type want is required, reporting T001
// on a mismatch, and returns e's type.
func (c *checker) exprAs(e ast.Expr, want Type) Type {
	got := c.exprWant(e, want)
	if !compatible(want, got) {
		c.errorf(diagnostic.TypeMismatch, e.Span(), "expected %s but found %s", want, got)
	}
	return got
}

// rootIdent returns the variable at the root of an index path, or nil.
func rootIdent(e ast.Expr) *ast.Ident {
	for {
		switch x := e.(type) {
		case *ast.Ident:
			return x
		case *ast.IndexExpr:
			e = x.X
		default:
			return nil
		}
	}
}

// checkMutable reports T004 unless the variable at the root of target is
// a `let mut` variable. whole is true for `x = v`, false for changes to
// an element or through a method.
func (c *checker) checkMutable(target ast.Expr, span source.Span, whole bool) bool {
	id := rootIdent(target)
	if id == nil {
		c.errorf(diagnostic.AssignToImmutable, span, "only a variable, or an element of one, can be changed")
		return false
	}
	sym := c.scope.lookup(id.Name)
	if sym == nil {
		return false // reported when the target is checked
	}
	verb := "change"
	if whole {
		verb = "assign to"
	}
	switch sym.Kind {
	case FuncSymbol, BuiltinSymbol:
		c.errorf(diagnostic.AssignToImmutable, span, "cannot %s function '%s'", verb, sym.Name)
		return false
	case ParamSymbol:
		c.errorf(diagnostic.AssignToImmutable, span, "cannot %s parameter '%s'", verb, sym.Name)
		return false
	case LoopVarSymbol:
		c.diags = append(c.diags, diagnostic.Errorf(diagnostic.AssignToImmutable, span,
			"cannot %s loop variable '%s'", verb, sym.Name).
			WithRelated(sym.Decl, "loop variable declared here"))
		return false
	case MatchVarSymbol:
		c.diags = append(c.diags, diagnostic.Errorf(diagnostic.AssignToImmutable, span,
			"cannot %s '%s': names bound by a pattern are immutable", verb, sym.Name).
			WithRelated(sym.Decl, "bound here"))
		return false
	}
	if !sym.Mutable {
		msg := "cannot assign twice to immutable variable '%s'"
		if !whole {
			msg = "cannot change immutable variable '%s'"
		}
		c.diags = append(c.diags, diagnostic.Errorf(diagnostic.AssignToImmutable, span, msg, sym.Name).
			WithHelp("declare it with 'let mut' to allow changes").
			WithRelated(sym.Decl, "variable declared here"))
		return false
	}
	return true
}

func (c *checker) checkAssign(s *ast.AssignStmt) {
	if s.Target == nil {
		return
	}
	if id, ok := s.Target.(*ast.Ident); ok {
		sym := c.scope.lookup(id.Name)
		if sym == nil {
			c.expr(s.Value)
			c.errorf(diagnostic.UndefinedName, id.Loc, "undefined name '%s'", id.Name)
			return
		}
		c.out.Uses[id] = sym
		if !c.checkMutable(id, s.Loc, true) {
			c.expr(s.Value)
			return
		}
		c.exprAs(s.Value, sym.Type)
		return
	}
	// An element: a[i] = v, grid[r][c] = v, m["k"] = v.
	target := c.expr(s.Target)
	c.exprAs(s.Value, target)
	c.checkMutable(s.Target, s.Loc, false)
}

func (c *checker) checkIf(s *ast.IfStmt) {
	if s.Cond != nil {
		if t := c.exprWant(s.Cond, Bool); !compatible(Bool, t) {
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
		if t := c.exprWant(s.Cond, Bool); !compatible(Bool, t) {
			c.errorf(diagnostic.TypeMismatch, s.Cond.Span(), "condition must be Bool, found %s", t)
		}
	}
	c.loops++
	c.checkBlock(s.Body)
	c.loops--
}

// checkFor checks `for name in start..end { }` (ADR-0012) and
// `for name in collection { }` (ADR-0013). The range or collection is
// checked in the enclosing scope, before the loop variable exists. The
// loop variable lives in its own scope around the body, so the body may
// shadow it like a parameter.
func (c *checker) checkFor(s *ast.ForStmt) {
	varType := Int
	if s.Iter != nil {
		switch t := c.expr(s.Iter); t.Kind {
		case KArray, KMap:
			varType = t.Args[0] // an array's elements, a map's keys
		case KInvalid:
			varType = Invalid
		default:
			c.diags = append(c.diags, diagnostic.Errorf(diagnostic.NotIndexable, s.Iter.Span(),
				"cannot iterate over a value of type %s", t).
				WithHelp("loop over a range such as 0..n, or over an array or a map"))
			varType = Invalid
		}
	} else {
		for _, bound := range []ast.Expr{s.Start, s.End} {
			if bound == nil {
				continue // parser already reported
			}
			if t := c.exprWant(bound, Int); !compatible(Int, t) {
				c.errorf(diagnostic.TypeMismatch, bound.Span(), "range bound must be Int, found %s", t)
			}
		}
	}
	saved := c.scope
	c.scope = newScope(saved)
	if s.Var != nil {
		sym := &Symbol{Name: s.Var.Name, Kind: LoopVarSymbol, Type: varType, Decl: s.Var.Loc}
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
	if want == Unit {
		c.expr(s.Value)
		c.errorf(diagnostic.InvalidReturn, s.Loc, "cannot return a value from a function returning Unit")
		return
	}
	c.exprAs(s.Value, want)
}

// alwaysReturns implements docs/12 §12.7.
func (c *checker) alwaysReturns(s ast.Stmt) bool {
	switch s := s.(type) {
	case *ast.ReturnStmt:
		return true
	case *ast.Block:
		if s == nil {
			return false
		}
		for _, st := range s.Stmts {
			if c.alwaysReturns(st) {
				return true
			}
		}
	case *ast.IfStmt:
		if s.Then == nil || s.Else == nil {
			return false
		}
		return c.alwaysReturns(s.Then) && c.alwaysReturns(s.Else)
	case *ast.WhileStmt:
		// ADR-0012: `while true` with no `break` of its own never
		// completes, so control cannot fall off the end after it. No
		// other loop counts: its body may run zero times.
		return isTrueLiteral(s.Cond) && !breaks(s.Body)
	case *ast.MatchStmt:
		// ADR-0013: an exhaustive match whose every arm returns.
		if !c.out.Exhaustive[s] || len(s.Arms) == 0 {
			return false
		}
		for _, a := range s.Arms {
			if a == nil || !c.alwaysReturns(a.Body) {
				return false
			}
		}
		return true
	}
	return false
}

// compatible reports whether a value of type got may be used where want
// is required: the types are identical, ignoring Invalid anywhere inside
// them (error recovery).
func compatible(want, got Type) bool {
	if want == got || want == Invalid || got == Invalid {
		return true
	}
	if want.Kind != got.Kind || want.Enum != got.Enum || want.Param != got.Param || len(want.Args) != len(got.Args) {
		return false
	}
	if !want.containsInvalid() && !got.containsInvalid() {
		return false // interned and different
	}
	for i := range want.Args {
		if !compatible(want.Args[i], got.Args[i]) {
			return false
		}
	}
	return true
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
	case *ast.MatchStmt:
		for _, a := range s.Arms {
			if a != nil && breaks(a.Body) {
				return true
			}
		}
	}
	return false
}

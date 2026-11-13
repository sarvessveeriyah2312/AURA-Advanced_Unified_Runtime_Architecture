package checker

import (
	"errors"
	"math"
	"strconv"
	"strings"

	"github.com/sarvessveeriyah2312/Tread/internal/ast"
	"github.com/sarvessveeriyah2312/Tread/internal/diagnostic"
	"github.com/sarvessveeriyah2312/Tread/internal/source"
)

// expr type-checks e, records its type, and returns it.
func (c *checker) expr(e ast.Expr) Type { return c.exprWant(e, nil) }

// exprWant type-checks e where a value of type want is expected (nil when
// nothing is expected). The expected type lets `[]`, `[:]`, `Option.None`
// and generic calls infer their types (ADR-0013). exprWant does not
// itself report a mismatch with want; exprAs does.
func (c *checker) exprWant(e ast.Expr, want Type) Type {
	t := c.exprType(e, want)
	if e != nil {
		c.out.Types[e] = t
	}
	return t
}

func (c *checker) exprType(e ast.Expr, want Type) Type {
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
		return c.exprWant(e.Inner, want)
	case *ast.UnaryExpr:
		return c.unary(e)
	case *ast.BinaryExpr:
		return c.binary(e)
	case *ast.CallExpr:
		return c.call(e, want)
	case *ast.ArrayLit:
		return c.arrayLit(e, want)
	case *ast.MapLit:
		return c.mapLit(e, want)
	case *ast.IndexExpr:
		return c.index(e)
	case *ast.MemberExpr:
		return c.member(e, want)
	case *ast.StructLit:
		return c.structLit(e, want)
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
		if en := c.enums[e.Name]; en != nil && len(en.Variants) > 0 {
			c.errorf(diagnostic.EnumAsValue, e.Loc,
				"'%s' is an enum type, not a value; use one of its variants, such as %s.%s", e.Name, e.Name, en.Variants[0].Name)
			return Invalid
		}
		if c.structs[e.Name] != nil {
			c.errorf(diagnostic.EnumAsValue, e.Loc,
				"'%s' is a struct type, not a value; build one with %s { ... }", e.Name, e.Name)
			return Invalid
		}
		c.errorf(diagnostic.UndefinedName, e.Loc, "undefined name '%s'", e.Name)
		return Invalid
	}
	c.out.Uses[e] = sym
	if sym.Kind == ModuleSymbol {
		c.errorf(diagnostic.EnumAsValue, e.Loc,
			"'%s' is a module, not a value; use one of its items, such as %s.name", e.Name, e.Name)
		return Invalid
	}
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

// binary implements the operator table of ADR-0004 item 6. `==` and `!=`
// compare any two values of the same type except Unit (ADR-0013).
func (c *checker) binary(e *ast.BinaryExpr) Type {
	op := e.Op.String()
	l := c.expr(e.Left)
	var r Type
	if (op == "==" || op == "!=") && l != Invalid {
		r = c.exprWant(e.Right, l) // lets `x == Option.None` infer
	} else {
		r = c.expr(e.Right)
	}
	if l == Invalid || r == Invalid {
		return Invalid
	}
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
		if compatible(l, r) && l != Unit {
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

// ---------------------------------------------------------------------------
// Calls

func (c *checker) call(e *ast.CallExpr, want Type) Type {
	switch callee := e.Callee.(type) {
	case *ast.Ident:
		sym := c.scope.lookup(callee.Name)
		if sym == nil {
			c.errorf(diagnostic.UndefinedName, callee.Loc, "undefined function '%s'", callee.Name)
			c.checkArgs(e.Args)
			return Invalid
		}
		c.out.Uses[callee] = sym
		c.out.Types[callee] = Invalid // a callee name has no value type
		switch sym.Kind {
		case BuiltinSymbol:
			return c.callBuiltin(e, sym.Func)
		case ModuleSymbol:
			c.errorf(diagnostic.NotCallable, callee.Loc, "'%s' is a module, not a function; call one of its functions, as in %s.name()", callee.Name, callee.Name)
			c.checkArgs(e.Args)
			return Invalid
		case FuncSymbol:
			fn := sym.Func
			paramTypes := make([]Type, len(fn.Params))
			for i, p := range fn.Params {
				paramTypes[i] = p.Type
			}
			return c.instantiate(call{
				span: e.Loc, args: e.Args, params: fn.TypeParams, paramTypes: paramTypes, result: fn.Result,
				arity: func(got int) string {
					return "function '" + fn.Name + "' expects " + strconv.Itoa(len(fn.Params)) + " argument(s) but " + strconv.Itoa(got) + " were given"
				},
				argName: "'" + fn.Name + "'",
				what:    "function '" + fn.Name + "'",
			}, want)
		default:
			c.errorf(diagnostic.NotCallable, callee.Loc, "'%s' is a %s, not a function", callee.Name, sym.Type)
			c.checkArgs(e.Args)
			return Invalid
		}
	case *ast.MemberExpr:
		if en, st, isType, ok := c.typeRef(callee.X); isType {
			switch {
			case !ok:
				c.checkArgs(e.Args)
				return Invalid
			case en != nil:
				return c.construct(e, callee, en, e.Args, true, want)
			default:
				return c.associatedCall(e, st, callee, want)
			}
		}
		if id, ok := callee.X.(*ast.Ident); ok {
			if m := c.moduleNamed(id); m != nil {
				return c.qualifiedCall(e, m, callee, want)
			}
		}
		return c.methodCall(e, callee, want)
	}
	// Not a name: e.g. f()() or (f)(1). Tread has no function values.
	if t := c.expr(e.Callee); t != Invalid {
		c.errorf(diagnostic.NotCallable, e.Callee.Span(), "expression of type %s is not callable", t)
	}
	c.checkArgs(e.Args)
	return Invalid
}

// checkArgs checks arguments of a call that cannot be resolved.
func (c *checker) checkArgs(args []ast.Expr) {
	for _, a := range args {
		c.expr(a)
	}
}

// call describes a call to a function or a variant for instantiate.
type call struct {
	span       source.Span
	args       []ast.Expr
	params     []*TypeParam // the callee's type parameters; nil if not generic
	paramTypes []Type
	result     Type
	arity      func(got int) string // the T006 message
	argName    string               // for "argument 1 of 'f'"
	what       string               // for "type parameter 'T' of function 'f'"
	labels     []string             // names the arguments, as "field 'x'"; nil for "argument 1"
}

// instantiate checks a call: arity, argument types, and, for a generic
// callee, inference of its type arguments from the arguments and from the
// expected result type (ADR-0013). It returns the result type.
func (c *checker) instantiate(k call, want Type) Type {
	bind := map[*TypeParam]Type{}
	isParam := map[*TypeParam]bool{}
	for _, p := range k.params {
		isParam[p] = true
	}
	if len(k.params) > 0 && want != nil && want != Invalid {
		trial := map[*TypeParam]Type{}
		if unify(k.result, want, trial, isParam) {
			bind = trial
		}
	}
	if len(k.args) != len(k.paramTypes) {
		c.errorf(diagnostic.WrongArgumentCount, k.span, "%s", k.arity(len(k.args)))
		c.checkArgs(k.args)
		if len(k.params) > 0 {
			return Invalid
		}
		return k.result
	}
	// Arguments that need an expected type to be typed at all (such as
	// Tree.Leaf, [] or [:]) are checked after the others, so that, in
	// Tree.Node(Tree.Leaf, "x", Tree.Leaf), "x" first binds T.
	checkArg := func(i int) {
		arg, pt := k.args[i], k.paramTypes[i]
		expected := subst(pt, bind)
		// Only parameters not yet bound are unknown. One bound to itself
		// (inside its own struct's impl, ADR-0014) is a known type there.
		unbound := map[*TypeParam]bool{}
		for p := range isParam {
			if _, ok := bind[p]; !ok {
				unbound[p] = true
			}
		}
		if mentions(expected, unbound) {
			expected = nil
		}
		at := c.exprWant(arg, expected)
		if !unify(pt, at, bind, isParam) {
			label := "argument " + strconv.Itoa(i+1)
			if k.labels != nil {
				label = k.labels[i]
			}
			c.errorf(diagnostic.TypeMismatch, arg.Span(),
				"%s of %s: expected %s but found %s", label, k.argName, displayType(subst(pt, bind)), at)
		}
	}
	var deferred []int
	for i, arg := range k.args {
		if len(k.params) > 0 && c.needsContext(arg) {
			deferred = append(deferred, i)
			continue
		}
		checkArg(i)
	}
	for _, i := range deferred {
		checkArg(i)
	}
	if len(k.params) == 0 {
		return k.result
	}
	if want != nil && want != Invalid {
		unify(k.result, want, bind, isParam)
	}
	ok := true
	for _, p := range k.params {
		t, bound := bind[p]
		switch {
		case !bound && want == Invalid:
			// The expected type already produced a diagnostic.
			bind[p] = Invalid
			ok = false
		case !bound:
			c.diags = append(c.diags, diagnostic.Errorf(diagnostic.CannotInfer, k.span,
				"cannot infer type parameter '%s' of %s", p.Name, k.what).
				WithHelp("add a type annotation, for example 'let x: Option<Int> = ...'"))
			bind[p] = Invalid
			ok = false
		case t.containsUnit():
			c.errorf(diagnostic.UnitInCompositeType, k.span, "type parameter '%s' of %s cannot be Unit", p.Name, k.what)
			bind[p] = Invalid
		}
	}
	if !ok {
		return Invalid
	}
	return subst(k.result, bind)
}

// unify matches the parameter type p (which may mention the callee's type
// parameters, isParam) against the argument type a, recording bindings.
// It reports whether they are consistent.
func unify(p, a Type, bind map[*TypeParam]Type, isParam map[*TypeParam]bool) bool {
	if p == Invalid || a == Invalid {
		return true
	}
	if p.Kind == KParam && isParam[p.Param] {
		if b, ok := bind[p.Param]; ok {
			if b.containsInvalid() && !a.containsInvalid() {
				bind[p.Param] = a // a better binding
			}
			return compatible(b, a)
		}
		bind[p.Param] = a
		return true
	}
	if p.Kind != a.Kind || p.Enum != a.Enum || p.Param != a.Param || p.Struct != a.Struct || len(p.Args) != len(a.Args) {
		return false
	}
	for i := range p.Args {
		if !unify(p.Args[i], a.Args[i], bind, isParam) {
			return false
		}
	}
	return true
}

// needsContext reports whether e can only be typed from an expected type:
// an empty collection literal, or a generic enum variant whose type
// arguments its own fields cannot determine.
func (c *checker) needsContext(e ast.Expr) bool {
	switch x := e.(type) {
	case *ast.ParenExpr:
		return c.needsContext(x.Inner)
	case *ast.ArrayLit:
		for _, el := range x.Elems {
			if !c.needsContext(el) {
				return false
			}
		}
		return true
	case *ast.MapLit:
		return len(x.Keys) == 0
	case *ast.MemberExpr:
		switch y := x.X.(type) {
		case *ast.Ident:
			return c.enums[y.Name] != nil && len(c.enums[y.Name].Params) > 0
		case *ast.MemberExpr:
			// module.Enum.Variant (ADR-0014)
			id, ok := y.X.(*ast.Ident)
			if !ok || c.enums[id.Name] != nil || c.structs[id.Name] != nil {
				return false
			}
			sym := c.lookup(id.Name)
			if sym == nil || sym.Kind != ModuleSymbol {
				return false
			}
			en := sym.mod.enums[y.Name.Name]
			return en != nil && len(en.Params) > 0
		}
		return false
	case *ast.CallExpr:
		m, ok := x.Callee.(*ast.MemberExpr)
		if !ok || !c.needsContext(m) {
			return false
		}
		for _, a := range x.Args {
			if !c.needsContext(a) {
				return false
			}
		}
		return true
	}
	return false
}

// mentions reports whether t refers to any of the given type parameters.
func mentions(t Type, params map[*TypeParam]bool) bool {
	if t.Kind == KParam && params[t.Param] {
		return true
	}
	for _, a := range t.Args {
		if mentions(a, params) {
			return true
		}
	}
	return false
}

// displayType renders a type in a message; unknown parts show as "_".
func displayType(t Type) string { return t.String() }

// callBuiltin checks print and toString (docs/02 §2.14, ADR-0004 items 7
// and 13). Both take exactly one value of any type except Unit
// (ADR-0013).
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
			"'%s' cannot take a value of type Unit; it accepts any other value", fn.Name)
	}
	return fn.Result
}

// ---------------------------------------------------------------------------
// Collections (ADR-0013)

func (c *checker) arrayLit(e *ast.ArrayLit, want Type) Type {
	var elem Type
	if want != nil && want.Kind == KArray {
		elem = want.Args[0]
	}
	if len(e.Elems) == 0 {
		switch {
		case elem != nil:
			return ArrayOf(elem)
		case want == Invalid:
			return Invalid
		}
		c.diags = append(c.diags, diagnostic.Errorf(diagnostic.CannotInfer, e.Loc,
			"cannot infer the element type of an empty array").
			WithHelp("add a type annotation, for example 'let xs: Array<Int> = []'"))
		return Invalid
	}
	first := -1
	if elem == nil {
		// The element type comes from the first element that can be
		// typed on its own, so [Option.None, Option.Some(2)] works.
		first = 0
		for i, el := range e.Elems {
			if !c.needsContext(el) {
				first = i
				break
			}
		}
		elem = c.expr(e.Elems[first])
		if elem == Unit {
			c.errorf(diagnostic.UnitInCompositeType, e.Elems[first].Span(), "an array cannot hold Unit values")
			elem = Invalid
		}
	}
	for i, el := range e.Elems {
		if i != first {
			c.exprAs(el, elem)
		}
	}
	return ArrayOf(elem)
}

func (c *checker) mapLit(e *ast.MapLit, want Type) Type {
	var key, value Type
	if want != nil && want.Kind == KMap {
		key, value = want.Args[0], want.Args[1]
	}
	if len(e.Keys) == 0 {
		switch {
		case key != nil:
			return MapOf(key, value)
		case want == Invalid:
			return Invalid
		}
		c.diags = append(c.diags, diagnostic.Errorf(diagnostic.CannotInfer, e.Loc,
			"cannot infer the key and value types of an empty map").
			WithHelp("add a type annotation, for example 'let m: Map<String, Int> = [:]'"))
		return Invalid
	}
	for i := range e.Keys {
		if key == nil {
			key = c.expr(e.Keys[i])
			if !key.isMapKey() {
				c.errorf(diagnostic.InvalidMapKey, e.Keys[i].Span(), "map keys must be Int, String or Bool, not %s", key)
				key = Invalid
			}
		} else {
			c.exprAs(e.Keys[i], key)
		}
		if value == nil {
			value = c.expr(e.Values[i])
			if value == Unit {
				c.errorf(diagnostic.UnitInCompositeType, e.Values[i].Span(), "a map cannot hold Unit values")
				value = Invalid
			}
		} else {
			c.exprAs(e.Values[i], value)
		}
	}
	return MapOf(key, value)
}

func (c *checker) index(e *ast.IndexExpr) Type {
	xt := c.expr(e.X)
	switch xt.Kind {
	case KArray:
		if t := c.exprWant(e.Index, Int); !compatible(Int, t) {
			c.errorf(diagnostic.TypeMismatch, e.Index.Span(), "an array index must be Int, found %s", t)
		}
		return xt.Args[0]
	case KMap:
		if t := c.exprWant(e.Index, xt.Args[0]); !compatible(xt.Args[0], t) {
			c.errorf(diagnostic.TypeMismatch, e.Index.Span(), "this map's keys are %s, found %s", xt.Args[0], t)
		}
		return xt.Args[1]
	case KInvalid:
		c.expr(e.Index)
		return Invalid
	}
	c.expr(e.Index)
	c.errorf(diagnostic.NotIndexable, e.X.Span(), "cannot index a value of type %s; only arrays and maps can be indexed", xt)
	return Invalid
}

// ---------------------------------------------------------------------------
// Enum variants and methods (ADR-0013)

// member checks `X.name` outside a call: a variant without fields, such as
// Shape.Empty or Option.None.
func (c *checker) member(e *ast.MemberExpr, want Type) Type {
	if en, st, isType, ok := c.typeRef(e.X); isType {
		switch {
		case !ok:
			return Invalid
		case en != nil:
			return c.construct(e, e, en, nil, false, want)
		default:
			if fn := st.Methods[e.Name.Name]; fn != nil {
				c.errorf(diagnostic.FunctionAsValue, e.Loc, "'%s.%s' is a function and cannot be used as a value; call it with ()", st.Name, e.Name.Name)
			} else {
				c.errorf(diagnostic.UnknownMethod, e.Name.Loc, "struct '%s' has no associated function '%s'", qualified(st.Module, st.Name), e.Name.Name)
			}
			return Invalid
		}
	}
	if id, ok := e.X.(*ast.Ident); ok {
		if m := c.moduleNamed(id); m != nil {
			name := e.Name.Name
			switch sym := m.scope.names[name]; {
			case sym != nil && sym.Kind == FuncSymbol:
				c.errorf(diagnostic.FunctionAsValue, e.Loc, "function '%s.%s' cannot be used as a value; functions can only be called", id.Name, name)
			case (m.enums[name] != nil && m.enums[name].Decl != nil) || m.structs[name] != nil:
				c.errorf(diagnostic.EnumAsValue, e.Loc, "'%s.%s' is a type, not a value", id.Name, name)
			default:
				c.errorf(diagnostic.UndefinedName, e.Name.Loc, "module '%s' has no item '%s'", m.path, name)
			}
			return Invalid
		}
	}
	t := c.expr(e.X)
	if t == Invalid {
		return Invalid
	}
	if t.Kind == KStruct {
		return c.field(e, t)
	}
	if _, ok := methodSig(t, e.Name.Name, c.enums["Option"]); ok {
		c.errorf(diagnostic.UnknownMethod, e.Loc, "'%s' is a method; call it with parentheses: %s()", e.Name.Name, e.Name.Name)
	} else {
		c.errorf(diagnostic.UnknownMethod, e.Name.Loc, "%s has no field or method '%s'", t, e.Name.Name)
	}
	return Invalid
}

// construct checks a variant construction: `Enum.Variant(args)` (called
// set) or `Enum.Variant`. node is the expression recorded in Variants.
func (c *checker) construct(node ast.Expr, m *ast.MemberExpr, en *Enum, args []ast.Expr, called bool, want Type) Type {
	v := en.Lookup(m.Name.Name)
	if v == nil {
		c.errorf(diagnostic.UnknownVariant, m.Name.Loc, "enum '%s' has no variant '%s'", en.Name, m.Name.Name)
		c.checkArgs(args)
		return Invalid
	}
	c.out.Variants[node] = v
	name := en.Name + "." + v.Name
	if called && len(v.Fields) == 0 {
		c.errorf(diagnostic.WrongArgumentCount, node.Span(), "%s has no fields; write it without parentheses", name)
		c.checkArgs(args)
		return Invalid
	}
	if !called && len(v.Fields) > 0 {
		c.errorf(diagnostic.WrongArgumentCount, node.Span(), "%s needs %d value(s): write %s(...)", name, len(v.Fields), name)
		return Invalid
	}
	resultArgs := make([]Type, len(en.Params))
	for i, p := range en.Params {
		resultArgs[i] = ParamType(p)
	}
	return c.instantiate(call{
		span: node.Span(), args: args, params: en.Params, paramTypes: v.Fields, result: EnumOf(en, resultArgs),
		arity: func(got int) string {
			return name + " has " + strconv.Itoa(len(v.Fields)) + " field(s) but " + strconv.Itoa(got) + " value(s) were given"
		},
		argName: name,
		what:    name,
	}, want)
}

// methodSig returns the signature of a built-in method of t (ADR-0013).
type methodInfo struct {
	method   Method
	params   []Type
	result   Type
	mutating bool
}

func methodSig(t Type, name string, option *Enum) (methodInfo, bool) {
	switch t.Kind {
	case KArray:
		elem := t.Args[0]
		switch name {
		case "len":
			return methodInfo{method: MethodLen, result: Int}, true
		case "push":
			return methodInfo{method: MethodPush, params: []Type{elem}, result: Unit, mutating: true}, true
		case "pop":
			return methodInfo{method: MethodPop, result: EnumOf(option, []Type{elem}), mutating: true}, true
		case "contains":
			return methodInfo{method: MethodContains, params: []Type{elem}, result: Bool}, true
		}
	case KMap:
		key, value := t.Args[0], t.Args[1]
		switch name {
		case "len":
			return methodInfo{method: MethodLen, result: Int}, true
		case "get":
			return methodInfo{method: MethodGet, params: []Type{key}, result: EnumOf(option, []Type{value})}, true
		case "contains":
			return methodInfo{method: MethodContains, params: []Type{key}, result: Bool}, true
		case "remove":
			return methodInfo{method: MethodRemove, params: []Type{key}, result: EnumOf(option, []Type{value}), mutating: true}, true
		case "keys":
			return methodInfo{method: MethodKeys, result: ArrayOf(key)}, true
		}
	}
	return methodInfo{}, false
}

func methodNames(t Type) string {
	switch t.Kind {
	case KArray:
		return "len, push, pop, contains"
	case KMap:
		return "len, get, contains, remove, keys"
	}
	return ""
}

func (c *checker) methodCall(e *ast.CallExpr, m *ast.MemberExpr, want Type) Type {
	recv := c.expr(m.X)
	if recv == Invalid {
		c.checkArgs(e.Args)
		return Invalid
	}
	if recv.Kind == KStruct {
		return c.structMethodCall(e, m, recv, want)
	}
	name := m.Name.Name
	sig, ok := methodSig(recv, name, c.enums["Option"])
	if !ok {
		d := diagnostic.Errorf(diagnostic.UnknownMethod, m.Name.Loc, "%s has no method '%s'", recv, name)
		if names := methodNames(recv); names != "" {
			d = d.WithHelp("the methods of " + recv.String() + " are " + names)
		}
		c.diags = append(c.diags, d)
		c.checkArgs(e.Args)
		return Invalid
	}
	c.out.Methods[e] = sig.method
	c.out.Types[m] = Invalid // a method name has no value type
	if len(e.Args) != len(sig.params) {
		c.errorf(diagnostic.WrongArgumentCount, e.Loc, "method '%s' expects %d argument(s) but %d were given", name, len(sig.params), len(e.Args))
		c.checkArgs(e.Args)
	} else {
		for i, a := range e.Args {
			if t := c.exprWant(a, sig.params[i]); !compatible(sig.params[i], t) {
				c.errorf(diagnostic.TypeMismatch, a.Span(), "argument %d of '%s': expected %s but found %s", i+1, name, sig.params[i], t)
			}
		}
	}
	if sig.mutating {
		c.checkMutable(m.X, e.Loc, false)
	}
	return sig.result
}

package checker

import (
	"sort"
	"strconv"
	"strings"

	"github.com/sarvessveeriyah2312/Tread/internal/ast"
	"github.com/sarvessveeriyah2312/Tread/internal/diagnostic"
)

// Structs, methods and module-qualified names (ADR-0014).

// lookup resolves a name in the current value scope: the function body's
// scope while checking a body, the module scope otherwise.
func (c *checker) lookup(name string) *Symbol {
	if c.scope != nil {
		return c.scope.lookup(name)
	}
	return c.module.lookup(name)
}

// moduleNamed returns the module that id names, or nil.
func (c *checker) moduleNamed(id *ast.Ident) *moduleInfo {
	sym := c.lookup(id.Name)
	if sym == nil || sym.Kind != ModuleSymbol {
		return nil
	}
	c.out.Uses[id] = sym
	return sym.mod
}

// private reports T034 for an item of module m that is not pub, used
// from another module. It reports whether the item is usable.
func (c *checker) private(m *moduleInfo, pub bool, what string, at *ast.Ident) bool {
	if pub || m == c.cur {
		return true
	}
	c.diags = append(c.diags, diagnostic.Errorf(diagnostic.PrivateItem, at.Loc,
		"%s is private to module '%s'", what, m.path).
		WithHelp("mark it 'pub' in "+m.path+" to use it from other files"))
	return false
}

// qualifiedType resolves `module.Name` to an enum or a struct. It reports
// whether the name resolved; errors are reported.
func (c *checker) qualifiedType(modID, name *ast.Ident) (*Enum, *Struct, bool) {
	m := c.moduleNamed(modID)
	if m == nil {
		c.errorf(diagnostic.UnknownType, modID.Loc, "unknown module '%s'; import it with 'import %s'", modID.Name, modID.Name)
		return nil, nil, false
	}
	if e := m.enums[name.Name]; e != nil && e.Decl != nil {
		return e, nil, c.private(m, e.Pub, "enum '"+e.Name+"'", name)
	}
	if s := m.structs[name.Name]; s != nil {
		return nil, s, c.private(m, s.Pub, "struct '"+s.Name+"'", name)
	}
	if sym := m.scope.names[name.Name]; sym != nil && sym.Kind == FuncSymbol {
		c.errorf(diagnostic.UnknownType, name.Loc, "'%s.%s' is a function, not a type", modID.Name, name.Name)
		return nil, nil, false
	}
	c.errorf(diagnostic.UnknownType, name.Loc, "module '%s' has no type '%s'", m.path, name.Name)
	return nil, nil, false
}

// typeRef reports whether e names a type: `Name` for an enum or struct of
// the current module, or `module.Name`. ok is false when e names a type
// but an error was reported.
func (c *checker) typeRef(e ast.Expr) (en *Enum, st *Struct, isType, ok bool) {
	switch x := e.(type) {
	case *ast.Ident:
		if en := c.enums[x.Name]; en != nil {
			return en, nil, true, true
		}
		if st := c.structs[x.Name]; st != nil {
			return nil, st, true, true
		}
	case *ast.MemberExpr:
		id, isIdent := x.X.(*ast.Ident)
		if !isIdent || c.enums[id.Name] != nil || c.structs[id.Name] != nil || c.moduleNamed(id) == nil {
			return nil, nil, false, false
		}
		en, st, ok := c.qualifiedType(id, x.Name)
		return en, st, true, ok
	}
	return nil, nil, false, false
}

// ---------------------------------------------------------------------------
// Struct literals and fields

func (c *checker) structLit(e *ast.StructLit, want Type) Type {
	var st *Struct
	if e.Module != nil {
		en, s, ok := c.qualifiedType(e.Module, e.Name)
		if ok && en != nil {
			c.errorf(diagnostic.InvalidStructLiteral, e.Name.Loc, "'%s' is an enum, not a struct; build a variant such as %s.%s.%s", e.Name.Name, e.Module.Name, en.Name, firstVariant(en))
		}
		if !ok || s == nil {
			c.checkFieldValues(e)
			return Invalid
		}
		st = s
	} else {
		st = c.structs[e.Name.Name]
		if st == nil {
			if en := c.enums[e.Name.Name]; en != nil {
				c.errorf(diagnostic.InvalidStructLiteral, e.Name.Loc, "'%s' is an enum, not a struct; build a variant such as %s.%s", en.Name, en.Name, firstVariant(en))
			} else {
				c.errorf(diagnostic.UnknownType, e.Name.Loc, "unknown struct '%s'", e.Name.Name)
			}
			c.checkFieldValues(e)
			return Invalid
		}
	}
	name := qualified(st.Module, st.Name)
	ok := true
	if st.mod != c.cur {
		for _, f := range st.Fields {
			if !f.Pub {
				c.diags = append(c.diags, diagnostic.Errorf(diagnostic.PrivateItem, e.Name.Loc,
					"cannot build '%s' here: its field '%s' is private to module '%s'", name, f.Name, st.mod.path).
					WithHelp("use a pub function of "+st.mod.path+" that builds it, or mark every field 'pub'"))
				ok = false
				break
			}
		}
	}
	values := make([]ast.Expr, len(st.Fields))
	seen := map[string]*ast.Ident{}
	for _, fi := range e.Fields {
		if fi == nil || fi.Name == nil {
			ok = false
			continue
		}
		f := st.Field(fi.Name.Name)
		switch {
		case seen[fi.Name.Name] != nil:
			c.diags = append(c.diags, diagnostic.Errorf(diagnostic.InvalidStructLiteral, fi.Name.Loc,
				"field '%s' is given twice", fi.Name.Name).
				WithRelated(seen[fi.Name.Name].Loc, "first given here"))
			ok = false
		case f == nil:
			c.diags = append(c.diags, diagnostic.Errorf(diagnostic.InvalidStructLiteral, fi.Name.Loc,
				"struct '%s' has no field '%s'", name, fi.Name.Name).
				WithHelp("its fields are "+fieldNames(st)))
			ok = false
		default:
			values[f.Index] = fi.Value
		}
		seen[fi.Name.Name] = fi.Name
	}
	var missing []string
	for i, f := range st.Fields {
		if values[i] == nil && seen[f.Name] == nil {
			missing = append(missing, f.Name)
		}
	}
	if len(missing) > 0 {
		c.errorf(diagnostic.InvalidStructLiteral, e.Name.Loc, "missing field(s) %s in '%s'", quoteAll(missing), name)
		ok = false
	}
	if !ok {
		c.checkFieldValues(e)
		if len(st.Params) == 0 {
			return StructOf(st, nil)
		}
		return Invalid
	}
	resultArgs := make([]Type, len(st.Params))
	for i, p := range st.Params {
		resultArgs[i] = ParamType(p)
	}
	paramTypes := make([]Type, len(st.Fields))
	for i, f := range st.Fields {
		paramTypes[i] = f.Type
	}
	return c.instantiate(call{
		span: e.Loc, args: values, params: st.Params, paramTypes: paramTypes, result: StructOf(st, resultArgs),
		arity:   func(int) string { return "" },
		argName: "'" + name + "'",
		what:    "struct '" + name + "'",
		labels:  fieldLabels(st),
	}, want)
}

func (c *checker) checkFieldValues(e *ast.StructLit) {
	for _, fi := range e.Fields {
		if fi != nil {
			c.expr(fi.Value)
		}
	}
}

func fieldLabels(s *Struct) []string {
	out := make([]string, len(s.Fields))
	for i, f := range s.Fields {
		out[i] = "field '" + f.Name + "'"
	}
	return out
}

func firstVariant(e *Enum) string {
	if len(e.Variants) == 0 {
		return "Variant"
	}
	return e.Variants[0].Name
}

func fieldNames(s *Struct) string {
	if len(s.Fields) == 0 {
		return "(none)"
	}
	names := make([]string, len(s.Fields))
	for i, f := range s.Fields {
		names[i] = f.Name
	}
	return strings.Join(names, ", ")
}

func quoteAll(names []string) string {
	q := make([]string, len(names))
	for i, n := range names {
		q[i] = "'" + n + "'"
	}
	return strings.Join(q, ", ")
}

// field checks the field access e.X.Name on a struct value of type t.
func (c *checker) field(e *ast.MemberExpr, t Type) Type {
	st := t.Struct
	f := st.Field(e.Name.Name)
	if f == nil {
		if _, isMethod := st.Methods[e.Name.Name]; isMethod {
			c.errorf(diagnostic.UnknownMethod, e.Loc, "'%s' is a method; call it with parentheses: %s()", e.Name.Name, e.Name.Name)
			return Invalid
		}
		c.diags = append(c.diags, diagnostic.Errorf(diagnostic.UnknownMethod, e.Name.Loc,
			"%s has no field '%s'", t, e.Name.Name).
			WithHelp("its fields are "+fieldNames(st)))
		return Invalid
	}
	if !c.private(st.mod, f.Pub, "field '"+f.Name+"' of '"+qualified(st.Module, st.Name)+"'", e.Name) {
		return Invalid
	}
	c.out.Fields[e] = f
	return subst(f.Type, enumArgs(t))
}

// ---------------------------------------------------------------------------
// Calls of methods, associated functions and module functions

// callFunction checks a call of a user function fn with the given
// arguments. bind holds already-known type arguments (from a receiver);
// params are the type parameters still to infer.
func (c *checker) callFunction(e *ast.CallExpr, fn *Function, args []ast.Expr, paramSyms []*Symbol, params []*TypeParam, bind map[*TypeParam]Type, want Type) Type {
	paramTypes := make([]Type, len(paramSyms))
	for i, p := range paramSyms {
		paramTypes[i] = subst(p.Type, bind)
	}
	name := fn.FullName()
	c.out.Calls[e] = fn
	return c.instantiate(call{
		span: e.Loc, args: args, params: params, paramTypes: paramTypes, result: subst(fn.Result, bind),
		arity: func(got int) string {
			return "function '" + name + "' expects " + strconv.Itoa(len(paramSyms)) + " argument(s) but " + strconv.Itoa(got) + " were given"
		},
		argName: "'" + name + "'",
		what:    "function '" + name + "'",
	}, want)
}

// qualifiedCall checks `module.f(args)`.
func (c *checker) qualifiedCall(e *ast.CallExpr, m *moduleInfo, callee *ast.MemberExpr, want Type) Type {
	name := callee.Name.Name
	sym := m.scope.names[name]
	if sym == nil || sym.Kind != FuncSymbol {
		switch {
		case m.enums[name] != nil && m.enums[name].Decl != nil:
			c.errorf(diagnostic.NotCallable, callee.Name.Loc, "'%s.%s' is an enum; call one of its variants, such as %s.%s.%s()", m.name, name, m.name, name, firstVariant(m.enums[name]))
		case m.structs[name] != nil:
			c.errorf(diagnostic.NotCallable, callee.Name.Loc, "'%s.%s' is a struct; build one with %s.%s { ... }", m.name, name, m.name, name)
		default:
			c.errorf(diagnostic.UndefinedName, callee.Name.Loc, "module '%s' has no function '%s'", m.path, name)
		}
		c.checkArgs(e.Args)
		return Invalid
	}
	fn := sym.Func
	if !c.private(m, fn.Pub, "function '"+name+"'", callee.Name) {
		c.checkArgs(e.Args)
		return Invalid
	}
	c.out.Types[callee] = Invalid // a callee has no value type
	return c.callFunction(e, fn, e.Args, fn.Params, fn.TypeParams, nil, want)
}

// associatedCall checks `Struct.f(args)` (or `module.Struct.f(args)`).
func (c *checker) associatedCall(e *ast.CallExpr, st *Struct, callee *ast.MemberExpr, want Type) Type {
	name := callee.Name.Name
	fn := st.Methods[name]
	sname := qualified(st.Module, st.Name)
	if fn == nil {
		c.diags = append(c.diags, diagnostic.Errorf(diagnostic.UnknownMethod, callee.Name.Loc,
			"struct '%s' has no associated function '%s'", sname, name).
			WithHelp("build a value with "+sname+" { ... }"))
		c.checkArgs(e.Args)
		return Invalid
	}
	if !c.private(st.mod, fn.Pub, "function '"+st.Name+"."+name+"'", callee.Name) {
		c.checkArgs(e.Args)
		return Invalid
	}
	if fn.HasSelf {
		c.errorf(diagnostic.UnknownMethod, callee.Loc, "'%s' is a method; call it on a value of type %s, as in value.%s()", name, sname, name)
		c.checkArgs(e.Args)
		return Invalid
	}
	c.out.Types[callee] = Invalid
	params := append(append([]*TypeParam(nil), st.Params...), fn.TypeParams...)
	return c.callFunction(e, fn, e.Args, fn.Params, params, nil, want)
}

// structMethodCall checks `value.method(args)` on a struct value of type
// recv.
func (c *checker) structMethodCall(e *ast.CallExpr, m *ast.MemberExpr, recv Type, want Type) Type {
	st := recv.Struct
	name := m.Name.Name
	fn := st.Methods[name]
	if fn == nil {
		if st.Field(name) != nil {
			c.errorf(diagnostic.NotCallable, m.Loc, "'%s' is a field, not a method", name)
		} else {
			d := diagnostic.Errorf(diagnostic.UnknownMethod, m.Name.Loc, "%s has no method '%s'", recv, name)
			if names := methodList(st); names != "" {
				d = d.WithHelp("the methods of " + recv.String() + " are " + names)
			}
			c.diags = append(c.diags, d)
		}
		c.checkArgs(e.Args)
		return Invalid
	}
	if !c.private(st.mod, fn.Pub, "method '"+name+"' of '"+qualified(st.Module, st.Name)+"'", m.Name) {
		c.checkArgs(e.Args)
		return Invalid
	}
	if !fn.HasSelf {
		c.errorf(diagnostic.UnknownMethod, m.Loc, "'%s' has no 'self' parameter; call it as %s.%s(...)", name, st.Name, name)
		c.checkArgs(e.Args)
		return Invalid
	}
	c.out.Types[m] = Invalid // a method name has no value type
	if fn.SelfMut {
		c.checkMutable(m.X, e.Loc, false)
	}
	return c.callFunction(e, fn, e.Args, fn.Params[1:], fn.TypeParams, enumArgs(recv), want)
}

func methodList(st *Struct) string {
	var names []string
	for n, fn := range st.Methods {
		if fn.HasSelf {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

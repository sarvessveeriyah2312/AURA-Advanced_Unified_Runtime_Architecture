package checker

import (
	"sort"
	"strings"

	"github.com/sarvessveeriyah2312/Tread/internal/ast"
	"github.com/sarvessveeriyah2312/Tread/internal/diagnostic"
	"github.com/sarvessveeriyah2312/Tread/internal/source"
)

// minIntMagnitude is the digit string of |math.MinInt64|. It is valid only
// as the direct operand of unary minus (ADR-0004 item 1).
const minIntMagnitude = "9223372036854775808"

// SourceModule is one file of a program, for CheckModules (ADR-0014).
type SourceModule struct {
	// Path is the dotted import path, such as "shapes.circle"; "" for the
	// root file.
	Path    string
	Program *ast.Program
	// Imports maps each import declaration of the file to the index of
	// the imported module. An import missing here was already reported.
	Imports map[*ast.ImportDecl]int
}

// Check performs name resolution and type checking of a single-file
// program (docs/12). It never panics on user programs and does not modify
// the AST. A program with any returned diagnostic must not be compiled.
func Check(prog *ast.Program) (*CheckedProgram, []diagnostic.Diagnostic) {
	return CheckModules([]SourceModule{{Program: prog}})
}

// CheckModules checks a program made of several files (ADR-0014).
// mods[0] is the root file, which holds main. The imports must not form
// a cycle.
func CheckModules(mods []SourceModule) (*CheckedProgram, []diagnostic.Diagnostic) {
	return CheckModulesWith(mods, Options{})
}

// CheckModulesWith is CheckModules with options.
func CheckModulesWith(mods []SourceModule, opts Options) (*CheckedProgram, []diagnostic.Diagnostic) {
	root := mods[0].Program
	c := &checker{
		out: &CheckedProgram{
			Program:     root,
			Types:       map[ast.Expr]Type{},
			Uses:        map[*ast.Ident]*Symbol{},
			Defs:        map[*ast.Ident]*Symbol{},
			IntValues:   map[ast.Expr]int64{},
			FloatValues: map[*ast.FloatLit]float64{},
			Variants:    map[ast.Node]*Variant{},
			Methods:     map[*ast.CallExpr]Method{},
			Exhaustive:  map[*ast.MatchStmt]bool{},
			Calls:       map[*ast.CallExpr]*Function{},
			Fields:      map[*ast.MemberExpr]*Field{},
			TypeNames:   map[*ast.TypeName]Type{},
		},
		file: root.Loc.File,
	}
	c.universe = newScope(nil)
	for _, b := range []*Function{{Name: "print", Result: Unit}, {Name: "toString", Result: String},
		{Name: "assert", Result: Unit}, {Name: "assertEqual", Result: Unit}} { // ADR-0015
		c.universe.names[b.Name] = &Symbol{Name: b.Name, Kind: BuiltinSymbol, Func: b}
	}
	option := newOption()
	c.out.Enums = append(c.out.Enums, option)

	infos := make([]*moduleInfo, len(mods))
	for i, m := range mods {
		info := &moduleInfo{
			path:    m.Path,
			prog:    m.Program,
			root:    i == 0,
			scope:   newScope(c.universe),
			enums:   map[string]*Enum{option.Name: option},
			structs: map[string]*Struct{},
		}
		if i > 0 {
			info.name = m.Path[strings.LastIndexByte(m.Path, '.')+1:]
		}
		infos[i] = info
	}
	// Dependencies first, so that a module's signatures can name the
	// types of the modules it imports.
	for _, i := range dependencyOrder(mods) {
		m := infos[i]
		c.enter(m)
		c.declareImports(mods[i], infos)
		c.collectTypes(m.prog)
		c.collectSignatures(m.prog)
		c.collectImpls(m.prog)
	}
	var tests []*Function
	for _, m := range infos {
		c.enter(m)
		for _, fn := range c.collectTests(m.prog) {
			tests = append(tests, fn)
			if m.root {
				c.out.Tests = append(c.out.Tests, fn)
			}
		}
	}
	c.enter(infos[0])
	if !opts.Tests {
		c.validateMain()
	} else if sym := c.module.names["main"]; sym != nil && sym.Kind == FuncSymbol && len(sym.Func.Params) == 0 && sym.Func.Result == Unit && len(sym.Func.TypeParams) == 0 {
		c.out.Main = sym.Func
	}
	for _, fn := range c.out.Functions {
		c.checkFunction(fn)
	}
	for _, fn := range tests {
		c.checkFunction(fn)
	}
	sort.SliceStable(c.diags, func(i, j int) bool {
		a, b := c.diags[i].Span, c.diags[j].Span
		if a.File != b.File {
			return a.File < b.File
		}
		return a.Start < b.Start
	})
	return c.out, c.diags
}

// dependencyOrder returns the module indexes so that every module comes
// after the modules it imports.
func dependencyOrder(mods []SourceModule) []int {
	var order []int
	seen := make([]bool, len(mods))
	var visit func(int)
	visit = func(i int) {
		if seen[i] {
			return
		}
		seen[i] = true
		for _, d := range mods[i].Program.Imports {
			if j, ok := mods[i].Imports[d]; ok && j >= 0 && j < len(mods) {
				visit(j)
			}
		}
		order = append(order, i)
	}
	for i := range mods {
		visit(i)
	}
	return order
}

// moduleInfo is one file of the program and its namespaces (ADR-0014).
type moduleInfo struct {
	name    string // the qualifier importers use; "" for the root file
	path    string
	prog    *ast.Program
	root    bool
	scope   *scope // functions and imported modules
	enums   map[string]*Enum
	structs map[string]*Struct
}

type checker struct {
	out      *CheckedProgram
	diags    []diagnostic.Diagnostic
	file     source.FileID
	universe *scope
	// cur is the module being checked; module, enums and structs are its
	// namespaces.
	cur     *moduleInfo
	module  *scope
	enums   map[string]*Enum
	structs map[string]*Struct
	// typeParams are the type parameters in scope (of the current
	// function, struct or enum declaration).
	typeParams map[string]*TypeParam
	// current function and scope while checking bodies
	fn    *Function
	scope *scope
	// loops is the number of loops enclosing the current statement
	// (ADR-0012).
	loops int
}

// enter makes m the module being checked.
func (c *checker) enter(m *moduleInfo) {
	c.cur, c.module, c.enums, c.structs = m, m.scope, m.enums, m.structs
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

// declareImports declares the qualifier of every import of a module and
// checks an imported file's module declaration (ADR-0014).
func (c *checker) declareImports(src SourceModule, infos []*moduleInfo) {
	m := c.cur
	if !m.root && m.prog.Module != nil && m.prog.Module.Name != nil && m.prog.Module.Name.Name != m.name {
		c.errorf(diagnostic.ModuleNameMismatch, m.prog.Module.Loc,
			"this file is imported as '%s', but declares 'module %s'", m.path, m.prog.Module.Name.Name)
	}
	for _, d := range m.prog.Imports {
		i, ok := src.Imports[d]
		if !ok || i < 0 || i >= len(infos) {
			continue // reported while loading
		}
		name := d.Name()
		sym := &Symbol{Name: name.Name, Kind: ModuleSymbol, Decl: name.Loc, mod: infos[i]}
		if c.declare(m.scope, sym, "module") {
			c.out.Defs[name] = sym
		}
	}
}

// declareTypeParams creates the type parameters of a function, struct or
// enum. Duplicates and names that hide a type are T005.
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
		case byName[id.Name] != nil || c.typeParams[id.Name] != nil:
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

// isTypeName reports whether name is a built-in type or a declared enum
// or struct of the current module.
func (c *checker) isTypeName(name string) bool {
	switch name {
	case "Array", "Map":
		return true
	}
	return builtinTypes[name] != nil || c.enums[name] != nil || c.structs[name] != nil
}

// resolveType resolves a type annotation in the type namespace: the
// primitives, Array<T>, Map<K, V>, enums and structs (with type
// arguments when generic, and qualified by a module when imported), and
// the type parameters in scope.
func (c *checker) resolveType(tn *ast.TypeName) Type {
	if tn == nil {
		return Invalid // parser already reported
	}
	t := c.resolveTypeName(tn)
	c.out.TypeNames[tn] = t
	return t
}

func (c *checker) resolveTypeName(tn *ast.TypeName) Type {
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
			c.errorf(diagnostic.WrongTypeArguments, tn.Loc, "type '%s' takes no type arguments", ast.TypeString(tn))
		} else {
			c.errorf(diagnostic.WrongTypeArguments, tn.Loc, "type '%s' takes %d type argument(s) but %d were given", tn.Name, want, len(args))
		}
		return false
	}
	if tn.Module != nil {
		en, st, ok := c.qualifiedType(tn.Module, &ast.Ident{Name: tn.Name, Loc: tn.Loc})
		switch {
		case !ok:
			return Invalid
		case en != nil:
			if !arity(len(en.Params)) {
				return Invalid
			}
			return EnumOf(en, args)
		default:
			if !arity(len(st.Params)) {
				return Invalid
			}
			return StructOf(st, args)
		}
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
	if s := c.structs[tn.Name]; s != nil {
		if !arity(len(s.Params)) {
			return Invalid
		}
		return StructOf(s, args)
	}
	if sym := c.module.lookup(tn.Name); sym != nil && sym.Kind == ModuleSymbol {
		c.errorf(diagnostic.UnknownType, tn.Loc, "'%s' is a module, not a type; name one of its types, such as %s.Name", tn.Name, tn.Name)
		return Invalid
	}
	c.errorf(diagnostic.UnknownType, tn.Loc, "unknown type '%s'", tn.Name)
	return Invalid
}

// collectTypes declares every enum and struct of the current module, then
// resolves their variant and field types, so types may refer to each
// other (and to themselves) in any order.
func (c *checker) collectTypes(prog *ast.Program) {
	declared := map[string]source.Span{}
	clash := func(id *ast.Ident, what string) bool {
		if prev, ok := declared[id.Name]; ok {
			c.diags = append(c.diags, diagnostic.Errorf(diagnostic.DuplicateDeclaration, id.Loc,
				"type '%s' is already declared", id.Name).
				WithRelated(prev, "previous declaration is here"))
			return true
		}
		if c.isTypeName(id.Name) {
			c.errorf(diagnostic.DuplicateDeclaration, id.Loc, "cannot declare %s '%s': '%s' is a built-in type", what, id.Name, id.Name)
			return true
		}
		declared[id.Name] = id.Loc
		return false
	}
	var enums []*Enum
	for _, decl := range prog.Enums {
		if decl == nil || decl.Name == nil || clash(decl.Name, "enum") {
			continue
		}
		e := &Enum{Name: decl.Name.Name, Module: c.cur.name, Pub: decl.Pub, mod: c.cur, Decl: decl, Index: len(c.out.Enums), id: nextID.Add(1)}
		c.enums[e.Name] = e
		c.out.Enums = append(c.out.Enums, e)
		enums = append(enums, e)
	}
	var structs []*Struct
	for _, decl := range prog.Structs {
		if decl == nil || decl.Name == nil || clash(decl.Name, "struct") {
			continue
		}
		s := &Struct{Name: decl.Name.Name, Module: c.cur.name, Pub: decl.Pub, mod: c.cur, Decl: decl,
			Methods: map[string]*Function{}, Index: len(c.out.Structs), id: nextID.Add(1)}
		c.structs[s.Name] = s
		c.out.Structs = append(c.out.Structs, s)
		structs = append(structs, s)
	}
	for _, e := range enums {
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
			variant := &Variant{Enum: e, Name: v.Name.Name, Index: len(e.Variants), Decl: v.Name.Loc}
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
	for _, s := range structs {
		var byName map[string]*TypeParam
		s.Params, byName = c.declareTypeParams(s.Decl.TypeParams)
		c.typeParams = byName
		for _, f := range s.Decl.Fields {
			if f == nil || f.Name == nil {
				continue
			}
			if prev := s.Field(f.Name.Name); prev != nil {
				c.diags = append(c.diags, diagnostic.Errorf(diagnostic.DuplicateDeclaration, f.Name.Loc,
					"field '%s' is already declared in struct '%s'", f.Name.Name, s.Name).
					WithRelated(prev.Decl, "previous declaration is here"))
				continue
			}
			t := c.resolveType(f.Type)
			if t == Unit {
				c.errorf(diagnostic.UnitInCompositeType, f.Type.Loc, "a struct field cannot be Unit")
				t = Invalid
			}
			s.Fields = append(s.Fields, &Field{Name: f.Name.Name, Type: t, Index: len(s.Fields), Pub: f.Pub, Decl: f.Name.Loc})
		}
		c.typeParams = nil
	}
	for _, s := range structs {
		for _, f := range s.Fields {
			if containsStruct(f.Type, s, map[*Struct]bool{}) {
				c.errorf(diagnostic.RecursiveStruct, f.Decl,
					"struct '%s' contains itself through field '%s', so no value of it could ever be built", s.Name, f.Name)
				break
			}
		}
	}
}

// containsStruct reports whether a value of type t directly holds a value
// of struct target, through struct fields only: arrays, maps and enums
// can be empty, so they break the chain (ADR-0014).
func containsStruct(t Type, target *Struct, seen map[*Struct]bool) bool {
	if t.Kind != KStruct {
		return false
	}
	if t.Struct == target {
		return true
	}
	if seen[t.Struct] {
		return false
	}
	seen[t.Struct] = true
	args := enumArgs(t)
	for _, f := range t.Struct.Fields {
		if containsStruct(subst(f.Type, args), target, seen) {
			return true
		}
	}
	return false
}

// signature resolves a function declaration's parameters and result. For
// a method, owner is its struct; the struct's type parameters are then in
// scope.
func (c *checker) signature(decl *ast.FuncDecl, owner *Struct) *Function {
	fn := &Function{Name: decl.Name.Name, Decl: decl, Pub: decl.Pub, Module: c.cur.name, Struct: owner, mod: c.cur}
	scopeParams := map[string]*TypeParam{}
	if owner != nil {
		for _, p := range owner.Params {
			scopeParams[p.Name] = p
		}
	}
	c.typeParams = scopeParams
	var own map[string]*TypeParam
	fn.TypeParams, own = c.declareTypeParams(decl.TypeParams)
	for n, p := range own {
		scopeParams[n] = p
	}
	params := newScope(nil) // only for duplicate-parameter detection
	if decl.Self != nil && owner != nil {
		self := &Symbol{Name: "self", Kind: ParamSymbol, Type: selfType(owner), Mutable: decl.Self.Mutable, Decl: decl.Self.Loc}
		params.names["self"] = self
		fn.Params = append(fn.Params, self)
		fn.HasSelf, fn.SelfMut = true, decl.Self.Mutable
	}
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
	case fn.Name == "main" && owner == nil && c.cur.root:
		fn.Result = Invalid // reported as T014 by validateMain
	default:
		c.errorf(diagnostic.MissingReturnType, decl.Name.Loc,
			"function '%s' must declare its return type (for example '-> Unit')", fn.Name)
		fn.Result = Invalid
	}
	c.typeParams = nil
	return fn
}

// selfType is the type of `self` in a method of s: s applied to its own
// type parameters.
func selfType(s *Struct) Type {
	args := make([]Type, len(s.Params))
	for i, p := range s.Params {
		args[i] = ParamType(p)
	}
	return StructOf(s, args)
}

func (c *checker) collectSignatures(prog *ast.Program) {
	for _, decl := range prog.Funcs {
		if decl == nil || decl.Name == nil {
			continue
		}
		fn := c.signature(decl, nil)
		sym := &Symbol{Name: fn.Name, Kind: FuncSymbol, Func: fn, Decl: decl.Name.Loc}
		if c.declare(c.module, sym, "function") {
			c.out.Defs[decl.Name] = sym
		}
		// Duplicates are still checked so their bodies get diagnostics.
		c.out.Functions = append(c.out.Functions, fn)
	}
}

// collectImpls resolves the methods of every impl block (ADR-0014).
func (c *checker) collectImpls(prog *ast.Program) {
	for _, impl := range prog.Impls {
		if impl == nil || impl.Name == nil {
			continue
		}
		s := c.structs[impl.Name.Name]
		if s == nil {
			switch {
			case c.enums[impl.Name.Name] != nil:
				c.errorf(diagnostic.UnknownType, impl.Name.Loc, "'%s' is an enum; impl blocks add methods to structs only", impl.Name.Name)
			default:
				c.errorf(diagnostic.UnknownType, impl.Name.Loc, "no struct '%s' in this file; impl blocks must be in the file that declares the struct", impl.Name.Name)
			}
			continue
		}
		for _, decl := range impl.Funcs {
			if decl == nil || decl.Name == nil {
				continue
			}
			fn := c.signature(decl, s)
			name := decl.Name.Name
			switch prev := s.Methods[name]; {
			case prev != nil:
				c.diags = append(c.diags, diagnostic.Errorf(diagnostic.DuplicateDeclaration, decl.Name.Loc,
					"'%s' already has a method named '%s'", s.Name, name).
					WithRelated(prev.Decl.Name.Loc, "previous declaration is here"))
			case s.Field(name) != nil:
				c.diags = append(c.diags, diagnostic.Errorf(diagnostic.DuplicateDeclaration, decl.Name.Loc,
					"'%s' already has a field named '%s'", s.Name, name).
					WithRelated(s.Field(name).Decl, "the field is declared here"))
			default:
				s.Methods[name] = fn
			}
			c.out.Functions = append(c.out.Functions, fn)
		}
	}
}

// collectTests creates a Unit function without parameters for every test
// block of the current module (ADR-0015). Names must be unique in a file.
func (c *checker) collectTests(prog *ast.Program) []*Function {
	var out []*Function
	seen := map[string]*ast.TestDecl{}
	for _, td := range prog.Tests {
		if td == nil || td.Name == nil || td.Body == nil {
			continue
		}
		if prev := seen[td.Name.Value]; prev != nil {
			c.diags = append(c.diags, diagnostic.Errorf(diagnostic.DuplicateDeclaration, td.Name.Loc,
				"test %q is already declared in this file", td.Name.Value).
				WithRelated(prev.Name.Loc, "previous declaration is here"))
			continue
		}
		seen[td.Name.Value] = td
		decl := &ast.FuncDecl{Name: &ast.Ident{Name: td.Name.Value, Loc: td.Name.Loc}, Body: td.Body, Loc: td.Loc}
		out = append(out, &Function{Name: td.Name.Value, Decl: decl, Result: Unit, Module: c.cur.name, Test: true, mod: c.cur})
	}
	return out
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
	case decl.Result.Name != "Unit" || len(decl.Result.Args) > 0 || decl.Result.Module != nil:
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
	c.enter(fn.mod)
	c.fn = fn
	c.typeParams = map[string]*TypeParam{}
	if fn.Struct != nil {
		for _, p := range fn.Struct.Params {
			c.typeParams[p.Name] = p
		}
	}
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

// rootIdent returns the variable at the root of a path of elements and
// fields (ADR-0014), or nil.
func (c *checker) rootIdent(e ast.Expr) *ast.Ident {
	for {
		switch x := e.(type) {
		case *ast.Ident:
			return x
		case *ast.IndexExpr:
			e = x.X
		case *ast.MemberExpr:
			if c.out.Fields[x] == nil {
				return nil
			}
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
	id := c.rootIdent(target)
	if id == nil {
		c.errorf(diagnostic.AssignToImmutable, span, "only a variable, or an element or field of one, can be changed")
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
	case ModuleSymbol:
		c.errorf(diagnostic.AssignToImmutable, span, "cannot %s module '%s'", verb, sym.Name)
		return false
	case ParamSymbol:
		if sym.Name == "self" && c.fn != nil && c.fn.HasSelf && c.fn.Params[0] == sym {
			if sym.Mutable {
				return true
			}
			c.diags = append(c.diags, diagnostic.Errorf(diagnostic.AssignToImmutable, span,
				"cannot %s 'self' in a method that takes 'self'", verb).
				WithHelp("declare the method with 'mut self' to change its receiver"))
			return false
		}
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
	// An element or a field: a[i] = v, grid[r][c] = v, m["k"] = v,
	// p.x = v (ADR-0014).
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
	if want.Kind != got.Kind || want.Enum != got.Enum || want.Param != got.Param || want.Struct != got.Struct || len(want.Args) != len(got.Args) {
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

package lsp

import (
	"sort"
	"strconv"
	"strings"

	"github.com/sarvessveeriyah2312/Tread/internal/ast"
	"github.com/sarvessveeriyah2312/Tread/internal/checker"
	"github.com/sarvessveeriyah2312/Tread/internal/lexer"
	"github.com/sarvessveeriyah2312/Tread/internal/parser"
	"github.com/sarvessveeriyah2312/Tread/internal/source"
)

// path returns the nodes of prog that contain offset, outermost first.
// A position just after a name still counts as inside it.
func path(prog *ast.Program, off int) []ast.Node {
	var best []ast.Node
	var stack []ast.Node
	var visit func(n ast.Node)
	visit = func(n ast.Node) {
		sp := n.Span()
		if _, isProg := n.(*ast.Program); !isProg && (off < sp.Start || off > sp.End) {
			return
		}
		stack = append(stack, n)
		if len(stack) > len(best) {
			best = append([]ast.Node(nil), stack...)
		}
		ast.Inspect(n, func(c ast.Node) bool {
			if c == n {
				return true
			}
			visit(c)
			return false
		})
		stack = stack[:len(stack)-1]
	}
	visit(prog)
	return best
}

// ---------------------------------------------------------------------------
// Hover

func (s *server) hover(d *document, off int) any {
	a := s.analyze(d)
	if a.checked == nil {
		return nil
	}
	nodes := path(a.checked.Program, off)
	text, span := describe(a.checked, nodes)
	if text == "" {
		return nil
	}
	r := rangeOf(d.text, span)
	return hover{Contents: markupContent{Kind: "markdown", Value: "```tread\n" + text + "\n```"}, Range: &r}
}

// at returns the node i levels above the innermost, or nil.
func at(nodes []ast.Node, i int) ast.Node {
	if len(nodes) <= i {
		return nil
	}
	return nodes[len(nodes)-1-i]
}

// describe returns hover text for the innermost node, and the span it
// describes.
func describe(cp *checker.CheckedProgram, nodes []ast.Node) (string, source.Span) {
	n := at(nodes, 0)
	if n == nil {
		return "", source.Span{}
	}
	parent, grand := at(nodes, 1), at(nodes, 2)
	if id, ok := n.(*ast.Ident); ok {
		if sym := cp.Uses[id]; sym != nil {
			return symbolText(sym), id.Loc
		}
		if sym := cp.Defs[id]; sym != nil {
			return symbolText(sym), id.Loc
		}
		switch p := parent.(type) {
		case *ast.MemberExpr:
			if p.Name == id {
				if f := cp.Fields[p]; f != nil {
					return "field " + f.Name + ": " + cp.Types[p].String(), id.Loc
				}
				if call, ok := grand.(*ast.CallExpr); ok && call.Callee == p {
					if fn := cp.Calls[call]; fn != nil {
						return signature(fn), id.Loc
					}
					if _, ok := cp.Methods[call]; ok {
						return "method " + id.Name + "() -> " + cp.Types[call].String(), id.Loc
					}
					if v := cp.Variants[call]; v != nil {
						return variantText(v), id.Loc
					}
				}
				if v := cp.Variants[p]; v != nil {
					return variantText(v), id.Loc
				}
			}
			if t := typeAt(cp, p, grand); t != "" {
				return t, id.Loc
			}
		case *ast.FuncDecl:
			if fn := funcOf(cp, p); fn != nil {
				return signature(fn), id.Loc
			}
		case *ast.StructDecl:
			if st := structOf(cp, p.Name); st != nil {
				return structText(st), id.Loc
			}
		case *ast.EnumDecl:
			return "enum " + p.Name.Name, id.Loc
		case *ast.FieldDecl:
			return "field " + p.Name.Name + ": " + ast.TypeString(p.Type), id.Loc
		case *ast.StructLit:
			if t := cp.Types[p]; t != nil && t.Kind == checker.KStruct {
				if p.Name == id {
					return structText(t.Struct), id.Loc
				}
				for _, fi := range p.Fields {
					if fi.Name == id {
						if f := t.Struct.Field(id.Name); f != nil {
							return "field " + f.Name + ": " + cp.Types[fi.Value].String(), id.Loc
						}
					}
				}
			}
		case *ast.TypeName:
			if t := cp.TypeNames[p]; t != nil && t != checker.Invalid {
				return typeText(t), p.Loc
			}
		case *ast.VariantPat:
			if v := cp.Variants[p]; v != nil {
				return variantText(v), id.Loc
			}
		}
	}
	if tn, ok := n.(*ast.TypeName); ok {
		if t := cp.TypeNames[tn]; t != nil && t != checker.Invalid {
			return typeText(t), tn.Loc
		}
	}
	if e, ok := n.(ast.Expr); ok {
		if t := cp.Types[e]; t != nil && t != checker.Invalid {
			return t.String(), e.Span()
		}
	}
	return "", source.Span{}
}

// typeAt describes `Type` in Type.member, as in Point.origin() or
// Shape.Circle.
func typeAt(cp *checker.CheckedProgram, m *ast.MemberExpr, grand ast.Node) string {
	if call, ok := grand.(*ast.CallExpr); ok {
		if fn := cp.Calls[call]; fn != nil && fn.Struct != nil && !fn.HasSelf {
			return structText(fn.Struct)
		}
		if v := cp.Variants[call]; v != nil {
			return "enum " + v.Enum.Name
		}
	}
	if v := cp.Variants[m]; v != nil {
		return "enum " + v.Enum.Name
	}
	return ""
}

func funcOf(cp *checker.CheckedProgram, d *ast.FuncDecl) *checker.Function {
	for _, fn := range cp.Functions {
		if fn.Decl == d {
			return fn
		}
	}
	return nil
}

func structOf(cp *checker.CheckedProgram, name *ast.Ident) *checker.Struct {
	for _, st := range cp.Structs {
		if st.Decl != nil && st.Decl.Name == name {
			return st
		}
	}
	return nil
}

func symbolText(sym *checker.Symbol) string {
	switch sym.Kind {
	case checker.VarSymbol:
		if sym.Mutable {
			return "let mut " + sym.Name + ": " + sym.Type.String()
		}
		return "let " + sym.Name + ": " + sym.Type.String()
	case checker.ParamSymbol:
		if sym.Name == "self" && sym.Mutable {
			return "mut self: " + sym.Type.String()
		}
		return sym.Name + ": " + sym.Type.String()
	case checker.LoopVarSymbol, checker.MatchVarSymbol:
		return sym.Name + ": " + sym.Type.String()
	case checker.FuncSymbol:
		return signature(sym.Func)
	case checker.BuiltinSymbol:
		return builtinDocs[sym.Name]
	case checker.ModuleSymbol:
		return "import " + sym.ModulePath()
	}
	return ""
}

var builtinDocs = map[string]string{
	"print":       "fn print(value: T) -> Unit  // any type except Unit",
	"toString":    "fn toString(value: T) -> String  // any type except Unit",
	"assert":      "fn assert(condition: Bool) -> Unit",
	"assertEqual": "fn assertEqual(actual: T, expected: T) -> Unit",
}

// signature renders a function's declaration line.
func signature(fn *checker.Function) string {
	if fn.Test {
		return "test " + strconv.Quote(fn.Name)
	}
	var b strings.Builder
	b.WriteString("fn " + fn.FullName())
	if len(fn.TypeParams) > 0 {
		names := make([]string, len(fn.TypeParams))
		for i, p := range fn.TypeParams {
			names[i] = p.Name
		}
		b.WriteString("<" + strings.Join(names, ", ") + ">")
	}
	b.WriteByte('(')
	for i, p := range fn.Params {
		if i > 0 {
			b.WriteString(", ")
		}
		if i == 0 && fn.HasSelf {
			if fn.SelfMut {
				b.WriteString("mut ")
			}
			b.WriteString("self")
			continue
		}
		b.WriteString(p.Name + ": " + p.Type.String())
	}
	b.WriteString(") -> " + fn.Result.String())
	return b.String()
}

func structText(st *checker.Struct) string {
	var b strings.Builder
	b.WriteString("struct " + st.Name)
	if len(st.Params) > 0 {
		names := make([]string, len(st.Params))
		for i, p := range st.Params {
			names[i] = p.Name
		}
		b.WriteString("<" + strings.Join(names, ", ") + ">")
	}
	b.WriteString(" {")
	for _, f := range st.Fields {
		b.WriteString("\n    ")
		if f.Pub {
			b.WriteString("pub ")
		}
		b.WriteString(f.Name + ": " + f.Type.String())
	}
	b.WriteString("\n}")
	return b.String()
}

func variantText(v *checker.Variant) string {
	s := v.Enum.Name + "." + v.Name
	if len(v.Fields) > 0 {
		parts := make([]string, len(v.Fields))
		for i, f := range v.Fields {
			parts[i] = f.String()
		}
		s += "(" + strings.Join(parts, ", ") + ")"
	}
	return s
}

func typeText(t checker.Type) string {
	switch t.Kind {
	case checker.KStruct:
		return structText(t.Struct)
	case checker.KEnum:
		return "enum " + t.String()
	}
	return t.String()
}

// ---------------------------------------------------------------------------
// Go to definition

func (s *server) definition(d *document, off int) any {
	a := s.analyze(d)
	if a.checked == nil {
		return nil
	}
	cp := a.checked
	nodes := path(cp.Program, off)
	n := at(nodes, 0)
	parent, grand := at(nodes, 1), at(nodes, 2)
	var target source.Span
	found := false
	set := func(sp source.Span) {
		if sp != (source.Span{}) || sp.File != 0 {
			target, found = sp, true
		}
	}
	if id, ok := n.(*ast.Ident); ok {
		sym := cp.Uses[id]
		if sym == nil {
			sym = cp.Defs[id]
		}
		switch {
		case sym != nil && sym.Kind == checker.ModuleSymbol:
			if prog := sym.ModuleProgram(); prog != nil {
				target, found = source.Span{File: prog.Loc.File}, true
			}
		case sym != nil && sym.Kind != checker.BuiltinSymbol:
			set(sym.Decl)
		default:
			switch p := parent.(type) {
			case *ast.MemberExpr:
				if p.Name == id {
					if f := cp.Fields[p]; f != nil {
						set(f.Decl)
					} else if call, ok := grand.(*ast.CallExpr); ok && cp.Calls[call] != nil && cp.Calls[call].Decl != nil {
						set(cp.Calls[call].Decl.Name.Loc)
					} else if v := variantOf(cp, p, grand); v != nil {
						set(v.Decl)
					}
				} else if call, ok := grand.(*ast.CallExpr); ok && cp.Calls[call] != nil && cp.Calls[call].Struct != nil && !cp.Calls[call].HasSelf {
					set(cp.Calls[call].Struct.Decl.Name.Loc)
				} else if v := variantOf(cp, p, grand); v != nil && v.Enum.Decl != nil {
					set(v.Enum.Decl.Name.Loc)
				}
			case *ast.StructLit:
				if t := cp.Types[p]; t != nil && t.Kind == checker.KStruct {
					if p.Name == id {
						set(t.Struct.Decl.Name.Loc)
					} else if f := t.Struct.Field(id.Name); f != nil {
						set(f.Decl)
					}
				}
			case *ast.TypeName:
				set(typeDecl(cp.TypeNames[p]))
			case *ast.VariantPat:
				if v := cp.Variants[p]; v != nil {
					if id == p.Variant {
						set(v.Decl)
					} else if id == p.Enum && v.Enum.Decl != nil {
						set(v.Enum.Decl.Name.Loc)
					}
				}
			}
		}
	}
	if tn, ok := n.(*ast.TypeName); ok && !found {
		set(typeDecl(cp.TypeNames[tn]))
	}
	if !found {
		return nil
	}
	uri := a.uri(target.File)
	text := a.texts[target.File]
	if uri == "" || text == nil {
		return nil
	}
	return location{URI: uri, Range: rangeOf(text, target)}
}

func variantOf(cp *checker.CheckedProgram, m *ast.MemberExpr, grand ast.Node) *checker.Variant {
	if v := cp.Variants[m]; v != nil {
		return v
	}
	if call, ok := grand.(*ast.CallExpr); ok && call.Callee == m {
		return cp.Variants[call]
	}
	return nil
}

func typeDecl(t checker.Type) source.Span {
	switch {
	case t == nil:
	case t.Kind == checker.KStruct && t.Struct.Decl != nil:
		return t.Struct.Decl.Name.Loc
	case t.Kind == checker.KEnum && t.Enum.Decl != nil:
		return t.Enum.Decl.Name.Loc
	}
	return source.Span{}
}

// ---------------------------------------------------------------------------
// Completion

var keywords = []string{"fn", "let", "mut", "if", "else", "return", "true", "false", "while", "for", "in",
	"break", "continue", "enum", "match", "struct", "impl", "pub", "import", "module", "test", "self"}

func isIdentByte(c byte) bool {
	return c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}

func (s *server) completion(d *document, off int) any {
	text := d.text
	start := off
	for start > 0 && isIdentByte(text[start-1]) {
		start--
	}
	a := s.analyze(d)
	if a.checked == nil {
		a = s.lastGood[d.uri]
	}
	items := []completionItem{}
	if a == nil || a.checked == nil {
		if start == 0 || text[start-1] != '.' {
			for _, k := range keywords {
				items = append(items, completionItem{Label: k, Kind: ciKeyword})
			}
		}
		return items
	}
	cp := a.checked
	if start > 0 && text[start-1] == '.' {
		end := start - 1
		begin := end
		for begin > 0 && isIdentByte(text[begin-1]) {
			begin--
		}
		return memberItems(cp, string(text[begin:end]), begin)
	}
	for _, k := range keywords {
		items = append(items, completionItem{Label: k, Kind: ciKeyword})
	}
	for name, doc := range builtinDocs {
		items = append(items, completionItem{Label: name, Kind: ciFunction, Detail: doc})
	}
	for _, fn := range cp.Functions {
		if fn.Module == "" && fn.Struct == nil && fn.Decl != nil {
			items = append(items, completionItem{Label: fn.Name, Kind: ciFunction, Detail: signature(fn)})
		}
	}
	for _, st := range cp.Structs {
		if st.Module == "" {
			items = append(items, completionItem{Label: st.Name, Kind: ciStruct})
		}
	}
	for _, e := range cp.Enums {
		if e.Module == "" {
			items = append(items, completionItem{Label: e.Name, Kind: ciEnum})
		}
	}
	// Names declared before the cursor in the enclosing function, and the
	// file's imports.
	body := enclosing(cp.Program, off)
	seen := map[string]bool{}
	for id, sym := range cp.Defs {
		if sym.Kind == checker.ModuleSymbol {
			if !seen[sym.Name] {
				seen[sym.Name] = true
				items = append(items, completionItem{Label: sym.Name, Kind: ciModule, Detail: "import " + sym.ModulePath()})
			}
			continue
		}
		if sym.Kind != checker.FuncSymbol && body != (source.Span{}) && id.Loc.File == body.File && id.Loc.Start >= body.Start && id.Loc.Start < off && !seen[sym.Name] {
			seen[sym.Name] = true
			items = append(items, completionItem{Label: sym.Name, Kind: ciVariable, Detail: symbolText(sym)})
		}
	}
	sortItems(items)
	return items
}

// enclosing returns the span of the function or test containing off.
func enclosing(prog *ast.Program, off int) source.Span {
	var out source.Span
	ast.Inspect(prog, func(n ast.Node) bool {
		switch n.(type) {
		case *ast.FuncDecl, *ast.TestDecl:
			if sp := n.Span(); sp.Start <= off && off <= sp.End {
				out = sp
			}
			return false
		}
		return true
	})
	return out
}

// memberItems completes after `name.`: fields and methods of a value,
// items of a module, associated functions of a struct, or variants of an
// enum.
func memberItems(cp *checker.CheckedProgram, name string, before int) []completionItem {
	items := []completionItem{}
	if name == "" {
		return items
	}
	for _, st := range cp.Structs {
		if st.Module == "" && st.Name == name {
			for n, fn := range st.Methods {
				if !fn.HasSelf {
					items = append(items, completionItem{Label: n, Kind: ciFunction, Detail: signature(fn)})
				}
			}
			sortItems(items)
			return items
		}
	}
	for _, e := range cp.Enums {
		if e.Module == "" && e.Name == name {
			for _, v := range e.Variants {
				items = append(items, completionItem{Label: v.Name, Kind: ciEnumMem, Detail: variantText(v)})
			}
			return items
		}
	}
	// The latest declaration of name before the cursor.
	var sym *checker.Symbol
	best := -1
	for id, s := range cp.Defs {
		if id.Name == name && id.Loc.File == cp.Program.Loc.File && id.Loc.Start < before && id.Loc.Start > best {
			sym, best = s, id.Loc.Start
		}
	}
	if sym == nil && name == "self" {
		for id, s := range cp.Uses {
			if id.Name == "self" && id.Loc.Start < before && id.Loc.Start > best {
				sym, best = s, id.Loc.Start
			}
		}
	}
	if sym == nil {
		return items
	}
	if sym.Kind == checker.ModuleSymbol {
		funcs, structs, enums := sym.Exports()
		for _, fn := range funcs {
			items = append(items, completionItem{Label: fn.Name, Kind: ciFunction, Detail: signature(fn)})
		}
		for _, st := range structs {
			items = append(items, completionItem{Label: st.Name, Kind: ciStruct})
		}
		for _, e := range enums {
			items = append(items, completionItem{Label: e.Name, Kind: ciEnum})
		}
		return items
	}
	t := sym.Type
	if t == nil {
		return items
	}
	switch t.Kind {
	case checker.KStruct:
		for _, f := range t.Struct.Fields {
			items = append(items, completionItem{Label: f.Name, Kind: ciField, Detail: f.Type.String()})
		}
		for n, fn := range t.Struct.Methods {
			if fn.HasSelf {
				items = append(items, completionItem{Label: n, Kind: ciMethod, Detail: signature(fn)})
			}
		}
	case checker.KArray:
		for _, m := range []string{"len", "push", "pop", "contains"} {
			items = append(items, completionItem{Label: m, Kind: ciMethod})
		}
	case checker.KMap:
		for _, m := range []string{"len", "get", "contains", "remove", "keys"} {
			items = append(items, completionItem{Label: m, Kind: ciMethod})
		}
	}
	sortItems(items)
	return items
}

func sortItems(items []completionItem) {
	sort.SliceStable(items, func(i, j int) bool { return items[i].Label < items[j].Label })
}

// ---------------------------------------------------------------------------
// Document symbols

func (s *server) symbols(uri string) []documentSymbol {
	d := s.docs[uri]
	out := []documentSymbol{}
	if d == nil {
		return out
	}
	toks, _ := lexer.Lex(source.NewFile(1, d.path, d.text))
	prog, _ := parser.Parse(toks)
	r := func(sp source.Span) lspRange { return rangeOf(d.text, sp) }
	sym := func(name string, kind int, loc, sel source.Span) documentSymbol {
		return documentSymbol{Name: name, Kind: kind, Range: r(loc), SelectionRange: r(sel)}
	}
	for _, st := range prog.Structs {
		if st == nil || st.Name == nil {
			continue
		}
		ds := sym(st.Name.Name, skStruct, st.Loc, st.Name.Loc)
		for _, f := range st.Fields {
			if f != nil && f.Name != nil {
				fs := sym(f.Name.Name, skField, f.Loc, f.Name.Loc)
				fs.Detail = ast.TypeString(f.Type)
				ds.Children = append(ds.Children, fs)
			}
		}
		out = append(out, ds)
	}
	for _, e := range prog.Enums {
		if e == nil || e.Name == nil {
			continue
		}
		ds := sym(e.Name.Name, skEnum, e.Loc, e.Name.Loc)
		for _, v := range e.Variants {
			if v != nil && v.Name != nil {
				ds.Children = append(ds.Children, sym(v.Name.Name, skEnumMem, v.Loc, v.Name.Loc))
			}
		}
		out = append(out, ds)
	}
	for _, im := range prog.Impls {
		if im == nil || im.Name == nil {
			continue
		}
		ds := sym("impl "+im.Name.Name, skStruct, im.Loc, im.Name.Loc)
		for _, f := range im.Funcs {
			if f != nil && f.Name != nil {
				ds.Children = append(ds.Children, sym(f.Name.Name, skMethod, f.Loc, f.Name.Loc))
			}
		}
		out = append(out, ds)
	}
	for _, f := range prog.Funcs {
		if f != nil && f.Name != nil {
			out = append(out, sym(f.Name.Name, skFunction, f.Loc, f.Name.Loc))
		}
	}
	for _, t := range prog.Tests {
		if t != nil && t.Name != nil {
			out = append(out, sym("test "+strconv.Quote(t.Name.Value), skEvent, t.Loc, t.Name.Loc))
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i].Range.Start, out[j].Range.Start
		return a.Line < b.Line || a.Line == b.Line && a.Character < b.Character
	})
	return out
}

// Package format implements `tread fmt` (ADR-0015): it prints a parsed
// program in the one Tread style, keeping every comment.
//
// The printer walks the syntax tree. Comments are not in the tree, so it
// places them by position: comments before a declaration, field,
// variant, statement, match arm or list item are printed on their own
// lines before it; a comment after it on the same line stays at the end
// of that line. A comment it cannot place (inside an expression on one
// line) makes it refuse the file. As a final check, the result is parsed
// again and must have the same tree and the same comments.
package format

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"github.com/sarvessveeriyah2312/Tread/internal/ast"
	"github.com/sarvessveeriyah2312/Tread/internal/diagnostic"
	"github.com/sarvessveeriyah2312/Tread/internal/lexer"
	"github.com/sarvessveeriyah2312/Tread/internal/parser"
	"github.com/sarvessveeriyah2312/Tread/internal/source"
)

const indentUnit = "    "

// Error is a file the formatter refuses to change, with the reason.
type Error struct {
	Span source.Span
	Msg  string
}

func (e *Error) Error() string { return e.Msg }

// File formats a source file. It returns the formatted text; or the
// file's syntax errors; or an *Error when the file cannot be formatted
// safely. The input is never modified.
func File(file *source.File) ([]byte, []diagnostic.Diagnostic, error) {
	toks, comments, lexDiags := lexer.LexComments(file)
	prog, parseDiags := parser.Parse(toks)
	if diags := append(append([]diagnostic.Diagnostic(nil), lexDiags...), parseDiags...); len(diags) > 0 {
		sort.SliceStable(diags, func(i, j int) bool { return diags[i].Span.Start < diags[j].Span.Start })
		return nil, diags, nil
	}
	p := &printer{file: file, src: file.Content(), comments: comments}
	if err := p.program(prog); err != nil {
		return nil, nil, err
	}
	out := []byte(p.out.String())
	if err := verify(file, prog, comments, out); err != nil {
		return nil, nil, err
	}
	return out, nil, nil
}

// verify parses the formatted text again: it must have the same tree and
// the same comments, in the same order.
func verify(orig *source.File, prog *ast.Program, comments []lexer.Comment, out []byte) error {
	f := source.NewFile(orig.ID(), orig.Name(), out)
	toks, got, lexDiags := lexer.LexComments(f)
	again, parseDiags := parser.Parse(toks)
	fail := func(what string) error {
		return &Error{Msg: "internal formatter error: the formatted file " + what + "; the file was not changed (please report this)"}
	}
	if len(lexDiags)+len(parseDiags) > 0 {
		return fail("does not parse")
	}
	if ast.Format(again) != ast.Format(prog) {
		return fail("means something different")
	}
	if len(got) != len(comments) {
		return fail("has different comments")
	}
	for i := range got {
		if strings.TrimSpace(got[i].Text) != strings.TrimSpace(comments[i].Text) {
			return fail("has different comments")
		}
	}
	return nil
}

type printer struct {
	file     *source.File
	src      []byte
	comments []lexer.Comment
	next     int // the first comment not yet printed
	out      strings.Builder
	indent   int
	// lineStart is true when the next text starts a new output line.
	lineStart bool
	// lastLine is the source line where the last printed item or comment
	// ended, for keeping blank lines; 0 before anything is printed.
	lastLine int
	// opened is set right after an opening brace or bracket: the next
	// line never has a blank line before it.
	opened bool
	err    error
}

func (p *printer) line(offset int) int { return p.file.Position(offset).Line }

func (p *printer) w(s string) {
	if p.lineStart {
		p.out.WriteString(strings.Repeat(indentUnit, p.indent))
		p.lineStart = false
	}
	p.out.WriteString(s)
}

func (p *printer) nl() {
	p.out.WriteByte('\n')
	p.lineStart = true
}

func (p *printer) blank() { p.out.WriteByte('\n') }

func (p *printer) fail(offset int, format string, args ...any) {
	if p.err == nil {
		pos := p.file.Position(offset)
		p.err = &Error{
			Span: source.Span{File: p.file.ID(), Start: offset, End: offset},
			Msg:  fmt.Sprintf("cannot format %s: %s (at %d:%d)", p.file.Name(), fmt.Sprintf(format, args...), pos.Line, pos.Column),
		}
	}
}

// blankMode says whether a blank line goes before an item.
type blankMode int

const (
	keepBlank  blankMode = iota // one blank line if the source has one
	forceBlank                  // always one blank line
	noBlank                     // never
)

// comments prints, on their own lines, the comments that start before
// offset. mode applies to the first line printed; later lines keep the
// source's blank lines.
func (p *printer) leading(offset int, mode blankMode) blankMode {
	for p.next < len(p.comments) && p.comments[p.next].Span.Start < offset {
		c := p.comments[p.next]
		p.separate(p.line(c.Span.Start), mode)
		mode = keepBlank
		p.w(strings.TrimRight(c.Text, " \t"))
		p.nl()
		p.lastLine = p.line(c.Span.End)
		p.next++
	}
	return mode
}

// separate writes the blank line, if any, before a line starting at
// source line l.
func (p *printer) separate(l int, mode blankMode) {
	if p.opened {
		p.opened = false
		return
	}
	switch {
	case p.lastLine == 0 || mode == noBlank:
	case mode == forceBlank:
		p.blank()
	case l > p.lastLine+1:
		p.blank()
	}
}

// item prints one line-level element spanning [start, end) of the source:
// its leading comments, the element, and a trailing comment on the same
// source line as its end.
func (p *printer) item(start, end int, mode blankMode, emit func()) {
	mode = p.leading(start, mode)
	p.separate(p.line(start), mode)
	emit()
	p.trailing(end)
	p.nl()
}

// trailing appends the comment that follows end on the same source line.
// A comment still unprinted before end is inside the element.
func (p *printer) trailing(end int) {
	if p.next < len(p.comments) && p.comments[p.next].Span.Start < end {
		p.fail(p.comments[p.next].Span.Start, "this comment is inside an expression; move it to its own line or to the end of the line")
		p.next++ // keep going; the error is reported
	}
	p.lastLine = p.line(max(end-1, 0))
	if p.next < len(p.comments) {
		c := p.comments[p.next]
		if p.line(c.Span.Start) == p.line(max(end-1, 0)) && !strings.ContainsAny(c.Text, "\r\n") {
			p.w(" " + strings.TrimRight(c.Text, " \t"))
			p.next++
		}
	}
}

// closing prints the comments before a closing brace, indented inside
// the construct it closes.
func (p *printer) closing(offset int) {
	p.indent++
	p.leading(offset, keepBlank)
	p.indent--
}

// ---------------------------------------------------------------------------
// Declarations

type decl struct {
	start, end int
	kind       string
	emit       func()
}

func (p *printer) program(prog *ast.Program) error {
	var decls []decl
	if m := prog.Module; m != nil {
		decls = append(decls, decl{m.Loc.Start, m.Loc.End, "module", func() { p.w("module " + m.Name.Name) }})
	}
	for _, d := range prog.Imports {
		d := d
		decls = append(decls, decl{d.Loc.Start, d.Loc.End, "import", func() { p.w("import " + d.PathString()) }})
	}
	for _, d := range prog.Funcs {
		d := d
		decls = append(decls, decl{d.Loc.Start, d.Loc.End, "fn", func() { p.funcDecl(d) }})
	}
	for _, d := range prog.Enums {
		d := d
		decls = append(decls, decl{d.Loc.Start, d.Loc.End, "enum", func() { p.enumDecl(d) }})
	}
	for _, d := range prog.Structs {
		d := d
		decls = append(decls, decl{d.Loc.Start, d.Loc.End, "struct", func() { p.structDecl(d) }})
	}
	for _, d := range prog.Impls {
		d := d
		decls = append(decls, decl{d.Loc.Start, d.Loc.End, "impl", func() { p.implDecl(d) }})
	}
	for _, d := range prog.Tests {
		d := d
		decls = append(decls, decl{d.Loc.Start, d.Loc.End, "test", func() {
			p.w("test " + p.text(d.Name.Loc) + " ")
			p.block(d.Body)
		}})
	}
	sort.Slice(decls, func(i, j int) bool { return decls[i].start < decls[j].start })
	for i, d := range decls {
		mode := forceBlank
		if i > 0 && d.kind == "import" && decls[i-1].kind == "import" {
			mode = keepBlank
		}
		p.item(d.start, d.end, mode, d.emit)
	}
	p.leading(len(p.src)+1, forceBlank)
	return p.err
}

// text returns the source text of a span.
func (p *printer) text(s source.Span) string { return string(p.src[s.Start:s.End]) }

func typeParams(ps []*ast.Ident) string {
	if len(ps) == 0 {
		return ""
	}
	names := make([]string, len(ps))
	for i, id := range ps {
		names[i] = id.Name
	}
	return "<" + strings.Join(names, ", ") + ">"
}

func (p *printer) funcDecl(d *ast.FuncDecl) {
	if d.Pub {
		p.w("pub ")
	}
	p.w("fn " + d.Name.Name + typeParams(d.TypeParams))
	var items []listItem
	if d.Self != nil {
		self := d.Self
		items = append(items, listItem{self.Loc.Start, self.Loc.End, func() {
			if self.Mutable {
				p.w("mut ")
			}
			p.w("self")
		}})
	}
	for _, prm := range d.Params {
		prm := prm
		items = append(items, listItem{prm.Loc.Start, prm.Loc.End, func() { p.w(prm.Name.Name + ": " + ast.TypeString(prm.Type)) }})
	}
	open := d.Name.Loc.End
	closeAt := d.Body.Loc.Start
	if d.Result != nil {
		closeAt = d.Result.Loc.Start
	}
	rparen := p.lastBefore(closeAt, ')')
	p.list("(", ")", items, ",", p.multiline(open, items, rparen), rparen)
	if d.Result != nil {
		p.w(" -> " + ast.TypeString(d.Result))
	}
	p.w(" ")
	p.block(d.Body)
}

// lastBefore returns the offset of the last byte c before offset, or
// offset itself.
func (p *printer) lastBefore(offset int, c byte) int {
	if i := bytes.LastIndexByte(p.src[:offset], c); i >= 0 {
		return i
	}
	return offset
}

func (p *printer) structDecl(d *ast.StructDecl) {
	if d.Pub {
		p.w("pub ")
	}
	p.w("struct " + d.Name.Name + typeParams(d.TypeParams) + " ")
	var items []listItem
	for _, f := range d.Fields {
		f := f
		items = append(items, listItem{f.Loc.Start, f.Loc.End, func() {
			if f.Pub {
				p.w("pub ")
			}
			p.w(f.Name.Name + ": " + ast.TypeString(f.Type))
		}})
	}
	p.braceList(d.Loc, items)
}

func (p *printer) enumDecl(d *ast.EnumDecl) {
	if d.Pub {
		p.w("pub ")
	}
	p.w("enum " + d.Name.Name + typeParams(d.TypeParams) + " ")
	var items []listItem
	for _, v := range d.Variants {
		v := v
		items = append(items, listItem{v.Loc.Start, v.Loc.End, func() {
			p.w(v.Name.Name)
			if len(v.Fields) > 0 {
				parts := make([]string, len(v.Fields))
				for i, f := range v.Fields {
					parts[i] = ast.TypeString(f)
				}
				p.w("(" + strings.Join(parts, ", ") + ")")
			}
		}})
	}
	p.braceList(d.Loc, items)
}

// braceList prints `{ a, b }` on one line, or one item per line without
// commas, for struct fields, enum variants and struct literals.
func (p *printer) braceList(loc source.Span, items []listItem) {
	closeAt := loc.End - 1
	open := p.lastBefore(firstStart(items, closeAt), '{')
	if len(items) == 0 && !p.hasComments(open, closeAt) {
		p.w("{}")
		return
	}
	if !p.multiline(open, items, closeAt) {
		p.w("{ ")
		for i, it := range items {
			if i > 0 {
				p.w(", ")
			}
			it.emit()
		}
		p.w(" }")
		return
	}
	p.w("{")
	p.nl()
	p.lastLine, p.opened = p.line(open), true
	p.indent++
	for _, it := range items {
		p.item(it.start, it.end, keepBlank, it.emit)
	}
	p.indent--
	p.closing(closeAt)
	p.w("}")
}

func firstStart(items []listItem, def int) int {
	if len(items) > 0 {
		return items[0].start
	}
	return def
}

// hasComments reports whether an unprinted comment starts in (from, to).
func (p *printer) hasComments(from, to int) bool {
	for _, c := range p.comments[p.next:] {
		if c.Span.Start >= to {
			return false
		}
		if c.Span.Start > from {
			return true
		}
	}
	return false
}

func (p *printer) implDecl(d *ast.ImplDecl) {
	p.w("impl " + d.Name.Name + " {")
	closeAt := d.Loc.End - 1
	if len(d.Funcs) == 0 && !p.hasComments(d.Name.Loc.End, closeAt) {
		p.w("}")
		return
	}
	p.nl()
	p.lastLine, p.opened = p.line(d.Name.Loc.End), true
	p.indent++
	for i, f := range d.Funcs {
		f := f
		mode := forceBlank
		if i == 0 {
			mode = noBlank
		}
		p.item(f.Loc.Start, f.Loc.End, mode, func() { p.funcDecl(f) })
	}
	p.indent--
	p.closing(closeAt)
	p.w("}")
}

// ---------------------------------------------------------------------------
// Lists

type listItem struct {
	start, end int
	emit       func()
}

// multiline reports whether a list was written over several lines: an
// item starts on a later line than the opening bracket or than the item
// before it, or the closing bracket is on a later line than the last
// item.
func (p *printer) multiline(open int, items []listItem, closeAt int) bool {
	prev := p.line(open)
	for _, it := range items {
		if p.line(it.start) > prev {
			return true
		}
		prev = p.line(max(it.end-1, it.start))
	}
	return p.line(closeAt) > prev
}

// list prints `(a, b)` or, when multiline, one item per line with commas.
func (p *printer) list(open, closer string, items []listItem, sep string, multi bool, closeAt int) {
	if !multi || len(items) == 0 {
		p.w(open)
		for i, it := range items {
			if i > 0 {
				p.w(sep + " ")
			}
			it.emit()
		}
		p.w(closer)
		return
	}
	p.w(open)
	p.nl()
	p.opened = true
	p.indent++
	for i, it := range items {
		it, last := it, i == len(items)-1
		p.item(it.start, it.end, keepBlank, func() {
			it.emit()
			if !last {
				p.w(sep)
			}
		})
	}
	p.indent--
	p.closing(closeAt)
	p.w(closer)
}

// ---------------------------------------------------------------------------
// Blocks and statements

// inline reports whether a block stays on one line: it was on one line,
// holds at most one simple statement, and has no comments.
func (p *printer) inline(b *ast.Block) bool {
	if p.line(b.Loc.Start) != p.line(b.Loc.End-1) || len(b.Stmts) > 1 || p.hasComments(b.Loc.Start, b.Loc.End) {
		return false
	}
	if len(b.Stmts) == 1 {
		switch b.Stmts[0].(type) {
		case *ast.IfStmt, *ast.WhileStmt, *ast.ForStmt, *ast.MatchStmt, *ast.Block:
			return false
		}
		return !p.multilineStmt(b.Stmts[0])
	}
	return true
}

// multilineStmt reports whether a statement contains a multi-line list.
func (p *printer) multilineStmt(s ast.Stmt) bool {
	return p.line(s.Span().Start) != p.line(s.Span().End-1)
}

func (p *printer) block(b *ast.Block) {
	if p.inline(b) {
		if len(b.Stmts) == 0 {
			p.w("{ }")
			return
		}
		p.w("{ ")
		p.stmt(b.Stmts[0])
		p.w(" }")
		return
	}
	p.w("{")
	p.nl()
	p.lastLine, p.opened = p.line(b.Loc.Start), true
	p.indent++
	for _, s := range b.Stmts {
		s := s
		p.item(s.Span().Start, s.Span().End, keepBlank, func() { p.stmt(s) })
	}
	p.indent--
	p.closing(b.Loc.End - 1)
	p.w("}")
}

func (p *printer) stmt(s ast.Stmt) {
	switch s := s.(type) {
	case *ast.LetStmt:
		p.w("let ")
		if s.Mutable {
			p.w("mut ")
		}
		p.w(s.Name.Name)
		if s.Type != nil {
			p.w(": " + ast.TypeString(s.Type))
		}
		p.w(" = ")
		p.expr(s.Value)
	case *ast.AssignStmt:
		p.expr(s.Target)
		p.w(" = ")
		p.expr(s.Value)
	case *ast.IfStmt:
		p.w("if ")
		p.expr(s.Cond)
		p.w(" ")
		p.block(s.Then)
		if s.Else != nil {
			p.w(" else ")
			p.stmt(s.Else)
		}
	case *ast.WhileStmt:
		p.w("while ")
		p.expr(s.Cond)
		p.w(" ")
		p.block(s.Body)
	case *ast.ForStmt:
		p.w("for " + s.Var.Name + " in ")
		if s.Iter != nil {
			p.expr(s.Iter)
		} else {
			p.expr(s.Start)
			p.w("..")
			p.expr(s.End)
		}
		p.w(" ")
		p.block(s.Body)
	case *ast.MatchStmt:
		p.w("match ")
		p.expr(s.Subject)
		p.w(" {")
		p.nl()
		p.lastLine, p.opened = p.line(s.Subject.Span().End), true
		p.indent++
		for _, a := range s.Arms {
			a := a
			p.item(a.Loc.Start, a.Loc.End, keepBlank, func() {
				p.pattern(a.Pattern)
				p.w(" => ")
				p.block(a.Body)
			})
		}
		p.indent--
		p.closing(s.Loc.End - 1)
		p.w("}")
	case *ast.ReturnStmt:
		p.w("return")
		if s.Value != nil {
			p.w(" ")
			p.expr(s.Value)
		}
	case *ast.BreakStmt:
		p.w("break")
	case *ast.ContinueStmt:
		p.w("continue")
	case *ast.ExprStmt:
		p.expr(s.Call)
	case *ast.Block:
		p.block(s)
	default:
		p.fail(s.Span().Start, "unexpected statement %T", s)
	}
}

func (p *printer) pattern(pat ast.Pattern) {
	switch x := pat.(type) {
	case *ast.WildcardPat:
		p.w("_")
	case *ast.BindPat:
		p.w(x.Name.Name)
	case *ast.LitPat:
		p.expr(x.Value)
	case *ast.VariantPat:
		if x.Module != nil {
			p.w(x.Module.Name + ".")
		}
		p.w(x.Enum.Name + "." + x.Variant.Name)
		if x.Args != nil {
			p.w("(")
			for i, a := range x.Args {
				if i > 0 {
					p.w(", ")
				}
				p.pattern(a)
			}
			p.w(")")
		}
	default:
		p.fail(pat.Span().Start, "unexpected pattern %T", pat)
	}
}

// ---------------------------------------------------------------------------
// Expressions

func (p *printer) expr(e ast.Expr) {
	switch x := e.(type) {
	case *ast.Ident:
		p.w(x.Name)
	case *ast.IntLit:
		p.w(x.Digits)
	case *ast.FloatLit:
		p.w(x.Lexeme)
	case *ast.StringLit, *ast.InterpString:
		p.w(p.text(e.Span())) // escapes are kept as written
	case *ast.BoolLit:
		if x.Value {
			p.w("true")
		} else {
			p.w("false")
		}
	case *ast.ParenExpr:
		p.w("(")
		p.expr(x.Inner)
		p.w(")")
	case *ast.UnaryExpr:
		p.w(x.Op.String())
		p.expr(x.Operand)
	case *ast.BinaryExpr:
		p.expr(x.Left)
		p.w(" " + x.Op.String() + " ")
		p.expr(x.Right)
	case *ast.CallExpr:
		p.expr(x.Callee)
		p.list("(", ")", p.exprItems(x.Args), ",", p.multiline(x.Callee.Span().End, p.exprItems(x.Args), x.Loc.End-1), x.Loc.End-1)
	case *ast.IndexExpr:
		p.expr(x.X)
		p.w("[")
		p.expr(x.Index)
		p.w("]")
	case *ast.MemberExpr:
		p.expr(x.X)
		p.w("." + x.Name.Name)
	case *ast.ArrayLit:
		items := p.exprItems(x.Elems)
		p.list("[", "]", items, ",", p.multiline(x.Loc.Start, items, x.Loc.End-1), x.Loc.End-1)
	case *ast.MapLit:
		if len(x.Keys) == 0 {
			p.w("[:]")
			return
		}
		var items []listItem
		for i := range x.Keys {
			k, v := x.Keys[i], x.Values[i]
			items = append(items, listItem{k.Span().Start, v.Span().End, func() {
				p.expr(k)
				p.w(": ")
				p.expr(v)
			}})
		}
		p.list("[", "]", items, ",", p.multiline(x.Loc.Start, items, x.Loc.End-1), x.Loc.End-1)
	case *ast.StructLit:
		if x.Module != nil {
			p.w(x.Module.Name + ".")
		}
		p.w(x.Name.Name + " ")
		var items []listItem
		for _, f := range x.Fields {
			f := f
			items = append(items, listItem{f.Name.Loc.Start, f.Value.Span().End, func() {
				p.w(f.Name.Name + ": ")
				p.expr(f.Value)
			}})
		}
		p.braceList(x.Loc, items)
	default:
		p.fail(e.Span().Start, "unexpected expression %T", e)
	}
}

func (p *printer) exprItems(es []ast.Expr) []listItem {
	items := make([]listItem, len(es))
	for i, e := range es {
		e := e
		items[i] = listItem{e.Span().Start, e.Span().End, func() { p.expr(e) }}
	}
	return items
}

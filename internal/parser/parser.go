package parser

import (
	"github.com/sarvessveeriyah2312/Tread/internal/ast"
	"github.com/sarvessveeriyah2312/Tread/internal/diagnostic"
	"github.com/sarvessveeriyah2312/Tread/internal/source"
	"github.com/sarvessveeriyah2312/Tread/internal/token"
)

// Parse builds an AST from a token stream produced by lexer.Lex. It
// implements the grammar and newline rules of docs/03 §3.1 and §3.2 and
// returns structured diagnostics; it never panics on user input.
func Parse(toks []token.Token) (*ast.Program, []diagnostic.Diagnostic) {
	p := newParser(toks)
	prog := p.parseProgram()
	return prog, p.diags
}

type parser struct {
	toks  []token.Token
	pos   int
	diags []diagnostic.Diagnostic

	// parens holds the currently open '(' tokens. While it is non-empty,
	// NEWLINE tokens are insignificant (N2).
	parens []token.Token
	// panicking suppresses cascading diagnostics until the parser reaches
	// a synchronization point (docs/04 §4.5).
	panicking bool
	// errorEvents counts every reported error, including suppressed ones.
	errorEvents int
	// unwinding is set after N10 recovery: a statement keyword appeared
	// inside unclosed parentheses. Parse functions return promptly so the
	// statement loop can resume at that keyword.
	unwinding bool
	// depth is the current nesting depth (ADR-0006).
	depth int
	// depthReported is set once P014 has been reported in the current
	// top-level declaration; exceeding the limit is one root cause, so
	// it is reported only once per declaration.
	depthReported bool
}

// MaxNestingDepth is the implementation limit on nesting (ADR-0006,
// docs/02 §2.17). It bounds the AST depth, and therefore the recursion
// depth of every later phase.
const MaxNestingDepth = 10000

// enter opens one nesting level at token t. At the limit it reports P014
// and returns false; the caller must then return without consuming
// input, and normal recovery resumes at the next statement boundary.
func (p *parser) enter(t token.Token) bool {
	if p.depth >= MaxNestingDepth {
		if !p.depthReported {
			p.depthReported = true
			p.errorf(diagnostic.NestingTooDeep, t.Span,
				"nesting too deep: the maximum nesting depth is %d", MaxNestingDepth)
		} else {
			// Already reported: still an error event, so callers recover
			// as after any error and no cascading diagnostic follows.
			p.errorEvents++
			p.panicking = true
		}
		return false
	}
	p.depth++
	return true
}

func (p *parser) leave() { p.depth-- }

func newParser(toks []token.Token) *parser {
	if len(toks) == 0 || toks[len(toks)-1].Kind != token.EOF {
		end := source.Span{}
		if len(toks) > 0 {
			last := toks[len(toks)-1].Span
			end = source.Span{File: last.File, Start: last.End, End: last.End}
		}
		toks = append(append([]token.Token(nil), toks...), token.Token{Kind: token.EOF, Span: end})
	}
	return &parser{toks: toks}
}

// ---------------------------------------------------------------------------
// Token access

// cur returns the current token, skipping NEWLINEs inside parentheses.
func (p *parser) cur() token.Token {
	if len(p.parens) > 0 {
		for p.toks[p.pos].Kind == token.Newline {
			p.pos++
		}
	}
	return p.toks[p.pos]
}

func (p *parser) at(k token.Kind) bool { return p.cur().Kind == k }

// next consumes and returns the current token. EOF is never consumed.
func (p *parser) next() token.Token {
	t := p.cur()
	if t.Kind != token.EOF {
		p.pos++
	}
	return t
}

// skipNewlines skips NEWLINE tokens, as after a continuation token (N3).
func (p *parser) skipNewlines() {
	for p.toks[p.pos].Kind == token.Newline {
		p.pos++
	}
}

// peekPastNewlines returns the first non-NEWLINE token without consuming.
func (p *parser) peekPastNewlines() token.Token {
	i := p.pos
	for p.toks[i].Kind == token.Newline {
		i++
	}
	return p.toks[i]
}

// ---------------------------------------------------------------------------
// Diagnostics

func (p *parser) report(d diagnostic.Diagnostic) {
	p.errorEvents++
	if p.panicking {
		return
	}
	p.panicking = true
	p.diags = append(p.diags, d)
}

func (p *parser) errorf(code diagnostic.Code, span source.Span, format string, args ...any) {
	p.report(diagnostic.Errorf(code, span, format, args...))
}

// describe names a token for messages.
func describe(t token.Token) string {
	switch t.Kind {
	case token.EOF:
		return "end of file"
	case token.Newline:
		return "end of line"
	case token.Ident, token.Reserved:
		return "'" + t.Lexeme + "'"
	case token.Int, token.Float:
		return "number " + t.Lexeme
	case token.String, token.TemplateHead:
		return "string"
	case token.Invalid:
		return "invalid input"
	}
	return "'" + t.Kind.String() + "'"
}

func isStatementKeyword(k token.Kind) bool {
	switch k {
	case token.Let, token.Fn, token.If, token.Return, token.Module,
		token.While, token.For, token.Break, token.Continue:
		return true
	}
	return false
}

// unwindIfUnclosed applies N10: if a token that cannot occur inside
// parentheses is reached while parentheses are open, report P010 at the
// innermost '(' and unwind. It reports whether unwinding started.
func (p *parser) unwindIfUnclosed(t token.Token) bool {
	if len(p.parens) == 0 || !(isStatementKeyword(t.Kind) || t.Kind == token.RBrace || t.Kind == token.EOF) {
		return false
	}
	open := p.parens[len(p.parens)-1]
	p.errorf(diagnostic.MissingRParen, open.Span, "missing ')' to close this '('")
	p.parens = nil
	p.unwinding = true
	return true
}

// ---------------------------------------------------------------------------
// Synchronization

// syncTop skips to the next top-level declaration keyword or EOF.
func (p *parser) syncTop() {
	p.parens = nil
	for {
		switch p.toks[p.pos].Kind {
		case token.EOF, token.Fn, token.Module:
			return
		}
		p.pos++
	}
}

// syncStatement skips to the end of the current statement: a NEWLINE
// outside parentheses and nested braces, the '}' that closes the enclosing
// block, a statement keyword inside unclosed parentheses (N10), or EOF.
func (p *parser) syncStatement() {
	depth := len(p.parens)
	p.parens = nil
	braces := 0
	for {
		switch p.toks[p.pos].Kind {
		case token.EOF:
			return
		case token.Newline:
			if depth == 0 && braces == 0 {
				return
			}
		case token.LParen:
			depth++
		case token.RParen:
			if depth > 0 {
				depth--
			}
		case token.LBrace:
			braces++
		case token.RBrace:
			if braces == 0 {
				return
			}
			braces--
		case token.Let, token.Fn, token.If, token.Return, token.Module,
			token.While, token.For, token.Break, token.Continue:
			if depth > 0 {
				return
			}
		}
		p.pos++
	}
}

// ---------------------------------------------------------------------------
// Declarations

func (p *parser) parseProgram() *ast.Program {
	prog := &ast.Program{}
	first := p.toks[0].Span
	sawFn := false
	for {
		p.skipNewlines()
		p.panicking = false
		p.depthReported = false
		t := p.cur()
		if t.Kind == token.EOF {
			break
		}
		start := p.pos
		switch t.Kind {
		case token.Module:
			p.parseModule(prog, sawFn)
		case token.Fn:
			sawFn = true
			if fd := p.parseFunc(); fd != nil {
				prog.Funcs = append(prog.Funcs, fd)
			}
		default:
			p.errorf(diagnostic.UnexpectedToken, t.Span,
				"expected 'fn' or 'module' declaration, found %s", describe(t))
			p.syncTop()
			if p.pos == start {
				p.pos++
			}
			continue
		}
		if p.unwinding {
			p.unwinding = false
		}
		if end := p.cur(); end.Kind != token.Newline && end.Kind != token.EOF {
			p.errorf(diagnostic.ExpectedNewline, end.Span,
				"expected newline after declaration, found %s", describe(end))
			p.syncTop()
		}
		if p.pos == start {
			p.pos++
		}
	}
	prog.Loc = source.Join(first, p.toks[len(p.toks)-1].Span)
	return prog
}

func (p *parser) parseModule(prog *ast.Program, sawFn bool) {
	kw := p.next()
	name := p.parseIdent("module name")
	if name == nil {
		return
	}
	decl := &ast.ModuleDecl{Name: name, Loc: source.Join(kw.Span, name.Loc)}
	switch {
	case sawFn:
		p.errorf(diagnostic.ModuleNotFirst, decl.Loc,
			"module declaration must precede all other declarations")
	case prog.Module != nil:
		p.report(diagnostic.Errorf(diagnostic.DuplicateModule, decl.Loc, "duplicate module declaration").
			WithRelated(prog.Module.Loc, "first module declaration is here"))
	default:
		prog.Module = decl
	}
}

// parseIdent parses an identifier. A keyword or reserved word in its place
// is reported and consumed as the name so parsing can continue.
func (p *parser) parseIdent(what string) *ast.Ident {
	t := p.cur()
	switch {
	case t.Kind == token.Ident:
		p.next()
		return &ast.Ident{Name: t.Lexeme, Loc: t.Span}
	case t.Kind == token.Reserved:
		p.errorf(diagnostic.ReservedWord, t.Span,
			"'%s' is a reserved word and cannot be used as %s", t.Lexeme, what)
		p.next()
		return &ast.Ident{Name: t.Lexeme, Loc: t.Span}
	case t.Kind.IsKeyword():
		p.errorf(diagnostic.ExpectedIdentifier, t.Span,
			"expected %s, found keyword '%s'", what, t.Lexeme)
		p.next()
		return &ast.Ident{Name: t.Lexeme, Loc: t.Span}
	default:
		p.errorf(diagnostic.ExpectedIdentifier, t.Span, "expected %s, found %s", what, describe(t))
		return nil
	}
}

// parseType parses a type name (an identifier; ADR-0004 item 3).
func (p *parser) parseType() *ast.TypeName {
	t := p.cur()
	if t.Kind == token.Ident {
		p.next()
		return &ast.TypeName{Name: t.Lexeme, Loc: t.Span}
	}
	p.errorf(diagnostic.ExpectedTypeName, t.Span, "expected type name, found %s", describe(t))
	return nil
}

func (p *parser) openParen() {
	t := p.next()
	p.parens = append(p.parens, t)
}

// closeParen consumes ')' for the innermost open parenthesis.
func (p *parser) closeParen(expected string) (token.Token, bool) {
	t := p.cur()
	if t.Kind == token.RParen {
		p.next()
		p.parens = p.parens[:len(p.parens)-1]
		return t, true
	}
	if !p.unwindIfUnclosed(t) {
		p.errorf(diagnostic.UnexpectedToken, t.Span, "expected %s, found %s", expected, describe(t))
	}
	return t, false
}

func (p *parser) parseFunc() *ast.FuncDecl {
	kw := p.next()
	name := p.parseIdent("function name")
	if name == nil {
		return nil
	}
	fd := &ast.FuncDecl{Name: name, Loc: source.Join(kw.Span, name.Loc)}
	if t := p.cur(); t.Kind != token.LParen {
		p.errorf(diagnostic.UnexpectedToken, t.Span, "expected '(' after function name, found %s", describe(t))
		return nil
	}
	p.openParen()
	if !p.at(token.RParen) {
		for {
			prm := p.parseParam()
			if p.unwinding || prm == nil {
				return nil
			}
			fd.Params = append(fd.Params, prm)
			if !p.at(token.Comma) {
				break
			}
			comma := p.next()
			if p.at(token.RParen) {
				p.errorf(diagnostic.TrailingComma, comma.Span, "trailing comma is not allowed in a parameter list")
				break
			}
		}
	}
	if _, ok := p.closeParen("',' or ')' in parameter list"); !ok {
		return nil
	}
	if p.at(token.Arrow) {
		p.next()
		p.skipNewlines() // N3
		fd.Result = p.parseType()
	}
	body := p.expectBlock("function signature")
	if body == nil {
		return nil
	}
	fd.Body = body
	fd.Loc = source.Join(kw.Span, body.Loc)
	return fd
}

func (p *parser) parseParam() *ast.Param {
	name := p.parseIdent("parameter name")
	if name == nil {
		return nil
	}
	if t := p.cur(); t.Kind != token.Colon {
		if !p.unwindIfUnclosed(t) {
			p.errorf(diagnostic.UnexpectedToken, t.Span, "expected ':' after parameter name, found %s", describe(t))
		}
		return nil
	}
	p.next()
	p.skipNewlines() // N3
	typ := p.parseType()
	if typ == nil {
		return nil
	}
	return &ast.Param{Name: name, Type: typ, Loc: source.Join(name.Loc, typ.Loc)}
}

// ---------------------------------------------------------------------------
// Blocks and statements

// expectBlock parses a block whose '{' must be on the current line (N6).
func (p *parser) expectBlock(after string) *ast.Block {
	t := p.cur()
	if t.Kind == token.LBrace {
		return p.parseBlock()
	}
	if t.Kind == token.Newline {
		if n := p.peekPastNewlines(); n.Kind == token.LBrace {
			p.errorf(diagnostic.SameLine, n.Span, "'{' must be on the same line as the %s", after)
			p.skipNewlines()
			return p.parseBlock()
		}
	}
	p.errorf(diagnostic.UnexpectedToken, t.Span, "expected '{' after %s, found %s", after, describe(t))
	return nil
}

// parseBlock parses a block. It returns nil, without consuming input, if
// the nesting limit is reached.
func (p *parser) parseBlock() *ast.Block {
	if !p.enter(p.cur()) {
		return nil
	}
	defer p.leave()
	open := p.next()
	stmts := p.parseStatements(token.RBrace)
	block := &ast.Block{Stmts: stmts, Loc: open.Span}
	if t := p.cur(); t.Kind == token.RBrace {
		p.next()
		block.Loc = source.Join(open.Span, t.Span)
	} else {
		p.panicking = false // reaching EOF or a new 'fn' is a sync point
		p.errorf(diagnostic.MissingRBrace, open.Span, "missing '}' to close this block")
	}
	return block
}

// parseStatements parses statements separated by newlines until end (RBrace
// for blocks). It implements N4 and N5.
func (p *parser) parseStatements(end token.Kind) []ast.Stmt {
	var stmts []ast.Stmt
	for {
		p.parens = nil
		p.skipNewlines()
		p.panicking = false
		t := p.cur()
		if t.Kind == end || t.Kind == token.EOF {
			return stmts
		}
		if t.Kind == token.Fn && end == token.RBrace {
			return stmts // start of a top-level fn: the block is unclosed (N10)
		}
		start := p.pos
		if s := p.parseStatement(); s != nil {
			stmts = append(stmts, s)
		}
		if p.unwinding {
			// N10: resume at the statement keyword found inside parentheses.
			p.unwinding = false
			p.panicking = false
			if p.pos == start {
				p.pos++
			}
			continue
		}
		if t := p.cur(); t.Kind != token.Newline && t.Kind != end && t.Kind != token.EOF {
			p.errorf(diagnostic.ExpectedNewline, t.Span,
				"expected newline or '}' after statement, found %s", describe(t))
			p.syncStatement()
		}
		if p.pos == start {
			p.syncStatement()
			if p.pos == start && p.toks[p.pos].Kind != token.EOF && p.toks[p.pos].Kind != end {
				p.pos++
			}
		}
	}
}

func (p *parser) parseStatement() ast.Stmt {
	t := p.cur()
	switch t.Kind {
	case token.Let:
		return p.parseLet()
	case token.If:
		if s := p.parseIf(); s != nil {
			return s
		}
		return nil
	case token.Return:
		return p.parseReturn()
	case token.While:
		if s := p.parseWhile(); s != nil {
			return s
		}
		return nil
	case token.For:
		if s := p.parseFor(); s != nil {
			return s
		}
		return nil
	case token.Break:
		p.next()
		return &ast.BreakStmt{Loc: t.Span}
	case token.Continue:
		p.next()
		return &ast.ContinueStmt{Loc: t.Span}
	case token.In:
		p.errorf(diagnostic.UnexpectedToken, t.Span, "unexpected 'in' outside a 'for' loop header")
		return nil
	case token.LBrace:
		if b := p.parseBlock(); b != nil {
			return b
		}
		return nil
	case token.Else:
		p.errorf(diagnostic.UnexpectedToken, t.Span, "unexpected 'else' without a matching 'if'")
		return nil
	case token.Fn, token.Module:
		p.errorf(diagnostic.UnexpectedToken, t.Span, "'%s' declarations are only allowed at top level", t.Lexeme)
		return nil
	case token.Ident:
		if p.toks[p.pos+1].Kind == token.Assign {
			return p.parseAssign()
		}
	}
	return p.parseExprStmt()
}

func (p *parser) parseLet() ast.Stmt {
	kw := p.next()
	mutable := false
	if p.at(token.Mut) {
		p.next()
		mutable = true
	}
	name := p.parseIdent("variable name")
	if name == nil {
		return nil
	}
	stmt := &ast.LetStmt{Mutable: mutable, Name: name}
	if p.at(token.Colon) {
		p.next()
		p.skipNewlines() // N3
		stmt.Type = p.parseType()
	}
	if t := p.cur(); t.Kind != token.Assign {
		p.errorf(diagnostic.UnexpectedToken, t.Span, "expected '=' in let declaration, found %s", describe(t))
		return nil
	}
	p.next()
	p.skipNewlines() // N3
	stmt.Value = p.parseExpr()
	stmt.Loc = source.Join(kw.Span, stmt.Value.Span())
	return stmt
}

func (p *parser) parseAssign() ast.Stmt {
	t := p.next()
	target := &ast.Ident{Name: t.Lexeme, Loc: t.Span}
	p.next() // '='
	p.skipNewlines()
	value := p.parseExpr()
	return &ast.AssignStmt{Target: target, Value: value, Loc: source.Join(t.Span, value.Span())}
}

func (p *parser) parseReturn() ast.Stmt {
	kw := p.next()
	switch p.cur().Kind {
	case token.Newline, token.RBrace, token.EOF:
		return &ast.ReturnStmt{Loc: kw.Span} // N5: bare return
	}
	value := p.parseExpr()
	return &ast.ReturnStmt{Value: value, Loc: source.Join(kw.Span, value.Span())}
}

// parseIf parses an if statement and its else / else-if chain. `else if`
// becomes a nested IfStmt in Else (ADR-0004 item 5). It returns nil,
// without consuming input, if the nesting limit is reached.
func (p *parser) parseIf() *ast.IfStmt {
	if !p.enter(p.cur()) {
		return nil
	}
	defer p.leave()
	kw := p.next()
	before := p.errorEvents
	cond := p.parseExpr()
	stmt := &ast.IfStmt{Cond: cond, Loc: source.Join(kw.Span, cond.Span())}
	if p.unwinding {
		return stmt
	}
	if p.errorEvents > before {
		p.parens = nil // a malformed condition may leave '(' open
	}
	if p.errorEvents > before && !p.at(token.LBrace) {
		// N10: after a malformed condition, look for the branch's '{'.
		if !p.scanToBrace() {
			return stmt
		}
	}
	stmt.Then = p.expectBlock("'if' condition")
	if stmt.Then == nil {
		return stmt
	}
	stmt.Loc = source.Join(kw.Span, stmt.Then.Loc)

	if p.at(token.Newline) {
		if e := p.peekPastNewlines(); e.Kind == token.Else {
			p.errorf(diagnostic.SameLine, e.Span, "'else' must be on the same line as the closing '}' of the 'if' block")
			p.skipNewlines()
		}
	}
	if !p.at(token.Else) {
		return stmt
	}
	elseTok := p.next()
	if p.at(token.Newline) {
		if n := p.peekPastNewlines(); n.Kind == token.If || n.Kind == token.LBrace {
			p.errorf(diagnostic.SameLine, n.Span, "'%s' must be on the same line as 'else'", n.Kind)
			p.skipNewlines()
		}
	}
	switch t := p.cur(); t.Kind {
	case token.If:
		if nested := p.parseIf(); nested != nil {
			stmt.Else = nested
			stmt.Loc = source.Join(stmt.Loc, nested.Loc)
		}
	case token.LBrace:
		if block := p.parseBlock(); block != nil {
			stmt.Else = block
			stmt.Loc = source.Join(stmt.Loc, block.Loc)
		}
	default:
		p.errorf(diagnostic.UnexpectedToken, t.Span, "expected '{' or 'if' after 'else', found %s", describe(t))
		stmt.Loc = source.Join(stmt.Loc, elseTok.Span)
	}
	return stmt
}

// headerBlock parses the block after a loop header (ADR-0012). After a
// malformed header it first looks for the block's '{' (N10), as for an
// 'if' condition.
func (p *parser) headerBlock(before int, after string) *ast.Block {
	if p.errorEvents > before {
		p.parens = nil // a malformed header may leave '(' open
		if !p.at(token.LBrace) && !p.scanToBrace() {
			return nil
		}
	}
	return p.expectBlock(after)
}

// parseWhile parses `while cond { }` (ADR-0012). It returns nil, without
// consuming input, if the nesting limit is reached.
func (p *parser) parseWhile() *ast.WhileStmt {
	if !p.enter(p.cur()) {
		return nil
	}
	defer p.leave()
	kw := p.next()
	before := p.errorEvents
	cond := p.parseExpr()
	stmt := &ast.WhileStmt{Cond: cond, Loc: source.Join(kw.Span, cond.Span())}
	if p.unwinding {
		return stmt
	}
	if stmt.Body = p.headerBlock(before, "'while' condition"); stmt.Body != nil {
		stmt.Loc = source.Join(kw.Span, stmt.Body.Loc)
	}
	return stmt
}

// parseFor parses `for name in start..end { }` (ADR-0012). It returns
// nil, without consuming input, if the nesting limit is reached.
func (p *parser) parseFor() *ast.ForStmt {
	if !p.enter(p.cur()) {
		return nil
	}
	defer p.leave()
	kw := p.next()
	stmt := &ast.ForStmt{Loc: kw.Span}
	before := p.errorEvents
	if t := p.cur(); t.Kind == token.In {
		// `for in ...`: report the missing name but keep the 'in'.
		p.errorf(diagnostic.ExpectedIdentifier, t.Span, "expected loop variable name, found keyword 'in'")
	} else if stmt.Var = p.parseIdent("loop variable name"); stmt.Var != nil {
		stmt.Loc = source.Join(kw.Span, stmt.Var.Loc)
	}
	if stmt.Var != nil || p.at(token.In) {
		if t := p.cur(); t.Kind == token.In {
			p.next()
			if !p.parseRange(stmt, kw, before) {
				return stmt
			}
		} else if !p.unwindIfUnclosed(t) {
			p.errorf(diagnostic.UnexpectedToken, t.Span, "expected 'in' after loop variable, found %s", describe(t))
		}
	}
	if stmt.Body = p.headerBlock(before, "'for' range"); stmt.Body != nil {
		stmt.Loc = source.Join(kw.Span, stmt.Body.Loc)
	}
	return stmt
}

// parseRange parses `start..end` after 'in'. It returns false if the
// parser is unwinding (N10).
func (p *parser) parseRange(stmt *ast.ForStmt, kw token.Token, before int) bool {
	stmt.Start = p.parseExpr()
	stmt.Loc = source.Join(kw.Span, stmt.Start.Span())
	if p.unwinding {
		return false
	}
	t := p.cur()
	if t.Kind != token.DotDot {
		if p.errorEvents == before {
			p.errorf(diagnostic.UnexpectedToken, t.Span, "expected '..' in 'for' range, found %s", describe(t))
		}
		return true
	}
	p.next()
	p.skipNewlines() // N3
	stmt.End = p.parseExpr()
	stmt.Loc = source.Join(kw.Span, stmt.End.Span())
	return !p.unwinding
}

// scanToBrace skips tokens after a malformed condition until the branch's
// '{'. It stops without consuming at '}', EOF or a statement keyword.
func (p *parser) scanToBrace() bool {
	p.parens = nil
	for {
		switch p.toks[p.pos].Kind {
		case token.LBrace:
			return true
		case token.RBrace, token.EOF, token.Let, token.Fn, token.If, token.Return, token.Module, token.Else,
			token.While, token.For, token.Break, token.Continue:
			return false
		}
		p.pos++
	}
}

// parseExprStmt parses an expression statement, which must be a call
// (N8, ADR-0003 Q4).
func (p *parser) parseExprStmt() ast.Stmt {
	first := p.cur()
	e := p.parseExpr()
	switch e := e.(type) {
	case *ast.CallExpr:
		return &ast.ExprStmt{Call: e, Loc: e.Loc}
	case *ast.ErrorExpr:
		return nil // already reported
	}
	if p.unwinding {
		return nil
	}
	d := diagnostic.Errorf(diagnostic.NonCallStatement, e.Span(), "only function calls may be used as statements")
	if first.Kind == token.Minus || first.Kind == token.Not || binaryPrecedence(first.Kind) > 0 {
		d = d.WithHelp("if this continues the previous line, move the operator to the end of that line")
	}
	p.report(d)
	return nil
}

// ---------------------------------------------------------------------------
// Expressions

func binaryPrecedence(k token.Kind) int {
	switch k {
	case token.OrOr:
		return 1
	case token.AndAnd:
		return 2
	case token.Eq, token.Ne:
		return 3
	case token.Lt, token.Le, token.Gt, token.Ge:
		return 4
	case token.Plus, token.Minus:
		return 5
	case token.Star, token.Slash, token.Percent:
		return 6
	}
	return 0
}

func (p *parser) parseExpr() ast.Expr { return p.parseBinary(1) }

// parseBinary is a precedence-climbing parser for left-associative binary
// operators (docs/02 §2.9). A NEWLINE after an operator is skipped (N3);
// a significant NEWLINE ends the expression.
func (p *parser) parseBinary(minPrec int) ast.Expr {
	left := p.parseUnary()
	entered := 0 // each operator of a chain nests the AST one level deeper
	defer func() { p.depth -= entered }()
	for !p.unwinding {
		op := p.cur()
		prec := binaryPrecedence(op.Kind)
		if prec == 0 || prec < minPrec {
			break
		}
		if !p.enter(op) {
			break
		}
		entered++
		p.next()
		p.skipNewlines()
		right := p.parseBinary(prec + 1)
		left = &ast.BinaryExpr{Op: op.Kind, Left: left, Right: right, Loc: source.Join(left.Span(), right.Span())}
	}
	return left
}

func (p *parser) parseUnary() ast.Expr {
	t := p.cur()
	if !p.enter(t) {
		return &ast.ErrorExpr{Loc: t.Span}
	}
	defer p.leave()
	if t.Kind == token.Not || t.Kind == token.Minus {
		p.next()
		p.skipNewlines() // N3
		operand := p.parseUnary()
		return &ast.UnaryExpr{Op: t.Kind, Operand: operand, Loc: source.Join(t.Span, operand.Span())}
	}
	return p.parsePostfix()
}

func (p *parser) parsePostfix() ast.Expr {
	e := p.parsePrimary()
	// N7: a '(' after a significant NEWLINE is not a call.
	for !p.unwinding && p.at(token.LParen) {
		e = p.parseCall(e)
	}
	return e
}

func (p *parser) parseCall(callee ast.Expr) ast.Expr {
	p.openParen()
	call := &ast.CallExpr{Callee: callee, Loc: callee.Span()}
	if !p.at(token.RParen) {
		for {
			arg := p.parseExpr()
			call.Args = append(call.Args, arg)
			call.Loc = source.Join(call.Loc, arg.Span())
			if p.unwinding {
				return call
			}
			if !p.at(token.Comma) {
				break
			}
			comma := p.next()
			if p.at(token.RParen) {
				p.errorf(diagnostic.TrailingComma, comma.Span, "trailing comma is not allowed in an argument list")
				break
			}
		}
	}
	if rp, ok := p.closeParen("',' or ')' in argument list"); ok {
		call.Loc = source.Join(call.Loc, rp.Span)
	}
	return call
}

func (p *parser) parsePrimary() ast.Expr {
	t := p.cur()
	switch t.Kind {
	case token.Int:
		p.next()
		return &ast.IntLit{Digits: t.Lexeme, Loc: t.Span}
	case token.Float:
		p.next()
		return &ast.FloatLit{Lexeme: t.Lexeme, Loc: t.Span}
	case token.String:
		p.next()
		return &ast.StringLit{Value: t.Value, Loc: t.Span}
	case token.TemplateHead:
		return p.parseTemplate()
	case token.True, token.False:
		p.next()
		return &ast.BoolLit{Value: t.Kind == token.True, Loc: t.Span}
	case token.Ident:
		p.next()
		return &ast.Ident{Name: t.Lexeme, Loc: t.Span}
	case token.LParen:
		return p.parseParen()
	case token.Invalid:
		p.next() // already reported by the lexer
		return &ast.ErrorExpr{Loc: t.Span}
	}
	if p.unwindIfUnclosed(t) {
		return &ast.ErrorExpr{Loc: source.Span{File: t.Span.File, Start: t.Span.Start, End: t.Span.Start}}
	}
	p.errorf(diagnostic.ExpectedExpression, t.Span, "expected expression, found %s", describe(t))
	return &ast.ErrorExpr{Loc: t.Span}
}

func (p *parser) parseParen() ast.Expr {
	lp := p.cur()
	p.openParen()
	inner := p.parseExpr()
	paren := &ast.ParenExpr{Inner: inner, Loc: source.Join(lp.Span, inner.Span())}
	if p.unwinding {
		return paren
	}
	if rp, ok := p.closeParen("')'"); ok {
		paren.Loc = source.Join(lp.Span, rp.Span)
	}
	return paren
}

// parseTemplate parses TEMPLATE_HEAD expr {TEMPLATE_MIDDLE expr}
// TEMPLATE_TAIL into an InterpString. An INVALID token where the template
// continues means the lexer already reported an unterminated string or
// interpolation.
func (p *parser) parseTemplate() ast.Expr {
	head := p.next()
	s := &ast.InterpString{Loc: head.Span}
	addText := func(text string) {
		if text != "" {
			s.Segments = append(s.Segments, ast.Segment{Text: text})
		}
	}
	addText(head.Value)
	for {
		e := p.parseExpr()
		s.Segments = append(s.Segments, ast.Segment{Expr: e})
		s.Loc = source.Join(s.Loc, e.Span())
		if p.unwinding {
			return s
		}
		t := p.cur()
		switch t.Kind {
		case token.TemplateMiddle:
			p.next()
			addText(t.Value)
			s.Loc = source.Join(s.Loc, t.Span)
			continue
		case token.TemplateTail:
			p.next()
			addText(t.Value)
			s.Loc = source.Join(s.Loc, t.Span)
			return s
		case token.Invalid:
			p.next()
			return s
		}
		if p.pos > 0 && p.toks[p.pos-1].Kind == token.Invalid {
			return s // the template was abandoned by the lexer
		}
		p.errorf(diagnostic.UnexpectedToken, t.Span, "expected '}' to close interpolation, found %s", describe(t))
		return s
	}
}

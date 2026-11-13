package compiler

import (
	"math"

	"github.com/sarvessveeriyah2312/Tread/internal/ast"
	"github.com/sarvessveeriyah2312/Tread/internal/bytecode"
	"github.com/sarvessveeriyah2312/Tread/internal/checker"
	"github.com/sarvessveeriyah2312/Tread/internal/diagnostic"
	"github.com/sarvessveeriyah2312/Tread/internal/source"
)

// Compile translates a checked program into a bytecode module (docs/05
// §5.4). The program must have passed the checker without diagnostics.
// The compiler never infers types: it reads them from the CheckedProgram
// side tables. A violated checked-program invariant is reported as
// TR-C001 and no module is returned; Compile never panics.
func Compile(cp *checker.CheckedProgram) (*bytecode.Module, []diagnostic.Diagnostic) {
	c := &compiler{
		cp:        cp,
		mod:       &bytecode.Module{},
		funcIndex: map[*checker.Function]int{},
		consts:    map[constKey]int{},
	}
	if cp == nil || cp.Program == nil {
		c.internal(source.Span{}, "no checked program")
		return nil, c.diags
	}
	if cp.Main == nil {
		c.internal(cp.Program.Loc, "checked program has no valid main function")
		return nil, c.diags
	}
	for i, fn := range cp.Functions {
		c.funcIndex[fn] = i
	}
	for _, e := range cp.Enums {
		info := bytecode.EnumInfo{Name: e.Name}
		for _, v := range e.Variants {
			info.Variants = append(info.Variants, bytecode.VariantInfo{Name: v.Name, Fields: len(v.Fields)})
		}
		c.mod.Enums = append(c.mod.Enums, info)
	}
	for _, fn := range cp.Functions {
		c.mod.Functions = append(c.mod.Functions, c.function(fn))
	}
	c.mod.Main = c.funcIndex[cp.Main]
	if len(c.diags) > 0 {
		return nil, c.diags
	}
	if err := c.mod.Validate(); err != nil {
		c.internal(cp.Program.Loc, "generated invalid bytecode: %v", err)
		return nil, c.diags
	}
	return c.mod, nil
}

type constKey struct {
	kind bytecode.Kind
	i    int64
	f    uint64
	s    string
	b    bool
}

type compiler struct {
	cp        *checker.CheckedProgram
	mod       *bytecode.Module
	funcIndex map[*checker.Function]int
	consts    map[constKey]int
	diags     []diagnostic.Diagnostic

	// Per-function state.
	fn       *bytecode.Function
	slots    map[*checker.Symbol]int
	nextSlot int
	loops    []*loop // enclosing loops, innermost last (ADR-0012)
}

// loop collects the jumps of `break` and `continue` statements until
// their targets are known.
type loop struct {
	breaks    []int
	continues []int
}

func (c *compiler) internal(span source.Span, format string, args ...any) {
	c.diags = append(c.diags, diagnostic.Errorf(diagnostic.InternalCompilerError, span,
		"internal compiler error: "+format, args...))
}

// emit2 emits an instruction with two operands.
func (c *compiler) emit2(op bytecode.Opcode, a, b int, span source.Span) int {
	c.fn.Code = append(c.fn.Code, bytecode.Instr{Op: op, A: a, B: b})
	c.fn.Spans = append(c.fn.Spans, span)
	return len(c.fn.Code) - 1
}

func (c *compiler) emit(op bytecode.Opcode, a int, span source.Span) int {
	c.fn.Code = append(c.fn.Code, bytecode.Instr{Op: op, A: a})
	c.fn.Spans = append(c.fn.Spans, span)
	return len(c.fn.Code) - 1
}

// patch sets the target of the jump at pc to the current end of code.
func (c *compiler) patch(pc int) { c.fn.Code[pc].A = len(c.fn.Code) }

func (c *compiler) constant(v bytecode.Value, span source.Span) {
	key := constKey{kind: v.Kind, i: v.Int, f: math.Float64bits(v.Float), s: v.Str, b: v.Bool}
	idx, ok := c.consts[key]
	if !ok {
		idx = len(c.mod.Constants)
		c.mod.Constants = append(c.mod.Constants, v)
		c.consts[key] = idx
	}
	c.emit(bytecode.OpConst, idx, span)
}

func kindOf(t checker.Type) (bytecode.Kind, bool) {
	if t == nil {
		return 0, false
	}
	switch t.Kind {
	case checker.KUnit:
		return bytecode.KindUnit, true
	case checker.KInt:
		return bytecode.KindInt, true
	case checker.KFloat:
		return bytecode.KindFloat, true
	case checker.KBool:
		return bytecode.KindBool, true
	case checker.KString:
		return bytecode.KindString, true
	case checker.KArray:
		return bytecode.KindArray, true
	case checker.KMap:
		return bytecode.KindMap, true
	case checker.KEnum:
		return bytecode.KindEnum, true
	case checker.KParam:
		return bytecode.KindAny, true // a type parameter: any value
	}
	return 0, false
}

// primitive reports whether k has typed instructions (docs/05 §5.3);
// other kinds use the generic instructions of ADR-0013.
func primitive(k bytecode.Kind) bool { return k <= bytecode.KindString }

// typeOf returns the checked type of e, reporting C001 if it is missing.
func (c *compiler) typeOf(e ast.Expr) (bytecode.Kind, bool) {
	t, ok := c.cp.Types[e]
	if !ok {
		c.internal(e.Span(), "expression has no checked type")
		return 0, false
	}
	k, ok := kindOf(t)
	if !ok {
		c.internal(e.Span(), "expression has invalid type %s", t)
	}
	return k, ok
}

// ---------------------------------------------------------------------------
// Functions and statements

func (c *compiler) function(fn *checker.Function) bytecode.Function {
	out := bytecode.Function{Name: fn.Name, NumParams: len(fn.Params)}
	result, ok := kindOf(fn.Result)
	if !ok || fn.Decl == nil || fn.Decl.Body == nil {
		c.internal(source.Span{}, "function %s has no valid checked signature or body", fn.Name)
		return out
	}
	out.Result = result
	c.fn = &out
	c.loops = nil
	c.slots = map[*checker.Symbol]int{}
	for i, p := range fn.Params {
		c.slots[p] = i
	}
	c.nextSlot = len(fn.Params)

	c.block(fn.Decl.Body)
	end := source.Span{File: fn.Decl.Body.Loc.File, Start: fn.Decl.Body.Loc.End - 1, End: fn.Decl.Body.Loc.End}
	if result == bytecode.KindUnit || c.jumpsToEnd() {
		// Unit functions return implicitly. Non-Unit functions never fall
		// off the end (checker T008); the epilogue then only gives a jump
		// to the end of the code a valid target (docs/05 §5.4).
		c.emit(bytecode.OpUnit, 0, end)
		c.emit(bytecode.OpReturn, 0, end)
	}
	out.NumLocals = c.nextSlot
	c.fn = nil
	return out
}

func (c *compiler) jumpsToEnd() bool {
	for _, in := range c.fn.Code {
		if in.Op.Operand() == bytecode.TargetOperand && in.A == len(c.fn.Code) {
			return true
		}
	}
	return false
}

func (c *compiler) block(b *ast.Block) {
	for _, s := range b.Stmts {
		c.stmt(s)
	}
}

func (c *compiler) slotOf(sym *checker.Symbol, span source.Span) (int, bool) {
	if sym == nil {
		c.internal(span, "name has no resolved symbol")
		return 0, false
	}
	slot, ok := c.slots[sym]
	if !ok {
		c.internal(span, "'%s' has no local slot", sym.Name)
	}
	return slot, ok
}

func (c *compiler) stmt(s ast.Stmt) {
	switch s := s.(type) {
	case *ast.Block:
		c.block(s)
	case *ast.LetStmt:
		sym := c.cp.Defs[s.Name]
		if sym == nil {
			c.internal(s.Loc, "let has no resolved symbol")
			return
		}
		c.expr(s.Value)
		slot := c.nextSlot
		c.nextSlot++
		c.slots[sym] = slot
		c.emit(bytecode.OpStoreLocal, slot, s.Loc)
	case *ast.AssignStmt:
		if id, isIdent := s.Target.(*ast.Ident); isIdent {
			slot, ok := c.slotOf(c.cp.Uses[id], s.Loc)
			if !ok {
				return
			}
			c.expr(s.Value)
			c.emit(bytecode.OpStoreLocal, slot, s.Loc)
			return
		}
		// An element (ADR-0013): the keys, the value, then one
		// copy-on-write store through the variable.
		slot, keys, ok := c.path(s.Target)
		if !ok {
			c.internal(s.Loc, "assignment target is not a variable path")
			return
		}
		for _, k := range keys {
			c.expr(k)
		}
		c.expr(s.Value)
		c.emit2(bytecode.OpStorePath, slot, len(keys), s.Loc)
	case *ast.MatchStmt:
		c.matchStmt(s)
	case *ast.IfStmt:
		c.ifStmt(s)
	case *ast.ReturnStmt:
		if s.Value != nil {
			c.expr(s.Value)
		} else {
			c.emit(bytecode.OpUnit, 0, s.Loc)
		}
		c.emit(bytecode.OpReturn, 0, s.Loc)
	case *ast.WhileStmt:
		c.whileStmt(s)
	case *ast.ForStmt:
		c.forStmt(s)
	case *ast.BreakStmt:
		c.loopJump(s.Loc, func(l *loop, pc int) { l.breaks = append(l.breaks, pc) })
	case *ast.ContinueStmt:
		c.loopJump(s.Loc, func(l *loop, pc int) { l.continues = append(l.continues, pc) })
	case *ast.ExprStmt:
		c.expr(s.Call)
		c.emit(bytecode.OpPop, 0, s.Loc)
	default:
		c.internal(source.Span{}, "unsupported statement %T", s)
	}
}

// ifStmt compiles if / else if / else (docs/05 §5.4).
func (c *compiler) ifStmt(s *ast.IfStmt) {
	if s.Then == nil {
		c.internal(s.Loc, "if statement without a block")
		return
	}
	c.expr(s.Cond)
	jumpElse := c.emit(bytecode.OpJumpIfFalse, 0, s.Cond.Span())
	c.block(s.Then)
	if s.Else == nil {
		c.patch(jumpElse)
		return
	}
	jumpEnd := c.emit(bytecode.OpJump, 0, s.Loc)
	c.patch(jumpElse)
	c.stmt(s.Else) // a *ast.Block or a nested *ast.IfStmt
	c.patch(jumpEnd)
}

// loopJump emits the jump of a `break` or `continue` and records it on
// the innermost loop. The operand stack is empty between statements, so
// the jump keeps every target's stack depth consistent (docs/05 §5.4).
func (c *compiler) loopJump(span source.Span, record func(*loop, int)) {
	if len(c.loops) == 0 {
		c.internal(span, "loop control statement outside a loop")
		return
	}
	record(c.loops[len(c.loops)-1], c.emit(bytecode.OpJump, 0, span))
}

// patchTo sets the targets of the jumps at pcs to target.
func (c *compiler) patchTo(pcs []int, target int) {
	for _, pc := range pcs {
		c.fn.Code[pc].A = target
	}
}

// loopBody compiles a loop body and returns its break and continue jumps.
func (c *compiler) loopBody(b *ast.Block) *loop {
	l := &loop{}
	c.loops = append(c.loops, l)
	c.block(b)
	c.loops = c.loops[:len(c.loops)-1]
	return l
}

// whileStmt compiles (ADR-0012, docs/05 §5.4):
//
//	top:  <cond>; JUMP_IF_FALSE end
//	      <body>              ; continue -> top, break -> end
//	      JUMP top
//	end:
func (c *compiler) whileStmt(s *ast.WhileStmt) {
	if s.Body == nil {
		c.internal(s.Loc, "while statement without a block")
		return
	}
	top := len(c.fn.Code)
	c.expr(s.Cond)
	exit := c.emit(bytecode.OpJumpIfFalse, 0, s.Cond.Span())
	l := c.loopBody(s.Body)
	c.emit(bytecode.OpJump, top, s.Loc)
	c.patch(exit)
	c.patchTo(l.continues, top)
	c.patchTo(l.breaks, len(c.fn.Code))
}

// forStmt compiles a half-open Int range (ADR-0012, docs/05 §5.4). Both
// bounds are evaluated once, start first; the end bound is kept in a
// hidden local. The increment cannot overflow, because it runs only
// when the variable is below the end bound.
//
//	      <start>; STORE_LOCAL var
//	      <end>;   STORE_LOCAL end
//	top:  LOAD_LOCAL var; LOAD_LOCAL end; LT_INT; JUMP_IF_FALSE exit
//	      <body>              ; continue -> next, break -> exit
//	next: LOAD_LOCAL var; CONST 1; ADD_INT; STORE_LOCAL var
//	      JUMP top
//	exit:
func (c *compiler) forStmt(s *ast.ForStmt) {
	if s.Iter != nil {
		c.forIn(s)
		return
	}
	sym := c.cp.Defs[s.Var]
	if sym == nil || s.Start == nil || s.End == nil || s.Body == nil {
		c.internal(s.Loc, "for statement without a resolved variable, range or block")
		return
	}
	header := source.Join(s.Var.Loc, s.End.Span())
	c.expr(s.Start)
	varSlot := c.nextSlot
	c.nextSlot++
	c.slots[sym] = varSlot
	c.emit(bytecode.OpStoreLocal, varSlot, s.Start.Span())
	c.expr(s.End)
	endSlot := c.nextSlot
	c.nextSlot++
	c.emit(bytecode.OpStoreLocal, endSlot, s.End.Span())

	top := len(c.fn.Code)
	c.emit(bytecode.OpLoadLocal, varSlot, header)
	c.emit(bytecode.OpLoadLocal, endSlot, header)
	c.emit(bytecode.OpLtInt, 0, header)
	exit := c.emit(bytecode.OpJumpIfFalse, 0, header)
	l := c.loopBody(s.Body)
	next := len(c.fn.Code)
	c.emit(bytecode.OpLoadLocal, varSlot, header)
	c.constant(bytecode.IntValue(1), header)
	c.emit(bytecode.OpAddInt, 0, header)
	c.emit(bytecode.OpStoreLocal, varSlot, header)
	c.emit(bytecode.OpJump, top, s.Loc)
	c.patch(exit)
	c.patchTo(l.continues, next)
	c.patchTo(l.breaks, len(c.fn.Code))
}

// ---------------------------------------------------------------------------
// Expressions

func (c *compiler) expr(e ast.Expr) {
	switch e := e.(type) {
	case *ast.IntLit:
		v, ok := c.cp.IntValues[e]
		if !ok {
			c.internal(e.Loc, "integer literal has no checked value")
			return
		}
		c.constant(bytecode.IntValue(v), e.Loc)
	case *ast.FloatLit:
		v, ok := c.cp.FloatValues[e]
		if !ok {
			c.internal(e.Loc, "float literal has no checked value")
			return
		}
		c.constant(bytecode.FloatValue(v), e.Loc)
	case *ast.StringLit:
		c.constant(bytecode.StringValue(e.Value), e.Loc)
	case *ast.BoolLit:
		c.constant(bytecode.BoolValue(e.Value), e.Loc)
	case *ast.InterpString:
		c.interp(e)
	case *ast.Ident:
		if slot, ok := c.slotOf(c.cp.Uses[e], e.Loc); ok {
			c.emit(bytecode.OpLoadLocal, slot, e.Loc)
		}
	case *ast.ParenExpr:
		c.expr(e.Inner)
	case *ast.UnaryExpr:
		c.unary(e)
	case *ast.BinaryExpr:
		c.binary(e)
	case *ast.CallExpr:
		c.call(e)
	case *ast.ArrayLit:
		for _, el := range e.Elems {
			c.expr(el)
		}
		c.emit(bytecode.OpMakeArray, len(e.Elems), e.Loc)
	case *ast.MapLit:
		for i := range e.Keys {
			c.expr(e.Keys[i])
			c.expr(e.Values[i])
		}
		c.emit(bytecode.OpMakeMap, len(e.Keys), e.Loc)
	case *ast.IndexExpr:
		c.index(e)
	case *ast.MemberExpr:
		c.construct(e, nil)
	case nil:
		c.internal(source.Span{}, "missing expression")
	default:
		c.internal(e.Span(), "unsupported expression %T", e)
	}
}

func (c *compiler) interp(e *ast.InterpString) {
	n := 0
	for _, seg := range e.Segments {
		if seg.Expr == nil {
			c.constant(bytecode.StringValue(seg.Text), e.Loc)
		} else {
			c.expr(seg.Expr)
			c.toString(seg.Expr)
		}
		n++
	}
	if n == 0 {
		c.internal(e.Loc, "interpolated string without segments")
		return
	}
	c.emit(bytecode.OpBuildString, n, e.Loc)
}

// toString emits the typed conversion for the value of e (none for
// String; ADR-0004 item 10).
func (c *compiler) toString(e ast.Expr) {
	k, ok := c.typeOf(e)
	if !ok {
		return
	}
	switch k {
	case bytecode.KindInt:
		c.emit(bytecode.OpToStringInt, 0, e.Span())
	case bytecode.KindFloat:
		c.emit(bytecode.OpToStringFloat, 0, e.Span())
	case bytecode.KindBool:
		c.emit(bytecode.OpToStringBool, 0, e.Span())
	case bytecode.KindString:
	case bytecode.KindUnit:
		c.internal(e.Span(), "value of kind %s cannot be converted to String", k)
	default:
		c.emit(bytecode.OpToStringValue, 0, e.Span()) // ADR-0013
	}
}

func (c *compiler) unary(e *ast.UnaryExpr) {
	if v, ok := c.cp.IntValues[e]; ok {
		c.constant(bytecode.IntValue(v), e.Loc) // the minimum Int (ADR-0004 item 1)
		return
	}
	c.expr(e.Operand)
	k, ok := c.typeOf(e.Operand)
	if !ok {
		return
	}
	switch op := e.Op.String(); {
	case op == "-" && k == bytecode.KindInt:
		c.emit(bytecode.OpNegInt, 0, e.Loc)
	case op == "-" && k == bytecode.KindFloat:
		c.emit(bytecode.OpNegFloat, 0, e.Loc)
	case op == "!" && k == bytecode.KindBool:
		c.emit(bytecode.OpNot, 0, e.Loc)
	default:
		c.internal(e.Loc, "unary %s on %s", op, k)
	}
}

type opKey struct {
	op   string
	kind bytecode.Kind
}

// binaryOps maps (operator, operand kind) to the typed opcode (ADR-0004
// items 6 and 10).
var binaryOps = map[opKey]bytecode.Opcode{
	{"+", bytecode.KindInt}: bytecode.OpAddInt, {"-", bytecode.KindInt}: bytecode.OpSubInt,
	{"*", bytecode.KindInt}: bytecode.OpMulInt, {"/", bytecode.KindInt}: bytecode.OpDivInt,
	{"%", bytecode.KindInt}:   bytecode.OpModInt,
	{"+", bytecode.KindFloat}: bytecode.OpAddFloat, {"-", bytecode.KindFloat}: bytecode.OpSubFloat,
	{"*", bytecode.KindFloat}: bytecode.OpMulFloat, {"/", bytecode.KindFloat}: bytecode.OpDivFloat,
	{"+", bytecode.KindString}: bytecode.OpConcatString,
	{"==", bytecode.KindInt}:   bytecode.OpEqInt, {"==", bytecode.KindFloat}: bytecode.OpEqFloat,
	{"==", bytecode.KindString}: bytecode.OpEqString, {"==", bytecode.KindBool}: bytecode.OpEqBool,
	{"!=", bytecode.KindInt}: bytecode.OpNeInt, {"!=", bytecode.KindFloat}: bytecode.OpNeFloat,
	{"!=", bytecode.KindString}: bytecode.OpNeString, {"!=", bytecode.KindBool}: bytecode.OpNeBool,
	{"<", bytecode.KindInt}: bytecode.OpLtInt, {"<", bytecode.KindFloat}: bytecode.OpLtFloat,
	{"<=", bytecode.KindInt}: bytecode.OpLeInt, {"<=", bytecode.KindFloat}: bytecode.OpLeFloat,
	{">", bytecode.KindInt}: bytecode.OpGtInt, {">", bytecode.KindFloat}: bytecode.OpGtFloat,
	{">=", bytecode.KindInt}: bytecode.OpGeInt, {">=", bytecode.KindFloat}: bytecode.OpGeFloat,
}

func (c *compiler) binary(e *ast.BinaryExpr) {
	op := e.Op.String()
	switch op {
	case "&&":
		// a && b: a; JUMP_IF_FALSE f; b; JUMP end; f: CONST false; end:
		c.expr(e.Left)
		jumpFalse := c.emit(bytecode.OpJumpIfFalse, 0, e.Loc)
		c.expr(e.Right)
		jumpEnd := c.emit(bytecode.OpJump, 0, e.Loc)
		c.patch(jumpFalse)
		c.constant(bytecode.BoolValue(false), e.Loc)
		c.patch(jumpEnd)
		return
	case "||":
		// a || b: a; JUMP_IF_FALSE r; CONST true; JUMP end; r: b; end:
		c.expr(e.Left)
		jumpRight := c.emit(bytecode.OpJumpIfFalse, 0, e.Loc)
		c.constant(bytecode.BoolValue(true), e.Loc)
		jumpEnd := c.emit(bytecode.OpJump, 0, e.Loc)
		c.patch(jumpRight)
		c.expr(e.Right)
		c.patch(jumpEnd)
		return
	}
	c.expr(e.Left)
	c.expr(e.Right)
	k, ok := c.typeOf(e.Left)
	if !ok {
		return
	}
	if !primitive(k) && (op == "==" || op == "!=") {
		// Structural equality of collections, enums and generic values
		// (ADR-0013).
		if op == "==" {
			c.emit(bytecode.OpEqValue, 0, e.Loc)
		} else {
			c.emit(bytecode.OpNeValue, 0, e.Loc)
		}
		return
	}
	opcode, ok := binaryOps[opKey{op, k}]
	if !ok {
		c.internal(e.Loc, "operator %s on %s has no opcode", op, k)
		return
	}
	c.emit(opcode, 0, e.Loc)
}

func (c *compiler) call(e *ast.CallExpr) {
	if _, isVariant := c.cp.Variants[e]; isVariant {
		c.construct(e, e.Args)
		return
	}
	if m, isMethod := c.cp.Methods[e]; isMethod {
		c.method(e, m)
		return
	}
	id, ok := e.Callee.(*ast.Ident)
	sym := c.cp.Uses[id]
	if !ok || sym == nil || sym.Func == nil {
		c.internal(e.Loc, "call target is not a resolved function")
		return
	}
	if sym.Kind == checker.BuiltinSymbol {
		c.builtin(e, sym.Func.Name)
		return
	}
	idx, ok := c.funcIndex[sym.Func]
	if !ok {
		c.internal(e.Loc, "call to unknown function '%s'", sym.Name)
		return
	}
	for _, a := range e.Args {
		c.expr(a)
	}
	c.emit(bytecode.OpCall, idx, e.Loc)
}

// builtin compiles print and toString (ADR-0004 items 10 and 13).
func (c *compiler) builtin(e *ast.CallExpr, name string) {
	if len(e.Args) != 1 {
		c.internal(e.Loc, "built-in %s with %d arguments", name, len(e.Args))
		return
	}
	arg := e.Args[0]
	c.expr(arg)
	if name == "toString" {
		c.toString(arg)
		return
	}
	k, ok := c.typeOf(arg)
	if !ok {
		return
	}
	switch k {
	case bytecode.KindInt:
		c.emit(bytecode.OpPrintInt, 0, e.Loc)
	case bytecode.KindFloat:
		c.emit(bytecode.OpPrintFloat, 0, e.Loc)
	case bytecode.KindString:
		c.emit(bytecode.OpPrintString, 0, e.Loc)
	case bytecode.KindBool:
		c.emit(bytecode.OpPrintBool, 0, e.Loc)
	case bytecode.KindUnit:
		c.internal(e.Loc, "print of %s", k)
	default:
		c.emit(bytecode.OpPrintValue, 0, e.Loc) // ADR-0013
	}
}

// ---------------------------------------------------------------------------
// Collections, enums and pattern matching (ADR-0013)

// path splits an index expression rooted in a local variable, such as
// grid[r][c], into the variable's slot and the keys, outermost first.
func (c *compiler) path(e ast.Expr) (slot int, keys []ast.Expr, ok bool) {
	for {
		switch x := e.(type) {
		case *ast.IndexExpr:
			keys = append([]ast.Expr{x.Index}, keys...)
			e = x.X
		case *ast.Ident:
			sym := c.cp.Uses[x]
			if sym == nil {
				return 0, nil, false
			}
			slot, ok := c.slots[sym]
			return slot, keys, ok
		default:
			return 0, nil, false
		}
	}
}

// index compiles x[k]. A path rooted in a variable is read with one
// LOAD_PATH, which does not copy anything on the way (ADR-0013).
func (c *compiler) index(e *ast.IndexExpr) {
	if slot, keys, ok := c.path(e); ok {
		for _, k := range keys {
			c.expr(k)
		}
		c.emit2(bytecode.OpLoadPath, slot, len(keys), e.Loc)
		return
	}
	c.expr(e.X)
	c.expr(e.Index)
	c.emit(bytecode.OpIndex, 0, e.Loc)
}

// construct compiles Enum.Variant(args) or Enum.Variant.
func (c *compiler) construct(e ast.Expr, args []ast.Expr) {
	v := c.cp.Variants[e]
	if v == nil {
		c.internal(e.Span(), "variant construction without a resolved variant")
		return
	}
	for _, a := range args {
		c.expr(a)
	}
	c.emit2(bytecode.OpMakeEnum, v.Enum.Index, v.Index, e.Span())
}

// pure reports whether evaluating e cannot change any variable: it
// contains no call.
func pure(e ast.Expr) bool {
	switch x := e.(type) {
	case *ast.CallExpr:
		return false
	case *ast.ParenExpr:
		return pure(x.Inner)
	case *ast.UnaryExpr:
		return pure(x.Operand)
	case *ast.BinaryExpr:
		return pure(x.Left) && pure(x.Right)
	case *ast.IndexExpr:
		return pure(x.X) && pure(x.Index)
	case *ast.InterpString:
		for _, s := range x.Segments {
			if s.Expr != nil && !pure(s.Expr) {
				return false
			}
		}
		return true
	case *ast.ArrayLit:
		for _, el := range x.Elems {
			if !pure(el) {
				return false
			}
		}
		return true
	case *ast.MapLit:
		for i := range x.Keys {
			if !pure(x.Keys[i]) || !pure(x.Values[i]) {
				return false
			}
		}
		return true
	case *ast.MemberExpr:
		return true // a variant without fields
	}
	return true
}

// method compiles a built-in collection method call.
func (c *compiler) method(e *ast.CallExpr, m checker.Method) {
	recv := e.Callee.(*ast.MemberExpr).X
	switch m {
	case checker.MethodPush, checker.MethodPop, checker.MethodRemove:
		slot, keys, ok := c.path(recv)
		if !ok {
			c.internal(e.Loc, "mutating method on a value that is not a variable path")
			return
		}
		for _, k := range keys {
			c.expr(k)
		}
		for _, a := range e.Args {
			c.expr(a)
		}
		op := map[checker.Method]bytecode.Opcode{
			checker.MethodPush: bytecode.OpPushPath, checker.MethodPop: bytecode.OpPopPath, checker.MethodRemove: bytecode.OpRemovePath,
		}[m]
		c.emit2(op, slot, len(keys), e.Loc)
		return
	}
	// Read-only methods. A receiver that is a variable path is read in
	// place, without marking it shared, when the arguments cannot change
	// it; otherwise it is loaded like any value.
	argsPure := true
	for _, a := range e.Args {
		argsPure = argsPure && pure(a)
	}
	if slot, keys, ok := c.path(recv); ok && argsPure {
		for _, k := range keys {
			c.expr(k)
		}
		c.emit2(bytecode.OpPeekPath, slot, len(keys), recv.Span())
	} else {
		c.expr(recv)
	}
	for _, a := range e.Args {
		c.expr(a)
	}
	switch m {
	case checker.MethodLen:
		c.emit(bytecode.OpLen, 0, e.Loc)
	case checker.MethodContains:
		c.emit(bytecode.OpContains, 0, e.Loc)
	case checker.MethodGet:
		c.emit(bytecode.OpMapGet, 0, e.Loc)
	case checker.MethodKeys:
		c.emit(bytecode.OpMapKeys, 0, e.Loc)
	default:
		c.internal(e.Loc, "unknown method %d", m)
	}
}

// newSlot allocates a hidden local.
func (c *compiler) newSlot() int {
	slot := c.nextSlot
	c.nextSlot++
	return slot
}

// forIn compiles `for x in collection` over a snapshot of the array, or
// of the map's keys (ADR-0013):
//
//	      <collection> [MAP_KEYS]; STORE_LOCAL c
//	      CONST 0; STORE_LOCAL i
//	top:  LOAD_LOCAL i; LOAD_LOCAL c; LEN; LT_INT; JUMP_IF_FALSE exit
//	      LOAD_LOCAL c; LOAD_LOCAL i; INDEX; STORE_LOCAL x
//	      <body>              ; continue -> next, break -> exit
//	next: LOAD_LOCAL i; CONST 1; ADD_INT; STORE_LOCAL i
//	      JUMP top
//	exit:
func (c *compiler) forIn(s *ast.ForStmt) {
	sym := c.cp.Defs[s.Var]
	if sym == nil || s.Body == nil {
		c.internal(s.Loc, "for statement without a resolved variable or block")
		return
	}
	header := source.Join(s.Var.Loc, s.Iter.Span())
	c.expr(s.Iter)
	if t := c.cp.Types[s.Iter]; t != nil && t.Kind == checker.KMap {
		c.emit(bytecode.OpMapKeys, 0, s.Iter.Span())
	}
	coll := c.newSlot()
	c.emit(bytecode.OpStoreLocal, coll, s.Iter.Span())
	idx := c.newSlot()
	c.constant(bytecode.IntValue(0), header)
	c.emit(bytecode.OpStoreLocal, idx, header)
	varSlot := c.newSlot()
	c.slots[sym] = varSlot

	top := len(c.fn.Code)
	c.emit(bytecode.OpLoadLocal, idx, header)
	c.emit(bytecode.OpLoadLocal, coll, header)
	c.emit(bytecode.OpLen, 0, header)
	c.emit(bytecode.OpLtInt, 0, header)
	exit := c.emit(bytecode.OpJumpIfFalse, 0, header)
	c.emit(bytecode.OpLoadLocal, coll, header)
	c.emit(bytecode.OpLoadLocal, idx, header)
	c.emit(bytecode.OpIndex, 0, header)
	c.emit(bytecode.OpStoreLocal, varSlot, header)
	l := c.loopBody(s.Body)
	next := len(c.fn.Code)
	c.emit(bytecode.OpLoadLocal, idx, header)
	c.constant(bytecode.IntValue(1), header)
	c.emit(bytecode.OpAddInt, 0, header)
	c.emit(bytecode.OpStoreLocal, idx, header)
	c.emit(bytecode.OpJump, top, s.Loc)
	c.patch(exit)
	c.patchTo(l.continues, next)
	c.patchTo(l.breaks, len(c.fn.Code))
}

// matchStmt compiles a match statement (ADR-0013). The subject is stored
// in a hidden local; each arm tests its pattern, jumping to the next arm
// on the first failed test, binds its names, runs its body and jumps to
// the end. The checker proved the match exhaustive, so some arm matches.
func (c *compiler) matchStmt(s *ast.MatchStmt) {
	c.expr(s.Subject)
	subject := c.newSlot()
	c.emit(bytecode.OpStoreLocal, subject, s.Subject.Span())
	var ends []int
	for _, arm := range s.Arms {
		if arm == nil || arm.Body == nil {
			c.internal(s.Loc, "match arm without a pattern or body")
			return
		}
		var fails []int
		c.pattern(arm.Pattern, subject, nil, &fails)
		c.block(arm.Body)
		ends = append(ends, c.emit(bytecode.OpJump, 0, arm.Loc))
		c.patchTo(fails, len(c.fn.Code))
	}
	c.patchTo(ends, len(c.fn.Code))
}

// access loads the value at a field path inside the match subject.
func (c *compiler) access(subject int, fields []int, span source.Span) {
	c.emit(bytecode.OpLoadLocal, subject, span)
	for _, f := range fields {
		c.emit(bytecode.OpEnumField, f, span)
	}
}

// pattern emits the tests and bindings of p for the value at fields; each
// failed test jumps to an address appended to fails.
func (c *compiler) pattern(p ast.Pattern, subject int, fields []int, fails *[]int) {
	switch p := p.(type) {
	case *ast.WildcardPat:
	case *ast.BindPat:
		sym := c.cp.Defs[p.Name]
		if sym == nil {
			c.internal(p.Span(), "pattern binding without a symbol")
			return
		}
		c.access(subject, fields, p.Span())
		slot := c.newSlot()
		c.slots[sym] = slot
		c.emit(bytecode.OpStoreLocal, slot, p.Span())
	case *ast.LitPat:
		c.access(subject, fields, p.Span())
		c.expr(p.Value)
		k, ok := c.typeOf(p.Value)
		if !ok {
			return
		}
		switch k {
		case bytecode.KindInt:
			c.emit(bytecode.OpEqInt, 0, p.Span())
		case bytecode.KindString:
			c.emit(bytecode.OpEqString, 0, p.Span())
		case bytecode.KindBool:
			c.emit(bytecode.OpEqBool, 0, p.Span())
		default:
			c.internal(p.Span(), "literal pattern of kind %s", k)
			return
		}
		*fails = append(*fails, c.emit(bytecode.OpJumpIfFalse, 0, p.Span()))
	case *ast.VariantPat:
		v := c.cp.Variants[p]
		if v == nil {
			c.internal(p.Loc, "variant pattern without a resolved variant")
			return
		}
		c.access(subject, fields, p.Loc)
		c.emit2(bytecode.OpIsVariant, v.Enum.Index, v.Index, p.Loc)
		*fails = append(*fails, c.emit(bytecode.OpJumpIfFalse, 0, p.Loc))
		for i, sub := range p.Args {
			c.pattern(sub, subject, append(append([]int(nil), fields...), i), fails)
		}
	default:
		c.internal(source.Span{}, "unsupported pattern %T", p)
	}
}

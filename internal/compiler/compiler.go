package compiler

import (
	"math"

	"github.com/sarvessveeriyah2312/AURA-Advanced_Unified_Runtime_Architecture/internal/ast"
	"github.com/sarvessveeriyah2312/AURA-Advanced_Unified_Runtime_Architecture/internal/bytecode"
	"github.com/sarvessveeriyah2312/AURA-Advanced_Unified_Runtime_Architecture/internal/checker"
	"github.com/sarvessveeriyah2312/AURA-Advanced_Unified_Runtime_Architecture/internal/diagnostic"
	"github.com/sarvessveeriyah2312/AURA-Advanced_Unified_Runtime_Architecture/internal/source"
)

// Compile translates a checked program into a bytecode module (docs/05
// §5.4). The program must have passed the checker without diagnostics.
// The compiler never infers types: it reads them from the CheckedProgram
// side tables. A violated checked-program invariant is reported as
// AURA-C001 and no module is returned; Compile never panics.
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
}

func (c *compiler) internal(span source.Span, format string, args ...any) {
	c.diags = append(c.diags, diagnostic.Errorf(diagnostic.InternalCompilerError, span,
		"internal compiler error: "+format, args...))
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
	switch t {
	case checker.Unit:
		return bytecode.KindUnit, true
	case checker.Int:
		return bytecode.KindInt, true
	case checker.Float:
		return bytecode.KindFloat, true
	case checker.Bool:
		return bytecode.KindBool, true
	case checker.String:
		return bytecode.KindString, true
	}
	return 0, false
}

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
		slot, ok := c.slotOf(c.cp.Uses[s.Target], s.Loc)
		if !ok {
			return
		}
		c.expr(s.Value)
		c.emit(bytecode.OpStoreLocal, slot, s.Loc)
	case *ast.IfStmt:
		c.ifStmt(s)
	case *ast.ReturnStmt:
		if s.Value != nil {
			c.expr(s.Value)
		} else {
			c.emit(bytecode.OpUnit, 0, s.Loc)
		}
		c.emit(bytecode.OpReturn, 0, s.Loc)
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
	default:
		c.internal(e.Span(), "value of kind %s cannot be converted to String", k)
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
	opcode, ok := binaryOps[opKey{op, k}]
	if !ok {
		c.internal(e.Loc, "operator %s on %s has no opcode", op, k)
		return
	}
	c.emit(opcode, 0, e.Loc)
}

func (c *compiler) call(e *ast.CallExpr) {
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
	default:
		c.internal(e.Loc, "print of %s", k)
	}
}

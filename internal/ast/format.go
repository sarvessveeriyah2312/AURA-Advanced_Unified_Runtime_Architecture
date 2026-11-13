package ast

import (
	"fmt"
	"strings"
)

// Format renders a node as a deterministic S-expression, for tests and
// debugging. Examples: (+ a (* b c)), (paren x), (call f 1),
// (if c (block) else (if d (block))).
func Format(n Node) string {
	var b strings.Builder
	format(&b, n)
	return b.String()
}

func format(b *strings.Builder, n Node) {
	switch n := n.(type) {
	case nil:
		b.WriteString("<nil>")
	case *Program:
		b.WriteString("(program")
		if n.Module != nil {
			b.WriteByte(' ')
			format(b, n.Module)
		}
		for _, f := range n.Funcs {
			b.WriteByte(' ')
			format(b, f)
		}
		b.WriteByte(')')
	case *ModuleDecl:
		fmt.Fprintf(b, "(module %s)", identName(n.Name))
	case *FuncDecl:
		fmt.Fprintf(b, "(fn %s (", identName(n.Name))
		for i, p := range n.Params {
			if i > 0 {
				b.WriteByte(' ')
			}
			b.WriteString(identName(p.Name))
			if p.Type != nil {
				b.WriteString(":" + p.Type.Name)
			}
		}
		b.WriteByte(')')
		if n.Result != nil {
			b.WriteString(" -> " + n.Result.Name)
		}
		if n.Body != nil {
			b.WriteByte(' ')
			format(b, n.Body)
		}
		b.WriteByte(')')
	case *Block:
		b.WriteString("(block")
		for _, s := range n.Stmts {
			b.WriteByte(' ')
			format(b, s)
		}
		b.WriteByte(')')
	case *LetStmt:
		b.WriteString("(let ")
		if n.Mutable {
			b.WriteString("mut ")
		}
		b.WriteString(identName(n.Name))
		if n.Type != nil {
			b.WriteString(":" + n.Type.Name)
		}
		b.WriteByte(' ')
		format(b, n.Value)
		b.WriteByte(')')
	case *AssignStmt:
		fmt.Fprintf(b, "(assign %s ", identName(n.Target))
		format(b, n.Value)
		b.WriteByte(')')
	case *IfStmt:
		b.WriteString("(if ")
		format(b, n.Cond)
		b.WriteByte(' ')
		if n.Then != nil {
			format(b, n.Then)
		} else {
			b.WriteString("<nil>")
		}
		if n.Else != nil {
			b.WriteString(" else ")
			format(b, n.Else)
		}
		b.WriteByte(')')
	case *WhileStmt:
		b.WriteString("(while ")
		format(b, n.Cond)
		b.WriteByte(' ')
		formatBlock(b, n.Body)
		b.WriteByte(')')
	case *ForStmt:
		fmt.Fprintf(b, "(for %s ", identName(n.Var))
		format(b, n.Start)
		b.WriteString(" .. ")
		format(b, n.End)
		b.WriteByte(' ')
		formatBlock(b, n.Body)
		b.WriteByte(')')
	case *BreakStmt:
		b.WriteString("(break)")
	case *ContinueStmt:
		b.WriteString("(continue)")
	case *ReturnStmt:
		if n.Value == nil {
			b.WriteString("(return)")
			return
		}
		b.WriteString("(return ")
		format(b, n.Value)
		b.WriteByte(')')
	case *ExprStmt:
		format(b, n.Call)
	case *Ident:
		b.WriteString(n.Name)
	case *IntLit:
		b.WriteString(n.Digits)
	case *FloatLit:
		b.WriteString(n.Lexeme)
	case *StringLit:
		fmt.Fprintf(b, "%q", n.Value)
	case *BoolLit:
		fmt.Fprintf(b, "%t", n.Value)
	case *InterpString:
		b.WriteString("(interp")
		for _, s := range n.Segments {
			b.WriteByte(' ')
			if s.Expr == nil {
				fmt.Fprintf(b, "%q", s.Text)
			} else {
				format(b, s.Expr)
			}
		}
		b.WriteByte(')')
	case *ParenExpr:
		b.WriteString("(paren ")
		format(b, n.Inner)
		b.WriteByte(')')
	case *UnaryExpr:
		fmt.Fprintf(b, "(%s ", n.Op)
		format(b, n.Operand)
		b.WriteByte(')')
	case *BinaryExpr:
		fmt.Fprintf(b, "(%s ", n.Op)
		format(b, n.Left)
		b.WriteByte(' ')
		format(b, n.Right)
		b.WriteByte(')')
	case *CallExpr:
		b.WriteString("(call ")
		format(b, n.Callee)
		for _, a := range n.Args {
			b.WriteByte(' ')
			format(b, a)
		}
		b.WriteByte(')')
	case *ErrorExpr:
		b.WriteString("<error>")
	default:
		fmt.Fprintf(b, "<unknown %T>", n)
	}
}

// formatBlock writes a possibly nil block; a nil *Block stored in the
// Node interface would not match case nil.
func formatBlock(b *strings.Builder, blk *Block) {
	if blk == nil {
		b.WriteString("<nil>")
		return
	}
	format(b, blk)
}

func identName(id *Ident) string {
	if id == nil {
		return "<nil>"
	}
	return id.Name
}

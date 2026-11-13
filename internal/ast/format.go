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
		for _, e := range n.Enums {
			b.WriteByte(' ')
			format(b, e)
		}
		for _, f := range n.Funcs {
			b.WriteByte(' ')
			format(b, f)
		}
		b.WriteByte(')')
	case *ModuleDecl:
		fmt.Fprintf(b, "(module %s)", identName(n.Name))
	case *FuncDecl:
		fmt.Fprintf(b, "(fn %s%s (", identName(n.Name), typeParams(n.TypeParams))
		for i, p := range n.Params {
			if i > 0 {
				b.WriteByte(' ')
			}
			b.WriteString(identName(p.Name))
			if p.Type != nil {
				b.WriteString(":" + TypeString(p.Type))
			}
		}
		b.WriteByte(')')
		if n.Result != nil {
			b.WriteString(" -> " + TypeString(n.Result))
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
			b.WriteString(":" + TypeString(n.Type))
		}
		b.WriteByte(' ')
		format(b, n.Value)
		b.WriteByte(')')
	case *AssignStmt:
		b.WriteString("(assign ")
		format(b, n.Target)
		b.WriteByte(' ')
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
		if n.Iter != nil {
			b.WriteString("in ")
			format(b, n.Iter)
		} else {
			format(b, n.Start)
			b.WriteString(" .. ")
			format(b, n.End)
		}
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
	case *EnumDecl:
		fmt.Fprintf(b, "(enum %s%s", identName(n.Name), typeParams(n.TypeParams))
		for _, v := range n.Variants {
			b.WriteByte(' ')
			format(b, v)
		}
		b.WriteByte(')')
	case *Variant:
		if len(n.Fields) == 0 {
			b.WriteString(identName(n.Name))
			return
		}
		b.WriteString("(" + identName(n.Name))
		for _, f := range n.Fields {
			b.WriteString(" " + TypeString(f))
		}
		b.WriteByte(')')
	case *MatchStmt:
		b.WriteString("(match ")
		format(b, n.Subject)
		for _, a := range n.Arms {
			b.WriteByte(' ')
			format(b, a)
		}
		b.WriteByte(')')
	case *MatchArm:
		b.WriteString("(=> ")
		format(b, n.Pattern)
		b.WriteByte(' ')
		formatBlock(b, n.Body)
		b.WriteByte(')')
	case *WildcardPat:
		b.WriteString("_")
	case *BindPat:
		b.WriteString(identName(n.Name))
	case *LitPat:
		format(b, n.Value)
	case *VariantPat:
		name := identName(n.Enum) + "." + identName(n.Variant)
		if n.Args == nil {
			b.WriteString(name)
			return
		}
		b.WriteString("(" + name)
		for _, a := range n.Args {
			b.WriteByte(' ')
			format(b, a)
		}
		b.WriteByte(')')
	case *ArrayLit:
		b.WriteString("(array")
		for _, e := range n.Elems {
			b.WriteByte(' ')
			format(b, e)
		}
		b.WriteByte(')')
	case *MapLit:
		b.WriteString("(map")
		for i := range n.Keys {
			b.WriteByte(' ')
			format(b, n.Keys[i])
			b.WriteByte(':')
			format(b, n.Values[i])
		}
		b.WriteByte(')')
	case *IndexExpr:
		b.WriteString("(index ")
		format(b, n.X)
		b.WriteByte(' ')
		format(b, n.Index)
		b.WriteByte(')')
	case *MemberExpr:
		b.WriteString("(. ")
		format(b, n.X)
		b.WriteString(" " + identName(n.Name) + ")")
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

// TypeString renders a type annotation, e.g. Map<String, Array<Int>>.
func TypeString(t *TypeName) string {
	if t == nil {
		return "<nil>"
	}
	if len(t.Args) == 0 {
		return t.Name
	}
	parts := make([]string, len(t.Args))
	for i, a := range t.Args {
		parts[i] = TypeString(a)
	}
	return t.Name + "<" + strings.Join(parts, ", ") + ">"
}

func typeParams(ps []*Ident) string {
	if len(ps) == 0 {
		return ""
	}
	names := make([]string, len(ps))
	for i, p := range ps {
		names[i] = identName(p)
	}
	return "<" + strings.Join(names, ", ") + ">"
}

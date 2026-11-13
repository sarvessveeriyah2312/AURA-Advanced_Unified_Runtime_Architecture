package ast

import "reflect"

// Inspect traverses the tree rooted at n in source order, calling f for
// each node. If f returns false, the node's children are skipped. Nil
// nodes are not visited. Used by the language server (ADR-0015).
func Inspect(n Node, f func(Node) bool) {
	if isNil(n) || !f(n) {
		return
	}
	walk := func(c Node) { Inspect(c, f) }
	switch n := n.(type) {
	case *Program:
		// Declarations are visited by kind, not in source order.
		if n.Module != nil {
			walk(n.Module)
		}
		for _, d := range n.Imports {
			walk(d)
		}
		for _, d := range n.Structs {
			walk(d)
		}
		for _, d := range n.Enums {
			walk(d)
		}
		for _, d := range n.Impls {
			walk(d)
		}
		for _, d := range n.Funcs {
			walk(d)
		}
		for _, d := range n.Tests {
			walk(d)
		}
	case *ModuleDecl:
		walk(n.Name)
	case *ImportDecl:
		for _, p := range n.Path {
			walk(p)
		}
	case *StructDecl:
		walk(n.Name)
		for _, p := range n.TypeParams {
			walk(p)
		}
		for _, fd := range n.Fields {
			walk(fd)
		}
	case *FieldDecl:
		walk(n.Name)
		walk(n.Type)
	case *EnumDecl:
		walk(n.Name)
		for _, p := range n.TypeParams {
			walk(p)
		}
		for _, v := range n.Variants {
			walk(v)
		}
	case *Variant:
		walk(n.Name)
		for _, t := range n.Fields {
			walk(t)
		}
	case *ImplDecl:
		walk(n.Name)
		for _, fd := range n.Funcs {
			walk(fd)
		}
	case *FuncDecl:
		walk(n.Name)
		for _, p := range n.TypeParams {
			walk(p)
		}
		for _, p := range n.Params {
			walk(p)
		}
		walk(n.Result)
		walk(n.Body)
	case *TestDecl:
		walk(n.Name)
		walk(n.Body)
	case *Param:
		walk(n.Name)
		walk(n.Type)
	case *TypeName:
		if n.Module != nil {
			walk(n.Module)
		}
		for _, a := range n.Args {
			walk(a)
		}
	case *Block:
		for _, s := range n.Stmts {
			walk(s)
		}
	case *LetStmt:
		walk(n.Name)
		walk(n.Type)
		walk(n.Value)
	case *AssignStmt:
		walk(n.Target)
		walk(n.Value)
	case *IfStmt:
		walk(n.Cond)
		walk(n.Then)
		walk(n.Else)
	case *ReturnStmt:
		walk(n.Value)
	case *WhileStmt:
		walk(n.Cond)
		walk(n.Body)
	case *ForStmt:
		walk(n.Var)
		walk(n.Start)
		walk(n.End)
		walk(n.Iter)
		walk(n.Body)
	case *MatchStmt:
		walk(n.Subject)
		for _, a := range n.Arms {
			walk(a)
		}
	case *MatchArm:
		walk(n.Pattern)
		walk(n.Body)
	case *BindPat:
		walk(n.Name)
	case *LitPat:
		walk(n.Value)
	case *VariantPat:
		if n.Module != nil {
			walk(n.Module)
		}
		walk(n.Enum)
		walk(n.Variant)
		for _, a := range n.Args {
			walk(a)
		}
	case *ExprStmt:
		walk(n.Call)
	case *InterpString:
		for _, s := range n.Segments {
			if s.Expr != nil {
				walk(s.Expr)
			}
		}
	case *ParenExpr:
		walk(n.Inner)
	case *UnaryExpr:
		walk(n.Operand)
	case *BinaryExpr:
		walk(n.Left)
		walk(n.Right)
	case *CallExpr:
		walk(n.Callee)
		for _, a := range n.Args {
			walk(a)
		}
	case *ArrayLit:
		for _, e := range n.Elems {
			walk(e)
		}
	case *MapLit:
		for i := range n.Keys {
			walk(n.Keys[i])
			walk(n.Values[i])
		}
	case *IndexExpr:
		walk(n.X)
		walk(n.Index)
	case *MemberExpr:
		walk(n.X)
		walk(n.Name)
	case *StructLit:
		if n.Module != nil {
			walk(n.Module)
		}
		walk(n.Name)
		for _, fi := range n.Fields {
			walk(fi.Name)
			walk(fi.Value)
		}
	}
}

// isNil reports whether n is nil or a typed nil pointer, such as a
// missing *Block stored in Node.
func isNil(n Node) bool {
	if n == nil {
		return true
	}
	v := reflect.ValueOf(n)
	return v.Kind() == reflect.Pointer && v.IsNil()
}

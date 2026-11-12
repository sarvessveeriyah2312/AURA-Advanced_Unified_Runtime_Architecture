package ast

import (
	"testing"

	"github.com/your-org/aura/internal/source"
	"github.com/your-org/aura/internal/token"
)

func TestModuleName(t *testing.T) {
	if got := (&Program{}).ModuleName(); got != "main" {
		t.Errorf("implicit module name = %q, want main", got)
	}
	p := &Program{Module: &ModuleDecl{Name: &Ident{Name: "app"}}}
	if got := p.ModuleName(); got != "app" {
		t.Errorf("explicit module name = %q, want app", got)
	}
}

func TestFormat(t *testing.T) {
	sp := source.Span{}
	a := &Ident{Name: "a", Loc: sp}
	elseIf := &IfStmt{Cond: &Ident{Name: "d"}, Then: &Block{}}
	tests := []struct {
		node Node
		want string
	}{
		{&BinaryExpr{Op: token.Plus, Left: a, Right: &IntLit{Digits: "9223372036854775808"}}, "(+ a 9223372036854775808)"},
		{&UnaryExpr{Op: token.Minus, Operand: &ParenExpr{Inner: &FloatLit{Lexeme: "1.050"}}}, "(- (paren 1.050))"},
		{&CallExpr{Callee: a, Args: []Expr{&StringLit{Value: "x\n"}, &BoolLit{Value: true}}}, `(call a "x\n" true)`},
		{&InterpString{Segments: []Segment{{Text: "n="}, {Expr: a}}}, `(interp "n=" a)`},
		{&IfStmt{Cond: a, Then: &Block{}, Else: elseIf}, "(if a (block) else (if d (block)))"},
		{&LetStmt{Mutable: true, Name: a, Type: &TypeName{Name: "Int"}, Value: &ErrorExpr{}}, "(let mut a:Int <error>)"},
		{&ReturnStmt{}, "(return)"},
		{&FuncDecl{Name: a, Params: []*Param{{Name: &Ident{Name: "x"}, Type: &TypeName{Name: "Int"}}}, Result: &TypeName{Name: "Unit"}, Body: &Block{}},
			"(fn a (x:Int) -> Unit (block))"},
		{&Program{Module: &ModuleDecl{Name: &Ident{Name: "m"}}}, "(program (module m))"},
	}
	for _, tt := range tests {
		if got := Format(tt.node); got != tt.want {
			t.Errorf("Format = %s, want %s", got, tt.want)
		}
	}
}

package token

import "testing"

func TestLookup(t *testing.T) {
	tests := map[string]Kind{
		"let": Let, "module": Module, "fn": Fn, "mut": Mut, "if": If, "else": Else,
		"true": True, "false": False, "return": Return,
		"class": Reserved, "interface": Reserved, "enum": Reserved, "match": Reserved,
		"for": Reserved, "while": Reserved, "async": Reserved, "await": Reserved,
		"import": Reserved, "pub": Reserved,
		// Built-in type names are identifiers (ADR-0004 item 3).
		"Int": Ident, "Float": Ident, "Bool": Ident, "String": Ident, "Unit": Ident,
		"letter": Ident, "_": Ident, "Let": Ident,
	}
	for in, want := range tests {
		if got := Lookup(in); got != want {
			t.Errorf("Lookup(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestIsKeyword(t *testing.T) {
	for _, k := range []Kind{Module, Fn, Let, Mut, If, Else, True, False, Return} {
		if !k.IsKeyword() {
			t.Errorf("%v should be a keyword", k)
		}
	}
	for _, k := range []Kind{Ident, Reserved, Plus, EOF, keywordStart, keywordEnd} {
		if k.IsKeyword() {
			t.Errorf("%v should not be a keyword", k)
		}
	}
}

func TestEveryKindHasName(t *testing.T) {
	for k := EOF; k <= Colon; k++ {
		if k == keywordStart || k == keywordEnd {
			continue
		}
		if k.String() == "Kind(?)" {
			t.Errorf("kind %d has no name", int(k))
		}
	}
}

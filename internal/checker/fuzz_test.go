package checker

import (
	"math/rand"
	"strings"
	"testing"

	"github.com/sarvessveeriyah2312/AURA-Advanced_Unified_Runtime_Architecture/internal/lexer"
	"github.com/sarvessveeriyah2312/AURA-Advanced_Unified_Runtime_Architecture/internal/parser"
	"github.com/sarvessveeriyah2312/AURA-Advanced_Unified_Runtime_Architecture/internal/source"
)

// checkAny runs the whole frontend and the checker on arbitrary input,
// including programs with syntax errors. It must never panic and every
// diagnostic span must be in bounds.
func checkAny(t *testing.T, data []byte) {
	t.Helper()
	f := source.NewFile(1, "fuzz.aura", data)
	toks, _ := lexer.Lex(f)
	prog, _ := parser.Parse(toks)
	_, diags := Check(prog)
	for _, d := range diags {
		if d.Span.Start < 0 || d.Span.End > f.Len() || d.Span.Start > d.Span.End {
			t.Fatalf("diagnostic %s span %v out of bounds for %q", d.Code, d.Span, data)
		}
	}
}

// FuzzCheck: go test ./internal/checker -fuzz=FuzzCheck
func FuzzCheck(f *testing.F) {
	for _, tc := range checkCases {
		f.Add([]byte(tc.source()))
	}
	f.Fuzz(func(t *testing.T, data []byte) { checkAny(t, data) })
}

func TestCheckRandomPrograms(t *testing.T) {
	frags := []string{
		"fn", "main", "f", "u", "print", "(", ")", "{", "}", "->", "Unit", "Int", "Float",
		"String", "Bool", "Foo", "let", "mut", "x", "y", "=", ":", ",", "1", "1.5", "\"s\"",
		"\"${x}\"", "true", "if", "else", "return", "+", "-", "*", "/", "%", "==", "<", "&&",
		"!", "9223372036854775808", "\n",
	}
	rng := rand.New(rand.NewSource(3))
	for i := 0; i < 20000; i++ {
		var b strings.Builder
		for j, n := 0, rng.Intn(40); j < n; j++ {
			b.WriteString(frags[rng.Intn(len(frags))])
			b.WriteByte(' ')
		}
		checkAny(t, []byte(b.String()))
	}
}

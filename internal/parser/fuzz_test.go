package parser

import (
	"math/rand"
	"strings"
	"testing"

	"github.com/sarvessveeriyah2312/AURA-Advanced_Unified_Runtime_Architecture/internal/lexer"
	"github.com/sarvessveeriyah2312/AURA-Advanced_Unified_Runtime_Architecture/internal/source"
)

// parseAll lexes and parses data, failing on panics or out-of-bounds
// diagnostic spans. Non-termination is caught by the `go test` timeout
// rather than a wall-clock check, so results do not depend on machine
// speed (docs/13 §E).
func parseAll(t *testing.T, data []byte) {
	t.Helper()
	f := source.NewFile(1, "fuzz.tr", data)
	toks, _ := lexer.Lex(f)
	_, diags := Parse(toks)
	for _, d := range diags {
		if d.Span.Start < 0 || d.Span.End > f.Len() || d.Span.Start > d.Span.End {
			t.Errorf("diagnostic %s span %v out of bounds for %q", d.Code, d.Span, data)
		}
	}
}

// FuzzParse is required case S86 (docs/03 §3.5).
// Run with: go test ./internal/parser -fuzz=FuzzParse
func FuzzParse(f *testing.F) {
	for _, tc := range parseCases {
		if len(tc.src) <= 1024 { // the nesting-limit cases are large; skip as seeds
			f.Add([]byte(tc.src))
		}
	}
	f.Fuzz(func(t *testing.T, data []byte) { parseAll(t, data) })
}

// TestParseRandomTokenStreams exercises S86 on every `go test` run with
// random sequences of Tread fragments.
func TestParseRandomTokenStreams(t *testing.T) {
	frags := []string{
		"fn", "main", "(", ")", "{", "}", "let", "mut", "x", "=", "1", "9abc", "1.5",
		"if", "else", "return", "module", ",", ":", "->", "+", "-", "*", "&&", "||",
		"!", "==", "<", "\"s\"", "\"${", "}\"", "f(", "\n", "\r\n", " ", "Int", "match", ";",
	}
	rng := rand.New(rand.NewSource(2))
	for i := 0; i < 20000; i++ {
		var b strings.Builder
		for j, n := 0, rng.Intn(30); j < n; j++ {
			b.WriteString(frags[rng.Intn(len(frags))])
			b.WriteByte(' ')
		}
		parseAll(t, []byte(b.String()))
		if t.Failed() {
			return
		}
	}
}

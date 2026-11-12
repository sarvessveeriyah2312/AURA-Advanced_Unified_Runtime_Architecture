package lexer

import (
	"testing"

	"github.com/sarvessveeriyah2312/AURA-Advanced_Unified_Runtime_Architecture/internal/source"
)

// FuzzLex is required case L69 (docs/03 §3.4): arbitrary bytes never
// panic, spans stay in bounds and increasing, and the stream ends in EOF.
// Run with: go test ./internal/lexer -fuzz=FuzzLex
func FuzzLex(f *testing.F) {
	for _, tc := range lexCases {
		f.Add([]byte(tc.src))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		file := source.NewFile(1, "fuzz.tr", data)
		toks, diags := Lex(file)
		checkInvariants(t, file, toks, diags)
	})
}

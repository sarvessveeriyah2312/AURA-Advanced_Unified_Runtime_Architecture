// Package pipeline runs the Tread compiler frontend and bytecode compiler
// over one source file (docs/04 §4.1). It is the SDK's compiler; the
// runtime edition does not link it (ADR-0011).
//
// It contains no language logic: each phase is delegated to its package.
package pipeline

import (
	"sort"

	"github.com/sarvessveeriyah2312/Tread/internal/bytecode"
	"github.com/sarvessveeriyah2312/Tread/internal/checker"
	"github.com/sarvessveeriyah2312/Tread/internal/compiler"
	"github.com/sarvessveeriyah2312/Tread/internal/diagnostic"
	"github.com/sarvessveeriyah2312/Tread/internal/lexer"
	"github.com/sarvessveeriyah2312/Tread/internal/parser"
	"github.com/sarvessveeriyah2312/Tread/internal/source"
)

// Compiler implements the CLI's compiler interface (cli.Compiler).
type Compiler struct{}

// Check runs lexing, parsing and type checking, and returns the
// diagnostics.
func (Compiler) Check(file *source.File) []diagnostic.Diagnostic {
	_, diags := Analyze(file)
	return diags
}

// Compile runs the frontend and the bytecode compiler.
func (Compiler) Compile(file *source.File) (*bytecode.Module, []diagnostic.Diagnostic) {
	return Build(file)
}

// Analyze runs the frontend: lexing and parsing, then type checking only
// if lexing and parsing produced no diagnostics. Checking a syntactically
// broken program would report spurious errors (for example a missing
// main whose declaration failed to parse). Diagnostics are sorted by
// position; the result is nil if any diagnostic was produced.
func Analyze(file *source.File) (*checker.CheckedProgram, []diagnostic.Diagnostic) {
	toks, lexDiags := lexer.Lex(file)
	prog, parseDiags := parser.Parse(toks)
	syntax := append(append([]diagnostic.Diagnostic(nil), lexDiags...), parseDiags...)
	if len(syntax) > 0 {
		sort.SliceStable(syntax, func(i, j int) bool { return syntax[i].Span.Start < syntax[j].Span.Start })
		return nil, syntax
	}
	checked, checkDiags := checker.Check(prog)
	if len(checkDiags) > 0 {
		return nil, checkDiags
	}
	return checked, nil
}

// Build runs the frontend and the bytecode compiler. The compiler
// validates the module it returns (docs/05 §5.5).
func Build(file *source.File) (*bytecode.Module, []diagnostic.Diagnostic) {
	checked, diags := Analyze(file)
	if len(diags) > 0 {
		return nil, diags
	}
	return compiler.Compile(checked)
}

package cli

import (
	"errors"
	"fmt"
	"io"
	"sort"

	"github.com/your-org/aura/internal/bytecode"
	"github.com/your-org/aura/internal/checker"
	"github.com/your-org/aura/internal/compiler"
	"github.com/your-org/aura/internal/diagnostic"
	"github.com/your-org/aura/internal/lexer"
	"github.com/your-org/aura/internal/parser"
	"github.com/your-org/aura/internal/source"
	"github.com/your-org/aura/internal/vm"
)

// This file orchestrates the compiler phases (docs/07 §7.4). It contains
// no language logic: each phase is delegated to its package, and the only
// I/O is the program output writer handed to the VM.

// analyze runs the frontend: lexing and parsing, then type checking only
// if lexing and parsing produced no diagnostics. Checking a syntactically
// broken program would report spurious errors (for example a missing
// main whose declaration failed to parse). Diagnostics are sorted by
// position; the result is nil if any diagnostic was produced.
func analyze(file *source.File) (*checker.CheckedProgram, []diagnostic.Diagnostic) {
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

// build runs the frontend and the bytecode compiler. The compiler
// validates the module it returns (docs/05 §5.5).
func build(file *source.File) (*bytecode.Module, []diagnostic.Diagnostic) {
	checked, diags := analyze(file)
	if len(diags) > 0 {
		return nil, diags
	}
	return compiler.Compile(checked)
}

// runtimeFailure is an AURA runtime error converted to a diagnostic.
// withSource is false for errors without a source location (bytecode
// validation failures), which are rendered without a source excerpt.
type runtimeFailure struct {
	diag       diagnostic.Diagnostic
	withSource bool
}

// execute runs a compiled module, writing program output to out. It
// returns a runtimeFailure for an AURA runtime error, or a plain error if
// writing program output failed.
func execute(m *bytecode.Module, out io.Writer) (*runtimeFailure, error) {
	err := vm.Run(m, out, vm.Config{})
	if err == nil {
		return nil, nil
	}
	var rt *vm.Error
	if !errors.As(err, &rt) {
		return nil, err
	}
	msg := rt.Message
	if rt.Function != "" {
		msg = fmt.Sprintf("%s (in function '%s')", rt.Message, rt.Function)
	}
	return &runtimeFailure{
		diag:       diagnostic.Errorf(diagnostic.Code(rt.Code), rt.Span, "%s", msg),
		withSource: rt.HasSpan,
	}, nil
}

// exitCodeFor maps a diagnostic code to the process exit code (docs/07
// §7.2 and the Exit column of the registry in docs/04 §4.4.3): internal
// failures exit 3, everything else 1.
func exitCodeFor(code diagnostic.Code) int {
	switch code {
	case diagnostic.InternalCompilerError, diagnostic.InvalidBytecode:
		return ExitInternal
	}
	return ExitFailure
}

// exitCodeForAll returns the most severe exit code of diags.
func exitCodeForAll(diags []diagnostic.Diagnostic) int {
	code := ExitOK
	for _, d := range diags {
		if c := exitCodeFor(d.Code); c > code {
			code = c
		}
	}
	return code
}

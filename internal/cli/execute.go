package cli

import (
	"errors"
	"fmt"
	"io"

	"github.com/sarvessveeriyah2312/Tread/internal/bytecode"
	"github.com/sarvessveeriyah2312/Tread/internal/diagnostic"
	"github.com/sarvessveeriyah2312/Tread/internal/vm"
)

// runtimeFailure is a Tread runtime error converted to a diagnostic.
// withSource is false for errors without a source location (bytecode
// validation failures), which are rendered without a source excerpt.
// trace is the call stack, innermost first (ADR-0009).
type runtimeFailure struct {
	diag       diagnostic.Diagnostic
	withSource bool
	trace      []diagnostic.TraceFrame
}

// execute runs a compiled module, writing program output to out. It
// returns a runtimeFailure for a Tread runtime error, or a plain error if
// writing program output failed.
func execute(m *bytecode.Module, out io.Writer, limits vm.Config) (*runtimeFailure, error) {
	err := vm.Run(m, out, limits)
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
	trace := make([]diagnostic.TraceFrame, len(rt.Trace))
	for i, f := range rt.Trace {
		trace[i] = diagnostic.TraceFrame{Function: f.Function, Span: f.Span}
	}
	return &runtimeFailure{
		diag:       diagnostic.Errorf(diagnostic.Code(rt.Code), rt.Span, "%s", msg),
		withSource: rt.HasSpan,
		trace:      trace,
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

// Package cli implements the aura command-line interface.
//
// The CLI is the only layer that decides process exit codes. Program output
// goes to stdout; diagnostics, errors and usage text go to stderr
// (docs/07-cli-and-tooling.md).
package cli

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/your-org/aura/internal/diagnostic"
	"github.com/your-org/aura/internal/source"
)

// Exit codes, as specified in docs/07-cli-and-tooling.md.
const (
	// ExitOK indicates success.
	ExitOK = 0
	// ExitFailure indicates a source, type, or runtime failure.
	ExitFailure = 1
	// ExitUsage indicates CLI usage errors or invalid arguments.
	ExitUsage = 2
	// ExitInternal indicates an internal compiler or runtime failure.
	ExitInternal = 3
)

const usage = `Usage: aura <command> [arguments]

Commands:
  run <file.aura>     Check, compile and run a program
  check <file.aura>   Check a program for errors without running it
  version             Print the AURA version
  help                Print this help

Flags:
  -h, --help     Print this help
  --version      Print the AURA version
`

// Run executes the aura CLI with the given arguments (excluding the program
// name), writing to stdout and stderr, and returns the process exit code.
func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return ExitUsage
	}

	cmd, rest := args[0], args[1:]
	switch cmd {
	case "version", "--version":
		if len(rest) > 0 {
			return usageError(stderr, "%s takes no arguments", cmd)
		}
		fmt.Fprintf(stdout, "aura %s\n", Version)
		return ExitOK
	case "help", "-h", "--help":
		if len(rest) > 0 {
			return usageError(stderr, "%s takes no arguments", cmd)
		}
		fmt.Fprint(stdout, usage)
		return ExitOK
	case "check", "run":
		if len(rest) != 1 {
			return usageError(stderr, "%s requires exactly one file argument", cmd)
		}
		file, code := readSource(rest[0], stderr)
		if file == nil {
			return code
		}
		if cmd == "check" {
			return runCheck(file, stderr)
		}
		return runProgram(file, stdout, stderr)
	default:
		return usageError(stderr, "unknown command %q", cmd)
	}
}

func usageError(stderr io.Writer, format string, a ...any) int {
	fmt.Fprintf(stderr, "error: "+format+"\n\n", a...)
	fmt.Fprint(stderr, usage)
	return ExitUsage
}

// SourceExtension is the required extension of AURA source files
// (docs/02 §2.1, ADR-0005).
const SourceExtension = ".aura"

// readSource loads the file named on the command line. A path without the
// .aura extension, or a file that cannot be read, is an invalid argument
// (exit 2); the message names the path (docs/07 §7.4).
func readSource(path string, stderr io.Writer) (*source.File, int) {
	if filepath.Ext(path) != SourceExtension {
		fmt.Fprintf(stderr, "error: %s is not an AURA source file: expected the %s extension\n", path, SourceExtension)
		return nil, ExitUsage
	}
	content, err := os.ReadFile(path)
	if err != nil {
		reason := err
		var pe *fs.PathError
		if errors.As(err, &pe) {
			reason = pe.Err
		}
		fmt.Fprintf(stderr, "error: cannot read %s: %v\n", path, reason)
		return nil, ExitUsage
	}
	return source.NewFile(1, path, content), ExitOK
}

// runCheck implements `aura check`: lex, parse and type-check, and report
// diagnostics. It never compiles or executes the program, and prints
// nothing on success.
func runCheck(file *source.File, stderr io.Writer) int {
	_, diags := analyze(file)
	if len(diags) > 0 {
		renderAll(stderr, file, diags)
		return exitCodeForAll(diags)
	}
	return ExitOK
}

// runProgram implements `aura run`. Each phase runs only if the previous
// one produced no diagnostics, so a program is never executed after a
// compile-time error.
func runProgram(file *source.File, stdout, stderr io.Writer) int {
	module, diags := build(file)
	if len(diags) > 0 {
		renderAll(stderr, file, diags)
		return exitCodeForAll(diags)
	}
	failure, err := execute(module, stdout)
	if err != nil {
		fmt.Fprintf(stderr, "error: cannot write program output: %v\n", err)
		return ExitInternal
	}
	if failure != nil {
		src := file
		if !failure.withSource {
			src = nil
		}
		renderAll(stderr, src, []diagnostic.Diagnostic{failure.diag})
		return exitCodeFor(failure.diag.Code)
	}
	return ExitOK
}

// renderAll writes diagnostics with the shared renderer, separated by a
// blank line (docs/07 §7.4).
func renderAll(w io.Writer, file *source.File, diags []diagnostic.Diagnostic) {
	for i, d := range diags {
		if i > 0 {
			fmt.Fprintln(w)
		}
		_ = diagnostic.Render(w, file, d)
	}
}

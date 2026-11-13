// Package cli implements the tread command-line interface.
//
// The CLI is the only layer that decides process exit codes. Program output
// goes to stdout; diagnostics, errors and usage text go to stderr
// (docs/07-cli-and-tooling.md).
//
// The same CLI serves both editions of tread (ADR-0011): the SDK passes a
// Compiler, the runtime passes nil. This package never imports the
// compiler frontend, so the runtime edition does not link it.
package cli

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/sarvessveeriyah2312/Tread/internal/bytecode"
	"github.com/sarvessveeriyah2312/Tread/internal/diagnostic"
	"github.com/sarvessveeriyah2312/Tread/internal/doctor"
	"github.com/sarvessveeriyah2312/Tread/internal/project"
	"github.com/sarvessveeriyah2312/Tread/internal/source"
	"github.com/sarvessveeriyah2312/Tread/internal/vm"
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

// Compiler is the SDK's source pipeline (internal/pipeline). It is nil in
// the runtime edition.
type Compiler interface {
	// Check loads, lexes, parses and type-checks a program whose root is
	// file. It returns every file loaded, root first (ADR-0014).
	Check(file *source.File) (source.Set, []diagnostic.Diagnostic)
	// Compile checks a program and compiles it to a validated module.
	Compile(file *source.File) (*bytecode.Module, source.Set, []diagnostic.Diagnostic)
	// CompileTests compiles a program with its tests (ADR-0015). The
	// module is nil, without diagnostics, when the file has no tests.
	CompileTests(file *source.File) (*bytecode.Module, source.Set, []diagnostic.Diagnostic)
	// Format formats one file: the formatted text, its syntax errors, or
	// an error when it cannot be formatted safely (ADR-0015).
	Format(file *source.File) ([]byte, []diagnostic.Diagnostic, error)
}

// LanguageServer is implemented by an SDK compiler that can serve the
// Language Server Protocol (ADR-0015).
type LanguageServer interface {
	ServeLSP(in io.Reader, out io.Writer) error
}

const sdkUsage = `Usage: tread <command> [arguments]

Commands:
  new <name>                       Create a new project in the directory <name>
  check <file.tr>                  Check a program for errors without running it
  fmt [--check] <file.tr>...       Format files in place (--check: list unformatted files)
  test <file.tr>...                Run the tests in files
  lsp                              Start the language server (used by editors)
  build <file.tr> [-o <file.tbc>]  Compile a program to a bytecode file
  run <file.tr|file.tbc>           Run a program from source or bytecode
  doctor                           Diagnose the Tread installation
  version                          Print the Tread version
  sdk --version                    Print the Tread SDK version
  runtime --version                Print the Tread Runtime version
  help                             Print this help

Flags:
  -h, --help     Print this help
  --version      Print the Tread version
`

const runtimeUsage = `Usage: tread <command> [arguments]

This is the Tread Runtime. It runs compiled Tread programs (.tbc files).
Install the Tread SDK to create, check, build and run .tr source files.

Commands:
  run <file.tbc>       Run a compiled program
  doctor               Diagnose the Tread installation
  version              Print the Tread version
  runtime --version    Print the Tread Runtime version
  help                 Print this help

Flags:
  -h, --help     Print this help
  --version      Print the Tread version
`

// tool is one invocation of the CLI.
type tool struct {
	compiler       Compiler
	stdout, stderr io.Writer
	// limits bounds execution. The CLI sets none; the web playground
	// does (RunSource).
	limits vm.Config
}

func (t *tool) sdk() bool { return t.compiler != nil }

func (t *tool) usage() string {
	if t.sdk() {
		return sdkUsage
	}
	return runtimeUsage
}

// Run executes the tread CLI with the given arguments (excluding the program
// name), writing to stdout and stderr, and returns the process exit code.
// compiler is nil for the runtime edition.
func Run(compiler Compiler, args []string, stdout, stderr io.Writer) int {
	t := &tool{compiler: compiler, stdout: stdout, stderr: stderr}
	if len(args) == 0 {
		fmt.Fprint(stderr, t.usage())
		return ExitUsage
	}

	cmd, rest := args[0], args[1:]
	switch cmd {
	case "version", "--version":
		if len(rest) > 0 {
			return t.usageError("%s takes no arguments", cmd)
		}
		if t.sdk() {
			fmt.Fprintf(stdout, "tread %s\n", Version)
		} else {
			fmt.Fprintf(stdout, "tread %s (runtime)\n", Version)
		}
		return ExitOK
	case "help", "-h", "--help":
		if len(rest) > 0 {
			return t.usageError("%s takes no arguments", cmd)
		}
		fmt.Fprint(stdout, t.usage())
		return ExitOK
	case "sdk", "runtime":
		if len(rest) != 1 || rest[0] != "--version" {
			return t.usageError("usage: tread %s --version", cmd)
		}
		if cmd == "runtime" {
			fmt.Fprintf(stdout, "Tread Runtime %s (bytecode format %d)\n", Version, bytecode.FormatVersion)
			return ExitOK
		}
		if !t.sdk() {
			fmt.Fprintf(stderr, "error: the Tread SDK is not installed: this is the Tread Runtime %s\n", Version)
			return ExitFailure
		}
		fmt.Fprintf(stdout, "Tread SDK %s\n", Version)
		return ExitOK
	case "doctor":
		if len(rest) > 0 {
			return t.usageError("doctor takes no arguments")
		}
		return t.doctor()
	case "new":
		if !t.sdk() {
			return t.needSDK(cmd)
		}
		if len(rest) != 1 {
			return t.usageError("new requires exactly one project name")
		}
		return t.newProject(rest[0])
	case "check":
		if !t.sdk() {
			return t.needSDK(cmd)
		}
		if len(rest) != 1 {
			return t.usageError("%s requires exactly one file argument", cmd)
		}
		file, code := t.readSource(rest[0])
		if file == nil {
			return code
		}
		return t.runCheck(file)
	case "build":
		if !t.sdk() {
			return t.needSDK(cmd)
		}
		return t.build(rest)
	case "fmt", "test", "lsp":
		if !t.sdk() {
			return t.needSDK(cmd)
		}
		switch cmd {
		case "fmt":
			return t.format(rest)
		case "test":
			return t.test(rest)
		}
		if len(rest) > 0 {
			return t.usageError("lsp takes no arguments")
		}
		return t.lsp()
	case "run":
		if len(rest) != 1 {
			return t.usageError("%s requires exactly one file argument", cmd)
		}
		return t.run(rest[0])
	default:
		return t.usageError("unknown command %q", cmd)
	}
}

func (t *tool) usageError(format string, a ...any) int {
	fmt.Fprintf(t.stderr, "error: "+format+"\n\n", a...)
	fmt.Fprint(t.stderr, t.usage())
	return ExitUsage
}

// needSDK rejects an SDK command in the runtime edition (ADR-0011 §1).
func (t *tool) needSDK(what string) int {
	fmt.Fprintf(t.stderr, "error: %s needs the Tread SDK; this is the Tread Runtime, which only runs compiled %s files\n", what, bytecode.Extension)
	return ExitUsage
}

// SourceExtension is the required extension of Tread source files
// (docs/02 §2.1, ADR-0005).
const SourceExtension = ".tr"

// readFile reads a file named on the command line. A file that cannot be
// read is an invalid argument (exit 2); the message names the path
// (docs/07 §7.4).
func (t *tool) readFile(path string) ([]byte, int) {
	content, err := os.ReadFile(path)
	if err != nil {
		reason := err
		var pe *fs.PathError
		if errors.As(err, &pe) {
			reason = pe.Err
		}
		fmt.Fprintf(t.stderr, "error: cannot read %s: %v\n", path, reason)
		return nil, ExitUsage
	}
	return content, ExitOK
}

// readSource loads a source file named on the command line. A path
// without the .tr extension is an invalid argument (exit 2).
func (t *tool) readSource(path string) (*source.File, int) {
	if filepath.Ext(path) != SourceExtension {
		fmt.Fprintf(t.stderr, "error: %s is not a Tread source file: expected the %s extension\n", path, SourceExtension)
		return nil, ExitUsage
	}
	content, code := t.readFile(path)
	if code != ExitOK {
		return nil, code
	}
	return source.NewFile(1, path, content), ExitOK
}

// runCheck implements `tread check`: lex, parse and type-check, and report
// diagnostics. It never compiles or executes the program, and prints
// nothing on success.
func (t *tool) runCheck(file *source.File) int {
	if files, diags := t.compiler.Check(file); len(diags) > 0 {
		renderAll(t.stderr, files, diags)
		return exitCodeForAll(diags)
	}
	return ExitOK
}

// run implements `tread run`, dispatching on the file extension.
func (t *tool) run(path string) int {
	switch filepath.Ext(path) {
	case bytecode.Extension:
		return t.runBytecode(path)
	case SourceExtension:
		if !t.sdk() {
			fmt.Fprintf(t.stderr, "error: %s is Tread source; running source needs the Tread SDK. This is the Tread Runtime, which only runs compiled %s files\n", path, bytecode.Extension)
			return ExitUsage
		}
		file, code := t.readSource(path)
		if file == nil {
			return code
		}
		return t.runSource(file)
	}
	if t.sdk() {
		fmt.Fprintf(t.stderr, "error: %s is not a Tread program: expected the %s or %s extension\n", path, SourceExtension, bytecode.Extension)
	} else {
		fmt.Fprintf(t.stderr, "error: %s is not a Tread bytecode file: expected the %s extension\n", path, bytecode.Extension)
	}
	return ExitUsage
}

// runSource runs a .tr file. Each phase runs only if the previous one
// produced no diagnostics, so a program is never executed after a
// compile-time error.
func (t *tool) runSource(file *source.File) int {
	module, files, diags := t.compiler.Compile(file)
	if len(diags) > 0 {
		renderAll(t.stderr, files, diags)
		return exitCodeForAll(diags)
	}
	return t.runModule(module, files)
}

// runBytecode runs a .tbc file. A file that cannot be loaded is TR-R006
// (exit 1); runtime errors render against the embedded source, exactly
// as `tread run <file.tr>` renders them.
func (t *tool) runBytecode(path string) int {
	data, code := t.readFile(path)
	if code != ExitOK {
		return code
	}
	f, err := bytecode.Decode(data)
	if err != nil {
		d := diagnostic.Errorf(diagnostic.BytecodeLoadError, source.Span{}, "cannot load %s: %v", path, err)
		renderAll(t.stderr, nil, []diagnostic.Diagnostic{d})
		return exitCodeFor(d.Code)
	}
	var files source.Set
	if len(f.Source) > 0 {
		files = append(files, source.NewFile(bytecode.SourceFileID, f.SourceName, f.Source))
		for i, imp := range f.Imports {
			files = append(files, source.NewFile(bytecode.SourceFileID+1+source.FileID(i), imp.Name, imp.Source))
		}
	}
	return t.runModule(f.Module, files)
}

// runModule executes a module and renders any runtime failure. file is
// nil when no source is available.
func (t *tool) runModule(module *bytecode.Module, files source.Set) int {
	failure, err := execute(module, t.stdout, t.limits)
	if err != nil {
		fmt.Fprintf(t.stderr, "error: cannot write program output: %v\n", err)
		return ExitInternal
	}
	if failure == nil {
		return ExitOK
	}
	src := files
	if !failure.withSource {
		src = nil
	}
	renderAll(t.stderr, src, []diagnostic.Diagnostic{failure.diag})
	// ADR-0009: errors below main add a stack trace after a blank line.
	if len(src) > 0 && len(failure.trace) >= 2 {
		fmt.Fprintln(t.stderr)
		_ = diagnostic.RenderTraceSet(t.stderr, src, failure.trace)
	}
	return exitCodeFor(failure.diag.Code)
}

// build implements `tread build <file.tr> [-o <file.tbc>]`. It writes
// nothing on a compile error and prints nothing on success.
func (t *tool) build(args []string) int {
	var in, out string
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "-o":
			if i+1 >= len(args) || out != "" {
				return t.usageError("-o requires exactly one output file")
			}
			out = args[i+1]
			i++
		case strings.HasPrefix(args[i], "-"):
			return t.usageError("unknown flag %q for build", args[i])
		case in != "":
			return t.usageError("build requires exactly one file argument")
		default:
			in = args[i]
		}
	}
	if in == "" {
		return t.usageError("build requires exactly one file argument")
	}
	if out == "" {
		out = strings.TrimSuffix(in, SourceExtension) + bytecode.Extension
	} else if filepath.Ext(out) != bytecode.Extension {
		fmt.Fprintf(t.stderr, "error: output %s must have the %s extension\n", out, bytecode.Extension)
		return ExitUsage
	}
	file, code := t.readSource(in)
	if file == nil {
		return code
	}
	module, files, diags := t.compiler.Compile(file)
	if len(diags) > 0 {
		renderAll(t.stderr, files, diags)
		return exitCodeForAll(diags)
	}
	var imports []bytecode.SourceFile
	for _, f := range files[1:] {
		imports = append(imports, bytecode.SourceFile{Name: f.Name(), Source: f.Content()})
	}
	data, err := bytecode.Encode(&bytecode.File{
		Producer:   "tread " + Version,
		SourceName: in,
		Source:     file.Content(),
		Imports:    imports,
		Module:     module,
	})
	if err != nil {
		d := diagnostic.Errorf(diagnostic.InternalCompilerError, source.Span{}, "cannot encode bytecode: %v", err)
		renderAll(t.stderr, nil, []diagnostic.Diagnostic{d})
		return ExitInternal
	}
	if err := writeFileAtomic(out, data); err != nil {
		fmt.Fprintf(t.stderr, "error: cannot write %s: %v\n", out, err)
		return ExitFailure
	}
	return ExitOK
}

// writeFileAtomic writes data to a temporary file next to path and
// renames it into place, so a failed build never leaves a partial file.
func writeFileAtomic(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tread-build-*")
	if err != nil {
		return unwrapPath(err)
	}
	name := tmp.Name()
	_, werr := tmp.Write(data)
	cerr := tmp.Close()
	if werr == nil {
		werr = cerr
	}
	if werr == nil {
		werr = os.Chmod(name, 0o644)
	}
	if werr == nil {
		werr = os.Rename(name, path)
	}
	if werr != nil {
		os.Remove(name)
		return unwrapPath(werr)
	}
	return nil
}

func unwrapPath(err error) error {
	var pe *fs.PathError
	if errors.As(err, &pe) {
		return pe.Err
	}
	var le *os.LinkError
	if errors.As(err, &le) {
		return le.Err
	}
	return err
}

// newProject implements `tread new <name>`.
func (t *tool) newProject(name string) int {
	if err := project.Create(name, name, Version); err != nil {
		fmt.Fprintf(t.stderr, "error: %v\n", err)
		var ne *project.NameError
		if errors.As(err, &ne) {
			return ExitUsage
		}
		return ExitFailure
	}
	fmt.Fprintf(t.stdout, "Created project %s in %s%c\n\nNext steps:\n  cd %s\n  tread run src/main.tr\n", name, name, filepath.Separator, name)
	return ExitOK
}

// doctor implements `tread doctor`.
func (t *tool) doctor() int {
	info := doctor.Info{Version: Version, Edition: doctor.Runtime}
	if t.sdk() {
		info.Edition = doctor.SDK
		info.Compile = func(name string, src []byte) (*bytecode.Module, error) {
			module, _, diags := t.compiler.Compile(source.NewFile(1, name, src))
			if len(diags) > 0 {
				return nil, fmt.Errorf("%s: %s", diags[0].Code, diags[0].Message)
			}
			return module, nil
		}
	}
	report := doctor.Diagnose(doctor.SystemEnv(), info)
	if err := report.Write(t.stdout); err != nil {
		fmt.Fprintf(t.stderr, "error: cannot write output: %v\n", err)
		return ExitInternal
	}
	if report.Failed() {
		return ExitFailure
	}
	return ExitOK
}

// RunSource checks, compiles and runs Tread source held in memory, as
// `tread run` does for a file: the same diagnostics, stack traces and
// exit codes, with name as the file name in messages. limits bounds
// execution (the web playground stops runaway loops with
// MaxInstructions, TR-R004). With check set it only checks, as
// `tread check` does.
func RunSource(compiler Compiler, name string, src []byte, check bool, limits vm.Config, stdout, stderr io.Writer) int {
	t := &tool{compiler: compiler, stdout: stdout, stderr: stderr, limits: limits}
	file := source.NewFile(1, name, src)
	if check {
		return t.runCheck(file)
	}
	return t.runSource(file)
}

// renderAll writes diagnostics with the shared renderer, separated by a
// blank line (docs/07 §7.4).
func renderAll(w io.Writer, files source.Set, diags []diagnostic.Diagnostic) {
	for i, d := range diags {
		if i > 0 {
			fmt.Fprintln(w)
		}
		_ = diagnostic.RenderSet(w, files, d)
	}
}

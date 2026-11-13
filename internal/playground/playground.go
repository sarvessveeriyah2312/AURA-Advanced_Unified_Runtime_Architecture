// Package playground runs one Tread program for the website's playground
// (cmd/tread-playground, compiled to WebAssembly). Output and diagnostics
// come from internal/cli.RunSource, so they are exactly what `tread run`
// and `tread check` print. It also returns each compiler stage (tokens,
// AST, bytecode) so visitors can see how the program is processed.
package playground

import (
	"errors"
	"fmt"
	"strings"

	"github.com/sarvessveeriyah2312/Tread/internal/ast"
	"github.com/sarvessveeriyah2312/Tread/internal/cli"
	"github.com/sarvessveeriyah2312/Tread/internal/lexer"
	"github.com/sarvessveeriyah2312/Tread/internal/parser"
	"github.com/sarvessveeriyah2312/Tread/internal/pipeline"
	"github.com/sarvessveeriyah2312/Tread/internal/source"
	"github.com/sarvessveeriyah2312/Tread/internal/token"
	"github.com/sarvessveeriyah2312/Tread/internal/vm"
)

// FileName is the name programs have in diagnostics.
const FileName = "main.tr"

// Limits keep a browser tab responsive. MaxInstructions stops a runaway
// loop with TR-R004; MaxOutput stops a loop that prints forever.
const (
	MaxInstructions = 50_000_000
	MaxOutput       = 1 << 20 // bytes of program output
	MaxSource       = 1 << 20 // bytes of source
)

// Result is one playground run.
type Result struct {
	Stdout, Stderr string
	Exit           int
	// Compiler stages; empty when the stage is unavailable (for example,
	// no bytecode after a compile-time error).
	Tokens, AST, Bytecode string
}

// Version is the Tread version the playground runs.
func Version() string { return cli.Version }

// Run checks (check set) or runs src.
func Run(src string, check bool) Result {
	if len(src) > MaxSource {
		return Result{Stderr: fmt.Sprintf("error: the program is larger than the playground limit of %d bytes\n", MaxSource), Exit: cli.ExitUsage}
	}
	var stdout limitedWriter
	var stderr strings.Builder
	exit := cli.RunSource(pipeline.Compiler{}, FileName, []byte(src), check,
		vm.Config{MaxInstructions: MaxInstructions}, &stdout, &stderr)
	r := Result{Stdout: stdout.b.String(), Stderr: stderr.String(), Exit: exit}
	r.Tokens, r.AST, r.Bytecode = stages(src)
	return r
}

// errOutputLimit is returned once a program has printed MaxOutput bytes.
// The CLI reports it as "cannot write program output" (exit 3).
var errOutputLimit = errors.New("the playground's output limit of 1 MiB was reached")

type limitedWriter struct{ b strings.Builder }

func (w *limitedWriter) Write(p []byte) (int, error) {
	if w.b.Len()+len(p) > MaxOutput {
		return 0, errOutputLimit
	}
	return w.b.Write(p)
}

// stages renders the token stream, the AST (when the program parses) and
// the bytecode (when it compiles).
func stages(src string) (tokens, tree, code string) {
	file := source.NewFile(1, FileName, []byte(src))
	toks, lexDiags := lexer.Lex(file)
	tokens = formatTokens(file, toks)
	prog, parseDiags := parser.Parse(toks)
	if len(lexDiags)+len(parseDiags) == 0 {
		tree = PrettyAST(ast.Format(prog))
	}
	if m, diags := pipeline.Build(file); len(diags) == 0 && m != nil {
		code = m.Disassemble()
	}
	return tokens, tree, code
}

func formatTokens(file *source.File, toks []token.Token) string {
	var b strings.Builder
	for _, t := range toks {
		p := file.Position(t.Span.Start)
		text := t.Lexeme
		switch t.Kind {
		case token.Newline:
			text = ""
		case token.String, token.TemplateHead, token.TemplateMiddle, token.TemplateTail:
			text = t.Lexeme + "   value " + fmt.Sprintf("%q", t.Value)
		}
		fmt.Fprintf(&b, "%4d:%-4d %-16s %s\n", p.Line, p.Column, t.Kind, text)
	}
	return b.String()
}

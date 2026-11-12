package vm_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/your-org/aura/internal/checker"
	"github.com/your-org/aura/internal/compiler"
	"github.com/your-org/aura/internal/lexer"
	"github.com/your-org/aura/internal/parser"
	"github.com/your-org/aura/internal/source"
	"github.com/your-org/aura/internal/vm"
)

// FuzzCompileAndRun drives the whole pipeline with arbitrary source. Any
// program the checker accepts must compile without diagnostics (a
// compiler diagnostic for a checked program is a bug) and must execute
// without panicking, within limits. Run with:
//
//	go test ./internal/vm -run '^$' -fuzz=FuzzCompileAndRun
func FuzzCompileAndRun(f *testing.F) {
	seeds, _ := filepath.Glob("../../testdata/e2e/*.aura")
	seeds = append(seeds, "../../examples/hello.aura")
	for _, p := range seeds {
		if src, err := os.ReadFile(p); err == nil {
			f.Add(src)
		}
	}
	f.Add([]byte(mainOf("let max = 9223372036854775807\nprint(max + 1)")))
	f.Add([]byte(mainOf("print(\"${1.5 / 0.0} ${toString(-0.0)}\" + \"x\")")))
	f.Fuzz(func(t *testing.T, data []byte) {
		file := source.NewFile(1, "fuzz.aura", data)
		toks, ld := lexer.Lex(file)
		prog, pd := parser.Parse(toks)
		if len(ld)+len(pd) > 0 {
			return
		}
		cp, cd := checker.Check(prog)
		if len(cd) > 0 {
			return
		}
		m, diags := compiler.Compile(cp)
		if len(diags) > 0 {
			t.Fatalf("checked program failed to compile: %+v\n%s", diags, data)
		}
		var out strings.Builder
		err := vm.Run(m, &out, vm.Config{MaxInstructions: 100000, MaxCallDepth: 200})
		if e, ok := err.(*vm.Error); ok && e.Code == vm.CodeInvalidBytecode {
			t.Fatalf("compiled program produced invalid bytecode: %v\n%s", e, data)
		}
	})
}

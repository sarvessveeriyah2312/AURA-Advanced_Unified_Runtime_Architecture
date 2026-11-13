//go:build js && wasm

// Command tread-playground is the Tread SDK compiled to WebAssembly for
// the website's playground (/compile/). It runs the real lexer, parser,
// checker, compiler and VM in the browser, and renders diagnostics
// exactly as `tread run` and `tread check` do.
//
// Build: scripts/build-playground.sh (GOOS=js GOARCH=wasm).
//
// It registers one JavaScript function and then waits forever:
//
//	treadPlay(source: string, check: boolean) -> {
//	  stdout, stderr: string, exit: number,
//	  tokens, ast, bytecode: string  // compiler stages; "" when unavailable
//	}
package main

import (
	"syscall/js"

	"github.com/sarvessveeriyah2312/Tread/internal/playground"
)

func main() {
	js.Global().Set("treadVersion", playground.Version())
	js.Global().Set("treadPlay", js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) < 1 {
			return nil
		}
		check := len(args) > 1 && args[1].Truthy()
		r := playground.Run(args[0].String(), check)
		return map[string]any{
			"stdout": r.Stdout, "stderr": r.Stderr, "exit": r.Exit,
			"tokens": r.Tokens, "ast": r.AST, "bytecode": r.Bytecode,
		}
	}))
	select {}
}

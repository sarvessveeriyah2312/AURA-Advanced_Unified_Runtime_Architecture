// Command tread is the Tread SDK: the complete toolchain (compiler,
// bytecode tools and virtual machine). See ADR-0011.
package main

import (
	"io"
	"os"

	"github.com/sarvessveeriyah2312/Tread/internal/cli"
	"github.com/sarvessveeriyah2312/Tread/internal/lsp"
	"github.com/sarvessveeriyah2312/Tread/internal/pipeline"
)

// sdk is the SDK's compiler, with the language server (ADR-0015).
type sdk struct{ pipeline.Compiler }

func (sdk) ServeLSP(in io.Reader, out io.Writer) error { return lsp.Serve(in, out, cli.Version) }

func main() {
	os.Exit(cli.Run(sdk{}, os.Args[1:], os.Stdout, os.Stderr))
}

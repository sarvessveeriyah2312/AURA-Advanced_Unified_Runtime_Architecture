// Command tread is the Tread SDK: the complete toolchain (compiler,
// bytecode tools and virtual machine). See ADR-0011.
package main

import (
	"os"

	"github.com/sarvessveeriyah2312/Tread/internal/cli"
	"github.com/sarvessveeriyah2312/Tread/internal/pipeline"
)

func main() {
	os.Exit(cli.Run(pipeline.Compiler{}, os.Args[1:], os.Stdout, os.Stderr))
}

// Command aura is the AURA language toolchain.
package main

import (
	"os"

	"github.com/your-org/aura/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr))
}

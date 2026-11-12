// Command tread is the Tread language toolchain.
package main

import (
	"os"

	"github.com/sarvessveeriyah2312/Tread/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr))
}

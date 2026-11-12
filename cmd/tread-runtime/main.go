// Command tread-runtime is the Tread Runtime edition of tread: it runs
// compiled .tbc programs and does not contain the compiler (ADR-0011).
// Release distributions install it under the name tread.
package main

import (
	"os"

	"github.com/sarvessveeriyah2312/Tread/internal/cli"
)

func main() {
	os.Exit(cli.Run(nil, os.Args[1:], os.Stdout, os.Stderr))
}

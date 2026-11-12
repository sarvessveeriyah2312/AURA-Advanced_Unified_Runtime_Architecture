# AURA

**Advanced Unified Runtime Architecture**

*Give ideas the power to become reality.*

AURA is a small, statically typed, general-purpose programming language
implemented in Go. It was developed under the codename NOVA; AURA is its
official name.

## What AURA is

A `.aura` source file goes through this pipeline:

1.  The **AURA Compiler** checks the program: lexer, parser, and type
    checker.
2.  The compiler translates the program into typed **AURA Bytecode**.
3.  The **AURA Virtual Machine**, a stack-based interpreter, executes
    the bytecode.
4.  The `aura` command-line tool (the **AURA CLI**) runs the whole
    pipeline.

## Design goals

-   **Readability:** clear, consistent syntax, with newlines instead of
    semicolons.
-   **Predictability:** specified evaluation, types, and errors. `Int`
    arithmetic is checked, and never wraps silently.
-   **Static safety:** type errors are caught before the program runs.
-   **Good diagnostics:** every error has a stable code (for example
    `AURA-T001`) and an exact source location.
-   **A small core:** a compiler and runtime that one developer can
    understand and extend.

## Status

**Version 0.1.0, pre-release.** The MVP is implemented and tested end
to end on Linux, macOS, and Windows. It is being prepared for its first
public release, which has not been published yet. AURA 0.x is
experimental: syntax and behavior may change between minor versions,
and every change is recorded in [`CHANGELOG.md`](CHANGELOG.md).

## Installing

### From a release archive

Each release provides one archive per platform, plus `SHA256SUMS`:

  Platform               Archive
  ---------------------- ------------------------------------
  macOS, Apple silicon   `aura_0.1.0_darwin_arm64.tar.gz`
  macOS, Intel           `aura_0.1.0_darwin_amd64.tar.gz`
  Linux, x86-64          `aura_0.1.0_linux_amd64.tar.gz`
  Linux, ARM64           `aura_0.1.0_linux_arm64.tar.gz`
  Windows, x86-64        `aura_0.1.0_windows_amd64.zip`

Each archive contains a folder with the `aura` executable (`aura.exe`
on Windows), `LICENSE`, `README.md`, `CHANGELOG.md`, and
`THIRD_PARTY_NOTICES.txt`.

On macOS or Linux:

``` bash
shasum -a 256 -c SHA256SUMS --ignore-missing    # or: sha256sum -c SHA256SUMS --ignore-missing
tar -xzf aura_0.1.0_linux_amd64.tar.gz
sudo install aura_0.1.0_linux_amd64/aura /usr/local/bin/aura   # or any directory on your PATH
aura version                                                   # aura 0.1.0
```

On Windows (PowerShell):

``` powershell
Get-FileHash aura_0.1.0_windows_amd64.zip -Algorithm SHA256   # compare with SHA256SUMS
Expand-Archive aura_0.1.0_windows_amd64.zip -DestinationPath .
.\aura_0.1.0_windows_amd64\aura.exe version                    # aura 0.1.0
```

Then add the folder to your `PATH`.

**Platform requirements:**

-   macOS 12 (Monterey) or later;
-   Linux with kernel 3.2 or later (the binaries are statically linked
    and need no system libraries; WSL 1 is not supported);
-   Windows 10 or Windows Server 2016, or later.

These are the minimums of the Go 1.25 toolchain that builds AURA.

**Unsigned binaries.** The binaries are not code-signed or notarized.

-   **macOS:** Gatekeeper may block a downloaded `aura`. After checking
    the checksum, remove the quarantine flag:

    ``` bash
    xattr -d com.apple.quarantine aura
    ```

-   **Windows:** SmartScreen may warn the first time `aura.exe` runs.

**Arch Linux users.** The AUR package manager `aura` (`fosskers/aura`)
installs a command with the same name. If you use it, install this
`aura` into a directory that comes later in your `PATH` and run it by
its full path, or give your copy a different file name, such as
`aura-lang`.

### With Go

AURA needs Go 1.25 or later, and has no other dependencies.

``` bash
go install github.com/sarvessveeriyah2312/AURA-Advanced_Unified_Runtime_Architecture/cmd/aura@latest
```

This installs `aura` into `$(go env GOPATH)/bin`.

### Building from source

``` bash
git clone https://github.com/sarvessveeriyah2312/AURA-Advanced_Unified_Runtime_Architecture.git
cd AURA-Advanced_Unified_Runtime_Architecture
go build -o ./bin/aura ./cmd/aura     # the AURA CLI, in ./bin/aura
./bin/aura version                    # aura 0.1.0
```

To build the release archives for every platform, reproducibly and with
checksums, into `dist/`:

``` bash
scripts/build-release.sh
```

## Your first AURA program

Save this as `hello.aura`. The same program is in
[`examples/hello.aura`](examples/hello.aura).

``` aura
fn main() -> Unit {
    let name = "AURA"
    print("Hello, ${name}!")
}
```

### Running a program

``` text
$ aura run hello.aura
Hello, AURA!
```

`aura run` checks, compiles, and executes the program. The program's
output goes to stdout. Diagnostics go to stderr, for example:

``` text
error[AURA-R001]: division by zero (in function 'main')
  --> crash.aura:4:11
   |
 4 |     print(10 / zero)
   |           ^^^^^^^^^
```

### Checking a program

``` text
$ aura check hello.aura
$ echo $?
0
```

`aura check` reports compile-time errors without running anything, and
prints nothing on success.

### Commands and exit codes

  Command                 Description
  ----------------------- --------------------------------------------------
  `aura run <file>`       check, compile, and run a program
  `aura check <file>`     check a program for errors without running it
  `aura version`          print the AURA version (also `--version`)
  `aura help`             print help (also `-h`, `--help`)

  Code   Meaning
  ------ ---------------------------------------------------------------
  0      success
  1      the program has errors (compile-time or runtime)
  2      usage error, a file not ending in `.aura`, or an unreadable file
  3      internal failure

## Supported language features (0.1)

-   **Types:** `Int` (64-bit, checked arithmetic), `Float` (IEEE-754
    binary64), `Bool`, `String`, `Unit`. There are no implicit
    conversions.
-   **Variables:** immutable `let`, mutable `let mut`, optional type
    annotations, and type inference from the initializer.
-   **Functions:** typed parameters, an explicit return type, `return`,
    recursion, and calls before declaration. The entry point is
    `fn main() -> Unit`.
-   **Control flow:** `if`, `else if`, and `else`.
-   **Operators:**
    -   arithmetic `+ - * / %` (`+` also concatenates strings);
    -   comparisons `== != < <= > >=`;
    -   short-circuit `&&`, `||`, and `!`.
-   **Strings:** escapes `\\ \" \n \r \t \$`, and interpolation
    `"${expression}"`.
-   **Built-ins:** `print(value)` and `toString(value)`, for `Int`,
    `Float`, `Bool`, and `String`.
-   **Other:** an optional `module` declaration, `//` and `/* */`
    comments, Unicode in strings and comments, and LF, CRLF, or CR line
    endings.

## Current limitations

AURA 0.1 is deliberately small. It does **not** have:

-   loops (`for` and `while` are reserved; use recursion);
-   arrays, maps, or other collections;
-   user-defined types (structs, classes, enums), generics, or pattern
    matching;
-   function values or closures;
-   imports, multiple source files, or packages;
-   input, file, or network access (the only output is `print`);
-   saved bytecode files, an optimizer, native compilation, a REPL, a
    debugger, a formatter, or an LSP.

Implementation limits:

-   nesting depth (parentheses, blocks, `else if` chains, operator
    chains) is limited to 10,000 levels (`AURA-P014`);
-   the call depth is limited to 10,000 frames, including `main`
    (`AURA-R003`);
-   runtime errors report the innermost function, without a full stack
    trace.

## Development and testing

``` bash
scripts/verify.sh                                   # the full local gate (also run in CI)
go test ./...                                       # all tests
go test ./internal/vm -run '^$' -fuzz=FuzzCompileAndRun -fuzztime=60s   # one of 4 fuzz targets
scripts/smoke-release.sh ./bin/aura                 # CLI smoke test of a built binary
```

`scripts/verify.sh` runs the formatting check, `go vet`, `go test`,
`go build`, and an `aura` smoke test.

-   **CI** (`.github/workflows/ci.yml`) runs the same checks on Linux,
    macOS, and Windows. It also builds the release archives twice to
    prove they are reproducible, then runs each one on its own platform.
-   **Tests** cover the lexer, parser, type checker, bytecode compiler,
    VM, and CLI, including golden-output end-to-end tests in
    `testdata/e2e/`. All tests are deterministic and OS-independent.
-   **Conventions:** read `CLAUDE.md` before contributing, and record
    user-visible changes in `CHANGELOG.md`.

### Repository layout

``` text
cmd/aura/            the aura executable (entry point only)
internal/source/     source files, spans, line/column mapping
internal/token/      token kinds
internal/lexer/      source -> tokens
internal/ast/        syntax tree
internal/parser/     tokens -> syntax tree
internal/diagnostic/ diagnostic codes and rendering
internal/checker/    name resolution and type checking
internal/bytecode/   AURA Bytecode: instructions, values, validation
internal/compiler/   checked program -> AURA Bytecode
internal/vm/         AURA Virtual Machine
internal/cli/        commands, pipeline orchestration, exit codes
examples/            example programs
testdata/e2e/        end-to-end fixtures and golden output
scripts/             verify.sh, build-release.sh, smoke-release.sh,
                     package/ (reproducible archive writer)
```

### Terminology

  Term                       Meaning
  -------------------------- ---------------------------------------------------------------
  AURA Language              the language described above
  AURA Compiler              the lexer, parser, type checker, and bytecode compiler (`internal/`)
  AURA Bytecode              the typed instruction set executed by the VM (in memory only)
  AURA Virtual Machine (VM)  the stack machine that executes AURA Bytecode
  AURA CLI                   the `aura` executable (`cmd/aura`)

## License

AURA is released under the [MIT License](LICENSE).
Copyright (c) 2026 Sarvess Veeriyah.

Release binaries also include the Go runtime and standard library,
which are distributed under Go's BSD-style license. The notice is in
`THIRD_PARTY_NOTICES.txt` in each release archive. AURA has no other
dependencies.

## The AURA name

AURA is the project's chosen name, and the project has decided to keep
it. The name has **not** been cleared as a trademark. The project claims
no trademark rights in it, and trademark clearance remains an open legal
question.

AURA is an independent open-source project. It is not affiliated with
or endorsed by:

-   Salesforce (Aura Components);
-   the Arch Linux `aura` package manager;
-   Neo4j (Aura);
-   any other product or project named Aura.

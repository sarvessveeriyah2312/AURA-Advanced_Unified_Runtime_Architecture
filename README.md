# AURA

**Advanced Unified Runtime Architecture**

*Give ideas the power to become reality.*

> AURA is the official name of this language (ADR-0005). It replaces the
> development codename NOVA. The name has not been cleared as a trademark
> or for availability.

## What AURA is

AURA is a small, statically typed, general-purpose programming language
implemented in Go. Its pipeline:

1.  The **AURA Compiler** checks a `.aura` source file: lexer, parser,
    and type checker.
2.  The compiler then translates the program into typed **AURA
    Bytecode**.
3.  The **AURA Virtual Machine**, a stack-based interpreter, executes
    the bytecode.
4.  The `aura` command-line tool (the **AURA CLI**) runs the whole
    pipeline.

## Design goals

From `docs/01-vision-and-goals.md`:

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

**Version 0.1.0 (pre-release).** The MVP is implemented and tested end
to end, and is being prepared for its first public release (Phase 5,
`docs/13-mvp-hardening.md`). It has not been published yet. Syntax and
behavior change only through documented decisions (`docs/adr/`).

## Installing and building

AURA needs Go 1.25 or later and has no other dependencies.

``` bash
go build -o ./bin/aura ./cmd/aura     # the AURA CLI, in ./bin/aura
./bin/aura version                    # aura 0.1.0
```

To build release binaries for macOS, Linux, and Windows
(reproducibly, with checksums, into `dist/`):

``` bash
scripts/build-release.sh
```

## Your first AURA program

Save this as `hello.aura` (the same program is in
`examples/hello.aura`):

``` aura
fn main() -> Unit {
    let name = "AURA"
    print("Hello, ${name}!")
}
```

### Running a program

``` text
$ ./bin/aura run hello.aura
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
$ ./bin/aura check hello.aura
$ echo $?
0
```

`aura check` reports compile-time errors without running anything, and
prints nothing on success.

**Exit codes** (`docs/07-cli-and-tooling.md`):

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

`docs/02-language-specification.md` is the normative AURA Language
Specification. `docs/03-grammar.ebnf.md` is the grammar.

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

Implementation limits (`docs/02` §2.17):

-   nesting depth is limited to 10,000 levels;
-   the call depth is limited to 10,000 frames;
-   runtime errors report the innermost function, without a full stack
    trace.

## Development and testing

``` bash
scripts/verify.sh                                   # the full local gate (also run in CI)
go test ./...                                       # all tests
go test ./internal/vm -run '^$' -fuzz=FuzzCompileAndRun -fuzztime=60s   # one of 4 fuzz targets
```

`scripts/verify.sh` runs the formatting check, `go vet`, `go test`,
`go build`, and an `aura` smoke test.

-   **CI** (`.github/workflows/ci.yml`) runs the same checks on Linux,
    macOS, and Windows, and verifies reproducible cross-compilation.
-   **Tests** cover every documented rule. The required cases are listed
    in `docs/03` and `docs/10`; the strategy is in `docs/08`.
-   **Conventions:** read `CLAUDE.md` before contributing. Design
    changes need an ADR (`docs/adr/`). Changes are recorded in
    `CHANGELOG.md`.

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
scripts/             verify.sh, build-release.sh
docs/                specification, design, and process (adr/: decisions)
```

### Documentation

-   `docs/01` to `docs/12`: vision, language specification, grammar,
    architecture, bytecode and VM, standard library, CLI, testing,
    roadmap, implementation plan, workflow, and type checker.
-   `docs/13-mvp-hardening.md`: the Phase 5 hardening record and QA
    report.
-   `docs/14-release-checklist.md`: release status and remaining
    prerequisites.
-   `docs/adr/`: architecture decision records.

### Terminology

  Term                       Meaning
  -------------------------- ---------------------------------------------------------------
  AURA Language              the language specified in `docs/02` and `docs/03`
  AURA Compiler              the lexer, parser, type checker, and bytecode compiler (`internal/`)
  AURA Bytecode              the typed instruction set in `docs/05` §5.3 (in memory only)
  AURA Virtual Machine (VM)  the stack machine that executes AURA Bytecode
  AURA CLI                   the `aura` executable (`cmd/aura`)

<div align="center">

# AURA

**Advanced Unified Runtime Architecture**

*Give ideas the power to become reality.*

[![CI](https://github.com/sarvessveeriyah2312/AURA-Advanced_Unified_Runtime_Architecture/actions/workflows/ci.yml/badge.svg)](https://github.com/sarvessveeriyah2312/AURA-Advanced_Unified_Runtime_Architecture/actions/workflows/ci.yml)
&nbsp;·&nbsp; Version 0.1.0 &nbsp;·&nbsp; [MIT License](LICENSE) &nbsp;·&nbsp; macOS · Linux · Windows

</div>

``` aura
fn main() -> Unit {
    let name = "AURA"
    print("Hello, ${name}!")
}
```

``` text
$ aura run hello.aura
Hello, AURA!
```

------------------------------------------------------------------------

## Contents

1.  [What is AURA?](#what-is-aura)
2.  [What AURA does](#what-aura-does)
3.  [Quick start](#quick-start)
4.  [Using the `aura` command](#using-the-aura-command)
5.  [Language guide](#language-guide)
6.  [Language reference](#language-reference)
7.  [What AURA 0.1 does not have yet](#what-aura-01-does-not-have-yet)
8.  [Installation in detail](#installation-in-detail)
9.  [Building and contributing](#building-and-contributing)
10. [The AURA name](#the-aura-name)
11. [License](#license)

------------------------------------------------------------------------

## What is AURA?

AURA is a small, **statically typed**, general-purpose programming
language. You write programs in `.aura` files. The `aura` tool checks
them for mistakes and then runs them.

It is built around five ideas:

| Goal | What it means in practice |
|---|---|
| **Readable** | Clean syntax, and no semicolons: a new line ends a statement. |
| **Predictable** | Every type, operator, and error is specified. Integer overflow is an error, never a silent wrap-around. |
| **Safe before running** | Type errors are caught before a single line executes. |
| **Helpful errors** | Every error has a stable code (like `AURA-T001`), the exact line and column, and a pointer to the problem. |
| **Small and understandable** | A compiler and runtime that one person can read, understand, and extend. |

> **Status: Version 0.1.0, released 2026-11-12.** The first version is
> complete and tested on Linux, macOS, and Windows. AURA 0.x is
> experimental: the language is deliberately small, and may change
> between versions. Every change is recorded in
> [`CHANGELOG.md`](CHANGELOG.md).

## What AURA does

When you run a program, AURA takes it through four stages:

``` text
  hello.aura
      │
      ▼
 ┌──────────────────┐   reads the text, checks the grammar,
 │  AURA Compiler   │   checks every type ──► errors? report them and stop
 └──────────────────┘
      │ AURA Bytecode   (compact, typed instructions)
      ▼
 ┌──────────────────┐
 │  AURA Virtual    │   executes the instructions; stops with a clear
 │  Machine (VM)    │   error on division by zero or integer overflow
 └──────────────────┘
      │
      ▼
   program output
```

-   **The AURA Compiler** reads your source code, checks its structure,
    and checks every type. If anything is wrong, it reports all the
    problems it can find, and nothing runs.
-   **AURA Bytecode** is the compiler's output: a list of simple, typed
    instructions. It lives only in memory; there are no bytecode files.
-   **The AURA Virtual Machine (VM)** executes the bytecode.
-   **The `aura` command (the AURA CLI)** connects the stages:
    -   `aura check` runs only the compiler;
    -   `aura run` runs everything.

## Quick start

**1. Install.** Choose one of these:

-   **Download a release archive.** Extract it and put `aura` on your
    `PATH`. See [Installation in detail](#installation-in-detail).
-   **Install with Go 1.25 or later:**

    ``` bash
    go install github.com/sarvessveeriyah2312/AURA-Advanced_Unified_Runtime_Architecture/cmd/aura@latest
    ```

-   **Build from source:**

    ``` bash
    git clone https://github.com/sarvessveeriyah2312/AURA-Advanced_Unified_Runtime_Architecture.git
    cd AURA-Advanced_Unified_Runtime_Architecture
    go build -o ./bin/aura ./cmd/aura
    ```

**2. Check it works:**

``` text
$ aura version
aura 0.1.0
```

**3. Write a program.** Save this as `hello.aura`. The same file is in
[`examples/hello.aura`](examples/hello.aura).

``` aura
fn main() -> Unit {
    let name = "AURA"
    print("Hello, ${name}!")
}
```

**4. Run it:**

``` text
$ aura run hello.aura
Hello, AURA!
```

## Using the `aura` command

| Command | What it does |
|---|---|
| `aura run <file.aura>` | Checks, compiles, and runs the program. |
| `aura check <file.aura>` | Checks the program for errors without running it. Prints nothing if it is correct. |
| `aura version` | Prints the version (`aura 0.1.0`). Also `aura --version`. |
| `aura help` | Prints the usage summary. Also `aura -h` and `aura --help`. |

-   **Output:** your program's output goes to **stdout**. Errors go to
    **stderr**, so you can redirect them separately.
-   **Files:** source files must end in `.aura`. Any other file name is
    rejected before it is read.

**Exit codes**, for scripts and CI:

| Code | Meaning |
|---|---|
| `0` | success |
| `1` | the program has errors (found by the compiler, or while running) |
| `2` | usage error: a wrong command, a file not ending in `.aura`, or an unreadable file |
| `3` | internal failure of the AURA toolchain itself |

For example, use `aura check` as a quick test in a script:

``` bash
aura check app.aura && echo "no errors"
```

## Language guide

This guide walks through all of AURA 0.1 in about ten minutes. Every
example is a complete program: save it as a `.aura` file and run it
with `aura run`. The output shown is exactly what AURA prints.

### 1. Program structure

Every program has a `main` function. It takes no parameters and returns
`Unit` (nothing). Execution starts there.

``` aura
// A comment runs to the end of the line.
/* A block comment
   can span lines. */
fn main() -> Unit {
    print("Statements end at the end of the line. No semicolons.")
}
```

-   An optional `module` declaration, for example `module app`, may
    come first in the file.
-   Functions can be declared in any order. A function can call another
    one that is declared further down.
-   A long expression can continue onto the next line after an operator
    such as `+`, or inside parentheses.

### 2. Variables and types

``` aura
fn main() -> Unit {
    let language = "AURA"        // String (inferred)
    let year: Int = 2026         // Int (annotated)
    let ratio = 0.75             // Float
    let ready = true             // Bool

    let mut visits = 0           // mutable
    visits = visits + 1

    print(language)
    print(year)
    print(ratio)
    print(ready)
    print(visits)
}
```

``` text
AURA
2026
0.75
true
1
```

-   **`let` creates a constant.** Assigning to it again is an error.
    Use `let mut` when the value needs to change.
-   **Types are inferred** from the value. You can also write them
    explicitly: `let year: Int = 2026`.
-   **The five types** are `Int`, `Float`, `Bool`, `String`, and
    `Unit`.
-   **There are no automatic conversions.** `1 + 2.0` is an error, so
    write `1.0 + 2.0`. Use `toString(value)` to turn a number into
    text.

### 3. Operators and arithmetic

``` aura
fn main() -> Unit {
    print(7 / 2)                 // Int division truncates
    print(7 % 2)
    print(-7 / 2)
    print(7.0 / 2.0)             // Float division
    print(2 + 3 * 4)             // precedence: * before +
    print((2 + 3) * 4)
    print("AU" + "RA")           // String concatenation
    print(10 > 3 && 2 > 5)
    print(!(1 == 2))
    print("count: " + toString(42))
}
```

``` text
3
1
-3
3.5
14
20
AURA
false
true
count: 42
```

-   **`Int` arithmetic is checked.** Overflow, and division or
    remainder by zero, stop the program with an error instead of giving
    a wrong answer.
-   **`Int` division** rounds toward zero, and `%` takes the sign of the
    left-hand number.
-   **`Float` follows IEEE-754.** Dividing a `Float` by zero gives
    infinity, not an error.
-   **`&&` and `||` short-circuit:** the right-hand side is evaluated
    only when it is needed.

### 4. Strings

``` aura
fn main() -> Unit {
    let item = "coffee"
    let price = 4
    let qty = 3
    print("${qty} x ${item} = ${price * qty}")
    print("Tab:\tdone")
    print("Quote: \"AURA\"")
    print("Literal: \${not interpolated}")
}
```

``` text
3 x coffee = 12
Tab:	done
Quote: "AURA"
Literal: ${not interpolated}
```

-   **`${...}` inserts any expression** whose type is `Int`, `Float`,
    `Bool`, or `String`. That includes arithmetic and function calls.
-   **Escapes:** `\n` (new line), `\t` (tab), `\r`, `\"`, `\\`, and `\$`
    (a literal `$`).
-   **One line only:** a string must start and end on the same line.

### 5. Making decisions with `if`

``` aura
fn main() -> Unit {
    grade(91)
    grade(72)
    grade(40)
}

fn grade(score: Int) -> Unit {
    if score >= 80 {
        print("${score}: A")
    } else if score >= 60 {
        print("${score}: B")
    } else {
        print("${score}: try again")
    }
}
```

``` text
91: A
72: B
40: try again
```

-   **The condition must be a `Bool`.** `if 1 { … }` is an error.
-   **Braces are required.** The `{` goes on the same line as the `if`,
    and `else` goes on the same line as the closing `}`.

### 6. Functions

``` aura
fn main() -> Unit {
    print(square(9))
    print(greet("Ada"))
    print(factorial(10))
}

fn square(n: Int) -> Int {
    return n * n
}

fn greet(name: String) -> String {
    return "Hello, ${name}!"
}

fn factorial(n: Int) -> Int {
    if n <= 1 {
        return 1
    }
    return n * factorial(n - 1)
}
```

``` text
81
Hello, Ada!
3628800
```

-   **Types are required** on every parameter, and every function
    declares its return type after `->`.
-   **Every path must return.** A function that returns a value must
    `return` one on every path. A `Unit` function may use a bare
    `return` to stop early.
-   **Recursion** is supported, up to a depth of 10,000 calls.

### 7. Repeating things

AURA 0.1 has no `for` or `while` loops yet. Repeat work with a function
that calls itself. Here is FizzBuzz from 1 to 15:

``` aura
// FizzBuzz from 1 to 15. AURA 0.1 has no loops, so we recurse.
fn main() -> Unit {
    fizzbuzz(1, 15)
}

fn fizzbuzz(n: Int, last: Int) -> Unit {
    if n > last {
        return
    }
    if n % 15 == 0 {
        print("FizzBuzz")
    } else if n % 3 == 0 {
        print("Fizz")
    } else if n % 5 == 0 {
        print("Buzz")
    } else {
        print(n)
    }
    fizzbuzz(n + 1, last)
}
```

``` text
1
2
Fizz
4
Buzz
Fizz
7
8
Fizz
Buzz
11
Fizz
13
14
FizzBuzz
```

### 8. Reading error messages

`aura` reports **every** error it can find, each with:

-   a code;
-   the file, line, and column;
-   the offending source line, with the problem underlined;
-   help where it can offer it.

**Compile-time errors.** Nothing runs:

``` aura
fn main() -> Unit {
    let total: Int = "ten"
    let limit = 5
    limit = 6
}
```

``` text
$ aura check mistakes.aura
error[AURA-T001]: expected Int but found String
  --> mistakes.aura:2:22
   |
 2 |     let total: Int = "ten"
   |                      ^^^^^

error[AURA-T004]: cannot assign twice to immutable variable 'limit'
  --> mistakes.aura:4:5
   |
 4 |     limit = 6
   |     ^^^^^^^^^
   = help: declare it with 'let mut' to allow assignment
   = note: variable declared here (mistakes.aura:3:9)
$ echo $?
1
```

**Runtime errors.** The program stops at the faulty operation. Output
printed before that point is kept:

``` aura
fn main() -> Unit {
    let items = 0
    print("Splitting the bill...")
    print(100 / items)
}
```

``` text
$ aura run crash.aura
Splitting the bill...
error[AURA-R001]: division by zero (in function 'main')
  --> crash.aura:4:11
   |
 4 |     print(100 / items)
   |           ^^^^^^^^^^^
```

The letter in a code tells you which stage found the problem:

| Code | Stage |
|---|---|
| `AURA-Lnnn` | lexer: invalid characters, bad escapes, unterminated strings, malformed numbers |
| `AURA-Pnnn` | parser: syntax and grammar |
| `AURA-Tnnn` | type checker: types, names, mutability, missing returns |
| `AURA-Rnnn` | runtime: division by zero, overflow, call depth |
| `AURA-Cnnn` | compiler internal error (please report it) |

## Language reference

**Types**

| Type | Values | Example |
|---|---|---|
| `Int` | 64-bit signed integers, checked | `42`, `-7` |
| `Float` | 64-bit IEEE-754 floating point; literals are plain decimals with a `.` (no exponent) | `3.14`, `1.0` |
| `Bool` | `true`, `false` | `true` |
| `String` | Unicode text | `"hi ${name}"` |
| `Unit` | "no value": the return type of `main` | — |

**Operators**, from highest to lowest precedence:

| Precedence | Operators | Operand types |
|---|---|---|
| 1 (highest) | `( )`, function calls | — |
| 2 | `!`, unary `-` | `Bool`; `Int` or `Float` |
| 3 | `*` `/` `%` | `Int` or `Float` (`%` is `Int` only) |
| 4 | `+` `-` | `Int` or `Float`; `+` also joins `String`s |
| 5 | `<` `<=` `>` `>=` | `Int` or `Float` |
| 6 | `==` `!=` | two values of the same type |
| 7 | `&&` | `Bool` |
| 8 (lowest) | `\|\|` | `Bool` |

Both operands of an operator must have the same type.

**Built-in functions**

| Function | Description |
|---|---|
| `print(value)` | Prints an `Int`, `Float`, `Bool`, or `String`, followed by a newline. |
| `toString(value)` | Returns the text of an `Int`, `Float`, `Bool`, or `String`. |

**Keywords:** `fn`, `let`, `mut`, `if`, `else`, `return`, `true`,
`false`, `module`.

**Reserved for the future**, and not usable as names: `for`, `while`,
`match`, `class`, `interface`, `enum`, `import`, `pub`, `async`,
`await`.

**Names** use ASCII letters, digits, and `_`, and must not start with a
digit. Strings and comments may contain any Unicode text.

## What AURA 0.1 does not have yet

AURA 0.1 is deliberately small. It does **not** yet have:

-   loops (`for` and `while` are reserved; use recursion);
-   arrays, maps, or other collections;
-   structs, classes, enums, generics, or pattern matching;
-   function values or closures;
-   imports or multi-file programs;
-   input, files, or networking (the only output is `print`);
-   a REPL, a formatter, a debugger, editor (LSP) support, or native
    compilation.

**Implementation limits:**

-   nesting is limited to 10,000 levels (`AURA-P014`);
-   calls are limited to 10,000 deep, including `main` (`AURA-R003`);
-   a runtime error names the function it happened in, but does not show
    a full stack trace.

## Installation in detail

### Release archives

| Platform | Archive | Requires |
|---|---|---|
| macOS, Apple silicon | `aura_0.1.0_darwin_arm64.tar.gz` | macOS 12 or later |
| macOS, Intel | `aura_0.1.0_darwin_amd64.tar.gz` | macOS 12 or later |
| Linux, x86-64 | `aura_0.1.0_linux_amd64.tar.gz` | kernel 3.2 or later (static binary; not WSL 1) |
| Linux, ARM64 | `aura_0.1.0_linux_arm64.tar.gz` | kernel 3.2 or later (static binary) |
| Windows, x86-64 | `aura_0.1.0_windows_amd64.zip` | Windows 10 or Server 2016, or later |

Each archive contains a single folder. It holds the `aura` executable
(`aura.exe` on Windows), `LICENSE`, `README.md`, `CHANGELOG.md`, and
`THIRD_PARTY_NOTICES.txt`. Each release also provides `SHA256SUMS`.

**macOS or Linux:**

``` bash
shasum -a 256 -c SHA256SUMS --ignore-missing    # or: sha256sum -c SHA256SUMS --ignore-missing
tar -xzf aura_0.1.0_linux_amd64.tar.gz
sudo install aura_0.1.0_linux_amd64/aura /usr/local/bin/aura   # or any directory on your PATH
aura version                                                   # aura 0.1.0
```

**Windows (PowerShell):**

``` powershell
Get-FileHash aura_0.1.0_windows_amd64.zip -Algorithm SHA256   # compare with SHA256SUMS
Expand-Archive aura_0.1.0_windows_amd64.zip -DestinationPath .
.\aura_0.1.0_windows_amd64\aura.exe version                    # aura 0.1.0
```

Then add the folder to your `PATH`.

### Platform notes

-   **macOS:** the binaries are not code-signed or notarized, so
    Gatekeeper may block a downloaded `aura`. After checking the
    checksum, run:

    ``` bash
    xattr -d com.apple.quarantine aura
    ```

-   **Windows:** SmartScreen may warn the first time `aura.exe` runs,
    for the same reason.
-   **Arch Linux:** the AUR package manager `aura` (`fosskers/aura`)
    installs a command with the same name. If you use it, either:
    -   install this `aura` into a directory that comes later in your
        `PATH`, and run it by its full path; or
    -   give your copy a different file name, such as `aura-lang`.

### With Go

AURA needs Go 1.25 or later, and has no other dependencies.

``` bash
go install github.com/sarvessveeriyah2312/AURA-Advanced_Unified_Runtime_Architecture/cmd/aura@latest
```

This installs `aura` into `$(go env GOPATH)/bin`.

## Building and contributing

``` bash
go build -o ./bin/aura ./cmd/aura      # build the CLI
scripts/verify.sh                      # the full local gate: format, vet, test, build, smoke test
go test ./...                          # all tests
scripts/build-release.sh               # reproducible release archives and checksums, in dist/
scripts/smoke-release.sh ./bin/aura    # CLI smoke test of a built binary
go test ./internal/vm -run '^$' -fuzz=FuzzCompileAndRun -fuzztime=60s   # one of 4 fuzz targets
```

-   **CI** (`.github/workflows/ci.yml`) runs the same gate on Linux,
    macOS, and Windows. It also builds the release archives twice, to
    prove they are byte-identical, and runs each archive on its own
    platform.
-   **Tests** cover the lexer, parser, type checker, bytecode compiler,
    VM, and CLI, including golden-output end-to-end tests in
    `testdata/e2e/`. All tests are deterministic and OS-independent.
-   **Contributing:** read `CLAUDE.md` first. Record user-visible
    changes in `CHANGELOG.md`.

**Repository layout**

``` text
cmd/aura/            the aura executable (entry point only)
internal/source/     source files, spans, line/column mapping
internal/token/      token kinds
internal/lexer/      source text -> tokens
internal/ast/        syntax tree
internal/parser/     tokens -> syntax tree
internal/diagnostic/ diagnostic codes and error rendering
internal/checker/    name resolution and type checking
internal/bytecode/   AURA Bytecode: instructions, values, validation
internal/compiler/   checked program -> AURA Bytecode
internal/vm/         AURA Virtual Machine
internal/cli/        commands, pipeline, exit codes
examples/            example programs
testdata/e2e/        end-to-end fixtures and expected output
scripts/             verify.sh, build-release.sh, smoke-release.sh,
                     package/ (reproducible archive writer)
```

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

## License

AURA is released under the [MIT License](LICENSE).
Copyright (c) 2026 Sarvess Veeriyah.

Release binaries also include the Go runtime and standard library,
which are distributed under Go's BSD-style license. The notice is in
`THIRD_PARTY_NOTICES.txt` in each release archive. AURA has no other
dependencies.

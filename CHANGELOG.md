# Changelog

All notable changes to Tread are documented in this file. The format is
based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and
Tread uses [Semantic Versioning](https://semver.org/). Entries for 0.1.0
and 0.2.0 are kept as written, under the language's name at the time.

## [Unreleased]

SDK and runtime editions, bytecode files, and installers (ADR-0011).

### Added

-   **Bytecode files.** `tread build <file.tr> [-o <file.tbc>]` compiles
    a program to a `.tbc` file (format version 1, docs/05 §5.10), and
    `tread run <file.tbc>` runs one. Loading checks the header, format
    version, checksum, structure, operands and stack depth before
    anything runs; a file that cannot be loaded is the new error
    **TR-R006** (exit 1). Runtime errors from a `.tbc` render exactly
    as from source, because the file embeds the source text.
-   **Runtime edition.** `cmd/tread-runtime` builds a `tread` that only
    runs `.tbc` files and does not contain the compiler.
-   **`tread new <name>`** creates a project (`tread.toml`,
    `src/main.tr`, `README.md`). It never overwrites anything.
-   **`tread doctor`** checks the installation, `TREAD_HOME`, `PATH`,
    and runs runtime, bytecode-compatibility and compiler self-tests.
-   **`tread sdk --version`** and **`tread runtime --version`**.

### Changed

-   `tread run` with neither a `.tr` nor a `.tbc` argument now says
    `expected the .tr or .tbc extension`.

## [0.3.0] - 2026-11-13

The language is renamed **Tread**: *a language for learning how
languages work*. This is a breaking change, with no backward
compatibility. Tread 0.x is experimental.

### Changed (breaking)

-   **Name.** The language, its compiler, bytecode, virtual machine, and
    CLI are now Tread.
-   **Command.** The executable is `tread`, with the same commands:
    `tread run`, `tread check`, `tread version` (prints `tread 0.3.0`),
    and `tread help`. The previous command name is no longer provided.
-   **Source extension.** Source files use `.tr`, in lower case only.
    Any other extension, including the previous one, is rejected with
    exit code 2, as before.
-   **Error codes.** Diagnostic codes use the prefix `TR-`. Every
    category letter and number is unchanged: for example, the type
    mismatch error is now `TR-T001`, and the call depth error is
    `TR-R003`.
-   **Releases.**
    -   Release archives are named `tread_<version>_<os>_<arch>`
        (`.tar.gz`, or `.zip` for Windows), and contain `tread`
        (`tread.exe` on Windows).
    -   The command builds from `cmd/tread`.
    -   The version override for release builds is `TREAD_VERSION`.

### Fixed

-   **Phase letter of an error code.** `Code.Phase()` read the letter at
    a fixed byte offset that only matched the previous five-character
    prefix. It now reads the letter right after the prefix, whatever its
    length, and has a regression test.

### Notes

-   **No other changes:** language behaviour, syntax, and error message
    wording are unchanged apart from the name and the code prefix.
-   **Example programs** greet `Tread` instead of the previous name.
-   **Repository and module path.** The GitHub repository is now
    `sarvessveeriyah2312/Tread`, and the Go module path is
    `github.com/sarvessveeriyah2312/Tread`; install with
    `go install github.com/sarvessveeriyah2312/Tread/cmd/tread@<version>`.
    Links to the previous repository name redirect on GitHub.

## [0.2.0] - 2026-11-12

Runtime stack traces. There are no language changes, and no breaking
changes.

### Added

-   **Stack traces for runtime errors.** A runtime error raised inside a
    function called from `main` is followed by a `stack trace:` block.
    It lists every active call, innermost first, with the function,
    location, and source line.
    -   Consecutive identical frames (recursion) merge into one line with
        an index range, such as `0-9998:`.
    -   At most 20 frame lines are printed: the 10 innermost, a count of
        the frames not shown, and the 10 outermost.
    -   The error message itself, the exit codes, and the output for
        errors raised in `main` are unchanged.

## [0.1.0] - 2026-11-12

The first version of AURA (Advanced Unified Runtime Architecture): a
statically typed language with a bytecode compiler and a stack-based
virtual machine. Released 2026-11-12.

### Language

-   **Types.**
    -   `Int`: 64-bit, with checked arithmetic.
    -   `Float`: IEEE-754 binary64.
    -   `Bool`, `String`, and `Unit`.
    -   There are no implicit conversions.
-   **Literals.**
    -   Strict decimal `Int` and `Float` literals, range-checked at
        compile time.
    -   `-9223372036854775808` denotes the minimum `Int`.
-   **Variables.** Immutable `let` and mutable `let mut`, with optional
    type annotations and type inference from the initializer.
-   **Functions.** Typed parameters, a required return type, `return`,
    and recursion. Functions may be called before their declaration.
-   **Entry point.** Exactly one `fn main() -> Unit`.
-   **Control flow.** `if`, `else if`, and `else`.
-   **Operators.**
    -   `+ - * / %`, with `+` also concatenating strings.
    -   `== != < <= > >=`.
    -   Short-circuit `&&`, `||`, and `!`.
-   **Strings.**
    -   Escapes `\\ \" \n \r \t \$`.
    -   Interpolation `"${expression}"` for `Int`, `Float`, `Bool`, and
        `String`.
-   **Built-in functions.** `print(value)` and `toString(value)`. They
    are protected: they cannot be redeclared or used as values.
-   **Syntax details.**
    -   Newline-separated statements, with explicit continuation rules.
    -   An optional `module` declaration.
    -   `//` and `/* */` comments.
    -   ASCII identifiers.
    -   Built-in type names are ordinary identifiers, in a separate type
        namespace.
-   **Runtime semantics.**
    -   `Int` overflow, division or remainder by zero, and minimum `Int`
        divided by `-1` are runtime errors.
    -   `/` truncates toward zero, and `%` takes the sign of the
        dividend.
    -   `Float` follows IEEE-754, including infinity and NaN.
    -   Floats are formatted as the shortest round-trip representation:
        `1.0`, `-0.0`, `1e+06`, `NaN`, `+Inf`.

### Tooling

-   **`aura` CLI.**
    -   `aura run <file.aura>`, `aura check <file.aura>`,
        `aura version`, and `aura help`.
    -   Documented exit codes: 0, 1 (program errors), 2 (usage or file
        errors), 3 (internal errors).
    -   Program output goes to stdout and diagnostics to stderr.
-   **Diagnostics.**
    -   Stable codes `AURA-Lnnn`, `AURA-Pnnn`, `AURA-Tnnn`, `AURA-Cnnn`,
        and `AURA-Rnnn` (47 codes).
    -   Precise line and column positions, counted in Unicode code
        points.
    -   Caret rendering aware of tabs and wide characters.
    -   Multiple errors are reported per run, without cascades.
-   **Implementation.** A typed bytecode instruction set (46 opcodes),
    validated before execution.
-   **Builds and CI.**
    -   Reproducible release archives for macOS (amd64, arm64), Linux
        (amd64, arm64), and Windows (amd64): `aura_<version>_<os>_<arch>`
        as `.tar.gz` (`.zip` on Windows), with `SHA256SUMS`
        (`scripts/build-release.sh`).
    -   Each archive contains the `aura` executable, `LICENSE`,
        `README.md`, `CHANGELOG.md`, and `THIRD_PARTY_NOTICES.txt` (the
        Go runtime license).
    -   CI on Linux, macOS, and Windows. Every release archive is also
        run on its own platform (`scripts/smoke-release.sh`).
-   **Installation.** From the release archives, or with
    `go install github.com/sarvessveeriyah2312/AURA-Advanced_Unified_Runtime_Architecture/cmd/aura@latest`.
-   **License.** MIT.

### Hardening (Phase 5)

-   **Fixed: a crash on deeply nested source.** Nesting of roughly
    391,000 or more levels (parentheses, blocks, `else if` chains,
    operator chains, or unary operators) crashed the toolchain with a Go
    stack overflow and exit status 2. AURA now enforces a nesting limit
    of 10,000 levels with diagnostic `AURA-P014`.
-   **Fixed: tests assumed Unix error messages and wall-clock timing.**
    They would have failed on Windows CI. Tests are now OS-independent
    and deterministic.
-   **Added:** the `FuzzCompileAndRun` fuzz target for the compiler and
    VM.

### Notes

-   The name AURA is retained. It has not been cleared as a trademark,
    the project claims no trademark rights in it, and it is not
    affiliated with other products or projects named Aura.
-   AURA was developed under the codename NOVA. There is no
    compatibility with the pre-release NOVA names (`.nova`, `NOVA-`
    codes, the `nova` command).
-   Not included in 0.1.0: loops, collections, user-defined types,
    generics, function values, imports, I/O beyond `print`, saved
    bytecode files, and native compilation.

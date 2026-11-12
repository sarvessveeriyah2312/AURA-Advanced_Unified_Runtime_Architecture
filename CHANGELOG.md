# Changelog

All notable changes to AURA are documented in this file. The format is
based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and
AURA uses [Semantic Versioning](https://semver.org/).

## [0.1.0] - Unreleased

The first version of AURA (Advanced Unified Runtime Architecture): a
statically typed language with a bytecode compiler and a stack-based
virtual machine. It has not been released yet.

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

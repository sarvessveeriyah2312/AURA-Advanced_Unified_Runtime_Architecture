# Changelog

All notable changes to Tread are documented in this file. The format is
based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and
Tread uses [Semantic Versioning](https://semver.org/). Entries for 0.1.0
and 0.2.0 are kept as written, under the language's name at the time.

## [Unreleased]

Structs, methods and multi-file programs (ADR-0014).

### Added

-   **Structs.** `struct Point { x: Int, y: Int }` (fields separated by
    newlines or commas), built with `Point { x: 1, y: 2 }` (fields in
    any order), read with `p.x`, and changed with `p.x = v`, also inside
    collections and other structs (`line.to.x = 9`, `grid[0].x = 1`).
    Structs have value semantics like arrays and maps, print as
    `Point { x: 1, y: 2 }`, and compare with `==`. Generic structs
    (`struct Stack<T> { items: Array<T> }`) infer their type arguments.
-   **Methods** in `impl` blocks. `fn len(self)` reads its receiver;
    `fn shift(mut self, d: Int)` changes the caller's variable, which
    must be `let mut`. Functions without `self` are associated
    functions, called as `Point.origin()`.
-   **Imports.** `import geometry` loads `geometry.tr`, and
    `import shapes.circle` loads `shapes/circle.tr`, relative to the
    directory of the file being run. Names are used qualified:
    `geometry.Point`, `circle.area(r)`, `geometry.Shape.Dot(p)`. Each
    file is loaded once; import cycles are reported.
-   **`pub` visibility.** Only `pub` functions, structs, enums, fields
    and methods can be used from other files.
-   `tread build` embeds every source file in the `.tbc` file, so runtime
    errors and stack traces show the right file and line.
-   New errors: **TR-P015** (an import after other declarations) and
    **TR-T031** to **TR-T036** (struct literal fields, a struct that
    contains itself, a module name that does not match its file, a
    private item, an import cycle, a file that cannot be imported).

### Changed (breaking)

-   **`struct`, `impl`, `pub` and `import` are keywords.** `struct` and
    `impl` were ordinary identifiers before, so programs using them as
    names must rename them.
-   **Bytecode format 3.** `.tbc` files built by 0.5.0 must be rebuilt.
-   A struct literal cannot appear directly in the condition of `if`,
    `while`, `match` or `for`; wrap it in parentheses there.

## [0.5.0] - 2026-11-13

Arrays, maps, enums, pattern matching and generics (ADR-0013).

### Added

-   **Arrays.** `Array<T>`, with literals `[1, 2, 3]` and `[]`, indexing
    `xs[i]`, element assignment `xs[i] = v` (also nested, `grid[r][c] = v`),
    `for x in xs`, and the methods `len()`, `push(x)`, `pop()` and
    `contains(x)`. An index out of range is the new runtime error
    **TR-R007**.
-   **Maps.** `Map<K, V>` with `Int`, `String` or `Bool` keys, kept in
    insertion order. Literals `["a": 1]` and `[:]`, `m[k]`, `m[k] = v`,
    `for k in m` (keys), and `len()`, `get(k)`, `contains(k)`,
    `remove(k)`, `keys()`. A missing key in `m[k]` is the new runtime
    error **TR-R008**.
-   **Value semantics.** Assigning or passing an array or map gives an
    independent copy, so `let` values never change. Copies are made only
    when a shared value is changed (copy-on-write).
-   **Enums** with optional payloads (`Shape.Circle(1.0)`), declared with
    `enum`, and the built-in `Option<T>` (`Option.Some(x)`,
    `Option.None`), which `pop`, `get` and `remove` return.
-   **`match` statements** with variant, literal (`Int`, `String`,
    `Bool`), binding and `_` patterns. The compiler rejects a match that
    misses a case, naming an example, and an arm that can never run.
-   **Generics.** Generic functions (`fn first<T>(xs: Array<T>) ->
    Option<T>`) and generic enums, with type arguments inferred from the
    arguments and from the expected type.
-   Every value except `Unit` can now be printed, interpolated, and
    compared with `==` and `!=`; collections and enums compare by
    contents and print as written, for example `["a": 1]`.
-   New checker errors **TR-T020** to **TR-T030** (for example TR-T020,
    "cannot infer"; TR-T027, a match that misses a case).

### Changed (breaking)

-   **`enum` and `match` are keywords** (they were reserved, so no
    program used them).
-   **`.` is a token**, so `x.y` is member access instead of an invalid
    character.
-   **Bytecode format 2.** `.tbc` files built by 0.4.0 must be rebuilt;
    a 0.5.0 runtime rejects them with a "rebuild" message (TR-R006), and
    so does a 0.4.0 runtime given a format-2 file.
-   The help for assigning to an immutable variable now says "declare it
    with 'let mut' to allow changes".

## [0.4.0] - 2026-11-13

SDK and runtime editions, bytecode files, and installers (ADR-0011).
Loops: `while`, `for`, `break` and `continue` (ADR-0012).

### Added

-   **`while` loops.** `while cond { … }` repeats while a `Bool`
    condition holds.
-   **`for` loops** over a half-open `Int` range: `for i in a..b { … }`
    runs with `i` from `a` up to, but not including, `b`. Both bounds
    are evaluated once, `a` first. The loop variable is an immutable
    `Int`; assigning to it is `TR-T004`.
-   **`break` and `continue`** leave, or start the next iteration of,
    the innermost loop. Outside a loop they are the new error
    **TR-T019**.
-   **Return paths.** A function that returns a value may end with
    `while true { … }` when that loop has no `break` of its own. No
    other loop counts as returning (docs/12 §12.7).
-   Loops compile to the existing `JUMP` and `JUMP_IF_FALSE`
    instructions. There are no new opcodes, and the `.tbc` format
    version stays 1.

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
-   **Installers.** A `.msi` (Windows, per user), `.pkg` (macOS), `.deb`
    and `.rpm` (Linux) for the SDK, and user-level `install.sh` and
    `install.ps1` for either edition. They put `tread` on `PATH`, set
    `TREAD_HOME`, upgrade in place, and uninstall cleanly. None needs
    Go.
-   **Release automation.** `release.yml` builds and tests every
    artifact on its own platform and creates a draft GitHub Release with
    `SHA256SUMS` and `release-manifest.json`.

### Changed

-   **Release artifacts** are now `tread-sdk-<version>-<platform>` and
    `tread-runtime-<version>-<platform>` (`.tar.gz`, `.zip` on Windows),
    with `<platform>` one of `windows-x64`, `macos-arm64`, `macos-x64`,
    `linux-x64`, `linux-arm64`. Each unpacks to `bin/tread`, `docs/`
    and a `release` file.
-   `tread run` with neither a `.tr` nor a `.tbc` argument now says
    `expected the .tr or .tbc extension`.

### Changed (breaking)

-   **New keywords.** `while`, `for`, `in`, `break` and `continue` are
    keywords and can no longer be used as names. `for` and `while` were
    already reserved; `in`, `break` and `continue` are new.
-   **`..` is a token.** A number now stops before `..`, so `1..2` lexes
    as `1`, `..`, `2` instead of one malformed number (`TR-L008`).

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

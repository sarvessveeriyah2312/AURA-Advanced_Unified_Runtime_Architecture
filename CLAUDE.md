# Claude Code Project Instructions

You are implementing **Tread**, an experimental statically typed
programming language in Go: *a language for learning how languages work*.
Every stage (lexer, parser, checker, compiler, VM) should stay small
and readable on its own.

Identity (ADR-0010, from 0.3.0):

-   the CLI executable is `tread`;
-   source files use `.tr`;
-   diagnostic codes use the `TR-` prefix.

The language was renamed to Tread in 0.3.0, with no backward
compatibility. NOVA was the development codename. Earlier names appear
only in historical ADRs and in the CHANGELOG entries for 0.1.0 and 0.2.0;
do not edit those.

The Go module path still contains the old repository name until the
owner renames the GitHub repository; do not change it on your own.

## Product decisions

Read these documents before making architectural changes:

1.  `docs/01-vision-and-goals.md`
2.  `docs/02-language-specification.md`
3.  `docs/04-compiler-architecture.md`
4.  `docs/05-bytecode-and-runtime.md`
5.  `docs/10-implementation-plan.md`

If two documents conflict, stop and report the conflict. Do not silently
invent a third design. Propose an Architecture Decision Record (ADR) and
wait for approval when the change affects syntax, semantics, bytecode
compatibility, or public CLI behavior.

## Engineering rules

-   Use idiomatic, well-tested Go.
-   Prefer small packages with clear responsibilities.
-   Keep compiler phases separate: lexing, parsing, semantic analysis,
    code generation, VM execution.
-   Do not mix parsing, type checking, and execution in one function.
-   Use source spans for tokens and AST nodes so diagnostics can
    identify line and column.
-   Return structured errors/diagnostics; do not use panics for
    user-program errors.
-   Panics are acceptable only for violated internal invariants, and
    should be covered by tests.
-   Keep deterministic output for the same source input.
-   Avoid global mutable state.
-   Do not add dependencies without explaining why they are needed.
-   Keep the MVP scope. Do not implement deferred features early.
-   Add unit tests with every feature and regression tests for every
    bug.
-   Run `go test ./...` after each meaningful change.
-   Run `go vet ./...` before completing a milestone.
-   `scripts/verify.sh` is the full gate. It runs the same checks as CI
    (`.github/workflows/ci.yml`). Release archives are built by
    `scripts/build-release.sh` (ADR-0007, ADR-0008) and checked with
    `scripts/smoke-release.sh`.
-   Keep tests deterministic and OS-independent (`docs/08` §8.8).
-   Never crash on any input. Pathological nesting is bounded by
    ADR-0006 (TR-P014).
-   Record user-visible changes in `CHANGELOG.md`.
-   Update documentation and examples whenever syntax or behavior
    changes.
-   Never claim a feature is implemented unless it is implemented and
    tested.

## Required workflow for each task

1.  Inspect the current repository and relevant docs.
2.  Summarize the intended change and affected packages.
3.  Identify edge cases and tests.
4.  Implement the smallest coherent change.
5.  Run focused tests, then `go test ./...`.
6.  Report files changed, behavior added, tests run, and known
    limitations.

## Coding conventions

-   The module path is
    `github.com/sarvessveeriyah2312/AURA-Advanced_Unified_Runtime_Architecture`,
    chosen by the owner in Phase 5.1. Do not change it without the
    owner's approval.
-   Tread is MIT-licensed (`LICENSE`). Do not add a dependency whose
    license conflicts with MIT or that needs a notice, without updating
    `THIRD_PARTY_NOTICES.txt` in `scripts/build-release.sh`.
-   Use Go's standard library unless a dependency provides clear value.
-   Prefer explicit types and descriptive names.
-   Keep exported APIs documented.
-   Avoid premature optimization.
-   Do not generate large amounts of code before confirming package
    boundaries and interfaces.

## Current implementation boundary

Implement only the MVP defined in `docs/09-roadmap.md` (Phases 0 to 5).
Phase 6 (post-MVP language evolution) must not start before 0.1.0 is
released.

`docs/` is kept local: it is excluded from the GitHub repository by the
owner's choice (`.git/info/exclude`). Published files (`README.md`,
`CHANGELOG.md`) must not link into `docs/`; this file may, because it
guides local development. Treat all later features as design notes, not permission to
implement them now. Release status and owner prerequisites are in
`docs/14-release-checklist.md`. Owner-only release steps (commit, push,
tag, publish) are in `docs/17-owner-release-steps.md`; never perform
them without the owner's explicit authorization.

The name Tread was adopted in 0.3.0 (ADR-0010). It has not been checked
for trademarks. Never claim that the name is
cleared or owned.

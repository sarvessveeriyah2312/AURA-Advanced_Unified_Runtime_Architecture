# Claude Code Project Instructions

You are implementing **AURA** (Advanced Unified Runtime Architecture),
an experimental statically typed programming language in Go.

Identity (ADR-0005):

-   the CLI executable is `aura`;
-   source files use `.aura`;
-   diagnostic codes use the `AURA-` prefix.

NOVA was the development codename and appears only in historical ADR
text.

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
    (`.github/workflows/ci.yml`). Release builds use
    `scripts/build-release.sh` (ADR-0007).
-   Keep tests deterministic and OS-independent (`docs/08` §8.8).
-   Never crash on any input. Pathological nesting is bounded by
    ADR-0006 (AURA-P014).
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

-   Module path is initially `github.com/your-org/aura`; keep it
    configurable until the repository owner chooses the final
    organization.
-   Use Go's standard library unless a dependency provides clear value.
-   Prefer explicit types and descriptive names.
-   Keep exported APIs documented.
-   Avoid premature optimization.
-   Do not generate large amounts of code before confirming package
    boundaries and interfaces.

## Current implementation boundary

Implement only the MVP defined in `docs/09-roadmap.md` (Phases 0 to 5).
Phase 6 (post-MVP language evolution) must not start before 0.1.0 is
released. Treat all later features as design notes, not permission to
implement them now. Release status and owner prerequisites are in
`docs/14-release-checklist.md`.

# M1 implementation evidence

M1 began by promoting the accepted backend without changing the public API.

## Selected-backend promotion

- Ordinary conversion tests and benchmarks call `internal/backend/selected`.
- The selected facade delegates to `treesitter-go` v0.1.0 with the locked
  repository-owned Java 25 grammar tables.
- The selected-vs-baseline shape review and full repository gate passed.
- The rejected `gotreesitter` implementation and its transitive module graph
  were then removed from executable code.
- Canonical M0 report filenames now describe the selected backend. Baseline
  artifacts carry a `-gotreesitter` suffix and retain an exact historical
  provenance lock.
- The promoted `backend-shapes.json` is byte-for-byte identical to the
  previously reviewed `backend-shapes-java25.json`.

The selected generation lock now owns `cst-go`, runtime, grammar, generator,
patch, and generated-table provenance. No active package depends on the
historical baseline lock.

Typed Java declaration, statement, expression, pattern, and type wrappers are
an M2 deliverable. M1 establishes only the backend-neutral syntax/tree seams
needed for parsing, lexical fidelity, diagnostics, and later generated views.

## Stable diagnostic foundation

- Public diagnostic codes cover the required configuration, lexical, parser,
  feature, backend-limit, and resource-limit families.
- Diagnostics carry raw byte spans, severity, optional feature identity, and
  defensively owned notes.
- Java Unicode translation now emits the public `diagnostic.Diagnostic` value
  rather than its temporary M0-only structure.
- Translation diagnostics remain in deterministic raw-source order, and
  returned slices do not expose stored diagnostic or note slices.
- The syntax substrate uses `diagnostic.Code` as its generic stable code type.

## Public lossless parse seam

- The root `javacst` package exposes `Parse` and `ParseContext`; zero options
  resolve to stable Java 25.
- `syntax.Tree` owns exact source, resolved language level, translation,
  diagnostics, and selected runtime/grammar provenance around its immutable
  `cst-go` core.
- Lexical and parser recovery diagnostics are deterministically merged while
  syntax errors still return a usable tree and nil operational error.
- Invalid configuration and cancellation return a nil tree and wrapped Go
  error.
- Public-pipeline tests cover Java 25 patched syntax, malformed input, empty
  and trivia-only files, BOM/line-ending fidelity, Unicode escapes, invalid
  UTF-8, defensive diagnostics, provenance, concurrent reads, and bounded
  fuzz round trips.
- A 30-second public `ParseContext` fuzz lane completed 289,095 executions
  without a panic, operational failure, or round-trip mismatch.
- The complete gate passes 303 tests under the race detector plus formatting,
  vet, lint, and module-tidy checks.

## Per-parse resource limits

- Zero-valued limit fields resolve independently to bounded defaults: 16 MiB
  of raw source, 2,000,000 detached snapshot nodes, and a snapshot depth of
  4,096 (the root is depth zero).
- The raw-source bound is checked before translation and before invoking the
  selected runtime.
- Node and depth bounds are checked while copying the runtime tree into the
  repository-owned backend snapshot; the runtime does not expose equivalent
  parser-internal knobs.
- Runtime parsing and snapshot publication observe the caller's context.
- Limit failures publish no partial public tree and expose both
  `ErrLimitExceeded` and a typed `LimitError`.

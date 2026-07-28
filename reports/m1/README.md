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

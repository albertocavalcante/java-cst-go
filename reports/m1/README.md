# M1 implementation evidence

M1 began by promoting the accepted backend without changing the public API.

## Selected-backend promotion

- Ordinary conversion tests and benchmarks call `internal/backend/selected`.
- The selected facade delegates to `treesitter-go` v0.1.0 with the locked
  repository-owned Java 25 grammar tables.
- The rejected `gotreesitter` implementation remains reachable only through
  the explicit M0 `baseline` report selector and direct comparison tests.
- Canonical M0 report filenames now describe the selected backend. Baseline
  artifacts carry a `-gotreesitter` suffix.
- The promoted `backend-shapes.json` is byte-for-byte identical to the
  previously reviewed `backend-shapes-java25.json`.

This slice does not remove the baseline dependency. Removal follows only after
the selected golden migration and full repository gate are reviewed.

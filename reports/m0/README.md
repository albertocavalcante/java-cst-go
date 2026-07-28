# M0 reports

`results.json` contains per-fixture evidence for the selected backend and
repository-owned patched Java tables:

```sh
go run ./cmd/m0report \
  -runs 5 \
  -out reports/m0/results.json \
  -shapes-out reports/m0/backend-shapes.json
```

The rejected `gotreesitter` comparison remains available as the explicitly
named baseline:

```sh
go run ./cmd/m0report \
  -backend baseline \
  -runs 5 \
  -out reports/m0/results-gotreesitter.json \
  -shapes-out reports/m0/backend-shapes-gotreesitter.json
```

The command performs one warmup, then records the mean wall time and allocated
bytes across the requested serial repetitions. These measurements are
comparative spike evidence, not stable performance promises.

`backend-shapes.json` stores one full selected-backend-neutral tree per unique
fixture source. Each case in `results.json` carries the corresponding SHA-256,
so repeated release-level probes share one inspectable golden shape.

`decision.md` selects **replace**: replace the original runtime with
`treesitter-go`, retain Tree-sitter's grammar architecture, and own a bounded
pinned patch for Java 25 syntax.

No release-support claim is implied by a backend accepting a fixture.

`GRAMMAR-RUNTIME-AUDIT.md` sizes the two confirmed Java grammar gaps and
compares the pathological malformed-input reproducer against native
Tree-sitter, the pinned pure-Go runtime, its post-release HEAD, and the
replacement pure-Go runtime adapter. The generated patched grammar tables and
their full provenance are locked under `internal/grammar/java25`.

## Compiler oracle toolchains

The OpenJDK 21-26 compiler archives used for differential checks are pinned in
`internal/oracle/toolchains.lock.json`. Each entry records the canonical
archive URL, byte length, SHA-256, build, and extracted Java home.

Install one compiler into a caller-selected, non-system cache:

```sh
go run ./cmd/jdkoracle \
  -release 26 \
  -cache /path/with/sufficient-space/java-cst-go-oracle
```

The archive is fully downloaded and SHA-256 verified before extraction.
Temporary downloads and partial installations are never published as valid
cache entries. The JDK is not added to the system Java installation.

Run compiler-backed evidence for fixtures matching one locked release:

```sh
go run ./cmd/javacoracle \
  -release 26 \
  -cache /path/with/sufficient-space/java-cst-go-oracle \
  -out reports/m0/javac-results-java26.json
```

The runner uses `-XDrawDiagnostics`, disables annotation processing, compiles
into an isolated temporary directory, applies a per-case deadline, and
correlates each result with `results.json`. A feature accepted by the matching
compiler but represented with backend error nodes is classified as a confirmed
upstream Java grammar gap.

The canonical `javac-results-javaNN.json` files correlate the selected
`results.json`. Across Java 21-26, all 52 compiler expectations match: 35
cases are aligned, 17 require release-aware validation, and none retain a
grammar gap. The `javac-results-javaNN-gotreesitter.json` files preserve the
original baseline correlation.

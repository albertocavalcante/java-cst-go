# M0 grammar-gap and runtime audit

Status: **complete; replacement runtime and patched grammar pass M0**
Date: 2026-07-27

## Outcome

The two Java grammar gaps are small and localized. A 21-line candidate change
to `grammar.js` adds a distinct `module_import_declaration` node and permits one
explicit constructor invocation after a non-empty statement prologue. The
generated parser accepts both Java 25 probes and all 108 upstream corpus and
highlight tests.

The pinned pure-Go runtime is not viable for malformed editor buffers. The
five-byte malformed input `#0#.}` takes about 0.11 ms in native Tree-sitter,
but `gotreesitter` v0.47.0 spends about 1.76 seconds and cumulatively allocates
about 1.71 GB before returning a partial `memory_budget` tree. Current
`gotreesitter` HEAD does not improve this reproducer.

An adapter for `github.com/dcosson/treesitter-go` v0.1.0 passes the replacement
gate. Behind the same repository-owned snapshot contract it returns a complete
error tree for the reproducer in about 0.113 ms with about 605 KB allocated.
With repository-owned tables generated from the patched Java grammar, all 69
M0 fixtures have valid ranges, byte-exact converted CSTs, and zero backend
error nodes. The exact Java 21-26 compiler matrix classifies 35 cases as
aligned, 17 as release-aware validation requirements, and zero as grammar
gaps.

A 30-second translated parse-and-convert fuzz lane completed 121,367
executions without a panic, conversion failure, range failure, or round-trip
failure. Each fuzz parse retained the 250 ms operational deadline; this lane
does not prove that every malformed input completes without cancellation.

## Provenance

- `tree-sitter-java`: `e10607b45ff745f5f876bfa3e94fbcc6b44bdc11`
- upstream default branch at audit time: the same commit
- `gotreesitter` release: v0.47.0,
  `000ae6c6e921c02ab8c183fb31f3f16429b0a95d`
- `gotreesitter` post-release comparison:
  `f639fbaa2fab31246e2b99996817837ba90bd91d`
- alternative runtime: `github.com/dcosson/treesitter-go` v0.1.0,
  commit `cf63d32385984f5accbd17557c0e26fcd281f61a`, module sum
  `h1:E+tXGxZTJT7wPlAAANLMbejaNhreemIKiZgxj4oXV5I=`
- grammar generator/native comparison: Tree-sitter CLI v0.26.8,
  macOS/arm64 archive SHA-256
  `9dc0dc3415a1cd30499750579defbf3f8e000a98f12a65cda8e25981f07e7b0f`

The candidate source delta and generated Go tables are repository-owned and
locked under `internal/grammar/java25`. The lock records every upstream
commit, tool version, archive digest, source-patch digest, generated
`parser.c` digest, generated-Go digest, and deterministic post-processing
step.

## Grammar patch sizing

### Module imports

The upstream declaration choice contains ordinary import declarations only.
The candidate adds a separate rule:

```js
module_import_declaration: $ => seq(
  'import',
  'module',
  field('module', $._name),
  ';',
),
```

A distinct named node preserves the syntactic distinction needed by typed
views and release-aware validation. It avoids overloading the existing
ordinary/static import node.

### Flexible constructor bodies

The upstream constructor body permits an explicit constructor invocation only
before all statements. The candidate keeps that original path and adds one
alternative containing a non-empty statement prologue, one explicit
constructor invocation, and a statement epilogue.

The grammar intentionally does not encode all JLS restrictions on prologue
statements. Those restrictions vary across preview generations and belong in
the release-aware validation layer already demonstrated by the exact `javac`
matrix.

### Generation result

Using the verified CLI:

- grammar generation completed without a conflict;
- `import module java.base;` parsed as
  `module_import_declaration`;
- a field assignment before `super(...)` parsed as a statement followed by
  `explicit_constructor_invocation`;
- all 108 upstream corpus/highlight tests passed.

Generated `parser.c` churn is not patch complexity. The maintained source
delta is 21 lines in one grammar file plus focused corpus fixtures. The
repository lock verifies the checked-in Go table and records:

- patched `grammar.js` SHA-256
  `71731ca48b47dba3f9c9c72029106cf68be19d191c76cc9d423c2b9183c071a6`;
- generated `parser.c` SHA-256
  `710ddd0c02708ee3a93f954b3897150c98173a523a9ff3540eb648d246aca425`;
- generated `language.go` SHA-256
  `69f6512ed2af2741a40c4dd2e5962169c525eb3e666885a541a9cc3f2c4b31da`.

The v0.1.0 table generator emits an import of its own `internal/core`
package, which is illegal from this module. The locked generation recipe
rewrites that import to the runtime's public root facade and runs `gofmt`;
the facade re-exports the same table types. This is deterministic and covered
by the generated-file digest, but it should become an upstream generator
option before regeneration is routine.

## Runtime comparison

Apple M4, darwin/arm64, Go 1.26.3:

| Runtime/configuration | Time per parse | Allocated bytes | Allocations | Result |
|---|---:|---:|---:|---|
| Native Tree-sitter 0.26.8 | 0.11 ms | not measured | not measured | error tree |
| `gotreesitter` v0.47.0, default, five iterations | 1,758.2 ms | 1,736,476,054 | 12,588,221 | partial `memory_budget` tree |
| `gotreesitter` post-release HEAD, five iterations | 1,851.3 ms | 1,736,482,073 | 12,588,289 | partial `memory_budget` tree |
| `gotreesitter` v0.47.0, 8 MB global budget | 26.6 ms | 35,045,664 | 146,947 | partial `memory_budget` tree |
| `treesitter-go` v0.1.0 through M0 snapshot adapter, 20 iterations | 0.113 ms | 605,162 | 378 | complete error tree |

The release profile attributes most allocation volume to GLR reduction
candidate construction, GSS nodes, raw shapes, and raw-shape children. The
largest individual sites were:

- `cReductionCandidatesForAction`: about 636 MB;
- `gssScratch.allocNodeSlow`: about 374 MB;
- `nodeArena.allocRawShape`: about 281 MB;
- `nodeArena.allocRawShapeChildren`: about 190 MB.

The 8 MB setting is useful containment evidence, not a production fix:

- it is process-global environment configuration rather than a parser-local
  API;
- it still returns a stopped, partial tree instead of useful recovery
  structure;
- lowering one global budget can reject legitimate larger Java files;
- the native runtime proves the grammar/input pair itself is not intrinsically
  expensive.

Context cancellation remains a mandatory outer safety bound during the spike,
but a deadline that abandons tiny malformed editor buffers does not meet the
product recovery requirement.

## Decision impact

- `cst-go`: no blocker or enhancement was exposed by this audit.
- Java grammar: a small pinned fork is technically reasonable.
- `gotreesitter`: reject as the production editor-facing runtime.
- `treesitter-go`: selected replacement runtime; full-file M0 behavior is
  compatible with the backend-neutral adapter.
- M0 strategy: **replace** the runtime while retaining Tree-sitter's grammar
  architecture and maintaining the bounded pinned Java grammar delta.

The runtime module has no transitive module dependencies, is MIT licensed, and
keeps its implementation types behind the repository-owned backend boundary.
M0 proves full-file parsing and recovery, not the quality or cost of
incremental reparsing; incremental behavior remains an explicit M1 gate.

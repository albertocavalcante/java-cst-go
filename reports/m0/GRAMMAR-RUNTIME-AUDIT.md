# M0 grammar-gap and runtime audit

Status: **alternative runtime passes initial gate; patched grammar integration pending**
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

An adapter for `github.com/dcosson/treesitter-go` v0.1.0 now passes the initial
replacement gate. Behind the same repository-owned snapshot contract it
returns a complete error tree for the reproducer in about 0.113 ms with about
605 KB allocated. All 69 M0 fixtures have valid ranges and byte-exact converted
CSTs. Forty-one clean non-template fixture cases have byte-for-byte identical
backend snapshots; all 21 existing recovery cases remain recovery cases.

The remaining seven clean cases are the same string-template source at
different language levels. Their structural difference is an improvement:
the alternative runtime and native Tree-sitter produce
`string_interpolation`, while the pinned runtime's handwritten Java token
source produces an ordinary `escape_sequence`.

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

The candidate source delta was tested in an isolated checkout. It is evidence,
not yet a production grammar dependency.

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
delta is 21 lines in one grammar file plus focused corpus fixtures. Production
integration must regenerate the blob with one repository-locked generator and
verify the resulting digest.

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
- `treesitter-go`: leading replacement candidate; full-file M0 behavior is
  compatible with the backend-neutral adapter.
- M0 strategy: likely **replace** the runtime while retaining and patching the
  Java grammar, pending generated-table and sustained-fuzz evidence.

The next decision tranche is:

1. generate repository-owned Java tables from the bounded grammar delta with
   a locked generator and verify their digest;
2. rerun all fixture, conversion, exact-compiler, fuzz, and benchmark gates
   through the alternative adapter;
3. audit the v0.1.0 dependency surface and incremental claims without exposing
   its types publicly;
4. write the M0 decision, expected to select **replace** for the runtime and a
   small pinned Java grammar fork unless the patched-table gate fails.

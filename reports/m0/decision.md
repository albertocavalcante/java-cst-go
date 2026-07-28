# M0 backend decision

Status: **accepted**
Date: 2026-07-27
Strategy: **replace**

## Decision

Replace `github.com/odvcencio/gotreesitter` as the Java parser runtime with
`github.com/dcosson/treesitter-go` v0.1.0. Retain Tree-sitter's grammar
architecture and maintain a small, repository-owned patch to the pinned
`tree-sitter-java` grammar for Java 25 module imports and flexible constructor
bodies.

Keep the runtime and generated grammar types behind the internal backend
snapshot boundary. The public CST remains the lossless `cst-go` token/node
model, with release-aware Java behavior implemented above the parser.

This is classified as **replace**, rather than fork or hybridize, because the
material M0 decision is to replace the unsafe original runtime. The bounded
grammar patch is an implementation obligation of the replacement, not a
second production parser.

## Evidence

- All 69 M0 fixtures parse through the selected backend with valid spans,
  byte-exact CST round trips, and zero backend error nodes.
- All 52 exact `javac` expectations for Java 21-26 match. Thirty-five cases
  are parser/compiler aligned, 17 correctly remain post-parse validation
  requirements, and no confirmed grammar gap remains.
- The five-byte malformed source `#0#.}` produces a complete error tree in
  about 0.113 ms with about 605 KB allocated. The rejected runtime needs about
  1.76 seconds and cumulatively allocates about 1.71 GB before returning a
  partial budget-stopped tree.
- The 21-line grammar-source delta generates without conflicts and passes all
  108 upstream corpus and highlight tests.
- A 30-second parse/convert fuzz lane completed 121,367 executions without a
  panic, conversion failure, range failure, or round-trip failure.
- The selected runtime has no transitive Go module dependencies, is MIT
  licensed, and does not leak implementation types through the public API.
- The checked-in generated table is reproducibly identified by locked runtime,
  grammar, generator, archive, patch, intermediate-parser, and final-Go
  digests.

No `cst-go` enhancement is required for the parser integration. Its existing
lossless token/node representation and invariant checks are sufficient.

## Rejected alternatives

### Keep

Rejected. A tiny malformed editor buffer can consume seconds and gigabytes in
the original runtime. A global memory budget limits exposure but still returns
a partial tree and can reject legitimate files.

### Fork the original runtime

Rejected for M1. The allocation profile points to several interacting GLR and
graph-structured-stack hot paths, making a safe fork substantially larger than
the Java CST work. The alternative runtime already meets the full-file M0
contract without this repair.

### Hybridize two runtimes

Rejected. Routing malformed inputs to a fallback would duplicate parser state,
recovery behavior, tests, and operational policy. The replacement runtime
handles both clean and malformed fixtures through one backend.

## Accepted risks and controls

- `treesitter-go` is an early v0.1.0 dependency. Its commit and module sum are
  pinned, its types remain internal, and the repository owns golden shape and
  conversion tests.
- M0 proves full-file parsing, not incremental edit quality. M1 must measure
  incremental correctness, reuse, latency, memory, cancellation, and
  adversarial edits before an editor-facing API promises incremental behavior.
- The table generator currently emits an illegal downstream
  `internal/core` import. The locked recipe performs one deterministic rewrite
  to the runtime's public facade before `gofmt`, and the final digest detects
  drift. A public-import generator option should be contributed upstream before
  routine regeneration.
- The Java grammar patch is intentionally syntactic. Seventeen current cases
  still need version- and preview-aware semantic validation with stable
  diagnostics.
- Java 12-20 preview compilers are not yet in the local exact-compiler matrix.
  Those historical checks remain a CI/toolchain expansion, not a blocker for
  the Java 21-26 M0 choice.

## M1 entry plan

1. Make the selected backend the internal default and migrate golden shapes;
   retain the old backend only until the migration diff is reviewed.
2. Implement release- and preview-aware feature validation with stable
   diagnostics for the 17 demonstrated validation cases.
3. Add typed Java syntax views over the lossless `cst-go` tree without leaking
   runtime nodes.
4. Gate incremental parsing with representative files, adversarial edit
   sequences, cancellation, reuse, latency, and allocation measurements.
5. Automate grammar regeneration from the lock and pursue upstream grammar and
   generator improvements as separately authorized external changes.
6. Extend the exact compiler lane to historical Java 12-20 preview releases.

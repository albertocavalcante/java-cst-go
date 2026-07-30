# M2 implementation evidence

M2 builds an independently useful Java 8 structural API over the lossless M1
tree. It does not create a second AST: generated wrappers retain the underlying
red node's tree identity, occurrence ID, spans, tokens, and recovery syntax.

## M2.0 schema and declaration-view foundation

- `schema/java-syntax.json` is the checked-in source for the initial typed
  surface.
- The project-specific generator is library-first: `internal/syntaxgen`
  validates and renders the schema, while `cmd/syntaxgen` only owns flags and
  atomic file publication.
- Checked-in generation provides repository-owned kind constants plus typed
  node and union views.
- Sequence accessors use `iter.Seq`; allocating conveniences carry an explicit
  `...Slice` suffix.
- Optional accessors return `(value, bool)` and safely tolerate missing or
  malformed children.
- `syntax.Token` now consistently means a positioned red token;
  `syntax.GreenToken` names immutable position-free token storage.

The first vertical slice covers:

- compilation units, package declarations, and imports;
- class, interface, enum, and annotation-type declarations;
- class/interface/annotation members and enum constants/declarations;
- fields, constants, methods, constructors, static initializers, and nested
  types;
- variable declarators, formal/varargs/receiver parameters, blocks, and
  constructor bodies;
- direct identifier names with occurrence-preserving positioned tokens.

A representative Java 8 fixture exercises all four top-level type forms,
multiple imports and declarators, constructors, methods, parameters, nested
types, enum bodies, and interface/annotation members. Recovery coverage calls
the same generated accessors on malformed input and asserts byte-identical
round trip.

The complete repository gate passes 379 tests under the race detector plus
formatting, vet, lint, generation-drift, and module-tidy checks.

## Boundary review

This slice requires no `cst-go` enhancement. Direct-child kind selection and
source-order traversal are sufficient for these grammar fields, and the typed
views preserve the shared core's existing occurrence identity. Java-specific
schema policy remains in `java-cst-go`.

M2 is not yet complete and no Java release is advertised as structurally
supported. Remaining slices close the full stable kind table, add types,
statements, and expressions, expand recovery/differential fixtures, and run
the Java 8 corpus exit gate.

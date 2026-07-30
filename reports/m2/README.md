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

## M2.1 stable kinds, names, modifiers, and Java 8 types

The schema now closes the selected grammar's repository-owned kind catalog:
327 stable identities comprising 185 node kinds and 142 token kinds. Generated
lookups translate grammar spelling to those identities; the converter rejects
unknown backend kinds instead of silently publishing an open-ended public
kind. Stable values retain `node:`/`token:` string identities and never adopt
backend numeric symbol IDs.

The extra token identity records `multiline_string_fragment`, a grammar
nonterminal whose invisible children are collapsed by the selected runtime so
that it is published as a source-bearing snapshot leaf.

Kind synchronization is append-only. Existing `(category, spelling, Go name)`
triples keep their order, and `Schema.ValidateCompatible` rejects a shortened,
reordered, or renamed kind table, remapped node kinds, narrowed unions, removed
views, and changed accessor signatures.

The typed surface additionally covers:

- qualified package/import names, static imports, and wildcard imports;
- annotation uses and declaration/parameter modifiers;
- class and method type parameters, intersection bounds, superclass and
  interface clauses;
- primitive, scoped, generic, array, annotated, and wildcard types;
- method return types, formal/varargs parameter types, and throws clauses.

Mixed node/token element unions model grammar categories such as `Type`,
`ReturnType`, `TypeArgument`, `Name`, and `Modifier` while preserving the
underlying red element's identity, spans, and text. A Java 8 source-shape
golden covers nested generic/wildcard types, arrays, type bounds, modifiers,
imports, parameters, returns, and throws. Converter tests cover an
unregistered backend kind, and all typed views remain recovery-safe.

The complete repository gate now passes 391 tests under the race detector plus
formatting, vet, zero-issue lint, generation-drift, and both module-tidy checks.

## Boundary review

This slice requires no `cst-go` enhancement. Direct-child kind selection and
source-order traversal are sufficient for these grammar fields, and the typed
views preserve the shared core's existing occurrence identity. Java-specific
schema policy remains in `java-cst-go`.

M2 is not yet complete and no Java release is advertised as structurally
supported. Remaining slices add statements and expressions, expand
recovery/differential fixtures, and run the Java 8 corpus exit gate.

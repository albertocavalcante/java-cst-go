# java-cst-go

> Experimental pure-Go, lossless, version-aware Java concrete syntax trees.

`java-cst-go` completed its M0 backend/versioning spike and the
implementation-dependent M1 parser-foundation work. The selected internal
parser uses the pure-Go `treesitter-go` runtime with repository-owned patched
Java grammar tables, then converts its backend-neutral snapshot into immutable
`cst-go` green/red trees. M1 is complete against the signed `cst-go` v0.3.0
release. M2 is underway: its first generated typed views cover Java 8
compilation units, top-level types, class/interface/enum members, parameters,
and declaration names without constructing a second AST.

The spike targets Java release modeling from 8 through 26, including exact
Java 21-26 preview boundaries. It does not yet advertise any Java release as
supported, and its syntax API is not frozen.

The normative design and executable spike plan live in the sibling
[`jvm-cst-plan`](../jvm-cst-plan/) workspace.

## Usage

```go
tree, err := javacst.Parse(source, javacst.Options{
	Level: language.Level{Release: language.Release25},
	Limits: javacst.Limits{
		MaxSourceBytes: 2 << 20,
	},
})
if err != nil {
	// Invalid configuration, cancellation, a resource limit, or an internal
	// invariant failure.
}

exactSource := tree.Text()
root := tree.Root()
diagnostics := tree.Diagnostics()
```

Typed views retain the identity and spans of the underlying red nodes:

```go
unit, ok := ast.AsCompilationUnit(tree.Root())
if !ok {
	// The root is not a Java compilation unit.
}
for declaration := range unit.Types() {
	if class, ok := declaration.AsClassDeclaration(); ok {
		name, present := class.Name()
		_ = name
		_ = present
	}
}
```

Java syntax, lexical, and release/preview feature errors are attached to a
usable tree. They do not become operational Go errors. A zero `Options` value
selects stable Java 25 and bounded defaults of 16 MiB of raw source,
2,000,000 snapshot nodes, and
a maximum snapshot depth of 4,096 (the root is depth zero). Source size is
checked before translation and runtime parsing;
node/depth bounds apply while publishing the detached CST. Use
`errors.Is(err, javacst.ErrLimitExceeded)` and `errors.As` to
`*javacst.LimitError` to inspect limit failures.

## Development

```sh
just generate
just check
```

`schema/java-syntax.json` is the source of generated syntax kinds and typed
views. The test gate rejects stale generated files.

The module is library-first. Reporting commands are thin measurement drivers;
backend implementation types and selection remain internal.

# java-cst-go

> Experimental pure-Go, lossless, version-aware Java concrete syntax trees.

`java-cst-go` completed its M0 backend and versioning spike and is entering M1
syntax-kernel and lexical-fidelity work. The selected internal parser uses the
pure-Go `treesitter-go` runtime with repository-owned patched Java grammar
tables, then converts its backend-neutral snapshot into immutable `cst-go`
green/red trees.

The spike targets Java release modeling from 8 through 26, including exact
Java 21-26 preview boundaries. It does not yet advertise any Java release as
supported, and its syntax API is not frozen.

The normative design and executable spike plan live in the sibling
[`jvm-cst-plan`](../jvm-cst-plan/) workspace.

## Usage

```go
tree, err := javacst.Parse(source, javacst.Options{
	Level: language.Level{Release: language.Release25},
})
if err != nil {
	// Invalid configuration, cancellation, a resource limit, or an internal
	// invariant failure.
}

exactSource := tree.Text()
root := tree.Root()
diagnostics := tree.Diagnostics()
```

Java syntax and lexical errors are attached to a usable tree. They do not
become operational Go errors. A zero `Options` value selects stable Java 25.

## Development

```sh
just check
```

The module is library-first. Reporting commands are thin measurement drivers;
backend implementation types and selection remain internal.

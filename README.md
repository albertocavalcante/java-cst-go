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

## Development

```sh
just check
```

The module is library-first. Reporting commands are thin measurement drivers;
backend implementation types and selection remain internal.

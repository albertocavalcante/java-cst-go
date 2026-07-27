# java-cst-go

> Experimental pure-Go, lossless, version-aware Java concrete syntax trees.

`java-cst-go` is currently an M0 backend and versioning spike. It is measuring
whether a pinned pure-Go tree-sitter runtime and Java grammar can be converted
into immutable `cst-go` green/red trees without losing source bytes, trivia,
punctuation, recovery structure, or occurrence identity.

The spike targets Java release modeling from 8 through 26, including exact
Java 21-26 preview boundaries. It does not yet advertise any Java release as
supported, and its syntax API is not frozen.

The normative design and executable spike plan live in the sibling
[`jvm-cst-plan`](../jvm-cst-plan/) workspace.

## Development

```sh
just check
```

The module is library-first. Any M0 reporting command is a thin measurement
driver; backend implementation types remain internal.

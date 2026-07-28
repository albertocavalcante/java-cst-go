# M0 `javac` oracle evidence

Status: **Java 21-26 minimum matrix complete for baseline and patched backend**
Compiler platform: macOS/AArch64

The exact OpenJDK archives, sizes, builds, and SHA-256 values are locked in
`internal/oracle/toolchains.lock.json`. Each compiler archive was verified
before extraction into a non-system cache.

| Release | Compiler cases | Expected results matched | Aligned | Confirmed grammar gaps | Validation required |
|---:|---:|---:|---:|---:|---:|
| 21 | 7 | 7 | 5 | 0 | 2 |
| 22 | 9 | 9 | 3 | 1 | 5 |
| 23 | 10 | 10 | 3 | 3 | 4 |
| 24 | 9 | 9 | 3 | 4 | 2 |
| 25 | 9 | 9 | 4 | 4 | 1 |
| 26 | 8 | 8 | 3 | 2 | 3 |
| **Total** | **52** | **52** | **21** | **14** | **17** |

The repository-owned patched Java 25 grammar resolves both feature-level
grammar gaps:

| Release | Compiler cases | Expected results matched | Aligned | Confirmed grammar gaps | Validation required |
|---:|---:|---:|---:|---:|---:|
| 21 | 7 | 7 | 5 | 0 | 2 |
| 22 | 9 | 9 | 4 | 0 | 5 |
| 23 | 10 | 10 | 6 | 0 | 4 |
| 24 | 9 | 9 | 7 | 0 | 2 |
| 25 | 9 | 9 | 8 | 0 | 1 |
| 26 | 8 | 8 | 5 | 0 | 3 |
| **Total** | **52** | **52** | **35** | **0** | **17** |

The second matrix is now canonical and was generated from `results.json`,
using the same verified compiler installations and fixture expectations as
the explicitly named `results-gotreesitter.json` baseline.
The 14 former fixture-level grammar gaps become aligned without changing the
17 cases that intentionally require release-aware validation.

## Baseline backend grammar gaps

The matching compiler accepts these enabled features while the pinned backend
produces an error node:

- flexible constructor bodies: Java 22 P1, 23 P2, 24 P3, and Java 25-26
  final, including field initialization before `super(...)` where allowed;
- module import declarations: Java 23 P1, 24 P2, and Java 25-26 final,
  including the P2 package-on-demand shadowing rule where allowed.

These are fourteen fixture-level confirmations of two feature-level gaps.

## Release-aware validation

Seventeen Java 21-26 cases require repository-owned validation. They cover
unavailable syntax, preview features compiled without `--enable-preview`, and
withdrawn string templates. They also demonstrate variant-specific semantic
rejections: Java 22 forbids field access before `super(...)`, Java 23 reports
an ambiguous module-import/package-import type, and Java 26 rejects a primitive
pattern switch under its tighter P4 dominance rule. Backend acceptance or
generic recovery is not a substitute for the stable feature diagnostics
required by the specification.

## Limits of this evidence

- The current source probes distinguish three material preview-generation
  changes but not every grammar or semantic change between preview rounds.
- Historical preview compilers for Java 12-20 are not locked or executed yet.
- `javac` is the language acceptance oracle; it does not define the desired CST
  shape or recovery tree.
- The malformed-input runtime pathology remains separate backend-decision
  evidence.

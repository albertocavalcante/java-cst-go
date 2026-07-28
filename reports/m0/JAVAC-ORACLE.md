# M0 `javac` oracle evidence

Status: **Java 21-26 minimum matrix complete**
Compiler platform: macOS/AArch64

The exact OpenJDK archives, sizes, builds, and SHA-256 values are locked in
`internal/oracle/toolchains.lock.json`. Each compiler archive was verified
before extraction into a non-system cache.

| Release | Compiler cases | Expected results matched | Aligned | Confirmed grammar gaps | Validation required |
|---:|---:|---:|---:|---:|---:|
| 21 | 7 | 7 | 5 | 0 | 2 |
| 22 | 8 | 8 | 3 | 1 | 4 |
| 23 | 8 | 8 | 3 | 2 | 3 |
| 24 | 7 | 7 | 3 | 2 | 2 |
| 25 | 6 | 6 | 3 | 2 | 1 |
| 26 | 7 | 7 | 3 | 2 | 2 |
| **Total** | **43** | **43** | **20** | **9** | **14** |

## Confirmed backend grammar gaps

The matching compiler accepts these enabled features while the pinned backend
produces an error node:

- flexible constructor bodies: Java 22 P1, 23 P2, 24 P3, and Java 25-26 final;
- module import declarations: Java 23 P1, 24 P2, and Java 25-26 final.

These are nine fixture-level confirmations of two feature-level gaps.

## Release-aware validation

Fourteen Java 21-26 cases require repository-owned validation. They cover
unavailable syntax, preview features compiled without `--enable-preview`, and
withdrawn string templates. Backend acceptance or generic recovery is not a
substitute for the stable feature diagnostics required by the specification.

## Limits of this evidence

- The current source probes establish compiler acceptance/rejection but do not
  yet distinguish every grammar change between preview generations.
- Historical preview compilers for Java 12-20 are not locked or executed yet.
- `javac` is the language acceptance oracle; it does not define the desired CST
  shape or recovery tree.
- The malformed-input runtime pathology remains separate backend-decision
  evidence.

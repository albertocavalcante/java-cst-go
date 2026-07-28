// Package selected exposes the repository's current internal Java parser
// backend without leaking its implementation to callers.
//
// Backend changes are made here after evidence review. Ordinary parser and
// conversion code should import this package; comparison tests and evidence
// tools may import a concrete backend explicitly.
package selected

package syntax

import (
	"errors"
	"fmt"

	"git.alberto.engineer/alberto/java-cst-go/diagnostic"
	"git.alberto.engineer/alberto/java-cst-go/language"
	"git.alberto.engineer/alberto/java-cst-go/source"
)

// Provenance identifies the library, grammar, and backend that built a tree.
type Provenance struct {
	LibraryVersion  string
	GrammarRevision string
	Backend         string
}

// Tree is an immutable Java syntax tree with its parse configuration and
// source provenance.
type Tree struct {
	core        *CoreTree
	text        string
	level       language.Level
	translation *source.Translation
	diagnostics []diagnostic.Diagnostic
	provenance  Provenance
}

// NewTree wraps one checked lossless core tree with Java parse metadata.
//
// This constructor primarily serves parser integrations. Normal callers
// should use javacst.Parse or javacst.ParseContext.
func NewTree(
	core *CoreTree,
	text string,
	level language.Level,
	translation *source.Translation,
	diagnostics []diagnostic.Diagnostic,
	provenance Provenance,
) (*Tree, error) {
	if core == nil {
		return nil, errors.New("construct Java syntax tree: core tree is nil")
	}
	if !level.Valid() {
		return nil, errors.New("construct Java syntax tree: invalid language level")
	}
	if translation == nil {
		return nil, errors.New("construct Java syntax tree: translation is nil")
	}
	if translation.Raw() != text {
		return nil, errors.New("construct Java syntax tree: translation raw source differs")
	}
	if got := core.Root().AppendText(); got != text {
		return nil, fmt.Errorf(
			"construct Java syntax tree: core text is %d bytes, source is %d bytes",
			len(got),
			len(text),
		)
	}
	if provenance.LibraryVersion == "" ||
		provenance.GrammarRevision == "" ||
		provenance.Backend == "" {
		return nil, errors.New("construct Java syntax tree: provenance is incomplete")
	}

	return &Tree{
		core:        core,
		text:        text,
		level:       level,
		translation: translation,
		diagnostics: diagnostic.Normalize(diagnostics),
		provenance:  provenance,
	}, nil
}

// Root returns the positioned root node.
func (t *Tree) Root() Node {
	if t == nil || t.core == nil {
		return Node{}
	}
	return t.core.Root()
}

// Text returns the exact raw source supplied to the parser.
func (t *Tree) Text() string {
	if t == nil {
		return ""
	}
	return t.text
}

// Level returns the resolved Java language level used to parse the tree.
func (t *Tree) Level() language.Level {
	if t == nil {
		return language.Level{}
	}
	return t.level
}

// Diagnostics returns a defensive copy in deterministic source order.
func (t *Tree) Diagnostics() []diagnostic.Diagnostic {
	if t == nil {
		return nil
	}
	return cloneDiagnostics(t.diagnostics)
}

// Provenance returns parser implementation identity for bug reports.
func (t *Tree) Provenance() Provenance {
	if t == nil {
		return Provenance{}
	}
	return t.provenance
}

func cloneDiagnostics(input []diagnostic.Diagnostic) []diagnostic.Diagnostic {
	result := make([]diagnostic.Diagnostic, len(input))
	for index := range input {
		result[index] = input[index].Clone()
	}
	return result
}

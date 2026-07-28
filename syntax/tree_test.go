package syntax_test

import (
	"testing"

	cst "github.com/albertocavalcante/cst-go"

	"git.alberto.engineer/alberto/java-cst-go/diagnostic"
	"git.alberto.engineer/alberto/java-cst-go/language"
	"git.alberto.engineer/alberto/java-cst-go/source"
	"git.alberto.engineer/alberto/java-cst-go/syntax"
)

func TestNewTreeOwnsDiagnosticsAndChecksMetadata(t *testing.T) {
	t.Parallel()

	token, err := cst.NewToken(cst.TokenSpec[
		syntax.Kind,
		syntax.TriviaKind,
		syntax.TokenData,
	]{
		Kind: syntax.TokenKind("identifier"),
		Text: "A",
		Data: syntax.TokenData{LogicalText: "A"},
	})
	if err != nil {
		t.Fatalf("NewToken: %v", err)
	}
	element, err := cst.TokenElement(token)
	if err != nil {
		t.Fatalf("TokenElement: %v", err)
	}
	root, err := cst.NewGreenNode(
		syntax.NodeKind("program"),
		[]syntax.GreenElement{element},
	)
	if err != nil {
		t.Fatalf("NewGreenNode: %v", err)
	}
	core := cst.NewTree[
		syntax.Kind,
		syntax.TriviaKind,
		syntax.TokenData,
		syntax.DiagnosticCode,
	](root)
	inputDiagnostics := []diagnostic.Diagnostic{diagnostic.New(
		diagnostic.CodePreviewDisabled,
		diagnostic.SeverityError,
		diagnostic.Span{},
		"preview disabled",
		language.FeatureStringTemplates,
		[]diagnostic.Note{{Message: "enable preview"}},
	)}
	tree, err := syntax.NewTree(
		core,
		"A",
		language.Level{Release: language.Release25},
		source.Translate("A"),
		inputDiagnostics,
		syntax.Provenance{
			LibraryVersion:  "test",
			GrammarRevision: "grammar",
			Backend:         "backend",
		},
	)
	if err != nil {
		t.Fatalf("NewTree: %v", err)
	}

	inputDiagnostics[0].Notes[0].Message = "input changed"
	first := tree.Diagnostics()
	first[0].Notes[0].Message = "result changed"
	if got, want := tree.Diagnostics()[0].Notes[0].Message, "enable preview"; got != want {
		t.Fatalf("stored note = %q, want %q", got, want)
	}
	if got, want := tree.Text(), "A"; got != want {
		t.Fatalf("Text() = %q, want %q", got, want)
	}
}

func TestNewTreeRejectsInconsistentInputs(t *testing.T) {
	t.Parallel()

	validProvenance := syntax.Provenance{
		LibraryVersion:  "test",
		GrammarRevision: "grammar",
		Backend:         "backend",
	}
	if tree, err := syntax.NewTree(
		nil,
		"",
		language.Level{Release: language.Release25},
		source.Translate(""),
		nil,
		validProvenance,
	); err == nil || tree != nil {
		t.Fatalf("NewTree nil core = (%v, %v), want nil and error", tree, err)
	}
}

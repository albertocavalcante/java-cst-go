package treesitter_test

import (
	"context"
	"testing"

	"git.alberto.engineer/alberto/java-cst-go/internal/backend/treesitter"
	"git.alberto.engineer/alberto/java-cst-go/language"
	"git.alberto.engineer/alberto/java-cst-go/source"
)

func TestUnicodeTranslationChangesBackendTokenization(t *testing.T) {
	t.Parallel()

	raw := `class \u0041 { int value\u003b }`
	translation := source.Translate(raw)
	if diagnostics := translation.Diagnostics(); len(diagnostics) != 0 {
		t.Fatalf("translation diagnostics: %+v", diagnostics)
	}

	level := language.Level{Release: language.Release21}
	rawResult, err := treesitter.Parse(context.Background(), []byte(raw), level)
	if err != nil {
		t.Fatalf("parse raw source: %v", err)
	}
	logicalResult, err := treesitter.Parse(
		context.Background(),
		[]byte(translation.Logical()),
		level,
	)
	if err != nil {
		t.Fatalf("parse logical source: %v", err)
	}

	if rawResult.ErrorCount == 0 {
		t.Fatal("raw backend parse unexpectedly has no errors")
	}
	if logicalResult.ErrorCount != 0 || logicalResult.Root.HasError {
		t.Fatalf(
			"logical backend parse errors=%d root_has_error=%t",
			logicalResult.ErrorCount,
			logicalResult.Root.HasError,
		)
	}
}

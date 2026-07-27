package grammar_test

import (
	"testing"

	"github.com/odvcencio/gotreesitter/grammars"

	"git.alberto.engineer/alberto/java-cst-go/internal/grammar"
)

func TestPinnedJavaGrammarBlob(t *testing.T) {
	t.Parallel()

	lock, err := grammar.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	blob := grammars.BlobByName("java")
	if len(blob) == 0 {
		t.Fatal("gotreesitter returned no embedded Java grammar blob")
	}
	if err := lock.VerifyBlob(blob); err != nil {
		t.Fatal(err)
	}
}

func TestWrongJavaGrammarDigestFails(t *testing.T) {
	t.Parallel()

	lock, err := grammar.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	lock.Grammar.BlobSHA256 = "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"

	if err := lock.VerifyBlob(grammars.BlobByName("java")); err == nil {
		t.Fatal("VerifyBlob with deliberately wrong digest returned nil")
	}
}

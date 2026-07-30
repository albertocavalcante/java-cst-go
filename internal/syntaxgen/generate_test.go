package syntaxgen_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"git.alberto.engineer/alberto/java-cst-go/internal/syntaxgen"
)

func TestCheckedInGenerationIsCurrent(t *testing.T) {
	t.Parallel()

	root := filepath.Join("..", "..")
	schema, err := syntaxgen.Load(filepath.Join(root, "schema", "java-syntax.json"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	wantSyntax, wantAST, err := syntaxgen.Generate(schema)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	assertFile(t, filepath.Join(root, "syntax", "kind_gen.go"), wantSyntax)
	assertFile(t, filepath.Join(root, "ast", "nodes_gen.go"), wantAST)
}

func TestSchemaRejectsInvalidReferences(t *testing.T) {
	t.Parallel()

	schema := syntaxgen.Schema{
		SchemaVersion: 1,
		Nodes: []syntaxgen.Node{{
			Type: "CompilationUnit",
			Kind: "program",
		}},
		Accessors: []syntaxgen.Accessor{{
			Owner:       "CompilationUnit",
			Name:        "Missing",
			Target:      "MissingNode",
			Cardinality: "optional",
		}},
	}
	if err := schema.Validate(); err == nil {
		t.Fatal("Validate() = nil, want unknown target error")
	}
}

func assertFile(t *testing.T, path string, want []byte) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q): %v", path, err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("%s is stale; run go generate ./ast", path)
	}
}

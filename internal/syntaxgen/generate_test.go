package syntaxgen_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"git.alberto.engineer/alberto/java-cst-go/internal/grammar/java25"
	"git.alberto.engineer/alberto/java-cst-go/internal/syntaxgen"
	"git.alberto.engineer/alberto/java-cst-go/syntax"
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
		Kinds: []syntaxgen.Kind{{
			Element: "node",
			Name:    "program",
			GoName:  "Program",
		}},
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

func TestStableKindsCoverSelectedGrammar(t *testing.T) {
	t.Parallel()

	grammar := java25.JavaLanguage()
	for index, name := range grammar.SymbolNames {
		switch {
		case uint32(index) < grammar.TokenCount:
			kind, ok := syntax.LookupTokenKind(name)
			if !ok || kind.Category() != syntax.KindCategoryToken {
				t.Errorf("token symbol %d %q = (%q, %v)", index, name, kind, ok)
			}
		case uint32(index) < grammar.SymbolCount:
			kind, ok := syntax.LookupNodeKind(name)
			if !ok || kind.Category() != syntax.KindCategoryNode {
				t.Errorf("node symbol %d %q = (%q, %v)", index, name, kind, ok)
			}
		default:
			if _, ok := syntax.LookupNodeKind(name); !ok {
				t.Errorf("alias node symbol %d %q is not registered", index, name)
			}
			if _, ok := syntax.LookupTokenKind(name); !ok {
				t.Errorf("alias token symbol %d %q is not registered", index, name)
			}
		}
	}
	for _, value := range []struct {
		element string
		name    string
	}{
		{element: "node", name: "ERROR"},
		{element: "token", name: "ERROR"},
		{element: "token", name: "eof"},
	} {
		var ok bool
		if value.element == "node" {
			_, ok = syntax.LookupNodeKind(value.name)
		} else {
			_, ok = syntax.LookupTokenKind(value.name)
		}
		if !ok {
			t.Errorf("%s:%s is not registered", value.element, value.name)
		}
	}
	for _, name := range java25.CollapsedLeafKinds() {
		kind, ok := syntax.LookupTokenKind(name)
		if !ok || kind.Category() != syntax.KindCategoryToken {
			t.Errorf("collapsed leaf %q = (%q, %v)", name, kind, ok)
		}
	}
	if syntax.NodeKind("not_a_java_kind") != syntax.KindUnknown {
		t.Fatal("NodeKind(unknown) did not return KindUnknown")
	}
	if syntax.TokenKind("not_a_java_kind") != syntax.KindUnknown {
		t.Fatal("TokenKind(unknown) did not return KindUnknown")
	}
}

func TestSchemaCompatibility(t *testing.T) {
	t.Parallel()

	previous := compatibilitySchema()
	compatible := compatibilitySchema()
	compatible.Kinds = append(compatible.Kinds, syntaxgen.Kind{
		Element: "node",
		Name:    "interface_declaration",
		GoName:  "InterfaceDeclaration",
	})
	compatible.Nodes = append(compatible.Nodes, syntaxgen.Node{
		Type: "InterfaceDeclaration",
		Kind: "interface_declaration",
	})
	compatible.Unions[0].Members = append(
		compatible.Unions[0].Members,
		"InterfaceDeclaration",
	)
	if err := compatible.ValidateCompatible(previous); err != nil {
		t.Fatalf("ValidateCompatible(compatible): %v", err)
	}

	tests := map[string]func(syntaxgen.Schema) syntaxgen.Schema{
		"reordered stable kinds": func(schema syntaxgen.Schema) syntaxgen.Schema {
			schema.Kinds[0], schema.Kinds[1] = schema.Kinds[1], schema.Kinds[0]
			return schema
		},
		"remapped node": func(schema syntaxgen.Schema) syntaxgen.Schema {
			schema.Nodes[0].Kind, schema.Nodes[1].Kind =
				schema.Nodes[1].Kind, schema.Nodes[0].Kind
			return schema
		},
		"narrowed union": func(schema syntaxgen.Schema) syntaxgen.Schema {
			schema.Unions[0].Members = schema.Unions[0].Members[:1]
			return schema
		},
		"changed accessor": func(schema syntaxgen.Schema) syntaxgen.Schema {
			schema.Accessors[0].Cardinality = "many"
			return schema
		},
	}
	for name, mutate := range tests {
		name, mutate := name, mutate
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if err := mutate(compatibilitySchema()).ValidateCompatible(previous); err == nil {
				t.Fatal("ValidateCompatible() = nil, want incompatibility")
			}
		})
	}
}

func compatibilitySchema() syntaxgen.Schema {
	return syntaxgen.Schema{
		SchemaVersion: 1,
		Kinds: []syntaxgen.Kind{
			{Element: "node", Name: "program", GoName: "Program"},
			{Element: "token", Name: "identifier", GoName: "Identifier"},
			{Element: "node", Name: "class_declaration", GoName: "ClassDeclaration"},
		},
		Nodes: []syntaxgen.Node{
			{
				Type: "CompilationUnit",
				Kind: "program",
			},
			{
				Type: "ClassDeclaration",
				Kind: "class_declaration",
			},
		},
		Unions: []syntaxgen.Union{{
			Type:    "TypeDeclaration",
			Members: []string{"CompilationUnit", "ClassDeclaration"},
		}},
		Accessors: []syntaxgen.Accessor{{
			Owner:       "CompilationUnit",
			Name:        "Name",
			Target:      "identifier",
			Element:     "token",
			Cardinality: "optional",
		}},
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

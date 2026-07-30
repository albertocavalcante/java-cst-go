package ast_test

import (
	"os"
	"path/filepath"
	"testing"

	javacst "git.alberto.engineer/alberto/java-cst-go"
	"git.alberto.engineer/alberto/java-cst-go/ast"
	"git.alberto.engineer/alberto/java-cst-go/language"
	"git.alberto.engineer/alberto/java-cst-go/syntax"
)

func TestJava8CompilationUnitAndMembers(t *testing.T) {
	t.Parallel()

	path := filepath.Join("..", "testdata", "m2", "java8", "declarations.java")
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	tree, err := javacst.Parse(string(source), javacst.Options{
		Level: language.Level{Release: language.Release8},
	})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if diagnostics := tree.Diagnostics(); len(diagnostics) != 0 {
		t.Fatalf("Diagnostics: %+v", diagnostics)
	}

	unit, ok := ast.AsCompilationUnit(tree.Root())
	if !ok {
		t.Fatalf("root kind = %q, want compilation unit", tree.Root().Kind())
	}
	if unit.Node().ID() != tree.Root().ID() {
		t.Fatalf("typed root ID = %d, want %d", unit.Node().ID(), tree.Root().ID())
	}
	if _, ok := unit.Package(); !ok {
		t.Fatal("Package() = absent, want package declaration")
	}
	if got, want := len(unit.ImportsSlice()), 2; got != want {
		t.Fatalf("len(ImportsSlice()) = %d, want %d", got, want)
	}

	types := unit.TypesSlice()
	if got, want := len(types), 4; got != want {
		t.Fatalf("len(TypesSlice()) = %d, want %d", got, want)
	}
	assertTypeName(t, types[0], "Sample")
	assertTypeName(t, types[1], "Service")
	assertTypeName(t, types[2], "Color")
	assertTypeName(t, types[3], "Marker")

	class, ok := types[0].AsClassDeclaration()
	if !ok {
		t.Fatalf("first type kind = %q, want class", types[0].Kind())
	}
	body, ok := class.Body()
	if !ok {
		t.Fatal("class Body() = absent")
	}
	members := body.MembersSlice()
	if got, want := len(members), 6; got != want {
		t.Fatalf("len(class MembersSlice()) = %d, want %d", got, want)
	}

	field, ok := members[0].AsFieldDeclaration()
	if !ok {
		t.Fatalf("first member kind = %q, want field", members[0].Kind())
	}
	declarators := field.DeclaratorsSlice()
	if got, want := len(declarators), 2; got != want {
		t.Fatalf("len(field DeclaratorsSlice()) = %d, want %d", got, want)
	}
	firstName, firstOK := declarators[0].Name()
	assertTokenText(t, firstName, firstOK, "first")
	secondName, secondOK := declarators[1].Name()
	assertTokenText(t, secondName, secondOK, "second")

	constructor, ok := members[2].AsConstructorDeclaration()
	if !ok {
		t.Fatalf("third member kind = %q, want constructor", members[2].Kind())
	}
	constructorName, constructorNameOK := constructor.Name()
	assertTokenText(t, constructorName, constructorNameOK, "Sample")
	parameters, ok := constructor.Parameters()
	if !ok {
		t.Fatal("constructor Parameters() = absent")
	}
	if got, want := len(parameters.ItemsSlice()), 2; got != want {
		t.Fatalf("len(constructor parameters) = %d, want %d", got, want)
	}
	if _, ok := constructor.Body(); !ok {
		t.Fatal("constructor Body() = absent")
	}

	method, ok := members[3].AsMethodDeclaration()
	if !ok {
		t.Fatalf("fourth member kind = %q, want method", members[3].Kind())
	}
	methodName, methodNameOK := method.Name()
	assertTokenText(t, methodName, methodNameOK, "values")
	methodParameters, ok := method.Parameters()
	if !ok || len(methodParameters.ItemsSlice()) != 1 {
		t.Fatalf("method Parameters() = (%v, %v), want one item", methodParameters, ok)
	}
	if _, ok := method.Body(); !ok {
		t.Fatal("method Body() = absent")
	}

	if _, ok := members[4].AsInterfaceDeclaration(); !ok {
		t.Fatalf("fifth member kind = %q, want nested interface", members[4].Kind())
	}
	nestedEnum, ok := members[5].AsEnumDeclaration()
	if !ok {
		t.Fatalf("sixth member kind = %q, want nested enum", members[5].Kind())
	}
	enumBody, ok := nestedEnum.Body()
	if !ok || len(enumBody.ConstantsSlice()) != 1 {
		t.Fatalf("nested enum body = (%v, %v), want one constant", enumBody, ok)
	}
	declarations, ok := enumBody.Declarations()
	if !ok || len(declarations.MembersSlice()) != 1 {
		t.Fatalf("nested enum declarations = (%v, %v), want one member", declarations, ok)
	}
}

func TestTypedViewsAreRecoverySafe(t *testing.T) {
	t.Parallel()

	const source = "class Broken { void run( }"
	tree, err := javacst.Parse(source, javacst.Options{
		Level: language.Level{Release: language.Release8},
	})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got := tree.Root().AppendText(); got != source {
		t.Fatalf("round trip = %q, want %q", got, source)
	}
	unit, ok := ast.AsCompilationUnit(tree.Root())
	if !ok {
		t.Fatal("AsCompilationUnit = false")
	}
	for declaration := range unit.Types() {
		class, ok := declaration.AsClassDeclaration()
		if !ok {
			continue
		}
		_, _ = class.Name()
		body, ok := class.Body()
		if !ok {
			continue
		}
		for member := range body.Members() {
			if method, ok := member.AsMethodDeclaration(); ok {
				_, _ = method.Name()
				_, _ = method.Parameters()
				_, _ = method.Body()
			}
		}
	}
}

func TestTypedConversionRejectsWrongKind(t *testing.T) {
	t.Parallel()

	tree, err := javacst.Parse("class Example {}", javacst.Options{
		Level: language.Level{Release: language.Release8},
	})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	unit, ok := ast.AsCompilationUnit(tree.Root())
	if !ok {
		t.Fatal("AsCompilationUnit = false")
	}
	class, ok := unit.TypesSlice()[0].AsClassDeclaration()
	if !ok {
		t.Fatal("AsClassDeclaration = false")
	}
	if _, ok := ast.AsMethodDeclaration(class.Node()); ok {
		t.Fatal("AsMethodDeclaration(class) = true")
	}
}

func assertTypeName(t *testing.T, declaration ast.TypeDeclaration, want string) {
	t.Helper()

	var token syntax.Token
	var ok bool
	switch {
	case declaration.Kind() == syntax.KindNodeClassDeclaration:
		value, converted := declaration.AsClassDeclaration()
		if !converted {
			t.Fatal("AsClassDeclaration = false")
		}
		token, ok = value.Name()
	case declaration.Kind() == syntax.KindNodeInterfaceDeclaration:
		value, converted := declaration.AsInterfaceDeclaration()
		if !converted {
			t.Fatal("AsInterfaceDeclaration = false")
		}
		token, ok = value.Name()
	case declaration.Kind() == syntax.KindNodeEnumDeclaration:
		value, converted := declaration.AsEnumDeclaration()
		if !converted {
			t.Fatal("AsEnumDeclaration = false")
		}
		token, ok = value.Name()
	case declaration.Kind() == syntax.KindNodeAnnotationTypeDeclaration:
		value, converted := declaration.AsAnnotationTypeDeclaration()
		if !converted {
			t.Fatal("AsAnnotationTypeDeclaration = false")
		}
		token, ok = value.Name()
	default:
		t.Fatalf("unexpected type kind %q", declaration.Kind())
	}
	assertTokenText(t, token, ok, want)
	if parent, parentOK := token.Parent(); !parentOK ||
		!parent.SameOccurrence(declaration.Node()) {
		t.Fatalf("name token parent does not preserve declaration identity")
	}
}

func assertTokenText(t *testing.T, token syntax.Token, ok bool, want string) {
	t.Helper()
	if !ok {
		t.Fatalf("token = absent, want %q", want)
	}
	if got := token.Text(); got != want {
		t.Fatalf("token text = %q, want %q", got, want)
	}
}

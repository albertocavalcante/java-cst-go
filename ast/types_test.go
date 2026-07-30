package ast_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	javacst "git.alberto.engineer/alberto/java-cst-go"
	"git.alberto.engineer/alberto/java-cst-go/ast"
	"git.alberto.engineer/alberto/java-cst-go/language"
	"git.alberto.engineer/alberto/java-cst-go/syntax"
)

func TestJava8NamesModifiersAndTypes(t *testing.T) {
	t.Parallel()

	path := filepath.Join("..", "testdata", "m2", "java8", "types.java")
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
		t.Fatal("AsCompilationUnit = false")
	}

	packageDeclaration, ok := unit.Package()
	if !ok {
		t.Fatal("Package() = absent")
	}
	packageName, packageNameOK := packageDeclaration.Name()
	assertElementText(t, packageName, packageNameOK, "example.types")
	if got, want := len(packageDeclaration.AnnotationsSlice()), 1; got != want {
		t.Fatalf("len(package annotations) = %d, want %d", got, want)
	}

	imports := unit.ImportsSlice()
	if got, want := len(imports), 2; got != want {
		t.Fatalf("len(imports) = %d, want %d", got, want)
	}
	firstImportName, firstImportNameOK := imports[0].Name()
	assertElementText(t, firstImportName, firstImportNameOK, "java.util.List")
	if _, ok := imports[0].Static(); ok {
		t.Fatal("ordinary import Static() = present")
	}
	secondImportName, secondImportNameOK := imports[1].Name()
	assertElementText(t, secondImportName, secondImportNameOK, "java.util.Collections")
	if token, ok := imports[1].Static(); !ok || token.Text() != "static" {
		t.Fatalf("static import Static() = (%q, %v)", token.Text(), ok)
	}
	if _, ok := imports[1].Wildcard(); !ok {
		t.Fatal("static import Wildcard() = absent")
	}

	types := unit.TypesSlice()
	if got, want := len(types), 1; got != want {
		t.Fatalf("len(types) = %d, want %d", got, want)
	}
	class, ok := types[0].AsClassDeclaration()
	if !ok {
		t.Fatal("AsClassDeclaration = false")
	}
	modifiers, ok := class.Modifiers()
	if !ok {
		t.Fatal("class Modifiers() = absent")
	}
	if got, want := modifierTexts(modifiers), []string{"@Deprecated", "public", "final"}; !equalStrings(got, want) {
		t.Fatalf("class modifiers = %q, want %q", got, want)
	}

	parameters, ok := class.TypeParameters()
	if !ok {
		t.Fatal("class TypeParameters() = absent")
	}
	typeParameters := parameters.ItemsSlice()
	if got, want := len(typeParameters), 1; got != want {
		t.Fatalf("len(class type parameters) = %d, want %d", got, want)
	}
	name, ok := typeParameters[0].Name()
	if !ok || name.Text() != "T" {
		t.Fatalf("type parameter name = (%q, %v)", name.Text(), ok)
	}
	bound, ok := typeParameters[0].Bound()
	if !ok {
		t.Fatal("type parameter Bound() = absent")
	}
	assertTypeTexts(t, bound.TypesSlice(), "Number", "Comparable<T>")

	superclass, ok := class.Superclass()
	if !ok {
		t.Fatal("Superclass() = absent")
	}
	superclassType, superclassTypeOK := superclass.Type()
	assertElementText(t, superclassType, superclassTypeOK, "Base")
	interfaces, ok := class.Interfaces()
	if !ok {
		t.Fatal("Interfaces() = absent")
	}
	typeList, ok := interfaces.Types()
	if !ok {
		t.Fatal("interface TypeList = absent")
	}
	assertTypeTexts(t, typeList.ItemsSlice(), "java.io.Serializable", "Cloneable")

	body, ok := class.Body()
	if !ok {
		t.Fatal("Body() = absent")
	}
	members := body.MembersSlice()
	if got, want := len(members), 6; got != want {
		t.Fatalf("len(members) = %d, want %d", got, want)
	}
	field, ok := members[0].AsFieldDeclaration()
	if !ok {
		t.Fatal("first member is not a field")
	}
	fieldType, ok := field.Type()
	if !ok {
		t.Fatal("field Type() = absent")
	}
	if got, want := compact(fieldType.Text()), "java.util.Map<String,?extendsNumber>[]"; got != want {
		t.Fatalf("field type = %q, want %q", got, want)
	}
	if fieldType.Kind() != syntax.KindNodeArrayType {
		t.Fatalf("field type kind = %q, want array type", fieldType.Kind())
	}
	arrayNode, ok := fieldType.Node()
	if !ok {
		t.Fatal("array type does not expose its node")
	}
	array, ok := ast.AsArrayType(arrayNode)
	if !ok {
		t.Fatal("AsArrayType = false")
	}
	elementType, ok := array.ElementType()
	if !ok {
		t.Fatal("array ElementType() = absent")
	}
	genericNode, ok := elementType.Node()
	if !ok {
		t.Fatal("array element type does not expose its node")
	}
	generic, ok := ast.AsGenericType(genericNode)
	if !ok {
		t.Fatal("array element AsGenericType = false")
	}
	arguments, ok := generic.Arguments()
	if !ok {
		t.Fatal("generic Arguments() = absent")
	}
	typeArguments := arguments.ItemsSlice()
	if got, want := len(typeArguments), 2; got != want {
		t.Fatalf("len(field type arguments) = %d, want %d", got, want)
	}
	wildcardNode, ok := typeArguments[1].Node()
	if !ok {
		t.Fatal("wildcard type argument does not expose its node")
	}
	wildcard, ok := ast.AsWildcard(wildcardNode)
	if !ok {
		t.Fatal("AsWildcard = false")
	}
	wildcardBound, ok := wildcard.Bound()
	if !ok || strings.TrimSpace(wildcardBound.Text()) != "Number" {
		t.Fatalf("wildcard bound = (%q, %v), want Number", wildcardBound.Text(), ok)
	}

	method, ok := members[4].AsMethodDeclaration()
	if !ok {
		t.Fatal("fifth member is not a method")
	}
	result, ok := method.Result()
	if !ok {
		t.Fatal("method Result() = absent")
	}
	if got, want := compact(result.Text()), "List<?superU>"; got != want {
		t.Fatalf("method result = %q, want %q", got, want)
	}
	resultNode, ok := result.Node()
	if !ok {
		t.Fatal("method result does not expose its node")
	}
	resultGeneric, ok := ast.AsGenericType(resultNode)
	if !ok {
		t.Fatal("method result AsGenericType = false")
	}
	resultArguments, ok := resultGeneric.Arguments()
	if !ok || len(resultArguments.ItemsSlice()) != 1 {
		t.Fatalf("method result arguments = (%v, %v), want one", resultArguments, ok)
	}
	methodTypeParameters, ok := method.TypeParameters()
	if !ok || len(methodTypeParameters.ItemsSlice()) != 1 {
		t.Fatalf("method type parameters = (%v, %v), want one", methodTypeParameters, ok)
	}
	methodParameters, ok := method.Parameters()
	if !ok {
		t.Fatal("method Parameters() = absent")
	}
	items := methodParameters.ItemsSlice()
	if got, want := len(items), 2; got != want {
		t.Fatalf("len(method parameters) = %d, want %d", got, want)
	}
	formal, ok := items[0].AsFormalParameter()
	if !ok {
		t.Fatal("first parameter is not formal")
	}
	formalType, formalTypeOK := formal.Type()
	assertElementText(t, formalType, formalTypeOK, "List<? extends T>")
	formalModifiers, ok := formal.Modifiers()
	if !ok || !equalStrings(modifierTexts(formalModifiers), []string{"final"}) {
		t.Fatalf("formal modifiers = (%q, %v)", modifierTexts(formalModifiers), ok)
	}
	spread, ok := items[1].AsSpreadParameter()
	if !ok {
		t.Fatal("second parameter is not spread")
	}
	spreadType, spreadTypeOK := spread.Type()
	assertElementText(t, spreadType, spreadTypeOK, "U")

	throws, ok := method.Throws()
	if !ok {
		t.Fatal("method Throws() = absent")
	}
	assertTypeTexts(
		t,
		throws.TypesSlice(),
		"java.io.IOException",
		"IllegalArgumentException",
	)

	voidMethod, ok := members[5].AsMethodDeclaration()
	if !ok {
		t.Fatal("sixth member is not a method")
	}
	voidResult, ok := voidMethod.Result()
	if !ok || voidResult.Kind() != syntax.KindTokenVoidType {
		t.Fatalf("void result = (%q, %v), want void token", voidResult.Kind(), ok)
	}
}

func TestJava8TypeViewsAreRecoverySafe(t *testing.T) {
	t.Parallel()

	const source = `class Broken<T extends Number & Comparable<T>> {
    java.util.List<? extends Number>[] field;
    <U extends Number> U run(final java.util.List<? super T> value throws E { }
}`
	tree, err := javacst.Parse(source, javacst.Options{
		Level: language.Level{Release: language.Release8},
	})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got := tree.Text(); got != source {
		t.Fatalf("round trip = %q, want %q", got, source)
	}
	if len(tree.Diagnostics()) == 0 {
		t.Fatal("Diagnostics() is empty for malformed type/member source")
	}
	unit, ok := ast.AsCompilationUnit(tree.Root())
	if !ok {
		t.Fatal("AsCompilationUnit = false")
	}
	visitedTypes := 0
	for declaration := range unit.Types() {
		class, ok := declaration.AsClassDeclaration()
		if !ok {
			continue
		}
		_, _ = class.Modifiers()
		if parameters, present := class.TypeParameters(); present {
			for parameter := range parameters.Items() {
				_, _ = parameter.Name()
				if bound, present := parameter.Bound(); present {
					for value := range bound.Types() {
						_ = value.Text()
						visitedTypes++
					}
				}
			}
		}
		_, _ = class.Superclass()
		_, _ = class.Interfaces()
		body, present := class.Body()
		if !present {
			continue
		}
		for member := range body.Members() {
			if field, converted := member.AsFieldDeclaration(); converted {
				_, _ = field.Modifiers()
				if value, present := field.Type(); present {
					_ = value.Text()
					visitedTypes++
				}
				continue
			}
			method, converted := member.AsMethodDeclaration()
			if !converted {
				continue
			}
			_, _ = method.Modifiers()
			_, _ = method.TypeParameters()
			if value, present := method.Result(); present {
				_ = value.Text()
				visitedTypes++
			}
			if parameters, present := method.Parameters(); present {
				for parameter := range parameters.Items() {
					if formal, converted := parameter.AsFormalParameter(); converted {
						if value, present := formal.Type(); present {
							_ = value.Text()
							visitedTypes++
						}
					}
				}
			}
			if clause, present := method.Throws(); present {
				for value := range clause.Types() {
					_ = value.Text()
					visitedTypes++
				}
			}
			_, _ = method.Body()
		}
	}
	if visitedTypes == 0 {
		t.Fatal("recovery traversal did not retain any typed type elements")
	}
}

func assertElementText[T interface{ Text() string }](
	t *testing.T,
	value T,
	ok bool,
	want string,
) {
	t.Helper()
	if !ok {
		t.Fatalf("element = absent, want %q", want)
	}
	if got := strings.TrimSpace(value.Text()); got != want {
		t.Fatalf("element text = %q, want %q", got, want)
	}
}

func assertTypeTexts(t *testing.T, values []ast.Type, wants ...string) {
	t.Helper()
	if len(values) != len(wants) {
		t.Fatalf("len(types) = %d, want %d", len(values), len(wants))
	}
	for index, value := range values {
		if got := strings.TrimSpace(value.Text()); got != wants[index] {
			t.Fatalf("types[%d] = %q, want %q", index, got, wants[index])
		}
	}
}

func modifierTexts(modifiers ast.Modifiers) []string {
	var result []string
	for modifier := range modifiers.Items() {
		result = append(result, strings.TrimSpace(modifier.Text()))
	}
	return result
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func compact(value string) string {
	return strings.Join(strings.Fields(value), "")
}

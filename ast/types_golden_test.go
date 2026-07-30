package ast_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	javacst "git.alberto.engineer/alberto/java-cst-go"
	"git.alberto.engineer/alberto/java-cst-go/ast"
	"git.alberto.engineer/alberto/java-cst-go/language"
)

type java8TypeShape struct {
	Package             string            `json:"package"`
	Imports             []importShape     `json:"imports"`
	ClassModifiers      []string          `json:"classModifiers"`
	TypeParameterBounds []string          `json:"typeParameterBounds"`
	Superclass          string            `json:"superclass"`
	Interfaces          []string          `json:"interfaces"`
	FieldTypes          []string          `json:"fieldTypes"`
	Methods             []methodTypeShape `json:"methods"`
}

type importShape struct {
	Name     string `json:"name"`
	Static   bool   `json:"static"`
	Wildcard bool   `json:"wildcard"`
}

type methodTypeShape struct {
	Name       string   `json:"name"`
	Result     string   `json:"result"`
	Parameters []string `json:"parameters"`
	Throws     []string `json:"throws"`
}

func TestJava8TypeShapeGolden(t *testing.T) {
	t.Parallel()

	root := filepath.Join("..", "testdata", "m2", "java8")
	source, err := os.ReadFile(filepath.Join(root, "types.java"))
	if err != nil {
		t.Fatalf("ReadFile source: %v", err)
	}
	tree, err := javacst.Parse(string(source), javacst.Options{
		Level: language.Level{Release: language.Release8},
	})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	unit, ok := ast.AsCompilationUnit(tree.Root())
	if !ok {
		t.Fatal("AsCompilationUnit = false")
	}
	got := summarizeJava8Types(t, unit)
	var gotJSON bytes.Buffer
	encoder := json.NewEncoder(&gotJSON)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(got); err != nil {
		t.Fatalf("Encode: %v", err)
	}

	wantJSON, err := os.ReadFile(filepath.Join(root, "types.shape.json"))
	if err != nil {
		t.Fatalf("ReadFile golden: %v", err)
	}
	if !bytes.Equal(gotJSON.Bytes(), wantJSON) {
		t.Fatalf("type shape differs:\ngot:\n%s\nwant:\n%s", gotJSON.Bytes(), wantJSON)
	}
}

func summarizeJava8Types(t *testing.T, unit ast.CompilationUnit) java8TypeShape {
	t.Helper()

	var shape java8TypeShape
	packageDeclaration, ok := unit.Package()
	if !ok {
		t.Fatal("Package() = absent")
	}
	name, ok := packageDeclaration.Name()
	if !ok {
		t.Fatal("package Name() = absent")
	}
	shape.Package = strings.TrimSpace(name.Text())

	for value := range unit.Imports() {
		name, ok := value.Name()
		if !ok {
			t.Fatal("import Name() = absent")
		}
		_, static := value.Static()
		_, wildcard := value.Wildcard()
		shape.Imports = append(shape.Imports, importShape{
			Name:     strings.TrimSpace(name.Text()),
			Static:   static,
			Wildcard: wildcard,
		})
	}

	declarations := unit.TypesSlice()
	class, ok := declarations[0].AsClassDeclaration()
	if !ok {
		t.Fatal("first declaration is not a class")
	}
	modifiers, ok := class.Modifiers()
	if !ok {
		t.Fatal("class Modifiers() = absent")
	}
	shape.ClassModifiers = modifierTexts(modifiers)

	parameters, ok := class.TypeParameters()
	if !ok {
		t.Fatal("class TypeParameters() = absent")
	}
	for parameter := range parameters.Items() {
		bound, ok := parameter.Bound()
		if !ok {
			continue
		}
		for value := range bound.Types() {
			shape.TypeParameterBounds = append(
				shape.TypeParameterBounds,
				strings.TrimSpace(value.Text()),
			)
		}
	}
	superclass, ok := class.Superclass()
	if !ok {
		t.Fatal("class Superclass() = absent")
	}
	superclassType, ok := superclass.Type()
	if !ok {
		t.Fatal("superclass Type() = absent")
	}
	shape.Superclass = strings.TrimSpace(superclassType.Text())

	interfaces, ok := class.Interfaces()
	if !ok {
		t.Fatal("class Interfaces() = absent")
	}
	interfaceList, ok := interfaces.Types()
	if !ok {
		t.Fatal("interfaces Types() = absent")
	}
	for value := range interfaceList.Items() {
		shape.Interfaces = append(shape.Interfaces, strings.TrimSpace(value.Text()))
	}

	body, ok := class.Body()
	if !ok {
		t.Fatal("class Body() = absent")
	}
	for member := range body.Members() {
		if field, ok := member.AsFieldDeclaration(); ok {
			fieldType, present := field.Type()
			if !present {
				t.Fatal("field Type() = absent")
			}
			shape.FieldTypes = append(shape.FieldTypes, compact(fieldType.Text()))
			continue
		}
		method, ok := member.AsMethodDeclaration()
		if !ok {
			continue
		}
		methodName, ok := method.Name()
		if !ok {
			t.Fatal("method Name() = absent")
		}
		result, ok := method.Result()
		if !ok {
			t.Fatal("method Result() = absent")
		}
		methodShape := methodTypeShape{
			Name:   methodName.Text(),
			Result: compact(result.Text()),
		}
		parameters, ok := method.Parameters()
		if !ok {
			t.Fatal("method Parameters() = absent")
		}
		for parameter := range parameters.Items() {
			var parameterType ast.Type
			var present bool
			if formal, ok := parameter.AsFormalParameter(); ok {
				parameterType, present = formal.Type()
			} else if spread, ok := parameter.AsSpreadParameter(); ok {
				parameterType, present = spread.Type()
			}
			if !present {
				t.Fatal("parameter Type() = absent")
			}
			methodShape.Parameters = append(
				methodShape.Parameters,
				compact(parameterType.Text()),
			)
		}
		if throws, ok := method.Throws(); ok {
			for value := range throws.Types() {
				methodShape.Throws = append(
					methodShape.Throws,
					strings.TrimSpace(value.Text()),
				)
			}
		}
		if methodShape.Parameters == nil {
			methodShape.Parameters = []string{}
		}
		if methodShape.Throws == nil {
			methodShape.Throws = []string{}
		}
		shape.Methods = append(shape.Methods, methodShape)
	}
	return shape
}

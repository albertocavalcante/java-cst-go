package ast_test

import (
	"fmt"

	javacst "git.alberto.engineer/alberto/java-cst-go"
	"git.alberto.engineer/alberto/java-cst-go/ast"
	"git.alberto.engineer/alberto/java-cst-go/language"
	"git.alberto.engineer/alberto/java-cst-go/syntax"
)

func ExampleCompilationUnit() {
	tree, err := javacst.Parse(
		"class Example {} interface Service {}",
		javacst.Options{
			Level: language.Level{Release: language.Release8},
		},
	)
	if err != nil {
		panic(err)
	}
	unit, ok := ast.AsCompilationUnit(tree.Root())
	if !ok {
		panic("unexpected root kind")
	}
	for declaration := range unit.Types() {
		switch {
		case declaration.Kind() == syntax.KindNodeClassDeclaration:
			class, _ := declaration.AsClassDeclaration()
			name, _ := class.Name()
			fmt.Println("class", name.Text())
		case declaration.Kind() == syntax.KindNodeInterfaceDeclaration:
			iface, _ := declaration.AsInterfaceDeclaration()
			name, _ := iface.Name()
			fmt.Println("interface", name.Text())
		}
	}
	// Output:
	// class Example
	// interface Service
}

package syntax_test

import (
	"fmt"

	javacst "git.alberto.engineer/alberto/java-cst-go"
	"git.alberto.engineer/alberto/java-cst-go/syntax"
)

func ExampleTree() {
	tree, err := javacst.Parse("class Example {}\n", javacst.Options{})
	if err != nil {
		panic(err)
	}

	printTree(tree)
	// Output:
	// node:program
	// treesitter-go/v0.1.0
}

func printTree(tree *syntax.Tree) {
	fmt.Println(tree.Root().Kind())
	fmt.Println(tree.Provenance().Backend)
}

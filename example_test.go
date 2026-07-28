package javacst_test

import (
	"fmt"

	javacst "git.alberto.engineer/alberto/java-cst-go"
)

func Example() {
	const source = "class Example {}\n"
	tree, err := javacst.Parse(source, javacst.Options{})
	if err != nil {
		panic(err)
	}

	fmt.Println(tree.Level())
	fmt.Println(tree.Root().Kind())
	fmt.Println(tree.Text() == source)
	// Output:
	// 25
	// node:program
	// true
}

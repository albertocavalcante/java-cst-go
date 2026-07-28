package source_test

import (
	"fmt"

	"git.alberto.engineer/alberto/java-cst-go/source"
)

func ExampleTranslate() {
	const raw = `class \u0041 {}`
	translation := source.Translate(raw)

	fmt.Println(translation.Logical())
	fmt.Println(translation.Raw() == raw)
	// Output:
	// class A {}
	// true
}

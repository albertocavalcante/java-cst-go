package language_test

import (
	"fmt"

	"git.alberto.engineer/alberto/java-cst-go/language"
)

func ExampleLevel() {
	level := language.Level{Release: language.Release23}
	support := level.Feature(language.FeatureModuleImports)

	fmt.Println(support.State)
	fmt.Println(level.Supports(language.FeatureModuleImports))
	// Output:
	// preview
	// false
}

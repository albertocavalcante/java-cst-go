package diagnostic_test

import (
	"fmt"

	"git.alberto.engineer/alberto/java-cst-go/diagnostic"
	"git.alberto.engineer/alberto/java-cst-go/language"
)

func ExampleDiagnostic() {
	value := diagnostic.New(
		diagnostic.CodePreviewDisabled,
		diagnostic.SeverityError,
		diagnostic.Span{Start: 7, End: 20},
		"preview feature is disabled",
		language.FeatureStringTemplates,
		nil,
	)

	fmt.Println(value.Code)
	fmt.Println(value.Severity)
	fmt.Println(value.Feature)
	// Output:
	// JAV3002
	// error
	// string-templates
}

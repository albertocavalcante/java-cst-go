package javacst_test

import (
	"os"
	"path/filepath"
	"testing"

	javacst "git.alberto.engineer/alberto/java-cst-go"
	"git.alberto.engineer/alberto/java-cst-go/diagnostic"
	"git.alberto.engineer/alberto/java-cst-go/language"
)

func TestParseValidatesDemonstratedJava21Through26FeatureCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		path    string
		level   language.Level
		feature language.FeatureID
		code    diagnostic.Code
	}{
		{
			name:    "java21/string-template/preview-disabled-p1",
			path:    "string-templates/basic.java",
			level:   language.Level{Release: language.Release21},
			feature: language.FeatureStringTemplates,
			code:    diagnostic.CodePreviewDisabled,
		},
		{
			name:    "java21/flexible-constructor/unavailable",
			path:    "flexible-constructor-bodies/basic.java",
			level:   language.Level{Release: language.Release21},
			feature: language.FeatureFlexibleConstructorBodies,
			code:    diagnostic.CodeFeatureUnavailable,
		},
		{
			name:    "java22/module-import/unavailable",
			path:    "module-imports/basic.java",
			level:   language.Level{Release: language.Release22},
			feature: language.FeatureModuleImports,
			code:    diagnostic.CodeFeatureUnavailable,
		},
		{
			name:    "java22/string-template/preview-disabled-p2",
			path:    "string-templates/basic.java",
			level:   language.Level{Release: language.Release22},
			feature: language.FeatureStringTemplates,
			code:    diagnostic.CodePreviewDisabled,
		},
		{
			name:    "java22/flexible-constructor/preview-disabled-p1",
			path:    "flexible-constructor-bodies/basic.java",
			level:   language.Level{Release: language.Release22},
			feature: language.FeatureFlexibleConstructorBodies,
			code:    diagnostic.CodePreviewDisabled,
		},
		{
			name:    "java22/primitive-pattern/unavailable",
			path:    "primitive-patterns/basic.java",
			level:   language.Level{Release: language.Release22, Preview: true},
			feature: language.FeaturePrimitivePatterns,
			code:    diagnostic.CodeFeatureUnavailable,
		},
		{
			name:    "java22/flexible-constructor/field-before-super-rejected-p1",
			path:    "flexible-constructor-bodies/field-before-super.java",
			level:   language.Level{Release: language.Release22, Preview: true},
			feature: language.FeatureFlexibleConstructorBodies,
			code:    diagnostic.CodeFeatureRestriction,
		},
		{
			name:    "java23/module-import/preview-disabled-p1",
			path:    "module-imports/basic.java",
			level:   language.Level{Release: language.Release23},
			feature: language.FeatureModuleImports,
			code:    diagnostic.CodePreviewDisabled,
		},
		{
			name:    "java23/string-template/withdrawn",
			path:    "string-templates/basic.java",
			level:   language.Level{Release: language.Release23, Preview: true},
			feature: language.FeatureStringTemplates,
			code:    diagnostic.CodeFeatureWithdrawn,
		},
		{
			name:    "java23/primitive-pattern/preview-disabled-p1",
			path:    "primitive-patterns/basic.java",
			level:   language.Level{Release: language.Release23},
			feature: language.FeaturePrimitivePatterns,
			code:    diagnostic.CodePreviewDisabled,
		},
		{
			name:    "java23/module-import/on-demand-shadow-rejected-p1",
			path:    "module-imports/on-demand-shadow.java",
			level:   language.Level{Release: language.Release23, Preview: true},
			feature: language.FeatureModuleImports,
			code:    diagnostic.CodeFeatureRestriction,
		},
		{
			name:    "java24/module-import/preview-disabled-p2",
			path:    "module-imports/basic.java",
			level:   language.Level{Release: language.Release24},
			feature: language.FeatureModuleImports,
			code:    diagnostic.CodePreviewDisabled,
		},
		{
			name:    "java24/primitive-pattern/preview-disabled-p2",
			path:    "primitive-patterns/basic.java",
			level:   language.Level{Release: language.Release24},
			feature: language.FeaturePrimitivePatterns,
			code:    diagnostic.CodePreviewDisabled,
		},
		{
			name:    "java25/primitive-pattern/preview-disabled-p3",
			path:    "primitive-patterns/basic.java",
			level:   language.Level{Release: language.Release25},
			feature: language.FeaturePrimitivePatterns,
			code:    diagnostic.CodePreviewDisabled,
		},
		{
			name:    "java26/string-template/withdrawn",
			path:    "string-templates/basic.java",
			level:   language.Level{Release: language.Release26, Preview: true},
			feature: language.FeatureStringTemplates,
			code:    diagnostic.CodeFeatureWithdrawn,
		},
		{
			name:    "java26/primitive-pattern/preview-disabled-p4",
			path:    "primitive-patterns/basic.java",
			level:   language.Level{Release: language.Release26},
			feature: language.FeaturePrimitivePatterns,
			code:    diagnostic.CodePreviewDisabled,
		},
		{
			name:    "java26/primitive-pattern/p4-dominance-rejected",
			path:    "primitive-patterns/p4-dominance.java",
			level:   language.Level{Release: language.Release26, Preview: true},
			feature: language.FeaturePrimitivePatterns,
			code:    diagnostic.CodeFeatureRestriction,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			raw, err := os.ReadFile(filepath.Join(
				"testdata",
				"m0",
				"features",
				filepath.FromSlash(test.path),
			))
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			tree, err := javacst.Parse(
				string(raw),
				javacst.Options{Level: test.level},
			)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			values := featureDiagnostics(tree.Diagnostics())
			if len(values) != 1 {
				t.Fatalf("feature diagnostics = %+v, want exactly one", values)
			}
			if values[0].Code != test.code || values[0].Feature != test.feature {
				t.Errorf(
					"feature diagnostic = %+v, want code %s feature %s",
					values[0],
					test.code,
					test.feature,
				)
			}
			if values[0].Span.Start < 0 ||
				values[0].Span.End <= values[0].Span.Start ||
				values[0].Span.End > len(raw) {
				t.Errorf("feature diagnostic has invalid raw span: %+v", values[0])
			}
			if got := tree.Text(); got != string(raw) {
				t.Errorf("round trip differs after validation")
			}
		})
	}
}

func TestFeatureValidatorRecognizesWithoutStealingOlderIdentifiers(t *testing.T) {
	t.Parallel()

	const raw = `
class module extends Object {
    int module;
    String text = "not a template";

    module() {
        super();
    }

    static String classify(Object value) {
        return switch (value) {
            case Integer number -> number.toString();
            default -> "other";
        };
    }
}
`
	tree, err := javacst.Parse(raw, javacst.Options{
		Level: language.Level{Release: language.Release21},
	})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if values := featureDiagnostics(tree.Diagnostics()); len(values) != 0 {
		t.Fatalf("feature diagnostics = %+v, want none", values)
	}
}

func TestFeatureValidatorUsesLogicalSyntaxAndReportsRawSpan(t *testing.T) {
	t.Parallel()

	const raw = `class A {
    String greet(String name) {
        return STR.\u0022Hello \{name}\u0022;
    }
}
`
	tree, err := javacst.Parse(raw, javacst.Options{
		Level: language.Level{Release: language.Release21},
	})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	values := featureDiagnostics(tree.Diagnostics())
	if len(values) != 1 ||
		values[0].Code != diagnostic.CodePreviewDisabled ||
		values[0].Feature != language.FeatureStringTemplates {
		t.Fatalf("feature diagnostics = %+v, want string-template preview error", values)
	}
	if got := raw[values[0].Span.Start:values[0].Span.End]; got !=
		`STR.\u0022Hello \{name}\u0022` {
		t.Errorf("raw diagnostic text = %q", got)
	}
	if tree.Text() != raw {
		t.Errorf("tree text differs after translated validation")
	}
}

func featureDiagnostics(
	values []diagnostic.Diagnostic,
) []diagnostic.Diagnostic {
	result := make([]diagnostic.Diagnostic, 0)
	for _, value := range values {
		switch value.Code {
		case diagnostic.CodeFeatureUnavailable,
			diagnostic.CodePreviewDisabled,
			diagnostic.CodeFeatureWithdrawn,
			diagnostic.CodeFeatureRestriction:
			result = append(result, value)
		}
	}
	return result
}

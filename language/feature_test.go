package language_test

import (
	"testing"

	"git.alberto.engineer/alberto/java-cst-go/language"
)

type expectedSpan struct {
	first   language.Release
	last    language.Release
	state   language.FeatureState
	variant uint8
}

func TestFeatureReleaseMatrix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		feature language.FeatureID
		spans   []expectedSpan
	}{
		{language.FeatureLambdaExpressions, finalFrom(language.Release8)},
		{language.FeatureMethodReferences, finalFrom(language.Release8)},
		{language.FeatureTypeAnnotations, finalFrom(language.Release8)},
		{language.FeatureModuleDeclarations, finalFrom(language.Release9)},
		{language.FeaturePrivateInterfaceMethods, finalFrom(language.Release9)},
		{language.FeatureTryResourceReference, finalFrom(language.Release9)},
		{language.FeatureDiamondAnonymousClass, finalFrom(language.Release9)},
		{language.FeatureUnderscoreReserved, finalFrom(language.Release9)},
		{language.FeatureLocalVarInference, finalFrom(language.Release10)},
		{language.FeatureLambdaVarParameters, finalFrom(language.Release11)},
		{language.FeatureSwitchExpressions, previewToFinal(language.Release12, language.Release14)},
		{language.FeatureTextBlocks, previewToFinal(language.Release13, language.Release15)},
		{language.FeatureRecords, previewToFinal(language.Release14, language.Release16)},
		{language.FeatureInstanceofPatterns, previewToFinal(language.Release14, language.Release16)},
		{language.FeatureSealedTypes, previewToFinal(language.Release15, language.Release17)},
		{language.FeatureSwitchPatterns, previewToFinal(language.Release17, language.Release21)},
		{language.FeatureRecordPatterns, previewToFinal(language.Release19, language.Release21)},
		{
			feature: language.FeatureStringTemplates,
			spans: []expectedSpan{
				{language.Release21, language.Release21, language.FeaturePreview, 1},
				{language.Release22, language.Release22, language.FeaturePreview, 2},
				{language.Release23, language.Release26, language.FeatureWithdrawn, 0},
			},
		},
		{
			language.FeatureUnnamedVariablesPatterns,
			previewToFinal(language.Release21, language.Release22),
		},
		{
			language.FeatureCompactSourceFiles,
			previewToFinal(language.Release21, language.Release25),
		},
		{
			language.FeatureInstanceMainMethods,
			previewToFinal(language.Release21, language.Release25),
		},
		{
			language.FeatureFlexibleConstructorBodies,
			previewToFinal(language.Release22, language.Release25),
		},
		{language.FeatureModuleImports, previewToFinal(language.Release23, language.Release25)},
		{
			feature: language.FeaturePrimitivePatterns,
			spans: []expectedSpan{
				{language.Release23, language.Release23, language.FeaturePreview, 1},
				{language.Release24, language.Release24, language.FeaturePreview, 2},
				{language.Release25, language.Release25, language.FeaturePreview, 3},
				{language.Release26, language.Release26, language.FeaturePreview, 4},
			},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.feature.String(), func(t *testing.T) {
			t.Parallel()

			for release := language.Release8; release <= language.Release26; release++ {
				level := language.Level{Release: release}
				got := level.Feature(test.feature)
				want := expectedSupport(release, test.spans)
				if got != want {
					t.Errorf(
						"Level{%s}.Feature(%s) = %+v, want %+v",
						release,
						test.feature,
						got,
						want,
					)
				}
			}
		})
	}
}

func TestSupportsAppliesPreviewFlag(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		level   language.Level
		feature language.FeatureID
		want    bool
	}{
		{
			name:    "unavailable",
			level:   language.Level{Release: language.Release22, Preview: true},
			feature: language.FeatureModuleImports,
			want:    false,
		},
		{
			name:    "preview disabled",
			level:   language.Level{Release: language.Release23},
			feature: language.FeatureModuleImports,
			want:    false,
		},
		{
			name:    "preview enabled",
			level:   language.Level{Release: language.Release23, Preview: true},
			feature: language.FeatureModuleImports,
			want:    true,
		},
		{
			name:    "final",
			level:   language.Level{Release: language.Release25},
			feature: language.FeatureModuleImports,
			want:    true,
		},
		{
			name:    "withdrawn with preview enabled",
			level:   language.Level{Release: language.Release23, Preview: true},
			feature: language.FeatureStringTemplates,
			want:    false,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if got := test.level.Supports(test.feature); got != test.want {
				t.Fatalf(
					"%s.Supports(%s) = %t, want %t",
					test.level,
					test.feature,
					got,
					test.want,
				)
			}
		})
	}
}

func TestFeatureReportsPreviewWhenDisabled(t *testing.T) {
	t.Parallel()

	level := language.Level{Release: language.Release24}
	got := level.Feature(language.FeatureModuleImports)
	want := language.FeatureSupport{State: language.FeaturePreview, Variant: 2}
	if got != want {
		t.Fatalf("Feature(module-imports) = %+v, want %+v", got, want)
	}
}

func TestFeatureRegistryIsStableAndComplete(t *testing.T) {
	t.Parallel()

	features := language.AllFeatures()
	if got, want := len(features), 24; got != want {
		t.Fatalf("len(AllFeatures()) = %d, want %d", got, want)
	}

	names := make(map[string]language.FeatureID, len(features))
	for index, feature := range features {
		wantValue := language.FeatureID(index + 1)
		if feature != wantValue {
			t.Errorf("AllFeatures()[%d] = %d, want %d", index, feature, wantValue)
		}
		if !feature.Valid() {
			t.Errorf("%d.Valid() = false", feature)
		}
		name := feature.String()
		if name == "" || name == "invalid" {
			t.Errorf("FeatureID(%d).String() = %q", feature, name)
		}
		if previous, exists := names[name]; exists {
			t.Errorf("duplicate feature name %q for %d and %d", name, previous, feature)
		}
		names[name] = feature

		parsed, err := language.ParseFeatureID(name)
		if err != nil {
			t.Errorf("ParseFeatureID(%q): %v", name, err)
		} else if parsed != feature {
			t.Errorf("ParseFeatureID(%q) = %d, want %d", name, parsed, feature)
		}
	}

	features[0] = 0
	if got := language.AllFeatures()[0]; got != language.FeatureLambdaExpressions {
		t.Fatalf("caller mutation changed registry: first feature = %d", got)
	}
}

func TestParseFeatureIDRejectsUnknownNames(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"", "records ", "Records", "not-a-feature"} {
		if got, err := language.ParseFeatureID(name); err == nil {
			t.Errorf("ParseFeatureID(%q) = %d, nil; want error", name, got)
		}
	}
}

func TestInvalidFeatureQueriesAreUnavailable(t *testing.T) {
	t.Parallel()

	for _, feature := range []language.FeatureID{0, 25, 65535} {
		if feature.Valid() {
			t.Errorf("FeatureID(%d).Valid() = true", feature)
		}
		if got := feature.String(); got != "invalid" {
			t.Errorf("FeatureID(%d).String() = %q, want invalid", feature, got)
		}
		level := language.Level{Release: language.Release26, Preview: true}
		if got := level.Feature(feature); got != (language.FeatureSupport{}) {
			t.Errorf("FeatureID(%d) support = %+v, want zero", feature, got)
		}
		if level.Supports(feature) {
			t.Errorf("Level.Supports(FeatureID(%d)) = true", feature)
		}
	}

	invalidLevel := language.Level{Release: 27, Preview: true}
	if got := invalidLevel.Feature(language.FeatureLambdaExpressions); got != (language.FeatureSupport{}) {
		t.Errorf("invalid level support = %+v, want zero", got)
	}
}

func TestFeatureStateString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		state language.FeatureState
		want  string
	}{
		{language.FeatureUnavailable, "unavailable"},
		{language.FeaturePreview, "preview"},
		{language.FeatureFinal, "final"},
		{language.FeatureWithdrawn, "withdrawn"},
		{language.FeatureState(255), "invalid"},
	}
	for _, test := range tests {
		if got := test.state.String(); got != test.want {
			t.Errorf("FeatureState(%d).String() = %q, want %q", test.state, got, test.want)
		}
	}
}

func finalFrom(first language.Release) []expectedSpan {
	return []expectedSpan{{first, language.Release26, language.FeatureFinal, 1}}
}

func previewToFinal(first, final language.Release) []expectedSpan {
	spans := make([]expectedSpan, 0, int(final-first)+1)
	variant := uint8(1)
	for release := first; release < final; release++ {
		spans = append(spans, expectedSpan{
			first:   release,
			last:    release,
			state:   language.FeaturePreview,
			variant: variant,
		})
		variant++
	}

	return append(spans, expectedSpan{
		first:   final,
		last:    language.Release26,
		state:   language.FeatureFinal,
		variant: variant,
	})
}

func expectedSupport(
	release language.Release,
	spans []expectedSpan,
) language.FeatureSupport {
	for _, span := range spans {
		if release >= span.first && release <= span.last {
			return language.FeatureSupport{State: span.state, Variant: span.variant}
		}
	}

	return language.FeatureSupport{}
}

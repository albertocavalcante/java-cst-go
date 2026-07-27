package language

// FeatureID identifies a parser-visible Java language feature.
//
// Numeric values are stable. New features must be appended so persisted
// manifests and downstream integrations do not silently change meaning.
type FeatureID uint16

const (
	FeatureLambdaExpressions FeatureID = iota + 1
	FeatureMethodReferences
	FeatureTypeAnnotations
	FeatureModuleDeclarations
	FeaturePrivateInterfaceMethods
	FeatureTryResourceReference
	FeatureDiamondAnonymousClass
	FeatureUnderscoreReserved
	FeatureLocalVarInference
	FeatureLambdaVarParameters
	FeatureSwitchExpressions
	FeatureTextBlocks
	FeatureRecords
	FeatureInstanceofPatterns
	FeatureSealedTypes
	FeatureSwitchPatterns
	FeatureRecordPatterns
	FeatureStringTemplates
	FeatureUnnamedVariablesPatterns
	FeatureCompactSourceFiles
	FeatureInstanceMainMethods
	FeatureFlexibleConstructorBodies
	FeatureModuleImports
	FeaturePrimitivePatterns

	featureIDLimit
)

// FeatureState describes a feature's specification status in one Java release.
type FeatureState uint8

const (
	FeatureUnavailable FeatureState = iota
	FeaturePreview
	FeatureFinal
	FeatureWithdrawn
)

// FeatureSupport is the release-specific state and grammar generation of a
// feature. Variant starts at 1 for the first available grammar and advances
// when a later preview or final release changes that grammar. It is zero when
// State is FeatureUnavailable or FeatureWithdrawn.
type FeatureSupport struct {
	State   FeatureState
	Variant uint8
}

type featureSpan struct {
	first   Release
	last    Release
	state   FeatureState
	variant uint8
}

type featureDefinition struct {
	name  string
	spans []featureSpan
}

var featureDefinitions = [...]featureDefinition{
	FeatureLambdaExpressions:       directFinal("lambda-expressions", Release8),
	FeatureMethodReferences:        directFinal("method-references", Release8),
	FeatureTypeAnnotations:         directFinal("type-annotations", Release8),
	FeatureModuleDeclarations:      directFinal("module-declarations", Release9),
	FeaturePrivateInterfaceMethods: directFinal("private-interface-methods", Release9),
	FeatureTryResourceReference:    directFinal("try-resource-reference", Release9),
	FeatureDiamondAnonymousClass:   directFinal("diamond-anonymous-class", Release9),
	FeatureUnderscoreReserved:      directFinal("underscore-reserved", Release9),
	FeatureLocalVarInference:       directFinal("local-var-inference", Release10),
	FeatureLambdaVarParameters:     directFinal("lambda-var-parameters", Release11),
	FeatureSwitchExpressions:       previewThenFinal("switch-expressions", Release12, Release14),
	FeatureTextBlocks:              previewThenFinal("text-blocks", Release13, Release15),
	FeatureRecords:                 previewThenFinal("records", Release14, Release16),
	FeatureInstanceofPatterns:      previewThenFinal("instanceof-patterns", Release14, Release16),
	FeatureSealedTypes:             previewThenFinal("sealed-types", Release15, Release17),
	FeatureSwitchPatterns:          previewThenFinal("switch-patterns", Release17, Release21),
	FeatureRecordPatterns:          previewThenFinal("record-patterns", Release19, Release21),
	FeatureStringTemplates: {
		name: "string-templates",
		spans: []featureSpan{
			{first: Release21, last: Release21, state: FeaturePreview, variant: 1},
			{first: Release22, last: Release22, state: FeaturePreview, variant: 2},
			{first: Release23, last: Release26, state: FeatureWithdrawn},
		},
	},
	FeatureUnnamedVariablesPatterns: previewThenFinal(
		"unnamed-variables-patterns",
		Release21,
		Release22,
	),
	FeatureCompactSourceFiles:  previewThenFinal("compact-source-files", Release21, Release25),
	FeatureInstanceMainMethods: previewThenFinal("instance-main-methods", Release21, Release25),
	FeatureFlexibleConstructorBodies: previewThenFinal(
		"flexible-constructor-bodies",
		Release22,
		Release25,
	),
	FeatureModuleImports:     previewThenFinal("module-imports", Release23, Release25),
	FeaturePrimitivePatterns: previewOnly("primitive-patterns", Release23, Release26),
}

func directFinal(name string, first Release) featureDefinition {
	return featureDefinition{
		name: name,
		spans: []featureSpan{{
			first:   first,
			last:    Release26,
			state:   FeatureFinal,
			variant: 1,
		}},
	}
}

func previewThenFinal(name string, first, final Release) featureDefinition {
	spans := make([]featureSpan, 0, int(final-first)+1)
	variant := uint8(1)
	for release := first; release < final; release++ {
		spans = append(spans, featureSpan{
			first:   release,
			last:    release,
			state:   FeaturePreview,
			variant: variant,
		})
		variant++
	}

	return featureDefinition{
		name: name,
		spans: append(spans, featureSpan{
			first:   final,
			last:    Release26,
			state:   FeatureFinal,
			variant: variant,
		}),
	}
}

func previewOnly(name string, first, last Release) featureDefinition {
	spans := make([]featureSpan, 0, int(last-first)+1)
	variant := uint8(1)
	for release := first; release <= last; release++ {
		spans = append(spans, featureSpan{
			first:   release,
			last:    release,
			state:   FeaturePreview,
			variant: variant,
		})
		variant++
	}

	return featureDefinition{name: name, spans: spans}
}

// Valid reports whether id is a registered feature.
func (id FeatureID) Valid() bool {
	return id > 0 && id < featureIDLimit
}

// String returns the stable manifest spelling of id, or "invalid" for an
// unregistered value.
func (id FeatureID) String() string {
	if !id.Valid() {
		return "invalid"
	}

	return featureDefinitions[id].name
}

// AllFeatures returns all registered feature IDs in stable numeric order.
// The returned slice is newly allocated and safe for the caller to modify.
func AllFeatures() []FeatureID {
	features := make([]FeatureID, 0, featureIDLimit-1)
	for id := FeatureLambdaExpressions; id < featureIDLimit; id++ {
		features = append(features, id)
	}

	return features
}

// String returns the canonical spelling of a feature state.
func (state FeatureState) String() string {
	switch state {
	case FeatureUnavailable:
		return "unavailable"
	case FeaturePreview:
		return "preview"
	case FeatureFinal:
		return "final"
	case FeatureWithdrawn:
		return "withdrawn"
	default:
		return "invalid"
	}
}

// Feature returns the feature state for l's release. Preview is reported
// independently of l.Preview so callers can distinguish a disabled preview
// feature from one that does not exist in that release.
func (l Level) Feature(id FeatureID) FeatureSupport {
	if !l.Valid() || !id.Valid() {
		return FeatureSupport{State: FeatureUnavailable}
	}

	for _, span := range featureDefinitions[id].spans {
		if l.Release >= span.first && l.Release <= span.last {
			return FeatureSupport{State: span.state, Variant: span.variant}
		}
	}

	return FeatureSupport{State: FeatureUnavailable}
}

// Supports reports whether id is enabled at l. Final features are always
// enabled, preview features require Level.Preview, and unavailable or
// withdrawn features are never enabled.
func (l Level) Supports(id FeatureID) bool {
	support := l.Feature(id)

	return support.State == FeatureFinal ||
		(support.State == FeaturePreview && l.Preview)
}

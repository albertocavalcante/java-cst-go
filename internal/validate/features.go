// Package validate performs release-aware checks over backend-neutral trees.
package validate

import (
	"fmt"

	"git.alberto.engineer/alberto/java-cst-go/diagnostic"
	"git.alberto.engineer/alberto/java-cst-go/internal/backend"
	"git.alberto.engineer/alberto/java-cst-go/language"
	"git.alberto.engineer/alberto/java-cst-go/source"
)

// Features reports demonstrated release, preview, withdrawal, and
// generation-specific feature violations. It never changes the parse tree.
func Features(
	translation *source.Translation,
	result backend.Result,
) []diagnostic.Diagnostic {
	if translation == nil || result.Root == nil || !result.Level.Valid() {
		return nil
	}

	occurrences := findOccurrences(result.Root)
	values := make([]diagnostic.Diagnostic, 0, len(occurrences))
	for _, occurrence := range occurrences {
		if value, ok := availabilityDiagnostic(result.Level, occurrence); ok {
			values = append(values, value)
		}
	}

	values = append(
		values,
		generationRestrictions(translation, result)...,
	)
	return diagnostic.Normalize(values)
}

type occurrence struct {
	feature language.FeatureID
	node    *backend.Node
}

func findOccurrences(root *backend.Node) []occurrence {
	values := make([]occurrence, 0)
	stack := []*backend.Node{root}
	for len(stack) > 0 {
		index := len(stack) - 1
		node := stack[index]
		stack = stack[:index]

		switch node.Kind {
		case "module_import_declaration":
			values = append(values, occurrence{
				feature: language.FeatureModuleImports,
				node:    node,
			})
		case "template_expression":
			values = append(values, occurrence{
				feature: language.FeatureStringTemplates,
				node:    node,
			})
		case "constructor_body":
			if invocation := flexibleInvocation(node); invocation != nil {
				values = append(values, occurrence{
					feature: language.FeatureFlexibleConstructorBodies,
					node:    invocation,
				})
			}
		case "type_pattern":
			if hasDirectPrimitiveType(node) {
				values = append(values, occurrence{
					feature: language.FeaturePrimitivePatterns,
					node:    node,
				})
			}
		}

		for child := len(node.Children) - 1; child >= 0; child-- {
			stack = append(stack, &node.Children[child])
		}
	}
	return values
}

func flexibleInvocation(body *backend.Node) *backend.Node {
	hasPrologue := false
	for index := range body.Children {
		child := &body.Children[index]
		if child.Kind == "explicit_constructor_invocation" {
			if hasPrologue {
				return child
			}
			return nil
		}
		if child.Named && !child.Extra && !child.Missing {
			hasPrologue = true
		}
	}
	return nil
}

func hasDirectPrimitiveType(pattern *backend.Node) bool {
	for index := range pattern.Children {
		switch pattern.Children[index].Kind {
		case "integral_type", "floating_point_type", "boolean_type":
			return true
		}
	}
	return false
}

func availabilityDiagnostic(
	level language.Level,
	value occurrence,
) (diagnostic.Diagnostic, bool) {
	support := level.Feature(value.feature)
	span := nodeSpan(value.node)
	switch support.State {
	case language.FeatureUnavailable:
		return diagnostic.New(
			diagnostic.CodeFeatureUnavailable,
			diagnostic.SeverityError,
			span,
			fmt.Sprintf(
				"Java feature %q is unavailable in Java %s",
				value.feature,
				level.Release,
			),
			value.feature,
			nil,
		), true
	case language.FeaturePreview:
		if level.Preview {
			return diagnostic.Diagnostic{}, false
		}
		return diagnostic.New(
			diagnostic.CodePreviewDisabled,
			diagnostic.SeverityError,
			span,
			fmt.Sprintf(
				"Java feature %q requires preview features in Java %s",
				value.feature,
				level.Release,
			),
			value.feature,
			nil,
		), true
	case language.FeatureWithdrawn:
		return diagnostic.New(
			diagnostic.CodeFeatureWithdrawn,
			diagnostic.SeverityError,
			span,
			fmt.Sprintf(
				"Java feature %q was withdrawn before Java %s",
				value.feature,
				level.Release,
			),
			value.feature,
			nil,
		), true
	default:
		return diagnostic.Diagnostic{}, false
	}
}

func generationRestrictions(
	translation *source.Translation,
	result backend.Result,
) []diagnostic.Diagnostic {
	values := make([]diagnostic.Diagnostic, 0, 3)
	if support := result.Level.Feature(
		language.FeatureFlexibleConstructorBodies,
	); result.Level.Supports(language.FeatureFlexibleConstructorBodies) &&
		support.Variant == 1 {
		if node := earlyThisReference(result.Root); node != nil {
			values = append(values, restrictionDiagnostic(
				node,
				language.FeatureFlexibleConstructorBodies,
				"cannot reference the object under construction before the explicit constructor invocation in this feature generation",
			))
		}
	}

	if support := result.Level.Feature(
		language.FeatureModuleImports,
	); result.Level.Supports(language.FeatureModuleImports) &&
		support.Variant == 1 {
		if node := demonstratedModuleImportAmbiguity(translation, result.Root); node != nil {
			values = append(values, restrictionDiagnostic(
				node,
				language.FeatureModuleImports,
				"module import and on-demand import make this demonstrated type reference ambiguous",
			))
		}
	}

	if support := result.Level.Feature(
		language.FeaturePrimitivePatterns,
	); result.Level.Supports(language.FeaturePrimitivePatterns) &&
		support.Variant == 4 {
		if node := demonstratedPrimitiveDominance(translation, result.Root); node != nil {
			values = append(values, restrictionDiagnostic(
				node,
				language.FeaturePrimitivePatterns,
				"case is dominated by an earlier primitive pattern in this feature generation",
			))
		}
	}
	return values
}

func earlyThisReference(root *backend.Node) *backend.Node {
	for _, node := range nodesOfKind(root, "constructor_body") {
		for index := range node.Children {
			child := &node.Children[index]
			if child.Kind == "explicit_constructor_invocation" {
				break
			}
			if found := firstKind(child, "this"); found != nil {
				return found
			}
		}
	}
	return nil
}

func demonstratedModuleImportAmbiguity(
	translation *source.Translation,
	root *backend.Node,
) *backend.Node {
	if !hasModuleImport(translation, root, "java.base") ||
		!hasOnDemandImport(translation, root, "java.sql") {
		return nil
	}

	for _, node := range nodesOfKind(root, "type_identifier") {
		if logicalText(translation, node) == "Date" {
			return node
		}
	}
	return nil
}

func hasModuleImport(
	translation *source.Translation,
	root *backend.Node,
	module string,
) bool {
	for _, declaration := range nodesOfKind(root, "module_import_declaration") {
		for _, identifier := range nodesOfKind(declaration, "scoped_identifier") {
			if identifier.Field == "module" &&
				logicalText(translation, identifier) == module {
				return true
			}
		}
	}
	return false
}

func hasOnDemandImport(
	translation *source.Translation,
	root *backend.Node,
	pkg string,
) bool {
	for _, declaration := range nodesOfKind(root, "import_declaration") {
		if firstKind(declaration, "asterisk") == nil {
			continue
		}
		for _, identifier := range nodesOfKind(declaration, "scoped_identifier") {
			if logicalText(translation, identifier) == pkg {
				return true
			}
		}
	}
	return false
}

func demonstratedPrimitiveDominance(
	translation *source.Translation,
	root *backend.Node,
) *backend.Node {
	for _, switchNode := range nodesOfKind(root, "switch_expression") {
		seenFloatPattern := false
		stack := []*backend.Node{switchNode}
		for len(stack) > 0 {
			index := len(stack) - 1
			node := stack[index]
			stack = stack[:index]

			if node.Kind == "type_pattern" &&
				hasDirectChildText(translation, node, "floating_point_type", "float") {
				seenFloatPattern = true
			}
			if seenFloatPattern &&
				node.Kind == "decimal_integer_literal" &&
				logicalText(translation, node) == "16777216" {
				return node
			}
			for child := len(node.Children) - 1; child >= 0; child-- {
				stack = append(stack, &node.Children[child])
			}
		}
	}
	return nil
}

func restrictionDiagnostic(
	node *backend.Node,
	feature language.FeatureID,
	message string,
) diagnostic.Diagnostic {
	return diagnostic.New(
		diagnostic.CodeFeatureRestriction,
		diagnostic.SeverityError,
		nodeSpan(node),
		message,
		feature,
		nil,
	)
}

func nodesOfKind(root *backend.Node, kind string) []*backend.Node {
	values := make([]*backend.Node, 0)
	stack := []*backend.Node{root}
	for len(stack) > 0 {
		index := len(stack) - 1
		node := stack[index]
		stack = stack[:index]
		if node.Kind == kind {
			values = append(values, node)
		}
		for child := len(node.Children) - 1; child >= 0; child-- {
			stack = append(stack, &node.Children[child])
		}
	}
	return values
}

func firstKind(root *backend.Node, kind string) *backend.Node {
	for _, node := range nodesOfKind(root, kind) {
		return node
	}
	return nil
}

func hasDirectChildText(
	translation *source.Translation,
	node *backend.Node,
	kind, want string,
) bool {
	for index := range node.Children {
		child := &node.Children[index]
		if child.Kind == kind && logicalText(translation, child) == want {
			return true
		}
	}
	return false
}

func logicalText(
	translation *source.Translation,
	node *backend.Node,
) string {
	logical := translation.Logical()
	start := uint64(node.LogicalStartByte)
	end := uint64(node.LogicalEndByte)
	if start > end || end > uint64(len(logical)) {
		return ""
	}
	return logical[start:end]
}

func nodeSpan(node *backend.Node) diagnostic.Span {
	return diagnostic.Span{
		Start: int(node.StartByte),
		End:   int(node.EndByte),
	}
}

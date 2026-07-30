// Package syntaxgen generates Java syntax kinds and typed red views.
package syntaxgen

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"go/format"
	"go/token"
	"io"
	"os"
	"strings"
	"unicode"
)

// Schema is the checked-in description of generated Java syntax views.
type Schema struct {
	SchemaVersion int            `json:"schemaVersion"`
	Kinds         []Kind         `json:"kinds"`
	Nodes         []Node         `json:"nodes"`
	Unions        []Union        `json:"unions"`
	ElementUnions []ElementUnion `json:"elementUnions,omitempty"`
	Accessors     []Accessor     `json:"accessors"`
}

// Kind defines one stable repository-owned node or token kind.
type Kind struct {
	Element string `json:"element"`
	Name    string `json:"name"`
	GoName  string `json:"goName"`
}

// Node defines one concrete typed node view.
type Node struct {
	Type string `json:"type"`
	Kind string `json:"kind"`
}

// Union defines one typed view accepted by multiple concrete node kinds.
type Union struct {
	Type    string   `json:"type"`
	Members []string `json:"members"`
}

// ElementUnion defines a typed view over node and token alternatives.
type ElementUnion struct {
	Type   string   `json:"type"`
	Nodes  []string `json:"nodes,omitempty"`
	Tokens []string `json:"tokens,omitempty"`
}

// Accessor defines one direct-child typed accessor.
type Accessor struct {
	Owner       string `json:"owner"`
	Name        string `json:"name"`
	Target      string `json:"target"`
	Element     string `json:"element,omitempty"`
	Cardinality string `json:"cardinality"`
}

// AppendKind adds one stable kind when it is not already registered.
func (s *Schema) AppendKind(element, name string) error {
	if s == nil {
		return errors.New("append syntax kind: nil schema")
	}
	key := element + ":" + name
	for _, kind := range s.Kinds {
		if kind.Element+":"+kind.Name == key {
			return nil
		}
	}
	goName, err := suggestedKindGoName(name)
	if err != nil {
		return fmt.Errorf("append syntax kind %s: %w", key, err)
	}
	for _, kind := range s.Kinds {
		if kind.Element == element && kind.GoName == goName {
			return fmt.Errorf(
				"append syntax kind %s: Go name %s conflicts with %s:%s",
				key,
				goName,
				kind.Element,
				kind.Name,
			)
		}
	}
	s.Kinds = append(s.Kinds, Kind{
		Element: element,
		Name:    name,
		GoName:  goName,
	})
	return nil
}

// Marshal returns the canonical indented schema representation.
func Marshal(schema Schema) ([]byte, error) {
	if err := schema.Validate(); err != nil {
		return nil, err
	}
	data, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal syntax schema: %w", err)
	}
	return append(data, '\n'), nil
}

// Load reads and validates one schema file.
func Load(path string) (Schema, error) {
	file, err := os.Open(path)
	if err != nil {
		return Schema{}, fmt.Errorf("open syntax schema: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()

	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	var schema Schema
	if err := decoder.Decode(&schema); err != nil {
		return Schema{}, fmt.Errorf("decode syntax schema: %w", err)
	}
	if err := ensureJSONEnd(decoder); err != nil {
		return Schema{}, err
	}
	if err := schema.Validate(); err != nil {
		return Schema{}, err
	}
	return schema, nil
}

func ensureJSONEnd(decoder *json.Decoder) error {
	var trailing any
	err := decoder.Decode(&trailing)
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("decode syntax schema trailing content: %w", err)
	}
	return errors.New("decode syntax schema: multiple JSON values")
}

// Validate checks schema references and generated Go identifiers.
func (s Schema) Validate() error {
	if s.SchemaVersion != 1 {
		return fmt.Errorf("validate syntax schema: unsupported version %d", s.SchemaVersion)
	}
	if len(s.Kinds) == 0 {
		return errors.New("validate syntax schema: no kinds")
	}
	if len(s.Nodes) == 0 {
		return errors.New("validate syntax schema: no nodes")
	}

	kindKeys := make(map[string]Kind, len(s.Kinds))
	kindIdentifiers := make(map[string]string, len(s.Kinds))
	for _, kind := range s.Kinds {
		if kind.Element != "node" && kind.Element != "token" {
			return fmt.Errorf(
				"validate syntax schema: kind %q has invalid element %q",
				kind.Name,
				kind.Element,
			)
		}
		if kind.Name == "" {
			return errors.New("validate syntax schema: kind has empty name")
		}
		if err := validateIdentifier("kind Go name", kind.GoName); err != nil {
			return err
		}
		key := kind.Element + ":" + kind.Name
		if _, ok := kindKeys[key]; ok {
			return fmt.Errorf("validate syntax schema: duplicate kind %q", key)
		}
		identifier := kind.Element + ":" + kind.GoName
		if prior, ok := kindIdentifiers[identifier]; ok {
			return fmt.Errorf(
				"validate syntax schema: kind Go name %s is shared by %q and %q",
				kind.GoName,
				prior,
				key,
			)
		}
		kindKeys[key] = kind
		kindIdentifiers[identifier] = key
	}

	types := make(map[string]string, len(s.Nodes)+len(s.Unions)+len(s.ElementUnions))
	nodeKinds := make(map[string]string, len(s.Nodes))
	for _, node := range s.Nodes {
		if err := validateIdentifier("node type", node.Type); err != nil {
			return err
		}
		if node.Kind == "" {
			return fmt.Errorf("validate syntax schema: node %s has empty kind", node.Type)
		}
		if _, ok := kindKeys["node:"+node.Kind]; !ok {
			return fmt.Errorf(
				"validate syntax schema: node %s uses unregistered kind %q",
				node.Type,
				node.Kind,
			)
		}
		if prior, ok := types[node.Type]; ok {
			return fmt.Errorf(
				"validate syntax schema: duplicate type %s (%s and node)",
				node.Type,
				prior,
			)
		}
		if prior, ok := nodeKinds[node.Kind]; ok {
			return fmt.Errorf(
				"validate syntax schema: kind %q belongs to both %s and %s",
				node.Kind,
				prior,
				node.Type,
			)
		}
		types[node.Type] = "node"
		nodeKinds[node.Kind] = node.Type
	}

	for _, union := range s.Unions {
		if err := validateIdentifier("union type", union.Type); err != nil {
			return err
		}
		if _, ok := types[union.Type]; ok {
			return fmt.Errorf("validate syntax schema: duplicate type %s", union.Type)
		}
		if len(union.Members) == 0 {
			return fmt.Errorf("validate syntax schema: union %s has no members", union.Type)
		}
		seen := make(map[string]struct{}, len(union.Members))
		for _, member := range union.Members {
			if _, ok := seen[member]; ok {
				return fmt.Errorf(
					"validate syntax schema: union %s repeats %s",
					union.Type,
					member,
				)
			}
			if types[member] != "node" {
				return fmt.Errorf(
					"validate syntax schema: union %s references unknown node %s",
					union.Type,
					member,
				)
			}
			seen[member] = struct{}{}
		}
		types[union.Type] = "union"
	}

	for _, union := range s.ElementUnions {
		if err := validateIdentifier("element union type", union.Type); err != nil {
			return err
		}
		if _, ok := types[union.Type]; ok {
			return fmt.Errorf("validate syntax schema: duplicate type %s", union.Type)
		}
		if len(union.Nodes) == 0 && len(union.Tokens) == 0 {
			return fmt.Errorf(
				"validate syntax schema: element union %s has no alternatives",
				union.Type,
			)
		}
		seen := make(map[string]struct{}, len(union.Nodes)+len(union.Tokens))
		for _, member := range union.Nodes {
			key := "node:" + member
			if _, ok := seen[key]; ok {
				return fmt.Errorf(
					"validate syntax schema: element union %s repeats %s",
					union.Type,
					key,
				)
			}
			if _, ok := kindKeys[key]; !ok {
				return fmt.Errorf(
					"validate syntax schema: element union %s references unknown kind %s",
					union.Type,
					key,
				)
			}
			seen[key] = struct{}{}
		}
		for _, member := range union.Tokens {
			key := "token:" + member
			if _, ok := seen[key]; ok {
				return fmt.Errorf(
					"validate syntax schema: element union %s repeats %s",
					union.Type,
					key,
				)
			}
			if _, ok := kindKeys[key]; !ok {
				return fmt.Errorf(
					"validate syntax schema: element union %s references unknown kind %s",
					union.Type,
					key,
				)
			}
			seen[key] = struct{}{}
		}
		types[union.Type] = "element"
	}

	methods := make(map[string]struct{}, len(s.Accessors))
	for _, accessor := range s.Accessors {
		if types[accessor.Owner] != "node" {
			return fmt.Errorf(
				"validate syntax schema: accessor owner %s is not a node",
				accessor.Owner,
			)
		}
		if err := validateIdentifier("accessor name", accessor.Name); err != nil {
			return err
		}
		key := accessor.Owner + "." + accessor.Name
		if _, ok := methods[key]; ok {
			return fmt.Errorf("validate syntax schema: duplicate accessor %s", key)
		}
		methods[key] = struct{}{}
		switch accessor.Element {
		case "", "node":
			targetType, ok := types[accessor.Target]
			if !ok || (targetType != "node" && targetType != "union" && targetType != "element") {
				return fmt.Errorf(
					"validate syntax schema: accessor %s targets unknown type %s",
					key,
					accessor.Target,
				)
			}
		case "token":
			if _, ok := kindKeys["token:"+accessor.Target]; !ok {
				return fmt.Errorf(
					"validate syntax schema: token accessor %s has unknown kind %q",
					key,
					accessor.Target,
				)
			}
		default:
			return fmt.Errorf(
				"validate syntax schema: accessor %s has invalid element %q",
				key,
				accessor.Element,
			)
		}
		switch accessor.Cardinality {
		case "optional", "many":
		default:
			return fmt.Errorf(
				"validate syntax schema: accessor %s has invalid cardinality %q",
				key,
				accessor.Cardinality,
			)
		}
	}
	return nil
}

// ValidateCompatible checks that s preserves the generated API and stable kind
// identities from previous. New kinds must be appended; new views and
// accessors may be added without changing existing definitions.
func (s Schema) ValidateCompatible(previous Schema) error {
	if err := previous.Validate(); err != nil {
		return fmt.Errorf("validate previous syntax schema: %w", err)
	}
	if err := s.Validate(); err != nil {
		return err
	}
	if s.SchemaVersion != previous.SchemaVersion {
		return fmt.Errorf(
			"validate syntax schema compatibility: version changed from %d to %d",
			previous.SchemaVersion,
			s.SchemaVersion,
		)
	}
	if len(s.Kinds) < len(previous.Kinds) {
		return errors.New(
			"validate syntax schema compatibility: stable kind table was shortened",
		)
	}
	for index, oldKind := range previous.Kinds {
		if newKind := s.Kinds[index]; newKind != oldKind {
			return fmt.Errorf(
				"validate syntax schema compatibility: stable kind %d changed from %s:%s/%s to %s:%s/%s",
				index,
				oldKind.Element,
				oldKind.Name,
				oldKind.GoName,
				newKind.Element,
				newKind.Name,
				newKind.GoName,
			)
		}
	}

	currentNodes := make(map[string]Node, len(s.Nodes))
	for _, node := range s.Nodes {
		currentNodes[node.Type] = node
	}
	for _, oldNode := range previous.Nodes {
		newNode, ok := currentNodes[oldNode.Type]
		if !ok {
			return fmt.Errorf(
				"validate syntax schema compatibility: node %s was removed",
				oldNode.Type,
			)
		}
		if newNode != oldNode {
			return fmt.Errorf(
				"validate syntax schema compatibility: node %s changed kind from %q to %q",
				oldNode.Type,
				oldNode.Kind,
				newNode.Kind,
			)
		}
	}

	currentUnions := make(map[string]Union, len(s.Unions))
	for _, union := range s.Unions {
		currentUnions[union.Type] = union
	}
	for _, oldUnion := range previous.Unions {
		newUnion, ok := currentUnions[oldUnion.Type]
		if !ok {
			return fmt.Errorf(
				"validate syntax schema compatibility: union %s was removed",
				oldUnion.Type,
			)
		}
		if missing := missingStrings(oldUnion.Members, newUnion.Members); len(missing) != 0 {
			return fmt.Errorf(
				"validate syntax schema compatibility: union %s removed members %q",
				oldUnion.Type,
				missing,
			)
		}
	}

	currentElementUnions := make(map[string]ElementUnion, len(s.ElementUnions))
	for _, union := range s.ElementUnions {
		currentElementUnions[union.Type] = union
	}
	for _, oldUnion := range previous.ElementUnions {
		newUnion, ok := currentElementUnions[oldUnion.Type]
		if !ok {
			return fmt.Errorf(
				"validate syntax schema compatibility: element union %s was removed",
				oldUnion.Type,
			)
		}
		if missing := missingStrings(oldUnion.Nodes, newUnion.Nodes); len(missing) != 0 {
			return fmt.Errorf(
				"validate syntax schema compatibility: element union %s removed node alternatives %q",
				oldUnion.Type,
				missing,
			)
		}
		if missing := missingStrings(oldUnion.Tokens, newUnion.Tokens); len(missing) != 0 {
			return fmt.Errorf(
				"validate syntax schema compatibility: element union %s removed token alternatives %q",
				oldUnion.Type,
				missing,
			)
		}
	}

	currentAccessors := make(map[string]Accessor, len(s.Accessors))
	for _, accessor := range s.Accessors {
		currentAccessors[accessor.Owner+"."+accessor.Name] = accessor
	}
	for _, oldAccessor := range previous.Accessors {
		key := oldAccessor.Owner + "." + oldAccessor.Name
		newAccessor, ok := currentAccessors[key]
		if !ok {
			return fmt.Errorf(
				"validate syntax schema compatibility: accessor %s was removed",
				key,
			)
		}
		if newAccessor != oldAccessor {
			return fmt.Errorf(
				"validate syntax schema compatibility: accessor %s changed",
				key,
			)
		}
	}
	return nil
}

func missingStrings(oldValues, newValues []string) []string {
	current := make(map[string]struct{}, len(newValues))
	for _, value := range newValues {
		current[value] = struct{}{}
	}
	var missing []string
	for _, value := range oldValues {
		if _, ok := current[value]; !ok {
			missing = append(missing, value)
		}
	}
	return missing
}

func validateIdentifier(description, value string) error {
	if !token.IsIdentifier(value) || !unicode.IsUpper([]rune(value)[0]) {
		return fmt.Errorf(
			"validate syntax schema: %s %q is not an exported Go identifier",
			description,
			value,
		)
	}
	return nil
}

// Generate returns gofmt-formatted syntax and ast source.
func Generate(schema Schema) (syntaxSource, astSource []byte, err error) {
	if err := schema.Validate(); err != nil {
		return nil, nil, err
	}
	syntaxSource, err = generateKinds(schema)
	if err != nil {
		return nil, nil, err
	}
	astSource, err = generateViews(schema)
	if err != nil {
		return nil, nil, err
	}
	return syntaxSource, astSource, nil
}

func generateKinds(schema Schema) ([]byte, error) {
	var output bytes.Buffer
	output.WriteString("// Code generated by syntaxgen; DO NOT EDIT.\n\n")
	output.WriteString("package syntax\n\n")
	output.WriteString("const (\n")
	output.WriteString("\tKindUnknown Kind = \"\"\n")
	for _, kind := range schema.Kinds {
		fmt.Fprintf(
			&output,
			"\tKind%s%s Kind = %q\n",
			kindIdentifier(kind.Element),
			kind.GoName,
			kind.Element+":"+kind.Name,
		)
	}
	output.WriteString(")\n\n")

	output.WriteString("func (kind Kind) Valid() bool {\n")
	output.WriteString("\treturn kind.Category() != KindCategoryUnknown\n")
	output.WriteString("}\n\n")
	output.WriteString("func (kind Kind) Category() KindCategory {\n")
	output.WriteString("\tswitch kind {\n")
	output.WriteString("\tcase\n")
	writeKindCases(&output, schema.Kinds, "node")
	output.WriteString(":\n\t\treturn KindCategoryNode\n")
	output.WriteString("\tcase\n")
	writeKindCases(&output, schema.Kinds, "token")
	output.WriteString(":\n\t\treturn KindCategoryToken\n")
	output.WriteString("\tdefault:\n\t\treturn KindCategoryUnknown\n\t}\n}\n\n")

	output.WriteString("func LookupNodeKind(name string) (Kind, bool) {\n")
	output.WriteString("\tswitch name {\n")
	writeKindLookupCases(&output, schema.Kinds, "node")
	output.WriteString("\tdefault:\n\t\treturn KindUnknown, false\n\t}\n}\n\n")
	output.WriteString("func LookupTokenKind(name string) (Kind, bool) {\n")
	output.WriteString("\tswitch name {\n")
	writeKindLookupCases(&output, schema.Kinds, "token")
	output.WriteString("\tdefault:\n\t\treturn KindUnknown, false\n\t}\n}\n")
	return formatGenerated("syntax kinds", output.Bytes())
}

func writeKindCases(output *bytes.Buffer, kinds []Kind, element string) {
	first := true
	for _, kind := range kinds {
		if kind.Element != element {
			continue
		}
		if !first {
			output.WriteString(",\n")
		}
		fmt.Fprintf(
			output,
			"\t\tKind%s%s",
			kindIdentifier(kind.Element),
			kind.GoName,
		)
		first = false
	}
}

func writeKindLookupCases(output *bytes.Buffer, kinds []Kind, element string) {
	for _, kind := range kinds {
		if kind.Element != element {
			continue
		}
		fmt.Fprintf(
			output,
			"\tcase %q:\n\t\treturn Kind%s%s, true\n",
			kind.Name,
			kindIdentifier(kind.Element),
			kind.GoName,
		)
	}
}

func generateViews(schema Schema) ([]byte, error) {
	nodes := make(map[string]Node, len(schema.Nodes))
	typeCategories := make(map[string]string, len(schema.Nodes)+len(schema.Unions)+len(schema.ElementUnions))
	kindNames := make(map[string]string, len(schema.Kinds))
	for _, kind := range schema.Kinds {
		kindNames[kind.Element+":"+kind.Name] = kind.GoName
	}
	for _, node := range schema.Nodes {
		nodes[node.Type] = node
		typeCategories[node.Type] = "node"
	}
	for _, union := range schema.Unions {
		typeCategories[union.Type] = "node"
	}
	for _, union := range schema.ElementUnions {
		typeCategories[union.Type] = "element"
	}

	var output bytes.Buffer
	output.WriteString("// Code generated by syntaxgen; DO NOT EDIT.\n\n")
	output.WriteString("package ast\n\n")
	output.WriteString("import (\n")
	output.WriteString("\t\"iter\"\n\n")
	output.WriteString("\t\"git.alberto.engineer/alberto/java-cst-go/syntax\"\n")
	output.WriteString(")\n\n")

	for _, node := range schema.Nodes {
		fmt.Fprintf(&output, "type %s struct {\n\tnode syntax.Node\n}\n\n", node.Type)
		fmt.Fprintf(
			&output,
			"func As%s(node syntax.Node) (%s, bool) {\n"+
				"\tif node.Kind() != syntax.KindNode%s {\n"+
				"\t\treturn %s{}, false\n\t}\n"+
				"\treturn %s{node: node}, true\n}\n\n",
			node.Type,
			node.Type,
			kindNames["node:"+node.Kind],
			node.Type,
			node.Type,
		)
		fmt.Fprintf(
			&output,
			"func (value %s) Node() syntax.Node {\n\treturn value.node\n}\n\n",
			node.Type,
		)
	}

	for _, union := range schema.Unions {
		fmt.Fprintf(&output, "type %s struct {\n\tnode syntax.Node\n}\n\n", union.Type)
		fmt.Fprintf(
			&output,
			"func As%s(node syntax.Node) (%s, bool) {\n\tswitch node.Kind() {\n",
			union.Type,
			union.Type,
		)
		for _, member := range union.Members {
			fmt.Fprintf(
				&output,
				"\tcase syntax.KindNode%s:\n"+
					"\t\treturn %s{node: node}, true\n",
				kindNames["node:"+nodes[member].Kind],
				union.Type,
			)
		}
		fmt.Fprintf(
			&output,
			"\tdefault:\n\t\treturn %s{}, false\n\t}\n}\n\n",
			union.Type,
		)
		fmt.Fprintf(
			&output,
			"func (value %s) Node() syntax.Node {\n\treturn value.node\n}\n\n"+
				"func (value %s) Kind() syntax.Kind {\n\treturn value.node.Kind()\n}\n\n",
			union.Type,
			union.Type,
		)
		for _, member := range union.Members {
			fmt.Fprintf(
				&output,
				"func (value %s) As%s() (%s, bool) {\n"+
					"\treturn As%s(value.node)\n}\n\n",
				union.Type,
				member,
				member,
				member,
			)
		}
	}

	for _, union := range schema.ElementUnions {
		generateElementUnion(&output, union, kindNames)
	}

	for _, accessor := range schema.Accessors {
		switch {
		case accessor.Element == "token" && accessor.Cardinality == "optional":
			generateOptionalTokenAccessor(&output, accessor, kindNames)
		case accessor.Element == "token" && accessor.Cardinality == "many":
			generateManyTokenAccessor(&output, accessor, kindNames)
		case typeCategories[accessor.Target] == "element" &&
			accessor.Cardinality == "optional":
			generateOptionalElementAccessor(&output, accessor)
		case typeCategories[accessor.Target] == "element" &&
			accessor.Cardinality == "many":
			generateManyElementAccessor(&output, accessor)
		case accessor.Cardinality == "optional":
			generateOptionalNodeAccessor(&output, accessor)
		case accessor.Cardinality == "many":
			generateManyNodeAccessor(&output, accessor)
		}
	}

	return formatGenerated("typed views", output.Bytes())
}

func generateElementUnion(
	output *bytes.Buffer,
	union ElementUnion,
	kindNames map[string]string,
) {
	fmt.Fprintf(output, "type %s struct {\n\telement syntax.Element\n}\n\n", union.Type)
	fmt.Fprintf(
		output,
		"func As%s(element syntax.Element) (%s, bool) {\n"+
			"\tif node, ok := element.Node(); ok {\n\t\tswitch node.Kind() {\n",
		union.Type,
		union.Type,
	)
	for _, kind := range union.Nodes {
		fmt.Fprintf(
			output,
			"\t\tcase syntax.KindNode%s:\n"+
				"\t\t\treturn %s{element: element}, true\n",
			kindNames["node:"+kind],
			union.Type,
		)
	}
	output.WriteString("\t\t}\n\t}\n")
	output.WriteString("\tif token, ok := element.Token(); ok {\n\t\tswitch token.Kind() {\n")
	for _, kind := range union.Tokens {
		fmt.Fprintf(
			output,
			"\t\tcase syntax.KindToken%s:\n"+
				"\t\t\treturn %s{element: element}, true\n",
			kindNames["token:"+kind],
			union.Type,
		)
	}
	fmt.Fprintf(
		output,
		"\t\t}\n\t}\n\treturn %s{}, false\n}\n\n"+
			"func (value %s) Element() syntax.Element {\n\treturn value.element\n}\n\n"+
			"func (value %s) Node() (syntax.Node, bool) {\n"+
			"\treturn value.element.Node()\n}\n\n"+
			"func (value %s) Token() (syntax.Token, bool) {\n"+
			"\treturn value.element.Token()\n}\n\n"+
			"func (value %s) Kind() syntax.Kind {\n"+
			"\tif node, ok := value.element.Node(); ok {\n\t\treturn node.Kind()\n\t}\n"+
			"\tif token, ok := value.element.Token(); ok {\n\t\treturn token.Kind()\n\t}\n"+
			"\treturn syntax.KindUnknown\n}\n\n"+
			"func (value %s) ID() syntax.ElementID {\n\treturn value.element.ID()\n}\n\n"+
			"func (value %s) Span() syntax.Span {\n\treturn value.element.Span()\n}\n\n"+
			"func (value %s) FullSpan() syntax.Span {\n\treturn value.element.FullSpan()\n}\n\n"+
			"func (value %s) Text() string {\n"+
			"\tif node, ok := value.element.Node(); ok {\n\t\treturn node.Text()\n\t}\n"+
			"\tif token, ok := value.element.Token(); ok {\n\t\treturn token.Text()\n\t}\n"+
			"\treturn \"\"\n}\n\n",
		union.Type,
		union.Type,
		union.Type,
		union.Type,
		union.Type,
		union.Type,
		union.Type,
		union.Type,
		union.Type,
	)
}

func generateOptionalNodeAccessor(output *bytes.Buffer, accessor Accessor) {
	fmt.Fprintf(
		output,
		"func (value %s) %s() (%s, bool) {\n"+
			"\tfor child := range value.node.ChildNodes() {\n"+
			"\t\tif result, ok := As%s(child); ok {\n"+
			"\t\t\treturn result, true\n\t\t}\n\t}\n"+
			"\treturn %s{}, false\n}\n\n",
		accessor.Owner,
		accessor.Name,
		accessor.Target,
		accessor.Target,
		accessor.Target,
	)
}

func generateManyNodeAccessor(output *bytes.Buffer, accessor Accessor) {
	fmt.Fprintf(
		output,
		"func (value %s) %s() iter.Seq[%s] {\n"+
			"\treturn func(yield func(%s) bool) {\n"+
			"\t\tfor child := range value.node.ChildNodes() {\n"+
			"\t\t\tif result, ok := As%s(child); ok && !yield(result) {\n"+
			"\t\t\t\treturn\n\t\t\t}\n\t\t}\n\t}\n}\n\n"+
			"func (value %s) %sSlice() []%s {\n"+
			"\tvar result []%s\n"+
			"\tfor item := range value.%s() {\n"+
			"\t\tresult = append(result, item)\n\t}\n"+
			"\treturn result\n}\n\n",
		accessor.Owner,
		accessor.Name,
		accessor.Target,
		accessor.Target,
		accessor.Target,
		accessor.Owner,
		accessor.Name,
		accessor.Target,
		accessor.Target,
		accessor.Name,
	)
}

func generateOptionalTokenAccessor(
	output *bytes.Buffer,
	accessor Accessor,
	kindNames map[string]string,
) {
	fmt.Fprintf(
		output,
		"func (value %s) %s() (syntax.Token, bool) {\n"+
			"\tfor token := range value.node.ChildTokens() {\n"+
			"\t\tif token.Kind() == syntax.KindToken%s {\n"+
			"\t\t\treturn token, true\n\t\t}\n\t}\n"+
			"\treturn syntax.Token{}, false\n}\n\n",
		accessor.Owner,
		accessor.Name,
		kindNames["token:"+accessor.Target],
	)
}

func generateManyTokenAccessor(
	output *bytes.Buffer,
	accessor Accessor,
	kindNames map[string]string,
) {
	fmt.Fprintf(
		output,
		"func (value %s) %s() iter.Seq[syntax.Token] {\n"+
			"\treturn func(yield func(syntax.Token) bool) {\n"+
			"\t\tfor token := range value.node.ChildTokens() {\n"+
			"\t\t\tif token.Kind() == syntax.KindToken%s && !yield(token) {\n"+
			"\t\t\t\treturn\n\t\t\t}\n\t\t}\n\t}\n}\n\n"+
			"func (value %s) %sSlice() []syntax.Token {\n"+
			"\tvar result []syntax.Token\n"+
			"\tfor item := range value.%s() {\n"+
			"\t\tresult = append(result, item)\n\t}\n"+
			"\treturn result\n}\n\n",
		accessor.Owner,
		accessor.Name,
		kindNames["token:"+accessor.Target],
		accessor.Owner,
		accessor.Name,
		accessor.Name,
	)
}

func generateOptionalElementAccessor(output *bytes.Buffer, accessor Accessor) {
	fmt.Fprintf(
		output,
		"func (value %s) %s() (%s, bool) {\n"+
			"\tfor child := range value.node.Children() {\n"+
			"\t\tif result, ok := As%s(child); ok {\n"+
			"\t\t\treturn result, true\n\t\t}\n\t}\n"+
			"\treturn %s{}, false\n}\n\n",
		accessor.Owner,
		accessor.Name,
		accessor.Target,
		accessor.Target,
		accessor.Target,
	)
}

func generateManyElementAccessor(output *bytes.Buffer, accessor Accessor) {
	fmt.Fprintf(
		output,
		"func (value %s) %s() iter.Seq[%s] {\n"+
			"\treturn func(yield func(%s) bool) {\n"+
			"\t\tfor child := range value.node.Children() {\n"+
			"\t\t\tif result, ok := As%s(child); ok && !yield(result) {\n"+
			"\t\t\t\treturn\n\t\t\t}\n\t\t}\n\t}\n}\n\n"+
			"func (value %s) %sSlice() []%s {\n"+
			"\tvar result []%s\n"+
			"\tfor item := range value.%s() {\n"+
			"\t\tresult = append(result, item)\n\t}\n"+
			"\treturn result\n}\n\n",
		accessor.Owner,
		accessor.Name,
		accessor.Target,
		accessor.Target,
		accessor.Target,
		accessor.Owner,
		accessor.Name,
		accessor.Target,
		accessor.Target,
		accessor.Name,
	)
}

func kindIdentifier(kind string) string {
	var output strings.Builder
	upper := true
	for _, character := range kind {
		if unicode.IsLetter(character) || unicode.IsDigit(character) {
			if upper {
				character = unicode.ToUpper(character)
			}
			output.WriteRune(character)
			upper = false
			continue
		}
		upper = true
	}
	return output.String()
}

func suggestedKindGoName(name string) (string, error) {
	if replacement, ok := punctuationKindNames[name]; ok {
		return replacement, nil
	}
	prefix := ""
	if strings.HasPrefix(name, "_") {
		prefix = "Internal"
	}
	identifier := prefix + kindIdentifier(name)
	if !token.IsIdentifier(identifier) || identifier == "" {
		return "", fmt.Errorf("cannot derive exported Go name from %q", name)
	}
	return identifier, nil
}

var punctuationKindNames = map[string]string{
	"\"":         "DoubleQuote",
	"\"\"\"":     "TripleQuote",
	"\\{":        "BackslashLBrace",
	"}":          "RBrace",
	"(":          "LParen",
	")":          "RParen",
	"&":          "Ampersand",
	"=":          "Equals",
	"+=":         "PlusEquals",
	"-=":         "MinusEquals",
	"*=":         "StarEquals",
	"/=":         "SlashEquals",
	"&=":         "AmpersandEquals",
	"|=":         "PipeEquals",
	"^=":         "CaretEquals",
	"%=":         "PercentEquals",
	"<<=":        "LShiftEquals",
	">>=":        "RShiftEquals",
	">>>=":       "UnsignedRShiftEquals",
	">":          "GreaterThan",
	"<":          "LessThan",
	">=":         "GreaterThanEquals",
	"<=":         "LessThanEquals",
	"==":         "EqualsEquals",
	"!=":         "BangEquals",
	"&&":         "AmpersandAmpersand",
	"||":         "PipePipe",
	"+":          "Plus",
	"-":          "Minus",
	"*":          "Star",
	"/":          "Slash",
	"|":          "Pipe",
	"^":          "Caret",
	"%":          "Percent",
	"<<":         "LShift",
	">>":         "RShift",
	">>>":        "UnsignedRShift",
	"->":         "Arrow",
	",":          "Comma",
	"\\?":        "BackslashQuestion",
	":":          "Colon",
	"!":          "Bang",
	"~":          "Tilde",
	"++":         "PlusPlus",
	"--":         "MinusMinus",
	"[":          "LBracket",
	"]":          "RBracket",
	".":          "Dot",
	"::":         "ColonColon",
	"{":          "LBrace",
	";":          "Semicolon",
	"@":          "At",
	"@interface": "AnnotationInterface",
	"...":        "Ellipsis",
}

func formatGenerated(description string, source []byte) ([]byte, error) {
	formatted, err := format.Source(source)
	if err != nil {
		return nil, fmt.Errorf("format generated %s: %w\n%s", description, err, source)
	}
	return formatted, nil
}

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
	SchemaVersion int        `json:"schemaVersion"`
	Nodes         []Node     `json:"nodes"`
	Unions        []Union    `json:"unions"`
	Accessors     []Accessor `json:"accessors"`
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

// Accessor defines one direct-child typed accessor.
type Accessor struct {
	Owner       string `json:"owner"`
	Name        string `json:"name"`
	Target      string `json:"target"`
	Element     string `json:"element,omitempty"`
	Cardinality string `json:"cardinality"`
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
	if len(s.Nodes) == 0 {
		return errors.New("validate syntax schema: no nodes")
	}

	types := make(map[string]string, len(s.Nodes)+len(s.Unions))
	kinds := make(map[string]string, len(s.Nodes))
	for _, node := range s.Nodes {
		if err := validateIdentifier("node type", node.Type); err != nil {
			return err
		}
		if node.Kind == "" {
			return fmt.Errorf("validate syntax schema: node %s has empty kind", node.Type)
		}
		if prior, ok := types[node.Type]; ok {
			return fmt.Errorf(
				"validate syntax schema: duplicate type %s (%s and node)",
				node.Type,
				prior,
			)
		}
		if prior, ok := kinds[node.Kind]; ok {
			return fmt.Errorf(
				"validate syntax schema: kind %q belongs to both %s and %s",
				node.Kind,
				prior,
				node.Type,
			)
		}
		types[node.Type] = "node"
		kinds[node.Kind] = node.Type
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
			if _, ok := types[accessor.Target]; !ok {
				return fmt.Errorf(
					"validate syntax schema: accessor %s targets unknown type %s",
					key,
					accessor.Target,
				)
			}
		case "token":
			if accessor.Target == "" {
				return fmt.Errorf(
					"validate syntax schema: token accessor %s has empty kind",
					key,
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
	for _, node := range schema.Nodes {
		fmt.Fprintf(
			&output,
			"\tKindNode%s Kind = %q\n",
			kindIdentifier(node.Kind),
			"node:"+node.Kind,
		)
	}
	tokenKinds := tokenKindSet(schema)
	for _, kind := range tokenKinds {
		fmt.Fprintf(
			&output,
			"\tKindToken%s Kind = %q\n",
			kindIdentifier(kind),
			"token:"+kind,
		)
	}
	output.WriteString(")\n")
	return formatGenerated("syntax kinds", output.Bytes())
}

func tokenKindSet(schema Schema) []string {
	var result []string
	seen := make(map[string]struct{})
	for _, accessor := range schema.Accessors {
		if accessor.Element != "token" {
			continue
		}
		if _, ok := seen[accessor.Target]; ok {
			continue
		}
		seen[accessor.Target] = struct{}{}
		result = append(result, accessor.Target)
	}
	return result
}

func generateViews(schema Schema) ([]byte, error) {
	nodes := make(map[string]Node, len(schema.Nodes))
	for _, node := range schema.Nodes {
		nodes[node.Type] = node
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
			kindIdentifier(node.Kind),
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
				kindIdentifier(nodes[member].Kind),
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

	for _, accessor := range schema.Accessors {
		switch {
		case accessor.Element == "token" && accessor.Cardinality == "optional":
			generateOptionalTokenAccessor(&output, accessor)
		case accessor.Element == "token" && accessor.Cardinality == "many":
			generateManyTokenAccessor(&output, accessor)
		case accessor.Cardinality == "optional":
			generateOptionalNodeAccessor(&output, accessor)
		case accessor.Cardinality == "many":
			generateManyNodeAccessor(&output, accessor)
		}
	}

	return formatGenerated("typed views", output.Bytes())
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

func generateOptionalTokenAccessor(output *bytes.Buffer, accessor Accessor) {
	fmt.Fprintf(
		output,
		"func (value %s) %s() (syntax.Token, bool) {\n"+
			"\tfor token := range value.node.ChildTokens() {\n"+
			"\t\tif token.Kind() == syntax.KindToken%s {\n"+
			"\t\t\treturn token, true\n\t\t}\n\t}\n"+
			"\treturn syntax.Token{}, false\n}\n\n",
		accessor.Owner,
		accessor.Name,
		kindIdentifier(accessor.Target),
	)
}

func generateManyTokenAccessor(output *bytes.Buffer, accessor Accessor) {
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
		kindIdentifier(accessor.Target),
		accessor.Owner,
		accessor.Name,
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

func formatGenerated(description string, source []byte) ([]byte, error) {
	formatted, err := format.Source(source)
	if err != nil {
		return nil, fmt.Errorf("format generated %s: %w\n%s", description, err, source)
	}
	return formatted, nil
}

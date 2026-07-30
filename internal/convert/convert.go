package convert

import (
	"errors"
	"fmt"
	"math"

	cst "github.com/albertocavalcante/cst-go"

	"git.alberto.engineer/alberto/java-cst-go/internal/backend"
	javasource "git.alberto.engineer/alberto/java-cst-go/source"
	"git.alberto.engineer/alberto/java-cst-go/syntax"
)

// Result contains the lossless shared CST and conversion measurements.
type Result struct {
	Tree              *syntax.CoreTree
	BackendLeaves     int
	SyntaxTokens      int
	TriviaItems       int
	ConvertedElements uint32
}

// Convert reconstructs an immutable cst-go tree from one detached backend
// snapshot and the exact raw source parsed to produce it.
func Convert(source string, snapshot backend.Result) (Result, error) {
	return convert(conversionInput{
		raw: source,
		classifyTrivia: func(start, end uint32) ([]syntax.Trivia, error) {
			return classifyTrivia(source, start, end), nil
		},
		logicalText: func(leaf *backend.Node) (string, error) {
			return source[leaf.StartByte:leaf.EndByte], nil
		},
	}, snapshot)
}

// ConvertTranslation reconstructs a lossless raw CST from a backend snapshot
// produced from translation.Logical().
func ConvertTranslation(
	translation *javasource.Translation,
	snapshot backend.Result,
) (Result, error) {
	if translation == nil {
		return Result{}, errors.New("convert backend snapshot: nil source translation")
	}
	if snapshot.RawBytes != uint32(len(translation.Raw())) {
		return Result{}, fmt.Errorf(
			"convert backend snapshot: snapshot raw length is %d, translation raw length is %d",
			snapshot.RawBytes,
			len(translation.Raw()),
		)
	}
	if snapshot.LogicalBytes != uint32(len(translation.Logical())) {
		return Result{}, fmt.Errorf(
			"convert backend snapshot: snapshot logical length is %d, translation logical length is %d",
			snapshot.LogicalBytes,
			len(translation.Logical()),
		)
	}

	return convert(conversionInput{
		raw: translation.Raw(),
		classifyTrivia: func(start, end uint32) ([]syntax.Trivia, error) {
			return classifyTranslationTrivia(translation, start, end)
		},
		logicalText: func(leaf *backend.Node) (string, error) {
			if leaf.LogicalEndByte > uint32(len(translation.Logical())) ||
				leaf.LogicalStartByte > leaf.LogicalEndByte {
				return "", fmt.Errorf(
					"convert backend snapshot: token %q has invalid logical range [%d,%d)",
					leaf.Kind,
					leaf.LogicalStartByte,
					leaf.LogicalEndByte,
				)
			}
			return translation.Logical()[leaf.LogicalStartByte:leaf.LogicalEndByte], nil
		},
	}, snapshot)
}

type conversionInput struct {
	raw            string
	classifyTrivia func(start, end uint32) ([]syntax.Trivia, error)
	logicalText    func(leaf *backend.Node) (string, error)
}

func convert(input conversionInput, snapshot backend.Result) (Result, error) {
	if snapshot.Root == nil {
		return Result{}, errors.New("convert backend snapshot: nil root")
	}
	if len(input.raw) > math.MaxUint32 {
		return Result{}, fmt.Errorf(
			"convert backend snapshot: source is %d bytes, maximum is %d",
			len(input.raw),
			uint64(math.MaxUint32),
		)
	}
	if snapshot.RawBytes != 0 && snapshot.RawBytes != uint32(len(input.raw)) {
		return Result{}, fmt.Errorf(
			"convert backend snapshot: snapshot raw length is %d, source length is %d",
			snapshot.RawBytes,
			len(input.raw),
		)
	}
	if issues := snapshot.ValidateRanges(uint32(len(input.raw))); len(issues) != 0 {
		return Result{}, fmt.Errorf(
			"convert backend snapshot: invalid range at %s: %s",
			issues[0].Path,
			issues[0].Message,
		)
	}

	leaves := make([]*backend.Node, 0)
	if len(snapshot.Root.Children) != 0 {
		collectLeaves(snapshot.Root, &leaves)
	}

	models, triviaCount, err := buildTokenModels(input, leaves)
	if err != nil {
		return Result{}, err
	}
	tokens, err := buildTokens(models)
	if err != nil {
		return Result{}, err
	}

	var root *syntax.GreenNode
	if len(tokens) == 0 {
		return Result{}, errors.New("convert backend snapshot: internal token construction failure")
	}

	if len(leaves) == 0 || countSyntaxLeaves(leaves) == 0 {
		element, elementErr := cst.TokenElement(tokens[0])
		if elementErr != nil {
			return Result{}, fmt.Errorf("convert EOF token: %w", elementErr)
		}
		root, err = cst.NewGreenNode(
			syntax.NodeKind(snapshot.Root.Kind),
			[]syntax.GreenElement{element},
		)
		if err != nil {
			return Result{}, fmt.Errorf("convert trivia-only root: %w", err)
		}
	} else {
		cursor := tokenCursor{tokens: tokens}
		element, present, convertErr := convertNode(snapshot.Root, &cursor)
		if convertErr != nil {
			return Result{}, convertErr
		}
		if !present {
			return Result{}, errors.New("convert backend snapshot: root has no syntax")
		}
		node, ok := element.Node()
		if !ok {
			return Result{}, errors.New("convert backend snapshot: root converted to token")
		}
		if cursor.index != len(tokens) {
			return Result{}, fmt.Errorf(
				"convert backend snapshot: consumed %d of %d tokens",
				cursor.index,
				len(tokens),
			)
		}
		root = node
	}

	if got := root.FullText(); got != input.raw {
		return Result{}, fmt.Errorf(
			"convert backend snapshot: round trip differs: got %d bytes, want %d",
			len(got),
			len(input.raw),
		)
	}

	tree := cst.NewTree[
		syntax.Kind,
		syntax.TriviaKind,
		syntax.TokenData,
		syntax.DiagnosticCode,
	](root)
	if tree == nil {
		return Result{}, errors.New("convert backend snapshot: construct tree")
	}

	return Result{
		Tree:              tree,
		BackendLeaves:     len(leaves),
		SyntaxTokens:      len(tokens),
		TriviaItems:       triviaCount,
		ConvertedElements: root.ElementCount(),
	}, nil
}

type tokenModel struct {
	kind     syntax.Kind
	text     string
	leading  []syntax.Trivia
	trailing []syntax.Trivia
	data     syntax.TokenData
	missing  bool
}

func collectLeaves(node *backend.Node, leaves *[]*backend.Node) {
	if len(node.Children) == 0 {
		*leaves = append(*leaves, node)
		return
	}
	for index := range node.Children {
		collectLeaves(&node.Children[index], leaves)
	}
}

func countSyntaxLeaves(leaves []*backend.Node) int {
	count := 0
	for _, leaf := range leaves {
		if !leaf.Extra {
			count++
		}
	}
	return count
}

func buildTokenModels(
	input conversionInput,
	leaves []*backend.Node,
) ([]tokenModel, int, error) {
	syntaxLeaves := make([]*backend.Node, 0, len(leaves))
	for _, leaf := range leaves {
		if !leaf.Extra {
			syntaxLeaves = append(syntaxLeaves, leaf)
		}
	}

	if len(syntaxLeaves) == 0 {
		trivia, err := input.classifyTrivia(0, uint32(len(input.raw)))
		if err != nil {
			return nil, 0, err
		}
		return []tokenModel{{
			kind:    syntax.TokenKind("eof"),
			leading: trivia,
		}}, len(trivia), nil
	}

	models := make([]tokenModel, len(syntaxLeaves))
	triviaCount := 0
	var cursor uint32
	previousSourceToken := -1
	for index, leaf := range syntaxLeaves {
		models[index].kind = syntax.TokenKind(leaf.Kind)
		models[index].missing = leaf.Missing
		if leaf.Missing {
			if leaf.StartByte != leaf.EndByte {
				return nil, 0, fmt.Errorf(
					"convert backend snapshot: missing token %q has range [%d,%d)",
					leaf.Kind,
					leaf.StartByte,
					leaf.EndByte,
				)
			}
			continue
		}

		if leaf.StartByte < cursor {
			return nil, 0, fmt.Errorf(
				"convert backend snapshot: token %q starts at %d before cursor %d",
				leaf.Kind,
				leaf.StartByte,
				cursor,
			)
		}

		gap, err := input.classifyTrivia(cursor, leaf.StartByte)
		if err != nil {
			return nil, 0, err
		}
		triviaCount += len(gap)
		if previousSourceToken < 0 {
			models[index].leading = gap
		} else {
			trailing, leading := splitInterTokenTrivia(gap)
			models[previousSourceToken].trailing = trailing
			models[index].leading = leading
		}

		models[index].text = input.raw[leaf.StartByte:leaf.EndByte]
		logicalText, err := input.logicalText(leaf)
		if err != nil {
			return nil, 0, err
		}
		models[index].data.LogicalText = logicalText
		if leaf.EndByte > cursor {
			cursor = leaf.EndByte
		}
		previousSourceToken = index
	}

	if previousSourceToken < 0 {
		return nil, 0, errors.New(
			"convert backend snapshot: tree contains only missing syntax tokens",
		)
	}
	finalTrivia, err := input.classifyTrivia(cursor, uint32(len(input.raw)))
	if err != nil {
		return nil, 0, err
	}
	triviaCount += len(finalTrivia)
	models[previousSourceToken].trailing = finalTrivia

	return models, triviaCount, nil
}

func buildTokens(models []tokenModel) ([]*syntax.GreenToken, error) {
	tokens := make([]*syntax.GreenToken, len(models))
	for index := range models {
		model := models[index]
		if model.missing {
			if model.text != "" || len(model.leading) != 0 || len(model.trailing) != 0 {
				return nil, fmt.Errorf(
					"convert backend snapshot: missing token %q owns source text",
					model.kind,
				)
			}
			tokens[index] = cst.NewMissingToken[
				syntax.Kind,
				syntax.TriviaKind,
				syntax.TokenData,
			](model.kind, syntax.TokenData{})
			continue
		}

		token, err := cst.NewToken(cst.TokenSpec[
			syntax.Kind,
			syntax.TriviaKind,
			syntax.TokenData,
		]{
			Kind:     model.kind,
			Text:     model.text,
			Leading:  model.leading,
			Trailing: model.trailing,
			Data:     model.data,
		})
		if err != nil {
			return nil, fmt.Errorf("convert token %q: %w", model.kind, err)
		}
		tokens[index] = token
	}

	return tokens, nil
}

type tokenCursor struct {
	tokens []*syntax.GreenToken
	index  int
}

func convertNode(
	node *backend.Node,
	cursor *tokenCursor,
) (syntax.GreenElement, bool, error) {
	if len(node.Children) == 0 {
		if node.Extra {
			return syntax.GreenElement{}, false, nil
		}
		if cursor.index >= len(cursor.tokens) {
			return syntax.GreenElement{}, false, fmt.Errorf(
				"convert backend snapshot: no token for leaf %q",
				node.Kind,
			)
		}
		token := cursor.tokens[cursor.index]
		cursor.index++
		element, err := cst.TokenElement(token)
		if err != nil {
			return syntax.GreenElement{}, false, fmt.Errorf(
				"convert leaf %q: %w",
				node.Kind,
				err,
			)
		}
		return element, true, nil
	}

	children := make([]syntax.GreenElement, 0, len(node.Children))
	for index := range node.Children {
		child, present, err := convertNode(&node.Children[index], cursor)
		if err != nil {
			return syntax.GreenElement{}, false, err
		}
		if present {
			children = append(children, child)
		}
	}
	if len(children) == 0 {
		return syntax.GreenElement{}, false, nil
	}

	green, err := cst.NewGreenNode(syntax.NodeKind(node.Kind), children)
	if err != nil {
		return syntax.GreenElement{}, false, fmt.Errorf(
			"convert node %q: %w",
			node.Kind,
			err,
		)
	}
	element, err := cst.NodeElement(green)
	if err != nil {
		return syntax.GreenElement{}, false, fmt.Errorf(
			"convert node element %q: %w",
			node.Kind,
			err,
		)
	}

	return element, true, nil
}

package treesittergo

import (
	"context"
	"errors"
	"fmt"
	"math"

	dts "github.com/dcosson/treesitter-go"
	dparser "github.com/dcosson/treesitter-go/parser"

	"git.alberto.engineer/alberto/java-cst-go/internal/backend"
	"git.alberto.engineer/alberto/java-cst-go/internal/grammar/java25"
	"git.alberto.engineer/alberto/java-cst-go/language"
	"git.alberto.engineer/alberto/java-cst-go/source"
)

const (
	rawBackendName        = "treesitter-go/v0.1.0:token-source:raw"
	translatedBackendName = "treesitter-go/v0.1.0:token-source:logical"
	maxSnapshotDepth      = 4096
	maxSnapshotNodes      = 2_000_000
)

// Parse parses source with the selected pure-Go Tree-sitter runtime and
// returns a detached, repository-owned diagnostic snapshot.
func Parse(
	ctx context.Context,
	input []byte,
	level language.Level,
) (backend.Result, error) {
	if len(input) > math.MaxUint32 {
		return backend.Result{}, fmt.Errorf(
			"parse selected Java backend snapshot: source is %d bytes, maximum is %d",
			len(input),
			uint64(math.MaxUint32),
		)
	}

	return parseSnapshot(
		ctx,
		input,
		level,
		rawBackendName,
		uint32(len(input)),
		identityProjector,
	)
}

// ParseTranslation parses the logical Java Unicode-translated stream while
// projecting every returned range back to the exact raw source.
func ParseTranslation(
	ctx context.Context,
	translation *source.Translation,
	level language.Level,
) (backend.Result, error) {
	if translation == nil {
		return backend.Result{}, errors.New(
			"parse selected Java backend snapshot: nil source translation",
		)
	}
	if len(translation.Raw()) > math.MaxUint32 {
		return backend.Result{}, fmt.Errorf(
			"parse selected Java backend snapshot: raw source is %d bytes, maximum is %d",
			len(translation.Raw()),
			uint64(math.MaxUint32),
		)
	}
	if len(translation.Logical()) > math.MaxUint32 {
		return backend.Result{}, fmt.Errorf(
			"parse selected Java backend snapshot: logical source is %d bytes, maximum is %d",
			len(translation.Logical()),
			uint64(math.MaxUint32),
		)
	}

	project := func(start, end uint32) (uint32, uint32, bool) {
		rawSpan, ok := translation.RawSpan(source.Span{
			Start: int(start),
			End:   int(end),
		})
		if !ok {
			return 0, 0, false
		}
		return uint32(rawSpan.Start), uint32(rawSpan.End), true
	}

	return parseSnapshot(
		ctx,
		[]byte(translation.Logical()),
		level,
		translatedBackendName,
		uint32(len(translation.Raw())),
		project,
	)
}

type rangeProjector func(start, end uint32) (rawStart, rawEnd uint32, ok bool)

func identityProjector(start, end uint32) (uint32, uint32, bool) {
	return start, end, true
}

func parseSnapshot(
	ctx context.Context,
	input []byte,
	level language.Level,
	name string,
	rawBytes uint32,
	project rangeProjector,
) (backend.Result, error) {
	if !level.Valid() {
		return backend.Result{}, errors.New(
			"parse selected Java backend snapshot: invalid language level",
		)
	}
	if err := ctx.Err(); err != nil {
		return backend.Result{}, fmt.Errorf(
			"parse selected Java backend snapshot: %w",
			err,
		)
	}

	javaLanguage := java25.JavaLanguage()
	parser := dparser.NewParser()
	parser.SetLanguage(javaLanguage)
	tree := parser.ParseString(ctx, input)
	if tree == nil {
		if err := ctx.Err(); err != nil {
			return backend.Result{}, fmt.Errorf(
				"parse selected Java backend snapshot: %w",
				err,
			)
		}
		return backend.Result{}, errors.New(
			"parse selected Java backend snapshot: runtime returned nil tree",
		)
	}

	result := backend.Result{
		Level:        level,
		Backend:      name,
		RawBytes:     rawBytes,
		LogicalBytes: uint32(len(input)),
		StopReason:   "accepted",
	}

	root := tree.RootNode()
	if root.IsNull() {
		return result, nil
	}

	counts := snapshotCounts{}
	cursor := dts.NewTreeCursor(root)
	snapshot, err := snapshotCursor(
		&cursor,
		javaLanguage,
		project,
		0,
		&counts,
	)
	if err != nil {
		return backend.Result{}, err
	}
	result.Root = &snapshot
	result.NodeCount = counts.nodes
	result.ErrorCount = counts.errors
	result.MissingCount = counts.missing

	if err := ctx.Err(); err != nil {
		return result, fmt.Errorf(
			"parse selected Java backend snapshot: %w",
			err,
		)
	}
	return result, nil
}

type snapshotCounts struct {
	nodes   int
	errors  int
	missing int
}

func snapshotCursor(
	cursor *dts.TreeCursor,
	javaLanguage *dts.Language,
	project rangeProjector,
	depth int,
	counts *snapshotCounts,
) (backend.Node, error) {
	if depth > maxSnapshotDepth {
		return backend.Node{}, fmt.Errorf(
			"parse selected Java backend snapshot: depth exceeds %d",
			maxSnapshotDepth,
		)
	}

	node := cursor.CurrentNode()
	counts.nodes++
	if counts.nodes > maxSnapshotNodes {
		return backend.Node{}, fmt.Errorf(
			"parse selected Java backend snapshot: node count exceeds %d",
			maxSnapshotNodes,
		)
	}

	isError := node.Symbol() == dts.SymbolError
	if isError {
		counts.errors++
	}
	if node.IsMissing() {
		counts.missing++
	}

	rawStart, rawEnd, ok := project(node.StartByte(), node.EndByte())
	if !ok {
		return backend.Node{}, fmt.Errorf(
			"parse selected Java backend snapshot: cannot project %s logical range [%d,%d)",
			node.Type(),
			node.StartByte(),
			node.EndByte(),
		)
	}

	snapshot := backend.Node{
		Kind:             node.Type(),
		Field:            currentField(cursor, javaLanguage),
		StartByte:        rawStart,
		EndByte:          rawEnd,
		LogicalStartByte: node.StartByte(),
		LogicalEndByte:   node.EndByte(),
		Named:            node.IsNamed(),
		Extra:            node.IsExtra(),
		Missing:          node.IsMissing(),
		Error:            isError,
		HasError:         isError || node.IsMissing(),
	}

	if !cursor.GotoFirstChild() {
		return snapshot, nil
	}
	for {
		child, err := snapshotCursor(
			cursor,
			javaLanguage,
			project,
			depth+1,
			counts,
		)
		if err != nil {
			return backend.Node{}, err
		}
		snapshot.Children = append(snapshot.Children, child)
		snapshot.HasError = snapshot.HasError || child.HasError

		if !cursor.GotoNextSibling() {
			break
		}
	}
	if !cursor.GotoParent() {
		return backend.Node{}, errors.New(
			"parse selected Java backend snapshot: cursor lost parent",
		)
	}
	return snapshot, nil
}

func currentField(cursor *dts.TreeCursor, javaLanguage *dts.Language) string {
	if cursor == nil || cursor.Tree == nil || len(cursor.Stack) < 2 {
		return ""
	}

	arena := cursor.Tree.Arena()
	field := ""
	for index := 1; index < len(cursor.Stack); index++ {
		parent := cursor.Stack[index-1]
		child := cursor.Stack[index]

		if dts.IsVisible(parent.Subtree, arena) {
			field = ""
		}
		if dts.IsExtra(child.Subtree, arena) {
			field = ""
			continue
		}

		production := dts.GetProductionID(parent.Subtree, arena)
		entries := javaLanguage.FieldMapForProduction(production)
		for _, entry := range entries {
			if !entry.Inherited &&
				entry.ChildIndex == uint16(child.StructuralChildIndex) {
				field = javaLanguage.FieldName(entry.FieldID)
				break
			}
		}
	}
	return field
}

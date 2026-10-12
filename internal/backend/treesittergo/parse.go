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
)

// Parse parses source with the selected pure-Go Tree-sitter runtime and
// returns a detached, repository-owned diagnostic snapshot.
func Parse(
	ctx context.Context,
	input []byte,
	level language.Level,
) (backend.Result, error) {
	return ParseWithLimits(ctx, input, level, backend.DefaultLimits())
}

// ParseWithLimits parses raw source with explicit per-parse resource limits.
func ParseWithLimits(
	ctx context.Context,
	input []byte,
	level language.Level,
	limits backend.Limits,
) (backend.Result, error) {
	limits = backend.ResolveLimits(limits)
	if uint64(len(input)) > uint64(limits.MaxSourceBytes) {
		return backend.Result{}, &backend.LimitError{
			Kind:   backend.LimitSourceBytes,
			Limit:  uint64(limits.MaxSourceBytes),
			Actual: uint64(len(input)),
		}
	}
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
		limits,
	)
}

// ParseTranslation parses the logical Java Unicode-translated stream while
// projecting every returned range back to the exact raw source.
func ParseTranslation(
	ctx context.Context,
	translation *source.Translation,
	level language.Level,
) (backend.Result, error) {
	return ParseTranslationWithLimits(
		ctx,
		translation,
		level,
		backend.DefaultLimits(),
	)
}

// ParseTranslationWithLimits parses translated source with explicit per-parse
// resource limits.
func ParseTranslationWithLimits(
	ctx context.Context,
	translation *source.Translation,
	level language.Level,
	limits backend.Limits,
) (backend.Result, error) {
	if translation == nil {
		return backend.Result{}, errors.New(
			"parse selected Java backend snapshot: nil source translation",
		)
	}
	limits = backend.ResolveLimits(limits)
	if uint64(len(translation.Raw())) > uint64(limits.MaxSourceBytes) {
		return backend.Result{}, &backend.LimitError{
			Kind:   backend.LimitSourceBytes,
			Limit:  uint64(limits.MaxSourceBytes),
			Actual: uint64(len(translation.Raw())),
		}
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
		limits,
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
	limits backend.Limits,
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
		ctx,
		&cursor,
		javaLanguage,
		project,
		limits,
		0,
		newFieldTracker(javaLanguage),
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
	ctx context.Context,
	cursor *dts.TreeCursor,
	javaLanguage *dts.Language,
	project rangeProjector,
	limits backend.Limits,
	depth int,
	fields *fieldTracker,
	counts *snapshotCounts,
) (backend.Node, error) {
	if counts.nodes&1023 == 0 {
		if err := ctx.Err(); err != nil {
			return backend.Node{}, fmt.Errorf(
				"parse selected Java backend snapshot: %w",
				err,
			)
		}
	}
	if uint64(depth) > uint64(limits.MaxDepth) {
		return backend.Node{}, &backend.LimitError{
			Kind:   backend.LimitDepth,
			Limit:  uint64(limits.MaxDepth),
			Actual: uint64(depth),
		}
	}

	node := cursor.CurrentNode()
	counts.nodes++
	if uint64(counts.nodes) > uint64(limits.MaxNodes) {
		return backend.Node{}, &backend.LimitError{
			Kind:   backend.LimitNodes,
			Limit:  uint64(limits.MaxNodes),
			Actual: uint64(counts.nodes),
		}
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
		Field:            fields.sync(cursor),
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
			ctx,
			cursor,
			javaLanguage,
			project,
			limits,
			depth+1,
			fields,
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

// fieldStep folds stack entry index into field, which is the field state
// accumulated for entries [0, index). It is the single-step form of the
// original whole-stack walk, so callers can carry state incrementally.
func fieldStep(
	cursor *dts.TreeCursor,
	javaLanguage *dts.Language,
	index int,
	field string,
) string {
	arena := cursor.Tree.Arena()
	parent := cursor.Stack[index-1]
	child := cursor.Stack[index]

	if dts.IsVisible(parent.Subtree, arena) {
		field = ""
	}
	if dts.IsExtra(child.Subtree, arena) {
		return ""
	}

	production := dts.GetProductionID(parent.Subtree, arena)
	for _, entry := range javaLanguage.FieldMapForProduction(production) {
		if !entry.Inherited &&
			entry.ChildIndex == uint16(child.StructuralChildIndex) {
			return javaLanguage.FieldName(entry.FieldID)
		}
	}
	return field
}

// fieldTracker caches the field state for every entry of the cursor stack so
// that, after a cursor move, only the entries that changed are re-folded.
// Hidden repeat nodes can make the stack O(members) deep, so recomputing the
// whole stack per node is quadratic.
type fieldTracker struct {
	language *dts.Language
	entries  []dts.TreeCursorEntry
	fields   []string // fields[i] is the folded state after entry i
}

func newFieldTracker(language *dts.Language) *fieldTracker {
	return &fieldTracker{language: language}
}

// sync returns the field of the cursor's current node.
func (t *fieldTracker) sync(cursor *dts.TreeCursor) string {
	stack := cursor.Stack
	n := len(stack)
	if n < 2 {
		return ""
	}
	// Cursor moves pop to some ancestor and push anew, so the unchanged
	// prefix is found by scanning down from the top until entries match.
	if len(t.entries) == 0 {
		t.entries = append(t.entries, stack[0])
		t.fields = append(t.fields, "")
	}
	keep := len(t.entries)
	if n < keep {
		keep = n
	}
	for keep > 1 && t.entries[keep-1] != stack[keep-1] {
		keep--
	}
	t.entries = append(t.entries[:keep], stack[keep:]...)
	t.fields = t.fields[:keep]
	for index := keep; index < n; index++ {
		t.fields = append(t.fields, fieldStep(cursor, t.language, index, t.fields[index-1]))
	}
	return t.fields[n-1]
}

// currentField computes the field of the cursor's node from scratch. It is
// kept as the simple form for tests; the snapshot walk uses fieldTracker.
func currentField(cursor *dts.TreeCursor, javaLanguage *dts.Language) string {
	return newFieldTracker(javaLanguage).sync(cursor)
}

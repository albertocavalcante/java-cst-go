package treesitter

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync/atomic"

	gotreesitter "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"

	"git.alberto.engineer/alberto/java-cst-go/internal/backend"
	"git.alberto.engineer/alberto/java-cst-go/language"
)

const (
	backendName      = "gotreesitter/v0.47.0:token-source"
	maxSnapshotDepth = 4096
	maxSnapshotNodes = 2_000_000
)

// Parse parses source with the pinned Java grammar and returns a detached,
// repository-owned diagnostic snapshot.
func Parse(
	ctx context.Context,
	source []byte,
	level language.Level,
) (backend.Result, error) {
	if !level.Valid() {
		return backend.Result{}, errors.New("parse Java backend snapshot: invalid language level")
	}
	if err := ctx.Err(); err != nil {
		return backend.Result{}, fmt.Errorf("parse Java backend snapshot: %w", err)
	}
	if len(source) > math.MaxUint32 {
		return backend.Result{}, fmt.Errorf(
			"parse Java backend snapshot: source is %d bytes, maximum is %d",
			len(source),
			uint64(math.MaxUint32),
		)
	}

	java := grammars.JavaLanguage()
	if java == nil {
		return backend.Result{}, errors.New("parse Java backend snapshot: load Java grammar")
	}

	parser := gotreesitter.NewParser(java)
	var cancelled uint32
	parser.SetCancellationFlag(&cancelled)
	stopCancellation := context.AfterFunc(ctx, func() {
		atomic.StoreUint32(&cancelled, 1)
	})
	defer stopCancellation()

	tree, err := parser.ParseWithTokenSourceFactory(
		source,
		func(input []byte) (gotreesitter.TokenSource, error) {
			return grammars.NewJavaTokenSource(input, java)
		},
	)
	if err != nil {
		return backend.Result{}, fmt.Errorf("parse Java backend snapshot: %w", err)
	}
	if tree == nil {
		return backend.Result{}, errors.New("parse Java backend snapshot: runtime returned nil tree")
	}
	defer tree.Release()

	result := backend.Result{
		Level:        level,
		Backend:      backendName,
		StopReason:   string(tree.ParseStopReason()),
		StoppedEarly: tree.ParseStoppedEarly(),
	}

	root := tree.RootNode()
	if root == nil {
		return result, nil
	}

	counts := snapshotCounts{}
	snapshot, err := snapshotNode(root, "", java, 0, &counts)
	if err != nil {
		return backend.Result{}, err
	}
	result.Root = &snapshot
	result.NodeCount = counts.nodes
	result.ErrorCount = counts.errors
	result.MissingCount = counts.missing

	if err := ctx.Err(); err != nil {
		return result, fmt.Errorf("parse Java backend snapshot: %w", err)
	}

	return result, nil
}

type snapshotCounts struct {
	nodes   int
	errors  int
	missing int
}

func snapshotNode(
	node *gotreesitter.Node,
	field string,
	java *gotreesitter.Language,
	depth int,
	counts *snapshotCounts,
) (backend.Node, error) {
	if depth > maxSnapshotDepth {
		return backend.Node{}, fmt.Errorf(
			"parse Java backend snapshot: depth exceeds %d",
			maxSnapshotDepth,
		)
	}

	counts.nodes++
	if counts.nodes > maxSnapshotNodes {
		return backend.Node{}, fmt.Errorf(
			"parse Java backend snapshot: node count exceeds %d",
			maxSnapshotNodes,
		)
	}
	if node.IsError() {
		counts.errors++
	}
	if node.IsMissing() {
		counts.missing++
	}

	snapshot := backend.Node{
		Kind:      node.Type(java),
		Field:     field,
		StartByte: node.StartByte(),
		EndByte:   node.EndByte(),
		Named:     node.IsNamed(),
		Extra:     node.IsExtra(),
		Missing:   node.IsMissing(),
		Error:     node.IsError(),
		HasError:  node.HasError(),
	}

	childCount := node.ChildCount()
	if childCount == 0 {
		return snapshot, nil
	}
	snapshot.Children = make([]backend.Node, 0, childCount)
	for index := 0; index < childCount; index++ {
		child := node.Child(index)
		if child == nil {
			return backend.Node{}, fmt.Errorf(
				"parse Java backend snapshot: %s child %d is nil",
				snapshot.Kind,
				index,
			)
		}

		childSnapshot, err := snapshotNode(
			child,
			node.FieldNameForChild(index, java),
			java,
			depth+1,
			counts,
		)
		if err != nil {
			return backend.Node{}, err
		}
		snapshot.Children = append(snapshot.Children, childSnapshot)
	}

	return snapshot, nil
}

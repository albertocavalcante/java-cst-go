package javacst

import (
	"errors"
	"fmt"

	"git.alberto.engineer/alberto/java-cst-go/internal/backend"
)

const (
	// DefaultMaxSourceBytes is the default maximum raw input size: 16 MiB.
	DefaultMaxSourceBytes uint32 = backend.DefaultMaxSourceBytes
	// DefaultMaxNodes is the default maximum number of published snapshot nodes.
	DefaultMaxNodes uint32 = backend.DefaultMaxNodes
	// DefaultMaxDepth is the default maximum published snapshot depth.
	DefaultMaxDepth uint16 = backend.DefaultMaxDepth
)

// Limits bounds one parse. A zero field selects its documented default.
//
// MaxSourceBytes is enforced before runtime parsing. MaxNodes and MaxDepth
// bound conversion of the runtime tree into the detached CST snapshot.
type Limits struct {
	MaxSourceBytes uint32
	MaxNodes       uint32
	MaxDepth       uint16
}

// LimitKind identifies one exhausted per-parse resource.
type LimitKind string

const (
	LimitSourceBytes LimitKind = "source bytes"
	LimitNodes       LimitKind = "nodes"
	LimitDepth       LimitKind = "depth"
)

// ErrLimitExceeded identifies a parser-local source, node, or depth limit.
var ErrLimitExceeded = errors.New("java parser resource limit exceeded")

// LimitError reports which per-parse resource was exhausted.
type LimitError struct {
	Kind   LimitKind
	Limit  uint64
	Actual uint64
}

// Error implements error.
func (e *LimitError) Error() string {
	if e == nil {
		return ErrLimitExceeded.Error()
	}
	return fmt.Sprintf(
		"java parser %s limit exceeded: got %d, maximum %d",
		e.Kind,
		e.Actual,
		e.Limit,
	)
}

// Unwrap makes every LimitError match ErrLimitExceeded with errors.Is.
func (e *LimitError) Unwrap() error {
	return ErrLimitExceeded
}

func (l Limits) resolve() backend.Limits {
	return backend.ResolveLimits(backend.Limits{
		MaxSourceBytes: l.MaxSourceBytes,
		MaxNodes:       l.MaxNodes,
		MaxDepth:       l.MaxDepth,
	})
}

package backend

import "fmt"

const (
	DefaultMaxSourceBytes uint32 = 16 << 20
	DefaultMaxNodes       uint32 = 2_000_000
	DefaultMaxDepth       uint16 = 4096
)

// Limits bounds one backend parse.
type Limits struct {
	MaxSourceBytes uint32
	MaxNodes       uint32
	MaxDepth       uint16
}

// DefaultLimits returns the repository's bounded parser defaults.
func DefaultLimits() Limits {
	return Limits{
		MaxSourceBytes: DefaultMaxSourceBytes,
		MaxNodes:       DefaultMaxNodes,
		MaxDepth:       DefaultMaxDepth,
	}
}

// ResolveLimits fills each zero field with its default.
func ResolveLimits(input Limits) Limits {
	defaults := DefaultLimits()
	if input.MaxSourceBytes == 0 {
		input.MaxSourceBytes = defaults.MaxSourceBytes
	}
	if input.MaxNodes == 0 {
		input.MaxNodes = defaults.MaxNodes
	}
	if input.MaxDepth == 0 {
		input.MaxDepth = defaults.MaxDepth
	}
	return input
}

// LimitKind identifies one exhausted parser resource.
type LimitKind string

const (
	LimitSourceBytes LimitKind = "source bytes"
	LimitNodes       LimitKind = "nodes"
	LimitDepth       LimitKind = "depth"
)

// LimitError reports an exhausted per-parse resource.
type LimitError struct {
	Kind   LimitKind
	Limit  uint64
	Actual uint64
}

// Error implements error.
func (e *LimitError) Error() string {
	if e == nil {
		return "java parser resource limit exceeded"
	}
	return fmt.Sprintf(
		"java parser %s limit exceeded: got %d, maximum %d",
		e.Kind,
		e.Actual,
		e.Limit,
	)
}

package backend

import (
	"fmt"

	"git.alberto.engineer/alberto/java-cst-go/language"
)

// Result is one backend parse represented entirely by repository-owned values.
type Result struct {
	Level        language.Level `json:"level"`
	Backend      string         `json:"backend"`
	RawBytes     uint32         `json:"rawBytes"`
	LogicalBytes uint32         `json:"logicalBytes"`
	StopReason   string         `json:"stopReason"`
	StoppedEarly bool           `json:"stoppedEarly"`
	NodeCount    int            `json:"nodeCount"`
	ErrorCount   int            `json:"errorCount"`
	MissingCount int            `json:"missingCount"`
	Root         *Node          `json:"root"`
}

// Node is a backend-neutral, diagnostic snapshot of one concrete parse node.
//
// Field is the grammar field assigned by the parent. Anonymous punctuation is
// retained as ordinary nodes with Named set to false.
type Node struct {
	Kind      string `json:"kind"`
	Field     string `json:"field,omitempty"`
	StartByte uint32 `json:"startByte"`
	EndByte   uint32 `json:"endByte"`
	// LogicalStartByte and LogicalEndByte preserve the backend's translated
	// input coordinates. StartByte and EndByte are always raw coordinates.
	LogicalStartByte uint32 `json:"logicalStartByte"`
	LogicalEndByte   uint32 `json:"logicalEndByte"`
	Named            bool   `json:"named"`
	Extra            bool   `json:"extra,omitempty"`
	Missing          bool   `json:"missing,omitempty"`
	Error            bool   `json:"error,omitempty"`
	HasError         bool   `json:"hasError,omitempty"`
	Children         []Node `json:"children,omitempty"`
}

// RangeIssue describes one malformed backend node relationship.
type RangeIssue struct {
	Path    string `json:"path"`
	Kind    string `json:"kind,omitempty"`
	Message string `json:"message"`
}

// ValidateRanges checks that all snapshot ranges are bounded, ordered, and
// contained by their parents.
func (r Result) ValidateRanges(sourceLength uint32) []RangeIssue {
	if r.Root == nil {
		if sourceLength == 0 {
			return nil
		}
		return []RangeIssue{{
			Path:    "root",
			Message: "non-empty source has no backend root",
		}}
	}

	type entry struct {
		node   *Node
		path   string
		parent *Node
	}

	issues := make([]RangeIssue, 0)
	stack := []entry{{node: r.Root, path: "root"}}
	for len(stack) > 0 {
		current := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		node := current.node

		if node.StartByte > node.EndByte {
			issues = append(issues, RangeIssue{
				Path: current.path,
				Kind: node.Kind,
				Message: fmt.Sprintf(
					"start byte %d exceeds end byte %d",
					node.StartByte,
					node.EndByte,
				),
			})
		}
		if node.EndByte > sourceLength {
			issues = append(issues, RangeIssue{
				Path: current.path,
				Kind: node.Kind,
				Message: fmt.Sprintf(
					"end byte %d exceeds source length %d",
					node.EndByte,
					sourceLength,
				),
			})
		}
		if current.parent != nil &&
			(node.StartByte < current.parent.StartByte ||
				node.EndByte > current.parent.EndByte) {
			issues = append(issues, RangeIssue{
				Path: current.path,
				Kind: node.Kind,
				Message: fmt.Sprintf(
					"range [%d,%d) is outside parent [%d,%d)",
					node.StartByte,
					node.EndByte,
					current.parent.StartByte,
					current.parent.EndByte,
				),
			})
		}

		var previousEnd uint32
		for index := range node.Children {
			child := &node.Children[index]
			path := fmt.Sprintf("%s/%d", current.path, index)
			if index > 0 && child.StartByte < previousEnd {
				issues = append(issues, RangeIssue{
					Path: path,
					Kind: child.Kind,
					Message: fmt.Sprintf(
						"start byte %d precedes prior sibling end %d",
						child.StartByte,
						previousEnd,
					),
				})
			}
			if child.EndByte > previousEnd {
				previousEnd = child.EndByte
			}

			stack = append(stack, entry{
				node:   child,
				path:   path,
				parent: node,
			})
		}
	}

	return issues
}

package backend_test

import (
	"testing"

	"git.alberto.engineer/alberto/java-cst-go/internal/backend"
)

func TestValidateRanges(t *testing.T) {
	t.Parallel()

	result := backend.Result{
		Root: &backend.Node{
			Kind:    "root",
			EndByte: 3,
			Children: []backend.Node{
				{Kind: "a", EndByte: 1},
				{Kind: "b", StartByte: 1, EndByte: 3},
			},
		},
	}
	if issues := result.ValidateRanges(3); len(issues) != 0 {
		t.Fatalf("ValidateRanges returned issues: %+v", issues)
	}
}

func TestValidateRangesReportsOverlapAndBounds(t *testing.T) {
	t.Parallel()

	result := backend.Result{
		Root: &backend.Node{
			Kind:    "root",
			EndByte: 4,
			Children: []backend.Node{
				{Kind: "a", EndByte: 3},
				{Kind: "b", StartByte: 2, EndByte: 5},
			},
		},
	}
	if issues := result.ValidateRanges(4); len(issues) < 2 {
		t.Fatalf("ValidateRanges issue count = %d, want at least 2: %+v", len(issues), issues)
	}
}

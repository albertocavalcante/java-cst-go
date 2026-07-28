package diagnose_test

import (
	"testing"

	"git.alberto.engineer/alberto/java-cst-go/diagnostic"
	"git.alberto.engineer/alberto/java-cst-go/internal/backend"
	"git.alberto.engineer/alberto/java-cst-go/internal/diagnose"
)

func TestBackendReportsExplicitRecoveryNodes(t *testing.T) {
	t.Parallel()

	result := backend.Result{
		ErrorCount:   1,
		MissingCount: 1,
		Root: &backend.Node{
			Kind:     "program",
			HasError: true,
			Children: []backend.Node{
				{
					Kind:      "identifier",
					StartByte: 3,
					EndByte:   3,
					Missing:   true,
				},
				{
					Kind:      "ERROR",
					StartByte: 1,
					EndByte:   2,
					Error:     true,
				},
			},
		},
	}
	values := diagnose.Backend(result)
	if got, want := len(values), 2; got != want {
		t.Fatalf("diagnostic count = %d, want %d: %+v", got, want, values)
	}
	if values[0].Code != diagnostic.CodeUnexpectedToken ||
		values[0].Span != (diagnostic.Span{Start: 1, End: 2}) {
		t.Errorf("first diagnostic = %+v, want unexpected [1,2)", values[0])
	}
	if values[1].Code != diagnostic.CodeMissingToken ||
		values[1].Span != (diagnostic.Span{Start: 3, End: 3}) {
		t.Errorf("second diagnostic = %+v, want missing [3,3)", values[1])
	}
}

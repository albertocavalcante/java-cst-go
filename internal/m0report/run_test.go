package m0report_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"git.alberto.engineer/alberto/java-cst-go/internal/m0report"
	"git.alberto.engineer/alberto/java-cst-go/language"
	"git.alberto.engineer/alberto/java-cst-go/testkit"
)

func TestRunMeasuresLosslessVersionDiscrepancy(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	source := []byte(`final class Example {
    static int value(int number) {
        return switch (number) {
            case 0 -> 1;
            default -> 2;
        };
    }
}
`)
	if err := os.WriteFile(filepath.Join(root, "switch.java"), source, 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	manifest := testkit.Manifest{
		SchemaVersion: 2,
		Fixtures: []testkit.Fixture{{
			ID:                     "java11/switch-expression/unavailable",
			Path:                   "switch.java",
			Release:                language.Release11,
			Category:               "feature-boundary",
			Feature:                "switch-expressions",
			ExpectedFeatureState:   "unavailable",
			ExpectedFeatureVariant: 0,
			ExpectedFeatureEnabled: false,
			ExpectedBackend:        "measure",
			ExpectedRoundTrip:      true,
		}},
	}

	evidence, err := m0report.Run(context.Background(), manifest, m0report.Options{
		FixtureRoot: root,
		Runs:        2,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	report := evidence.Report
	if got, want := len(report.Cases), 1; got != want {
		t.Fatalf("case count = %d, want %d", got, want)
	}
	result := report.Cases[0]
	if !result.RoundTrip || !result.SpanInvariants || result.Panic {
		t.Fatalf("unsafe result: %+v", result)
	}
	if result.BackendErrors != 0 {
		t.Fatalf("backend errors = %d, want 0", result.BackendErrors)
	}
	if got, want := result.Classification, "post-parse feature-validation requirement"; got != want {
		t.Fatalf("classification = %q, want %q", got, want)
	}
	if result.ElapsedNS <= 0 || result.AllocatedBytes == 0 {
		t.Fatalf(
			"measurements = elapsed %d ns, allocated %d bytes",
			result.ElapsedNS,
			result.AllocatedBytes,
		)
	}
	if len(report.Run.CSTGoCommit) != 40 ||
		len(report.Run.RuntimeCommit) != 40 ||
		len(report.Run.GrammarCommit) != 40 {
		t.Fatalf("incomplete provenance: %+v", report.Run)
	}
	if len(result.BackendShapeSHA256) != 64 {
		t.Fatalf("backend shape digest = %q", result.BackendShapeSHA256)
	}
	if got, want := len(evidence.Shapes.Shapes), 1; got != want {
		t.Fatalf("shape count = %d, want %d", got, want)
	}
	if evidence.Shapes.Shapes[0].SHA256 != result.BackendShapeSHA256 {
		t.Fatalf("case and golden shape digests differ")
	}
}

func TestRunRejectsInvalidOptions(t *testing.T) {
	t.Parallel()

	if _, err := m0report.Run(
		context.Background(),
		testkit.Manifest{},
		m0report.Options{},
	); err == nil {
		t.Fatal("Run with empty options returned nil error")
	}
}

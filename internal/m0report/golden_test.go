package m0report_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"git.alberto.engineer/alberto/java-cst-go/internal/m0report"
	"git.alberto.engineer/alberto/java-cst-go/testkit"
)

func TestSelectedBackendMatchesCanonicalGoldenShapes(t *testing.T) {
	t.Parallel()

	root := filepath.Join("..", "..")
	manifestFile, err := os.Open(filepath.Join(root, "testdata", "m0", "fixtures.json"))
	if err != nil {
		t.Fatalf("open manifest: %v", err)
	}
	t.Cleanup(func() {
		if err := manifestFile.Close(); err != nil {
			t.Errorf("close manifest: %v", err)
		}
	})
	manifest, err := testkit.DecodeManifest(manifestFile)
	if err != nil {
		t.Fatalf("decode manifest: %v", err)
	}

	evidence, err := m0report.Run(context.Background(), manifest, m0report.Options{
		FixtureRoot: filepath.Join(root, "testdata", "m0"),
		Runs:        1,
	})
	if err != nil {
		t.Fatalf("run selected evidence: %v", err)
	}

	goldenData, err := os.ReadFile(filepath.Join(root, "reports", "m0", "backend-shapes.json"))
	if err != nil {
		t.Fatalf("read canonical golden shapes: %v", err)
	}
	var golden m0report.ShapeReport
	if err := json.Unmarshal(goldenData, &golden); err != nil {
		t.Fatalf("decode canonical golden shapes: %v", err)
	}
	if !reflect.DeepEqual(evidence.Shapes, golden) {
		t.Fatal("selected backend shapes differ from canonical golden shapes")
	}
}

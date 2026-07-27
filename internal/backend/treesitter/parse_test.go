package treesitter_test

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"git.alberto.engineer/alberto/java-cst-go/internal/backend"
	"git.alberto.engineer/alberto/java-cst-go/internal/backend/treesitter"
	"git.alberto.engineer/alberto/java-cst-go/language"
	"git.alberto.engineer/alberto/java-cst-go/testkit"
)

func TestReleaseAnchorSnapshotsAreDeterministicAndBounded(t *testing.T) {
	t.Parallel()

	manifestPath := filepath.Join("..", "..", "..", "testdata", "m0", "fixtures.json")
	manifestFile, err := os.Open(manifestPath)
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
		t.Fatalf("DecodeManifest: %v", err)
	}

	for _, fixture := range manifest.Fixtures {
		t.Run(fixture.ID, func(t *testing.T) {
			t.Parallel()

			sourcePath := filepath.Join(
				"..",
				"..",
				"..",
				"testdata",
				"m0",
				filepath.FromSlash(fixture.Path),
			)
			source, err := os.ReadFile(sourcePath)
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}

			level := language.Level{
				Release: fixture.Release,
				Preview: fixture.Preview,
			}
			first, err := treesitter.Parse(context.Background(), source, level)
			if err != nil {
				t.Fatalf("first Parse: %v", err)
			}
			second, err := treesitter.Parse(context.Background(), source, level)
			if err != nil {
				t.Fatalf("second Parse: %v", err)
			}

			if !reflect.DeepEqual(first, second) {
				t.Fatal("repeated backend snapshots differ")
			}
			if issues := first.ValidateRanges(uint32(len(source))); len(issues) != 0 {
				t.Fatalf("range validation issues: %+v", issues)
			}
			if first.Root == nil {
				t.Fatal("backend snapshot has nil root")
			}
			if first.Root.StartByte != 0 || first.Root.EndByte != uint32(len(source)) {
				t.Fatalf(
					"root range = [%d,%d), want [0,%d)",
					first.Root.StartByte,
					first.Root.EndByte,
					len(source),
				)
			}
			if !hasAnonymousLeaf(first.Root) {
				t.Fatal("backend snapshot contains no anonymous punctuation leaf")
			}

			t.Logf(
				"nodes=%d errors=%d missing=%d stop=%s root_has_error=%t",
				first.NodeCount,
				first.ErrorCount,
				first.MissingCount,
				first.StopReason,
				first.Root.HasError,
			)
		})
	}
}

func TestParseRejectsInvalidLevel(t *testing.T) {
	t.Parallel()

	if _, err := treesitter.Parse(context.Background(), nil, language.Level{}); err == nil {
		t.Fatal("Parse with invalid level returned nil error")
	}
}

func TestParseHonorsPreCancelledContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := treesitter.Parse(
		ctx,
		[]byte("class Example {}"),
		language.Level{Release: language.Release21},
	)
	if err == nil {
		t.Fatal("Parse with cancelled context returned nil error")
	}
}

func TestParseEmptySource(t *testing.T) {
	t.Parallel()

	result, err := treesitter.Parse(
		context.Background(),
		nil,
		language.Level{Release: language.Release21},
	)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if result.Root == nil {
		t.Fatal("empty source root is nil")
	}
	if result.Root.Kind != "program" ||
		result.Root.StartByte != 0 ||
		result.Root.EndByte != 0 {
		t.Fatalf("empty source root = %+v, want zero-width program", result.Root)
	}
	if issues := result.ValidateRanges(0); len(issues) != 0 {
		t.Fatalf("range validation issues: %+v", issues)
	}
}

func TestMalformedSourceReturnsBoundedErrorTree(t *testing.T) {
	t.Parallel()

	source := []byte("class Broken<T { void method( {")
	result, err := treesitter.Parse(
		context.Background(),
		source,
		language.Level{Release: language.Release21},
	)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if result.Root == nil {
		t.Fatal("malformed source root is nil")
	}
	if !result.Root.HasError {
		t.Fatal("malformed source root HasError = false")
	}
	if result.ErrorCount == 0 && result.MissingCount == 0 {
		t.Fatal("malformed source has neither error nor missing nodes")
	}
	if issues := result.ValidateRanges(uint32(len(source))); len(issues) != 0 {
		t.Fatalf("range validation issues: %+v", issues)
	}
}

func hasAnonymousLeaf(root *backend.Node) bool {
	stack := []*backend.Node{root}
	for len(stack) > 0 {
		node := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if !node.Named && len(node.Children) == 0 {
			return true
		}
		for index := range node.Children {
			stack = append(stack, &node.Children[index])
		}
	}

	return false
}

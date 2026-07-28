package treesittergo_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"git.alberto.engineer/alberto/java-cst-go/internal/backend"
	"git.alberto.engineer/alberto/java-cst-go/internal/backend/treesitter"
	"git.alberto.engineer/alberto/java-cst-go/internal/backend/treesittergo"
	"git.alberto.engineer/alberto/java-cst-go/internal/convert"
	"git.alberto.engineer/alberto/java-cst-go/language"
	"git.alberto.engineer/alberto/java-cst-go/source"
	"git.alberto.engineer/alberto/java-cst-go/testkit"
)

func TestPathologicalRecoveryReturnsCompleteErrorTree(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	start := time.Now()
	result, err := treesittergo.Parse(
		ctx,
		[]byte("#0#.}"),
		language.Level{Release: language.Release21},
	)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if result.StoppedEarly || result.StopReason != "accepted" {
		t.Fatalf(
			"stop = (%q, %t), want accepted complete tree",
			result.StopReason,
			result.StoppedEarly,
		)
	}
	if result.ErrorCount == 0 || result.Root == nil || !result.Root.HasError {
		t.Fatalf("recovery result = %+v, want error tree", result)
	}
	if elapsed := time.Since(start); elapsed > 100*time.Millisecond {
		t.Fatalf("Parse took %s, want at most 100ms", elapsed)
	}
}

func TestFixtureSnapshotsMatchPinnedBackend(t *testing.T) {
	t.Parallel()

	manifestPath := filepath.Join("..", "..", "..", "testdata", "m0", "fixtures.json")
	manifestFile, err := os.Open(manifestPath)
	if err != nil {
		t.Fatalf("open manifest: %v", err)
	}
	defer func() {
		if err := manifestFile.Close(); err != nil {
			t.Errorf("close manifest: %v", err)
		}
	}()

	manifest, err := testkit.DecodeManifest(manifestFile)
	if err != nil {
		t.Fatalf("DecodeManifest: %v", err)
	}

	for _, fixture := range manifest.Fixtures {
		t.Run(fixture.ID, func(t *testing.T) {
			t.Parallel()

			raw, err := os.ReadFile(filepath.Join(
				"..",
				"..",
				"..",
				"testdata",
				"m0",
				filepath.FromSlash(fixture.Path),
			))
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			level := language.Level{
				Release: fixture.Release,
				Preview: fixture.Preview,
			}
			translation := source.Translate(string(raw))

			want, err := treesitter.ParseTranslation(
				context.Background(),
				translation,
				level,
			)
			if err != nil {
				t.Fatalf("pinned ParseTranslation: %v", err)
			}
			got, err := treesittergo.ParseTranslation(
				context.Background(),
				translation,
				level,
			)
			if err != nil {
				t.Fatalf("alternative ParseTranslation: %v", err)
			}

			if issues := got.ValidateRanges(uint32(len(raw))); len(issues) != 0 {
				t.Fatalf("alternative range issues: %+v", issues)
			}
			converted, err := convert.ConvertTranslation(translation, got)
			if err != nil {
				t.Fatalf("alternative ConvertTranslation: %v", err)
			}
			if text := converted.Tree.Root().AppendText(); text != string(raw) {
				t.Fatalf("alternative round trip = %q, want %q", text, raw)
			}
			assertEquivalent(t, fixture, got, want)
		})
	}
}

func TestParseHonorsPreCancelledContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := treesittergo.Parse(
		ctx,
		nil,
		language.Level{Release: language.Release21},
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Parse error = %v, want context canceled", err)
	}
}

func FuzzTranslatedBackendConversionRoundTrip(f *testing.F) {
	for _, seed := range []string{
		"",
		"class A {}",
		"#0#.}",
		`class \u0041 {}`,
		`class A { String value = STR."hello \{name}"; }`,
		"import module java.base;\n",
		"class A { int \xffvalue; }",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, raw string) {
		if len(raw) > 4096 {
			t.Skip()
		}

		translation := source.Translate(raw)
		ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
		defer cancel()
		snapshot, err := treesittergo.ParseTranslation(
			ctx,
			translation,
			language.Level{Release: language.Release26},
		)
		if errors.Is(err, context.DeadlineExceeded) {
			return
		}
		if err != nil {
			t.Fatalf("ParseTranslation: %v", err)
		}
		result, err := convert.ConvertTranslation(translation, snapshot)
		if err != nil {
			t.Fatalf("ConvertTranslation: %v", err)
		}
		if got := result.Tree.Root().AppendText(); got != raw {
			t.Fatalf("round trip = %q, want %q", got, raw)
		}
	})
}

func assertEquivalent(
	t *testing.T,
	fixture testkit.Fixture,
	got, want backend.Result,
) {
	t.Helper()

	if got.Level != want.Level ||
		got.RawBytes != want.RawBytes ||
		got.LogicalBytes != want.LogicalBytes ||
		got.StoppedEarly ||
		got.Root == nil {
		t.Fatalf(
			"alternative metadata = %+v, pinned metadata = %+v",
			got,
			want,
		)
	}
	if want.ErrorCount > 0 {
		if got.ErrorCount == 0 || !got.Root.HasError {
			t.Fatalf(
				"alternative accepted pinned recovery case: alternative=%+v pinned=%+v",
				got,
				want,
			)
		}
		return
	}
	if fixture.Feature == "string-templates" {
		if got.ErrorCount != 0 ||
			!hasKind(got.Root, "string_interpolation") ||
			hasKind(got.Root, "escape_sequence") {
			t.Fatalf(
				"alternative string-template shape does not match native grammar: %+v",
				got,
			)
		}
		return
	}

	got.Backend = ""
	want.Backend = ""
	got.StopReason = ""
	want.StopReason = ""
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("clean alternative snapshot differs from pinned snapshot")
	}
}

func hasKind(root *backend.Node, kind string) bool {
	stack := []*backend.Node{root}
	for len(stack) > 0 {
		index := len(stack) - 1
		node := stack[index]
		stack = stack[:index]
		if node.Kind == kind {
			return true
		}
		for child := range node.Children {
			stack = append(stack, &node.Children[child])
		}
	}
	return false
}

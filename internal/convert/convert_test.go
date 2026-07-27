package convert_test

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"git.alberto.engineer/alberto/java-cst-go/internal/backend/treesitter"
	"git.alberto.engineer/alberto/java-cst-go/internal/convert"
	"git.alberto.engineer/alberto/java-cst-go/language"
	"git.alberto.engineer/alberto/java-cst-go/syntax"
	"git.alberto.engineer/alberto/java-cst-go/testkit"
)

func TestReleaseAnchorsRoundTripThroughSharedCST(t *testing.T) {
	t.Parallel()

	manifestPath := filepath.Join("..", "..", "testdata", "m0", "fixtures.json")
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
				"testdata",
				"m0",
				filepath.FromSlash(fixture.Path),
			)
			sourceBytes, err := os.ReadFile(sourcePath)
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			source := string(sourceBytes)
			snapshot, err := treesitter.Parse(
				context.Background(),
				sourceBytes,
				language.Level{
					Release: fixture.Release,
					Preview: fixture.Preview,
				},
			)
			if err != nil {
				t.Fatalf("backend Parse: %v", err)
			}

			result, err := convert.Convert(source, snapshot)
			if err != nil {
				t.Fatalf("Convert: %v", err)
			}
			assertTreeInvariants(t, result.Tree, source)

			t.Logf(
				"backend_leaves=%d tokens=%d trivia=%d elements=%d backend_errors=%d",
				result.BackendLeaves,
				result.SyntaxTokens,
				result.TriviaItems,
				result.ConvertedElements,
				snapshot.ErrorCount,
			)
		})
	}
}

func TestLexicalAdversariesRoundTrip(t *testing.T) {
	t.Parallel()

	sources := map[string]string{
		"empty":         "",
		"trivia-only":   " \t// comment\r\n/** doc */",
		"bom-crlf":      "\xef\xbb\xbfclass A {\r\n\tint value;\r\n}\r\n",
		"invalid-byte":  "class A { int \xffvalue; }\n",
		"missing-token": "class A {\n    int value\n}\n",
		"unterminated":  "class A { String value = \"unterminated",
		"mixed-newline": "class A {\rint a;\nint b;\r\n}\n",
	}
	for name, source := range sources {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			snapshot, err := treesitter.Parse(
				context.Background(),
				[]byte(source),
				language.Level{Release: language.Release21},
			)
			if err != nil {
				t.Fatalf("backend Parse: %v", err)
			}
			result, err := convert.Convert(source, snapshot)
			if err != nil {
				t.Fatalf("Convert: %v", err)
			}
			assertTreeInvariants(t, result.Tree, source)
		})
	}
}

func TestInterTokenTriviaOwnership(t *testing.T) {
	t.Parallel()

	source := "class A { int a; // comment\r\n    int b; }\n"
	snapshot, err := treesitter.Parse(
		context.Background(),
		[]byte(source),
		language.Level{Release: language.Release21},
	)
	if err != nil {
		t.Fatalf("backend Parse: %v", err)
	}
	result, err := convert.Convert(source, snapshot)
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}

	var semicolon, secondInt *syntax.Token
	intCount := 0
	for token := range result.Tree.Root().DescendantTokens() {
		switch token.Text() {
		case ";":
			if semicolon == nil {
				semicolon = token.Green()
			}
		case "int":
			intCount++
			if intCount == 2 {
				secondInt = token.Green()
			}
		}
	}
	if semicolon == nil || secondInt == nil {
		t.Fatalf("did not find expected tokens: semicolon=%v secondInt=%v", semicolon, secondInt)
	}

	if got, want := triviaText(semicolon.TrailingTrivia()), " // comment\r\n"; got != want {
		t.Fatalf("semicolon trailing trivia = %q, want %q", got, want)
	}
	if got, want := triviaText(secondInt.LeadingTrivia()), "    "; got != want {
		t.Fatalf("second int leading trivia = %q, want %q", got, want)
	}
}

func TestConvertedTreeSupportsConcurrentReads(t *testing.T) {
	t.Parallel()

	source := "class Concurrent { /* trivia */ int value = 1; }\n"
	snapshot, err := treesitter.Parse(
		context.Background(),
		[]byte(source),
		language.Level{Release: language.Release21},
	)
	if err != nil {
		t.Fatalf("backend Parse: %v", err)
	}
	result, err := convert.Convert(source, snapshot)
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}

	const readers = 16
	var group sync.WaitGroup
	group.Add(readers)
	for range readers {
		go func() {
			defer group.Done()
			assertTreeInvariants(t, result.Tree, source)
		}()
	}
	group.Wait()
}

func FuzzBackendConversionRoundTrip(f *testing.F) {
	for _, seed := range []string{
		"",
		"class A {}",
		" \t// comment\r\n",
		"class A { int value\n}",
		"\xef\xbb\xbfclass A {}\r\n",
		"class A { String value = \"unterminated",
		"class A { int \xffvalue; }",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, source string) {
		if len(source) > 4096 {
			t.Skip()
		}

		snapshot, err := treesitter.Parse(
			context.Background(),
			[]byte(source),
			language.Level{Release: language.Release21},
		)
		if err != nil {
			t.Fatalf("backend Parse: %v", err)
		}
		result, err := convert.Convert(source, snapshot)
		if err != nil {
			t.Fatalf("Convert: %v", err)
		}
		if got := result.Tree.Root().AppendText(); got != source {
			t.Fatalf("round trip = %q, want %q", got, source)
		}
	})
}

func assertTreeInvariants(t *testing.T, tree *syntax.Tree, source string) {
	t.Helper()

	if tree == nil {
		t.Fatal("tree is nil")
	}
	root := tree.Root()
	if got := root.AppendText(); got != source {
		t.Errorf("round trip = %q, want %q", got, source)
	}
	if got, want := root.Green().FullWidth(), len(source); got != want {
		t.Errorf("root full width = %d, want %d", got, want)
	}
	if root.FullSpan().Start != 0 || root.FullSpan().End != len(source) {
		t.Errorf("root full span = %+v, want [0,%d)", root.FullSpan(), len(source))
	}

	seen := make(map[syntax.ElementID]struct{})
	seen[root.ID()] = struct{}{}
	for node := range root.DescendantNodes() {
		if _, ok := seen[node.ID()]; ok {
			t.Errorf("duplicate node occurrence ID %d", node.ID())
		}
		seen[node.ID()] = struct{}{}
		if !contains(root.FullSpan(), node.FullSpan()) {
			t.Errorf("node span %+v outside root %+v", node.FullSpan(), root.FullSpan())
		}
	}
	for token := range root.DescendantTokens() {
		if _, ok := seen[token.ID()]; ok {
			t.Errorf("duplicate token occurrence ID %d", token.ID())
		}
		seen[token.ID()] = struct{}{}
		if !contains(root.FullSpan(), token.FullSpan()) {
			t.Errorf("token span %+v outside root %+v", token.FullSpan(), root.FullSpan())
		}
	}
}

func contains(outer, inner syntax.Span) bool {
	return outer.Start <= inner.Start && inner.End <= outer.End
}

func triviaText(items func(func(syntax.Trivia) bool)) string {
	var text string
	for item := range items {
		text += item.Text()
	}
	return text
}

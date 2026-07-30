package convert_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"git.alberto.engineer/alberto/java-cst-go/internal/backend/selected"
	"git.alberto.engineer/alberto/java-cst-go/internal/convert"
	"git.alberto.engineer/alberto/java-cst-go/language"
	"git.alberto.engineer/alberto/java-cst-go/source"
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
			snapshot, err := selected.Parse(
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

			snapshot, err := selected.Parse(
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
	snapshot, err := selected.Parse(
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

	var semicolon, secondInt *syntax.GreenToken
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

func TestTranslatedTokensRetainRawAndLogicalSpellings(t *testing.T) {
	t.Parallel()

	raw := `class \u0041 { String value = \u0022hi\u0022\u003b }`
	translation := source.Translate(raw)
	if diagnostics := translation.Diagnostics(); len(diagnostics) != 0 {
		t.Fatalf("translation diagnostics: %+v", diagnostics)
	}
	snapshot, err := selected.ParseTranslation(
		context.Background(),
		translation,
		language.Level{Release: language.Release21},
	)
	if err != nil {
		t.Fatalf("backend ParseTranslation: %v", err)
	}
	if snapshot.ErrorCount != 0 {
		t.Fatalf("backend error count = %d, want 0", snapshot.ErrorCount)
	}

	result, err := convert.ConvertTranslation(translation, snapshot)
	if err != nil {
		t.Fatalf("ConvertTranslation: %v", err)
	}
	assertTreeInvariants(t, result.Tree, raw)

	wantLogical := map[string]string{
		`\u0041`: "A",
		`\u0022`: `"`,
		"hi":     "hi",
		`\u003b`: ";",
	}
	wantCount := map[string]int{
		`\u0041`: 1,
		`\u0022`: 2,
		"hi":     1,
		`\u003b`: 1,
	}
	found := make(map[string]int, len(wantLogical))
	observed := make(map[string]string)
	for token := range result.Tree.Root().DescendantTokens() {
		observed[token.Text()] = token.Data().LogicalText
		logical, ok := wantLogical[token.Text()]
		if !ok {
			continue
		}
		found[token.Text()]++
		if got := token.Data().LogicalText; got != logical {
			t.Errorf(
				"token %q logical text = %q, want %q",
				token.Text(),
				got,
				logical,
			)
		}
	}
	for rawToken, count := range wantCount {
		if found[rawToken] != count {
			t.Errorf(
				"raw token %q count = %d, want %d; observed=%v",
				rawToken,
				found[rawToken],
				count,
				observed,
			)
		}
	}
}

func TestTranslatedTriviaRetainsRawEscapeSpellings(t *testing.T) {
	t.Parallel()

	raw := `class\u0020A { int a; \u002f\u002f comment\u000d\u000a    int b; }`
	translation := source.Translate(raw)
	if diagnostics := translation.Diagnostics(); len(diagnostics) != 0 {
		t.Fatalf("translation diagnostics: %+v", diagnostics)
	}
	snapshot, err := selected.ParseTranslation(
		context.Background(),
		translation,
		language.Level{Release: language.Release21},
	)
	if err != nil {
		t.Fatalf("backend ParseTranslation: %v", err)
	}
	if snapshot.ErrorCount != 0 {
		t.Fatalf("backend error count = %d, want 0", snapshot.ErrorCount)
	}

	result, err := convert.ConvertTranslation(translation, snapshot)
	if err != nil {
		t.Fatalf("ConvertTranslation: %v", err)
	}
	assertTreeInvariants(t, result.Tree, raw)

	var classToken, firstSemicolon, secondInt *syntax.GreenToken
	intCount := 0
	for token := range result.Tree.Root().DescendantTokens() {
		switch token.Text() {
		case "class":
			classToken = token.Green()
		case ";":
			if firstSemicolon == nil {
				firstSemicolon = token.Green()
			}
		case "int":
			intCount++
			if intCount == 2 {
				secondInt = token.Green()
			}
		}
	}
	if classToken == nil || firstSemicolon == nil || secondInt == nil {
		t.Fatalf(
			"missing expected tokens: class=%v semicolon=%v secondInt=%v",
			classToken,
			firstSemicolon,
			secondInt,
		)
	}

	classTrailing := slices.Collect(classToken.TrailingTrivia())
	if got, want := len(classTrailing), 1; got != want {
		t.Fatalf("class trailing trivia count = %d, want %d", got, want)
	}
	if classTrailing[0].Kind() != syntax.TriviaWhitespace ||
		classTrailing[0].Text() != `\u0020` {
		t.Fatalf("class trailing trivia = %+v", classTrailing[0])
	}

	semicolonTrailing := slices.Collect(firstSemicolon.TrailingTrivia())
	wantKinds := []syntax.TriviaKind{
		syntax.TriviaWhitespace,
		syntax.TriviaLineComment,
		syntax.TriviaLineTerminator,
	}
	gotKinds := make([]syntax.TriviaKind, len(semicolonTrailing))
	for index := range semicolonTrailing {
		gotKinds[index] = semicolonTrailing[index].Kind()
	}
	if !slices.Equal(gotKinds, wantKinds) {
		t.Fatalf("semicolon trailing kinds = %v, want %v", gotKinds, wantKinds)
	}
	if got, want := triviaText(firstSemicolon.TrailingTrivia()),
		` \u002f\u002f comment\u000d\u000a`; got != want {
		t.Fatalf("semicolon trailing trivia = %q, want %q", got, want)
	}
	if got, want := triviaText(secondInt.LeadingTrivia()), "    "; got != want {
		t.Fatalf("second int leading trivia = %q, want %q", got, want)
	}
}

func TestMalformedUnicodeEscapeStillRoundTrips(t *testing.T) {
	t.Parallel()

	raw := `class \u12 Broken {}`
	translation := source.Translate(raw)
	if got, want := len(translation.Diagnostics()), 1; got != want {
		t.Fatalf("translation diagnostic count = %d, want %d", got, want)
	}
	snapshot, err := selected.ParseTranslation(
		context.Background(),
		translation,
		language.Level{Release: language.Release21},
	)
	if err != nil {
		t.Fatalf("backend ParseTranslation: %v", err)
	}
	result, err := convert.ConvertTranslation(translation, snapshot)
	if err != nil {
		t.Fatalf("ConvertTranslation: %v", err)
	}
	assertTreeInvariants(t, result.Tree, raw)
}

func TestConvertedTreeSupportsConcurrentReads(t *testing.T) {
	t.Parallel()

	source := "class Concurrent { /* trivia */ int value = 1; }\n"
	snapshot, err := selected.Parse(
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

		ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
		defer cancel()
		snapshot, err := selected.Parse(
			ctx,
			[]byte(source),
			language.Level{Release: language.Release21},
		)
		if errors.Is(err, context.DeadlineExceeded) {
			return
		}
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

func FuzzTranslatedBackendConversionRoundTrip(f *testing.F) {
	for _, seed := range []string{
		"",
		"class A {}",
		`class \u0041 {}`,
		`class A { int value\u003b }`,
		`class A { int a; \u002f\u002f comment\u000a int b; }`,
		`class A { String value = \u0022text\u0022; }`,
		`class \u12 Broken {}`,
		`\\u2122`,
		`\uD83D\uDE00`,
		"\xff\\u000a",
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
		snapshot, err := selected.ParseTranslation(
			ctx,
			translation,
			language.Level{Release: language.Release21},
		)
		if errors.Is(err, context.DeadlineExceeded) {
			return
		}
		if err != nil {
			t.Fatalf("backend ParseTranslation: %v", err)
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

func assertTreeInvariants(t *testing.T, tree *syntax.CoreTree, source string) {
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

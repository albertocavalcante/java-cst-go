package convert

import (
	"slices"
	"testing"

	"git.alberto.engineer/alberto/java-cst-go/syntax"
)

func TestClassifyTrivia(t *testing.T) {
	t.Parallel()

	source := "\xef\xbb\xbf \t// line\r\n/** doc */\n/* block */\f"
	items := classifyTrivia(source, 0, uint32(len(source)))

	gotKinds := make([]syntax.TriviaKind, len(items))
	var roundTrip string
	for index := range items {
		gotKinds[index] = items[index].Kind()
		roundTrip += items[index].Text()
	}
	wantKinds := []syntax.TriviaKind{
		syntax.TriviaBOM,
		syntax.TriviaWhitespace,
		syntax.TriviaLineComment,
		syntax.TriviaLineTerminator,
		syntax.TriviaDocumentationComment,
		syntax.TriviaLineTerminator,
		syntax.TriviaBlockComment,
		syntax.TriviaWhitespace,
	}
	if !slices.Equal(gotKinds, wantKinds) {
		t.Fatalf("trivia kinds = %v, want %v", gotKinds, wantKinds)
	}
	if roundTrip != source {
		t.Fatalf("trivia round trip = %q, want %q", roundTrip, source)
	}
}

func TestSplitInterTokenTrivia(t *testing.T) {
	t.Parallel()

	items := []syntax.Trivia{
		syntax.NewTrivia(syntax.TriviaWhitespace, " "),
		syntax.NewTrivia(syntax.TriviaLineComment, "// comment"),
		syntax.NewTrivia(syntax.TriviaLineTerminator, "\r\n"),
		syntax.NewTrivia(syntax.TriviaWhitespace, "    "),
	}
	trailing, leading := splitInterTokenTrivia(items)
	if got, want := len(trailing), 3; got != want {
		t.Fatalf("trailing length = %d, want %d", got, want)
	}
	if got, want := len(leading), 1; got != want {
		t.Fatalf("leading length = %d, want %d", got, want)
	}
}

package diagnose_test

import (
	"strings"
	"testing"

	"git.alberto.engineer/alberto/java-cst-go/diagnostic"
	"git.alberto.engineer/alberto/java-cst-go/internal/diagnose"
	"git.alberto.engineer/alberto/java-cst-go/source"
)

func TestLexicalDiagnosesUnterminatedConstructs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		raw     string
		code    diagnostic.Code
		message string
	}{
		{
			name:    "block-comment",
			raw:     `class A { /* unterminated`,
			code:    diagnostic.CodeUnterminatedComment,
			message: "block comment",
		},
		{
			name:    "unicode-created-block-comment",
			raw:     `class A { \u002f\u002a unterminated`,
			code:    diagnostic.CodeUnterminatedComment,
			message: "block comment",
		},
		{
			name:    "string",
			raw:     `class A { String value = "unterminated`,
			code:    diagnostic.CodeUnterminatedLiteral,
			message: "string literal",
		},
		{
			name:    "character",
			raw:     `class A { char value = 'x`,
			code:    diagnostic.CodeUnterminatedLiteral,
			message: "character literal",
		},
		{
			name:    "text-block",
			raw:     "class A { String value = \"\"\"\nunterminated",
			code:    diagnostic.CodeUnterminatedLiteral,
			message: "text block",
		},
		{
			name:    "unicode-created-line-end",
			raw:     `class A { String value = "unterminated\u000a}`,
			code:    diagnostic.CodeUnterminatedLiteral,
			message: "string literal",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			values := diagnose.Lexical(source.Translate(test.raw))
			if len(values) != 1 {
				t.Fatalf("Lexical() = %+v, want one diagnostic", values)
			}
			if values[0].Code != test.code ||
				!strings.Contains(values[0].Message, test.message) {
				t.Errorf(
					"diagnostic = %+v, want %s containing %q",
					values[0],
					test.code,
					test.message,
				)
			}
			if values[0].Span.Start < 0 ||
				values[0].Span.End <= values[0].Span.Start ||
				values[0].Span.End > len(test.raw) {
				t.Errorf("raw span is invalid: %+v", values[0].Span)
			}
		})
	}
}

func TestLexicalIgnoresDelimitersInsideClosedConstructs(t *testing.T) {
	t.Parallel()

	const raw = `
class A {
    String string = "/* not a comment */ and \"quoted\"";
    char quote = '\'';
    String block = """
        "quoted" and /* text */
        """;
    String template = STR."\{call("argument", '}')}";
    String blockTemplate = STR."""
        \{call("argument", '}')}
        """;
    // "not a string"
    /* 'not a character' */
}
`
	if values := diagnose.Lexical(source.Translate(raw)); len(values) != 0 {
		t.Fatalf("Lexical() = %+v, want none", values)
	}
}

func TestLexicalNilTranslationIsEmpty(t *testing.T) {
	t.Parallel()

	if values := diagnose.Lexical(nil); len(values) != 0 {
		t.Fatalf("Lexical(nil) = %+v, want none", values)
	}
}

func FuzzLexicalDiagnostics(f *testing.F) {
	for _, seed := range []string{
		"",
		`class A { /* unterminated`,
		`class A { String value = "unterminated`,
		`class A { char value = 'x`,
		"class A { String value = \"\"\"\nunterminated",
		`class A { String value = STR."\{call("x", '}')}"; }`,
		`\u002f\u002a unterminated`,
		"\xff\"unterminated",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, raw string) {
		if len(raw) > 4096 {
			t.Skip()
		}
		values := diagnose.Lexical(source.Translate(raw))
		for _, value := range values {
			if value.Code != diagnostic.CodeUnterminatedComment &&
				value.Code != diagnostic.CodeUnterminatedLiteral {
				t.Fatalf("unexpected lexical code: %+v", value)
			}
			if value.Span.Start < 0 ||
				value.Span.End < value.Span.Start ||
				value.Span.End > len(raw) {
				t.Fatalf("diagnostic span is out of bounds: %+v", value)
			}
		}
	})
}

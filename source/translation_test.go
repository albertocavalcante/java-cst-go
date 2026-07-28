package source_test

import (
	"slices"
	"testing"

	"git.alberto.engineer/alberto/java-cst-go/diagnostic"
	"git.alberto.engineer/alberto/java-cst-go/source"
)

func TestTranslateUnicodeEscapes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "basic", raw: `\u0041`, want: "A"},
		{name: "repeated-u", raw: `\uuuu0041`, want: "A"},
		{name: "ineligible-second-backslash", raw: `\\u2122`, want: `\\u2122`},
		{name: "eligible-third-backslash", raw: `\\\u2122`, want: `\\™`},
		{name: "produced-backslash-not-reprocessed", raw: `\u005cu005a`, want: `\u005a`},
		{name: "escape-after-produced-backslash", raw: `\u005c\u005a`, want: `\Z`},
		{name: "escape-creates-comment", raw: `\u002f\u002f hello`, want: "// hello"},
		{name: "surrogate-pair", raw: `\uD83D\uDE00`, want: "😀"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			translation := source.Translate(test.raw)
			if got := translation.Raw(); got != test.raw {
				t.Fatalf("Raw() = %q, want %q", got, test.raw)
			}
			if got := translation.Logical(); got != test.want {
				t.Fatalf("Logical() = %q, want %q", got, test.want)
			}
			if diagnostics := translation.Diagnostics(); len(diagnostics) != 0 {
				t.Fatalf("Diagnostics() = %+v, want none", diagnostics)
			}
		})
	}
}

func TestTranslateDiagnosesMalformedEligibleEscape(t *testing.T) {
	t.Parallel()

	translation := source.Translate(`class \u12 Broken {}`)
	if got, want := translation.Logical(), `class \u12 Broken {}`; got != want {
		t.Fatalf("Logical() = %q, want %q", got, want)
	}
	diagnostics := translation.Diagnostics()
	if got, want := len(diagnostics), 1; got != want {
		t.Fatalf("diagnostic count = %d, want %d: %+v", got, want, diagnostics)
	}
	if diagnostics[0].Code != diagnostic.CodeInvalidUnicodeEscape ||
		diagnostics[0].Severity != diagnostic.SeverityError {
		t.Fatalf("diagnostic = %+v, want invalid-Unicode error", diagnostics[0])
	}
}

func TestTranslateDoesNotDiagnoseIneligibleUnicodeSpelling(t *testing.T) {
	t.Parallel()

	translation := source.Translate(`\\u12`)
	if diagnostics := translation.Diagnostics(); len(diagnostics) != 0 {
		t.Fatalf("Diagnostics() = %+v, want none", diagnostics)
	}
}

func TestTranslatePreservesAndDiagnosesInvalidUTF8(t *testing.T) {
	t.Parallel()

	raw := "class \xff Broken {}"
	translation := source.Translate(raw)
	if got := translation.Logical(); got != raw {
		t.Fatalf("Logical() = %q, want exact invalid bytes %q", got, raw)
	}
	diagnostics := translation.Diagnostics()
	if got, want := len(diagnostics), 1; got != want {
		t.Fatalf("diagnostic count = %d, want %d", got, want)
	}
	if diagnostics[0].Code != diagnostic.CodeInvalidUTF8 ||
		diagnostics[0].Severity != diagnostic.SeverityError {
		t.Fatalf("diagnostic = %+v, want invalid-UTF-8 error", diagnostics[0])
	}
}

func TestTranslationSpanMapping(t *testing.T) {
	t.Parallel()

	translation := source.Translate(`a\u0042c`)
	if got, ok := translation.RawSpan(source.Span{Start: 1, End: 2}); !ok ||
		got != (source.Span{Start: 1, End: 7}) {
		t.Fatalf("RawSpan(B) = %+v, %t; want [1,7), true", got, ok)
	}
	if got, ok := translation.LogicalSpan(source.Span{Start: 1, End: 7}); !ok ||
		got != (source.Span{Start: 1, End: 2}) {
		t.Fatalf("LogicalSpan(escape) = %+v, %t; want [1,2), true", got, ok)
	}
	if got, ok := translation.RawSpan(source.Span{Start: 0, End: 3}); !ok ||
		got != (source.Span{Start: 0, End: 8}) {
		t.Fatalf("RawSpan(all) = %+v, %t; want [0,8), true", got, ok)
	}
}

func TestTranslationSegmentsAreStableAndDiagnosticsAreDefensive(t *testing.T) {
	t.Parallel()

	translation := source.Translate("a\\u0042\xffc")
	first := slices.Collect(translation.Segments())
	second := slices.Collect(translation.Segments())
	if !slices.Equal(first, second) {
		t.Fatalf("segments differ: first=%v second=%v", first, second)
	}

	diagnostics := translation.Diagnostics()
	diagnostics[0].Code = "changed"
	if got := translation.Diagnostics()[0].Code; got != diagnostic.CodeInvalidUTF8 {
		t.Fatalf("stored diagnostic code = %q, want %s", got, diagnostic.CodeInvalidUTF8)
	}
}

func TestTranslateIsolatedSurrogate(t *testing.T) {
	t.Parallel()

	translation := source.Translate(`\uD800`)
	if got := translation.Logical(); got != "�" {
		t.Fatalf("Logical() = %q, want replacement rune", got)
	}
	diagnostics := translation.Diagnostics()
	if got, want := len(diagnostics), 1; got != want {
		t.Fatalf("diagnostic count = %d, want %d", got, want)
	}
	if diagnostics[0].Code != diagnostic.CodeInvalidUnicodeEscape ||
		diagnostics[0].Severity != diagnostic.SeverityError {
		t.Fatalf("diagnostic = %+v, want invalid-Unicode error", diagnostics[0])
	}
}

func TestTranslationDiagnosticsFollowRawSourceOrder(t *testing.T) {
	t.Parallel()

	translation := source.Translate("\xff\\u12\xfe")
	diagnostics := translation.Diagnostics()
	if got, want := len(diagnostics), 3; got != want {
		t.Fatalf("diagnostic count = %d, want %d: %+v", got, want, diagnostics)
	}
	wantCodes := []diagnostic.Code{
		diagnostic.CodeInvalidUTF8,
		diagnostic.CodeInvalidUnicodeEscape,
		diagnostic.CodeInvalidUTF8,
	}
	previousStart := 0
	for index, value := range diagnostics {
		if value.Code != wantCodes[index] {
			t.Errorf("diagnostic %d code = %q, want %q", index, value.Code, wantCodes[index])
		}
		if value.Span.Start < previousStart {
			t.Errorf(
				"diagnostic %d span = %+v, starts before byte %d",
				index,
				value.Span,
				previousStart,
			)
		}
		previousStart = value.Span.Start
	}
}

func FuzzTranslationMapping(f *testing.F) {
	for _, seed := range []string{
		"",
		"class A {}",
		`class \u0041 {}`,
		`\\u2122`,
		`\\\u2122`,
		`\u005c\u005a`,
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
		if translation.Raw() != raw {
			t.Fatal("translation changed raw source")
		}

		rawCursor := 0
		logicalCursor := 0
		for segment := range translation.Segments() {
			if segment.RawSpan.Start != rawCursor {
				t.Fatalf(
					"raw segment starts at %d, want %d",
					segment.RawSpan.Start,
					rawCursor,
				)
			}
			if segment.LogicalSpan.Start != logicalCursor {
				t.Fatalf(
					"logical segment starts at %d, want %d",
					segment.LogicalSpan.Start,
					logicalCursor,
				)
			}
			rawCursor = segment.RawSpan.End
			logicalCursor = segment.LogicalSpan.End
		}
		if rawCursor != len(raw) {
			t.Fatalf("raw segments end at %d, want %d", rawCursor, len(raw))
		}
		if logicalCursor != len(translation.Logical()) {
			t.Fatalf(
				"logical segments end at %d, want %d",
				logicalCursor,
				len(translation.Logical()),
			)
		}

		rawFull, ok := translation.RawSpan(source.Span{
			End: len(translation.Logical()),
		})
		if !ok || rawFull != (source.Span{End: len(raw)}) {
			t.Fatalf("RawSpan(full) = %+v, %t", rawFull, ok)
		}
		logicalFull, ok := translation.LogicalSpan(source.Span{End: len(raw)})
		if !ok || logicalFull != (source.Span{End: len(translation.Logical())}) {
			t.Fatalf("LogicalSpan(full) = %+v, %t", logicalFull, ok)
		}

		for _, diagnostic := range translation.Diagnostics() {
			if diagnostic.Span.Start < 0 ||
				diagnostic.Span.End < diagnostic.Span.Start ||
				diagnostic.Span.End > len(raw) {
				t.Fatalf("diagnostic span out of bounds: %+v", diagnostic)
			}
		}
	})
}

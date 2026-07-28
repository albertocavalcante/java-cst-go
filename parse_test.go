package javacst_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	javacst "git.alberto.engineer/alberto/java-cst-go"
	"git.alberto.engineer/alberto/java-cst-go/diagnostic"
	"git.alberto.engineer/alberto/java-cst-go/language"
)

func TestParseDefaultsToJava25AndRoundTrips(t *testing.T) {
	t.Parallel()

	raw := `import module java.base;
class Base {}
class Example extends Base {
    Example() {
        int before = 1;
        super();
    }
}
`
	tree, err := javacst.Parse(raw, javacst.Options{})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got, want := tree.Text(), raw; got != want {
		t.Fatalf("Text() = %q, want %q", got, want)
	}
	if got, want := tree.Root().AppendText(), raw; got != want {
		t.Fatalf("Root().AppendText() = %q, want %q", got, want)
	}
	if got, want := tree.Level(), (language.Level{Release: language.Release25}); got != want {
		t.Fatalf("Level() = %+v, want %+v", got, want)
	}
	if diagnostics := tree.Diagnostics(); len(diagnostics) != 0 {
		t.Fatalf("Diagnostics() = %+v, want none", diagnostics)
	}
	provenance := tree.Provenance()
	if provenance.LibraryVersion == "" ||
		!strings.Contains(provenance.GrammarRevision, "+patch-sha256:") ||
		provenance.Backend != "treesitter-go/v0.1.0" {
		t.Fatalf("Provenance() = %+v", provenance)
	}
}

func TestParseReturnsLexicalAndRecoveryDiagnosticsWithTree(t *testing.T) {
	t.Parallel()

	raw := "class \xff Broken {"
	tree, err := javacst.Parse(raw, javacst.Options{
		Level: language.Level{Release: language.Release21},
	})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if tree.Text() != raw || tree.Root().AppendText() != raw {
		t.Fatalf("malformed source did not round trip")
	}
	diagnostics := tree.Diagnostics()
	if !hasCode(diagnostics, diagnostic.CodeInvalidUTF8) {
		t.Errorf("Diagnostics() = %+v, want %s", diagnostics, diagnostic.CodeInvalidUTF8)
	}
	if !hasCode(diagnostics, diagnostic.CodeUnexpectedToken) &&
		!hasCode(diagnostics, diagnostic.CodeMissingToken) {
		t.Errorf("Diagnostics() = %+v, want parser recovery code", diagnostics)
	}
	for index := 1; index < len(diagnostics); index++ {
		if diagnostics[index].Span.Start < diagnostics[index-1].Span.Start {
			t.Errorf("diagnostics are not in source order: %+v", diagnostics)
			break
		}
	}
}

func TestParseLexicalInputsRoundTrip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		raw       string
		wantCodes []diagnostic.Code
	}{
		{name: "empty"},
		{name: "trivia-only", raw: " \t// comment\r\n/** doc */"},
		{
			name: "bom-crlf",
			raw:  "\xef\xbb\xbfclass A {\r\n\tint value;\r\n}\r\n",
		},
		{
			name: "mixed-line-terminators",
			raw:  "class A {\rint a;\nint b;\r\n}\n",
		},
		{
			name: "unicode-created-identifier-and-punctuation",
			raw:  `class \uuuu0041 \u007b int value\u003b \u007d`,
		},
		{
			name: "unicode-created-whitespace-comment-and-line-end",
			raw:  `class\u0020A { int a; \u002f\u002f comment\u000a int b; }`,
		},
		{
			name: "ineligible-unicode-spelling",
			raw:  `class A { String value = "\\u0041"; }`,
		},
		{
			name:      "malformed-unicode-escape",
			raw:       `class \u12 Broken {}`,
			wantCodes: []diagnostic.Code{diagnostic.CodeInvalidUnicodeEscape},
		},
		{
			name:      "isolated-surrogate",
			raw:       `class A { String value = "\uD800"; }`,
			wantCodes: []diagnostic.Code{diagnostic.CodeInvalidUnicodeEscape},
		},
		{
			name:      "invalid-utf8",
			raw:       "class A { int \xffvalue; }\n",
			wantCodes: []diagnostic.Code{diagnostic.CodeInvalidUTF8},
		},
		{
			name: "closed-literals-and-comments",
			raw: "class A {\n" +
				"  char quote = '\\'';\n" +
				"  String text = \"/* text */\";\n" +
				"  String block = \"\"\"\ntext\n\"\"\";\n" +
				"  // \" comment\n" +
				"  /* ' comment */\n" +
				"}\n",
		},
		{
			name:      "unterminated-block-comment",
			raw:       "class A { /* unterminated",
			wantCodes: []diagnostic.Code{diagnostic.CodeUnterminatedComment},
		},
		{
			name:      "unterminated-string",
			raw:       `class A { String value = "unterminated`,
			wantCodes: []diagnostic.Code{diagnostic.CodeUnterminatedLiteral},
		},
		{
			name:      "unterminated-character",
			raw:       `class A { char value = 'x`,
			wantCodes: []diagnostic.Code{diagnostic.CodeUnterminatedLiteral},
		},
		{
			name:      "unterminated-text-block",
			raw:       "class A { String value = \"\"\"\nunterminated",
			wantCodes: []diagnostic.Code{diagnostic.CodeUnterminatedLiteral},
		},
		{
			name:      "missing-token",
			raw:       "class A { int value }",
			wantCodes: []diagnostic.Code{diagnostic.CodeMissingToken},
		},
		{
			name:      "skipped-error-text",
			raw:       "#0#.}",
			wantCodes: []diagnostic.Code{diagnostic.CodeUnexpectedToken},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			tree, err := javacst.Parse(test.raw, javacst.Options{})
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if got := tree.Text(); got != test.raw {
				t.Fatalf("Text() = %q, want %q", got, test.raw)
			}
			if got := tree.Root().AppendText(); got != test.raw {
				t.Fatalf("Root().AppendText() = %q, want %q", got, test.raw)
			}
			diagnostics := tree.Diagnostics()
			for _, code := range test.wantCodes {
				if !hasCode(diagnostics, code) {
					t.Errorf("Diagnostics() = %+v, want %s", diagnostics, code)
				}
			}
			if len(test.wantCodes) == 0 {
				for _, value := range diagnostics {
					if value.Code >= "JAV1001" && value.Code <= "JAV1004" {
						t.Errorf("unexpected lexical diagnostic: %+v", value)
					}
				}
			}
		})
	}
}

func TestParseRejectsInvalidConfiguration(t *testing.T) {
	t.Parallel()

	tree, err := javacst.Parse("class A {}", javacst.Options{
		Level: language.Level{Release: language.Release(7)},
	})
	if err == nil || tree != nil {
		t.Fatalf("Parse invalid level = (%v, %v), want nil tree and error", tree, err)
	}
	if !strings.Contains(err.Error(), "invalid language level") {
		t.Fatalf("Parse error = %q, want invalid language level", err)
	}

	var nilContext context.Context
	tree, err = javacst.ParseContext(nilContext, "", javacst.Options{})
	if err == nil || tree != nil {
		t.Fatalf("ParseContext nil context = (%v, %v), want nil tree and error", tree, err)
	}
}

func TestParseContextHonorsPreCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	tree, err := javacst.ParseContext(ctx, "class A {}", javacst.Options{})
	if !errors.Is(err, context.Canceled) || tree != nil {
		t.Fatalf("ParseContext cancelled = (%v, %v), want nil and context.Canceled", tree, err)
	}
}

func TestParseAppliesPerParseResourceLimits(t *testing.T) {
	t.Parallel()

	const raw = "class Example { int value = 1; }\n"
	tree, err := javacst.Parse(raw, javacst.Options{
		Limits: javacst.Limits{MaxSourceBytes: uint32(len(raw))},
	})
	if err != nil || tree == nil {
		t.Fatalf("Parse at source boundary = (%v, %v), want tree and nil", tree, err)
	}

	tests := []struct {
		name   string
		source string
		limits javacst.Limits
		want   javacst.LimitKind
	}{
		{
			name:   "source",
			limits: javacst.Limits{MaxSourceBytes: uint32(len(raw) - 1)},
			want:   javacst.LimitSourceBytes,
		},
		{
			name:   "nodes",
			source: "#0#.}",
			limits: javacst.Limits{MaxNodes: 1},
			want:   javacst.LimitNodes,
		},
		{
			name:   "depth",
			source: "#0#.}",
			limits: javacst.Limits{MaxDepth: 1},
			want:   javacst.LimitDepth,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			source := test.source
			if source == "" {
				source = raw
			}
			tree, err := javacst.Parse(source, javacst.Options{Limits: test.limits})
			if tree != nil || !errors.Is(err, javacst.ErrLimitExceeded) {
				t.Fatalf(
					"Parse with %s limit = (%v, %v), want nil and ErrLimitExceeded",
					test.name,
					tree,
					err,
				)
			}
			if !strings.Contains(err.Error(), string(test.want)) {
				t.Errorf("limit error = %q, want resource %q", err, test.want)
			}
			var limitErr *javacst.LimitError
			if !errors.As(err, &limitErr) {
				t.Fatalf("limit error %T does not expose *javacst.LimitError", err)
			}
			if limitErr.Kind != test.want {
				t.Errorf("LimitError.Kind = %q, want %q", limitErr.Kind, test.want)
			}
		})
	}
}

func TestParsedTreeSupportsConcurrentReads(t *testing.T) {
	t.Parallel()

	const raw = "class Concurrent { /* trivia */ int value = 1; }\n"
	tree, err := javacst.Parse(raw, javacst.Options{})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	const readers = 16
	var group sync.WaitGroup
	group.Add(readers)
	for range readers {
		go func() {
			defer group.Done()
			if tree.Text() != raw || tree.Root().AppendText() != raw {
				t.Errorf("concurrent tree read differs")
			}
			_ = tree.Diagnostics()
			_ = tree.Provenance()
		}()
	}
	group.Wait()
}

func hasCode(values []diagnostic.Diagnostic, code diagnostic.Code) bool {
	for _, value := range values {
		if value.Code == code {
			return true
		}
	}
	return false
}

func FuzzParseRoundTrip(f *testing.F) {
	for _, seed := range []string{
		"",
		"class A {}",
		"#0#.}",
		`class \u0041 {}`,
		" \t// comment\r\n",
		"class A { String value = \"unterminated",
		"class A { /* unterminated",
		"class A { char value = 'x",
		"class A { String value = \"\"\"\nunterminated",
		`class A { String value = STR."\{call("x", '}')}"; }`,
		"class A { int \xffvalue; }",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, raw string) {
		if len(raw) > 4096 {
			t.Skip()
		}
		ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
		defer cancel()
		tree, err := javacst.ParseContext(ctx, raw, javacst.Options{
			Level: language.Level{Release: language.Release26},
		})
		if errors.Is(err, context.DeadlineExceeded) {
			return
		}
		if err != nil {
			t.Fatalf("ParseContext: %v", err)
		}
		if got := tree.Text(); got != raw {
			t.Fatalf("Text() = %q, want %q", got, raw)
		}
		if got := tree.Root().AppendText(); got != raw {
			t.Fatalf("Root().AppendText() = %q, want %q", got, raw)
		}
	})
}

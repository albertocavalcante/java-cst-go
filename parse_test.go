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

	inputs := map[string]string{
		"empty":         "",
		"trivia-only":   " \t// comment\r\n/** doc */",
		"bom-crlf":      "\xef\xbb\xbfclass A {\r\n\tint value;\r\n}\r\n",
		"mixed-newline": "class A {\rint a;\nint b;\r\n}\n",
		"unicode":       `class \u0041 { int value\u003b }`,
		"invalid-utf8":  "class A { int \xffvalue; }\n",
	}
	for name, raw := range inputs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			tree, err := javacst.Parse(raw, javacst.Options{})
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if got := tree.Text(); got != raw {
				t.Fatalf("Text() = %q, want %q", got, raw)
			}
			if got := tree.Root().AppendText(); got != raw {
				t.Fatalf("Root().AppendText() = %q, want %q", got, raw)
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

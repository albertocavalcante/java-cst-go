package diagnostic_test

import (
	"testing"

	"git.alberto.engineer/alberto/java-cst-go/diagnostic"
	"git.alberto.engineer/alberto/java-cst-go/language"
)

func TestNewAndCloneOwnNotes(t *testing.T) {
	t.Parallel()

	notes := []diagnostic.Note{{
		Span:    diagnostic.Span{Start: 4, End: 7},
		Message: "available with preview enabled",
	}}
	value := diagnostic.New(
		diagnostic.CodePreviewDisabled,
		diagnostic.SeverityError,
		diagnostic.Span{Start: 1, End: 3},
		"preview feature is disabled",
		language.FeatureStringTemplates,
		notes,
	)
	notes[0].Message = "changed"
	if got, want := value.Notes[0].Message, "available with preview enabled"; got != want {
		t.Fatalf("stored note = %q, want %q", got, want)
	}

	clone := value.Clone()
	clone.Notes[0].Message = "clone changed"
	if got, want := value.Notes[0].Message, "available with preview enabled"; got != want {
		t.Fatalf("original note after clone mutation = %q, want %q", got, want)
	}
}

func TestSeverityString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		severity diagnostic.Severity
		want     string
	}{
		{severity: diagnostic.SeverityError, want: "error"},
		{severity: diagnostic.SeverityWarning, want: "warning"},
		{severity: diagnostic.SeverityInfo, want: "info"},
		{severity: diagnostic.Severity(255), want: "unknown"},
	}
	for _, test := range tests {
		if got := test.severity.String(); got != test.want {
			t.Errorf("Severity(%d).String() = %q, want %q", test.severity, got, test.want)
		}
	}
}

func TestRequiredCodesAreUnique(t *testing.T) {
	t.Parallel()

	codes := []diagnostic.Code{
		diagnostic.CodeInvalidLanguageLevel,
		diagnostic.CodeInvalidUTF8,
		diagnostic.CodeInvalidUnicodeEscape,
		diagnostic.CodeUnterminatedComment,
		diagnostic.CodeUnterminatedLiteral,
		diagnostic.CodeUnexpectedToken,
		diagnostic.CodeMissingToken,
		diagnostic.CodeFeatureUnavailable,
		diagnostic.CodePreviewDisabled,
		diagnostic.CodeFeatureWithdrawn,
		diagnostic.CodeBackendLimit,
		diagnostic.CodeFeatureRestriction,
		diagnostic.CodeResourceLimit,
	}
	seen := make(map[diagnostic.Code]struct{}, len(codes))
	for _, code := range codes {
		if code == "" {
			t.Error("required diagnostic code is empty")
		}
		if _, exists := seen[code]; exists {
			t.Errorf("duplicate required diagnostic code %q", code)
		}
		seen[code] = struct{}{}
	}
}

func TestNormalizeOrdersDeduplicatesAndOwnsNotes(t *testing.T) {
	t.Parallel()

	note := []diagnostic.Note{{Message: "original"}}
	later := diagnostic.New(
		diagnostic.CodeUnexpectedToken,
		diagnostic.SeverityError,
		diagnostic.Span{Start: 4, End: 5},
		"later",
		0,
		note,
	)
	earlier := diagnostic.NewSource(
		diagnostic.CodeInvalidUTF8,
		diagnostic.SeverityError,
		diagnostic.Span{Start: 1, End: 2},
		"earlier",
	)
	values := diagnostic.Normalize([]diagnostic.Diagnostic{later}, []diagnostic.Diagnostic{
		earlier,
		later,
	})
	if got, want := len(values), 2; got != want {
		t.Fatalf("diagnostic count = %d, want %d: %+v", got, want, values)
	}
	if values[0].Code != diagnostic.CodeInvalidUTF8 ||
		values[1].Code != diagnostic.CodeUnexpectedToken {
		t.Fatalf("diagnostic order = %+v", values)
	}
	values[1].Notes[0].Message = "changed"
	if got, want := later.Notes[0].Message, "original"; got != want {
		t.Fatalf("input note after result mutation = %q, want %q", got, want)
	}
}

package diagnostic

import (
	"cmp"
	"slices"

	cst "github.com/albertocavalcante/cst-go"

	"git.alberto.engineer/alberto/java-cst-go/language"
)

// Span is a half-open UTF-8 byte range in the exact raw source.
type Span = cst.Span

// Severity classifies a diagnostic's gravity.
type Severity uint8

const (
	SeverityError Severity = iota
	SeverityWarning
	SeverityInfo
)

// String returns the human label for a severity.
func (s Severity) String() string {
	switch s {
	case SeverityError:
		return "error"
	case SeverityWarning:
		return "warning"
	case SeverityInfo:
		return "info"
	default:
		return "unknown"
	}
}

// Note carries deterministic supplementary diagnostic information.
type Note struct {
	Span    Span
	Message string
}

// Diagnostic describes one Java source or configuration problem.
//
// Span is always expressed in raw input bytes. Feature is zero for
// diagnostics unrelated to a language feature. Callers that retain or publish
// diagnostics must own the Notes slice; New and Clone provide that ownership.
type Diagnostic struct {
	Code     Code
	Severity Severity
	Span     Span
	Message  string
	Feature  language.FeatureID
	Notes    []Note
}

// NewSource constructs a diagnostic unrelated to one language feature.
func NewSource(
	code Code,
	severity Severity,
	span Span,
	message string,
) Diagnostic {
	return New(code, severity, span, message, 0, nil)
}

// New constructs a diagnostic and takes a defensive copy of notes.
func New(
	code Code,
	severity Severity,
	span Span,
	message string,
	feature language.FeatureID,
	notes []Note,
) Diagnostic {
	return Diagnostic{
		Code:     code,
		Severity: severity,
		Span:     span,
		Message:  message,
		Feature:  feature,
		Notes:    append([]Note(nil), notes...),
	}
}

// Clone returns a value with independent slice storage.
func (d Diagnostic) Clone() Diagnostic {
	d.Notes = append([]Note(nil), d.Notes...)
	return d
}

// Normalize clones, orders, and exactly deduplicates diagnostic groups.
func Normalize(groups ...[]Diagnostic) []Diagnostic {
	total := 0
	for _, group := range groups {
		total += len(group)
	}
	values := make([]Diagnostic, 0, total)
	for _, group := range groups {
		for _, value := range group {
			values = append(values, value.Clone())
		}
	}

	slices.SortStableFunc(values, compare)
	output := values[:0]
	for _, value := range values {
		if len(output) > 0 && equal(output[len(output)-1], value) {
			continue
		}
		output = append(output, value)
	}
	return output
}

func compare(left, right Diagnostic) int {
	if order := cmp.Compare(left.Span.Start, right.Span.Start); order != 0 {
		return order
	}
	if order := cmp.Compare(left.Span.End, right.Span.End); order != 0 {
		return order
	}
	if order := cmp.Compare(left.Code, right.Code); order != 0 {
		return order
	}
	if order := cmp.Compare(left.Message, right.Message); order != 0 {
		return order
	}
	if order := cmp.Compare(left.Feature, right.Feature); order != 0 {
		return order
	}
	return cmp.Compare(left.Severity, right.Severity)
}

func equal(left, right Diagnostic) bool {
	return left.Code == right.Code &&
		left.Severity == right.Severity &&
		left.Span == right.Span &&
		left.Message == right.Message &&
		left.Feature == right.Feature &&
		slices.Equal(left.Notes, right.Notes)
}

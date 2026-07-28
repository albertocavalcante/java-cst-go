package source

import (
	"fmt"
	"iter"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	cst "github.com/albertocavalcante/cst-go"

	"git.alberto.engineer/alberto/java-cst-go/diagnostic"
)

// Span is a half-open UTF-8 byte range.
type Span = cst.Span

// Segment maps one contiguous raw region to its logical UTF-8 representation.
type Segment struct {
	RawSpan     Span
	LogicalSpan Span
}

// Translation is an immutable raw/logical Java Unicode-escape translation.
type Translation struct {
	raw         string
	logical     string
	segments    []Segment
	diagnostics []diagnostic.Diagnostic
}

// Translate applies the Java Unicode-escape eligibility rules to raw.
//
// The translator preserves invalid UTF-8 bytes in the logical string while
// diagnosing them. An isolated UTF-16 surrogate escape is represented as
// U+FFFD and diagnosed because Go strings have no UTF-16-code-unit encoding.
func Translate(raw string) *Translation {
	builder := translationBuilder{raw: raw}
	builder.translate()
	return &Translation{
		raw:         raw,
		logical:     builder.logical.String(),
		segments:    builder.segments,
		diagnostics: builder.diagnostics,
	}
}

// Raw returns the exact source supplied to Translate.
func (t *Translation) Raw() string {
	if t == nil {
		return ""
	}
	return t.raw
}

// Logical returns the Java Unicode-escape-translated UTF-8 stream.
func (t *Translation) Logical() string {
	if t == nil {
		return ""
	}
	return t.logical
}

// Segments yields immutable raw/logical mapping segments.
func (t *Translation) Segments() iter.Seq[Segment] {
	return func(yield func(Segment) bool) {
		if t == nil {
			return
		}
		for index := range t.segments {
			if !yield(t.segments[index]) {
				return
			}
		}
	}
}

// Diagnostics returns a defensive copy of translation diagnostics.
func (t *Translation) Diagnostics() []diagnostic.Diagnostic {
	if t == nil {
		return nil
	}
	result := make([]diagnostic.Diagnostic, len(t.diagnostics))
	for index := range t.diagnostics {
		result[index] = t.diagnostics[index].Clone()
	}
	return result
}

// RawSpan maps a bounded logical span to the smallest covering raw span.
func (t *Translation) RawSpan(logical Span) (Span, bool) {
	if t == nil {
		return Span{}, false
	}
	return mapSpan(logical, len(t.logical), t.segments, false)
}

// LogicalSpan maps a bounded raw span to the smallest covering logical span.
func (t *Translation) LogicalSpan(raw Span) (Span, bool) {
	if t == nil {
		return Span{}, false
	}
	return mapSpan(raw, len(t.raw), t.segments, true)
}

type translationBuilder struct {
	raw                   string
	logical               strings.Builder
	segments              []Segment
	diagnostics           []diagnostic.Diagnostic
	trailingBackslashes   int
	lastFromUnicodeEscape bool
}

func (b *translationBuilder) translate() {
	for rawOffset := 0; rawOffset < len(b.raw); {
		if b.raw[rawOffset] == '\\' && b.backslashEligible() {
			value, end, status := parseUnicodeEscape(b.raw, rawOffset)
			switch status {
			case escapeValid:
				if utf16.IsSurrogate(rune(value)) {
					if utf16.IsSurrogate(rune(value)) &&
						rune(value) >= 0xd800 &&
						rune(value) <= 0xdbff {
						if low, lowEnd, ok := b.parseFollowingLowSurrogate(end); ok {
							b.appendTranslated(
								rawOffset,
								lowEnd,
								string(utf16.DecodeRune(rune(value), rune(low))),
							)
							rawOffset = lowEnd
							continue
						}
					}
					b.diagnostics = append(b.diagnostics, diagnostic.NewSource(
						diagnostic.CodeInvalidUnicodeEscape,
						diagnostic.SeverityError,
						Span{Start: rawOffset, End: end},
						"isolated UTF-16 surrogate in Unicode escape",
					))
					b.appendTranslated(rawOffset, end, string(utf8.RuneError))
					rawOffset = end
					continue
				}

				b.appendTranslated(rawOffset, end, string(rune(value)))
				rawOffset = end
				continue
			case escapeMalformed:
				b.diagnostics = append(b.diagnostics, diagnostic.NewSource(
					diagnostic.CodeInvalidUnicodeEscape,
					diagnostic.SeverityError,
					Span{Start: rawOffset, End: end},
					"invalid Java Unicode escape",
				))
			case escapeAbsent:
			}
		}

		width := 1
		if b.raw[rawOffset] >= utf8.RuneSelf {
			_, decodedWidth := utf8.DecodeRuneInString(b.raw[rawOffset:])
			if decodedWidth > 1 {
				width = decodedWidth
			} else {
				b.diagnostics = append(b.diagnostics, diagnostic.NewSource(
					diagnostic.CodeInvalidUTF8,
					diagnostic.SeverityError,
					Span{Start: rawOffset, End: rawOffset + 1},
					"invalid UTF-8 byte in Java source",
				))
			}
		}
		b.appendRaw(rawOffset, rawOffset+width)
		rawOffset += width
	}
}

func (b *translationBuilder) backslashEligible() bool {
	return b.lastFromUnicodeEscape || b.trailingBackslashes%2 == 0
}

func (b *translationBuilder) parseFollowingLowSurrogate(
	rawOffset int,
) (uint16, int, bool) {
	if rawOffset >= len(b.raw) || b.raw[rawOffset] != '\\' {
		return 0, rawOffset, false
	}

	value, end, status := parseUnicodeEscape(b.raw, rawOffset)
	if status != escapeValid || value < 0xdc00 || value > 0xdfff {
		return 0, rawOffset, false
	}
	return value, end, true
}

func (b *translationBuilder) appendRaw(start, end int) {
	logicalStart := b.logical.Len()
	text := b.raw[start:end]
	b.logical.WriteString(text)
	b.appendSegment(Segment{
		RawSpan:     Span{Start: start, End: end},
		LogicalSpan: Span{Start: logicalStart, End: b.logical.Len()},
	})

	if text == "\\" {
		b.trailingBackslashes++
	} else {
		b.trailingBackslashes = 0
	}
	b.lastFromUnicodeEscape = false
}

func (b *translationBuilder) appendTranslated(start, end int, text string) {
	logicalStart := b.logical.Len()
	b.logical.WriteString(text)
	b.segments = append(b.segments, Segment{
		RawSpan:     Span{Start: start, End: end},
		LogicalSpan: Span{Start: logicalStart, End: b.logical.Len()},
	})

	if text == "\\" {
		b.trailingBackslashes++
	} else {
		b.trailingBackslashes = 0
	}
	b.lastFromUnicodeEscape = true
}

func (b *translationBuilder) appendSegment(segment Segment) {
	if len(b.segments) > 0 {
		last := &b.segments[len(b.segments)-1]
		if last.RawSpan.End == segment.RawSpan.Start &&
			last.LogicalSpan.End == segment.LogicalSpan.Start &&
			last.RawSpan.Len() == last.LogicalSpan.Len() &&
			segment.RawSpan.Len() == segment.LogicalSpan.Len() {
			last.RawSpan.End = segment.RawSpan.End
			last.LogicalSpan.End = segment.LogicalSpan.End
			return
		}
	}
	b.segments = append(b.segments, segment)
}

type escapeStatus uint8

const (
	escapeAbsent escapeStatus = iota
	escapeValid
	escapeMalformed
)

func parseUnicodeEscape(raw string, start int) (uint16, int, escapeStatus) {
	if start+1 >= len(raw) || raw[start] != '\\' || raw[start+1] != 'u' {
		return 0, start + 1, escapeAbsent
	}

	offset := start + 1
	for offset < len(raw) && raw[offset] == 'u' {
		offset++
	}
	digitsStart := offset
	end := min(offset+4, len(raw))
	if end-digitsStart != 4 {
		return 0, end, escapeMalformed
	}

	var value uint16
	for ; offset < end; offset++ {
		digit, ok := hexValue(raw[offset])
		if !ok {
			return 0, end, escapeMalformed
		}
		value = value*16 + uint16(digit)
	}
	return value, end, escapeValid
}

func hexValue(value byte) (uint8, bool) {
	switch {
	case value >= '0' && value <= '9':
		return value - '0', true
	case value >= 'a' && value <= 'f':
		return value - 'a' + 10, true
	case value >= 'A' && value <= 'F':
		return value - 'A' + 10, true
	default:
		return 0, false
	}
}

func mapSpan(
	input Span,
	inputLength int,
	segments []Segment,
	reverse bool,
) (Span, bool) {
	if input.Start < 0 || input.End < input.Start || input.End > inputLength {
		return Span{}, false
	}
	if input.Start == input.End {
		return mapPoint(input.Start, inputLength, segments, reverse)
	}

	first := -1
	last := -1
	for index := range segments {
		from, _ := segmentSides(segments[index], reverse)
		if from.End <= input.Start || from.Start >= input.End {
			continue
		}
		if first < 0 {
			first = index
		}
		last = index
	}
	if first < 0 {
		return Span{}, false
	}

	firstFrom, firstTo := segmentSides(segments[first], reverse)
	lastFrom, lastTo := segmentSides(segments[last], reverse)
	start := firstTo.Start
	if firstFrom.Len() == firstTo.Len() {
		start += input.Start - firstFrom.Start
	}
	end := lastTo.End
	if lastFrom.Len() == lastTo.Len() {
		end -= lastFrom.End - input.End
	}

	return Span{Start: start, End: end}, true
}

func mapPoint(
	point int,
	inputLength int,
	segments []Segment,
	reverse bool,
) (Span, bool) {
	if point < 0 || point > inputLength {
		return Span{}, false
	}
	if len(segments) == 0 {
		return Span{}, point == 0
	}
	if point == inputLength {
		_, to := segmentSides(segments[len(segments)-1], reverse)
		return Span{Start: to.End, End: to.End}, true
	}

	for index := range segments {
		from, to := segmentSides(segments[index], reverse)
		if point < from.Start || point >= from.End {
			continue
		}
		if from.Len() == to.Len() {
			mapped := to.Start + point - from.Start
			return Span{Start: mapped, End: mapped}, true
		}
		if point == from.Start {
			return Span{Start: to.Start, End: to.Start}, true
		}
		return Span{Start: to.Start, End: to.End}, true
	}

	return Span{}, false
}

func segmentSides(segment Segment, reverse bool) (from, to Span) {
	if reverse {
		return segment.RawSpan, segment.LogicalSpan
	}
	return segment.LogicalSpan, segment.RawSpan
}

// String provides compact debugging output for mapping diagnostics.
func (s Segment) String() string {
	return fmt.Sprintf(
		"raw[%d,%d)->logical[%d,%d)",
		s.RawSpan.Start,
		s.RawSpan.End,
		s.LogicalSpan.Start,
		s.LogicalSpan.End,
	)
}

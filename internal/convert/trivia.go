package convert

import (
	"bytes"
	"fmt"

	javasource "git.alberto.engineer/alberto/java-cst-go/source"
	"git.alberto.engineer/alberto/java-cst-go/syntax"
)

var utf8BOM = []byte{0xef, 0xbb, 0xbf}

func classifyTrivia(source string, start, end uint32) []syntax.Trivia {
	pieces := scanTrivia(source, start, end)
	trivia := make([]syntax.Trivia, 0, len(pieces))
	for _, piece := range pieces {
		trivia = append(
			trivia,
			syntax.NewTrivia(piece.kind, source[piece.start:piece.end]),
		)
	}
	return trivia
}

func classifyTranslationTrivia(
	translation *javasource.Translation,
	rawStart, rawEnd uint32,
) ([]syntax.Trivia, error) {
	if rawStart == rawEnd {
		return nil, nil
	}

	logicalSpan, ok := translation.LogicalSpan(javasource.Span{
		Start: int(rawStart),
		End:   int(rawEnd),
	})
	if !ok {
		return nil, fmt.Errorf(
			"convert backend snapshot: cannot map raw trivia range [%d,%d) to logical source",
			rawStart,
			rawEnd,
		)
	}

	pieces := scanTrivia(
		translation.Logical(),
		uint32(logicalSpan.Start),
		uint32(logicalSpan.End),
	)
	trivia := make([]syntax.Trivia, 0, len(pieces))
	cursor := rawStart
	for _, piece := range pieces {
		rawSpan, mapped := translation.RawSpan(javasource.Span{
			Start: int(piece.start),
			End:   int(piece.end),
		})
		if !mapped {
			return nil, fmt.Errorf(
				"convert backend snapshot: cannot map logical trivia range [%d,%d) to raw source",
				piece.start,
				piece.end,
			)
		}
		if rawSpan.Start != int(cursor) ||
			rawSpan.End < rawSpan.Start ||
			rawSpan.End > int(rawEnd) {
			return nil, fmt.Errorf(
				"convert backend snapshot: logical trivia range [%d,%d) maps to non-contiguous raw range [%d,%d), cursor %d, end %d",
				piece.start,
				piece.end,
				rawSpan.Start,
				rawSpan.End,
				cursor,
				rawEnd,
			)
		}

		trivia = append(
			trivia,
			syntax.NewTrivia(
				piece.kind,
				translation.Raw()[rawSpan.Start:rawSpan.End],
			),
		)
		cursor = uint32(rawSpan.End)
	}
	if cursor != rawEnd {
		return nil, fmt.Errorf(
			"convert backend snapshot: translated trivia ends at raw byte %d, want %d",
			cursor,
			rawEnd,
		)
	}

	return trivia, nil
}

type triviaPiece struct {
	kind       syntax.TriviaKind
	start, end uint32
}

func scanTrivia(source string, start, end uint32) []triviaPiece {
	if start == end {
		return nil
	}

	data := []byte(source[start:end])
	pieces := make([]triviaPiece, 0, 4)
	for offset := 0; offset < len(data); {
		itemStart := offset
		kind := syntax.TriviaInvalidText

		switch {
		case start+uint32(offset) == 0 && bytes.HasPrefix(data[offset:], utf8BOM):
			kind = syntax.TriviaBOM
			offset += len(utf8BOM)
		case isHorizontalWhitespace(data[offset]):
			kind = syntax.TriviaWhitespace
			for offset < len(data) && isHorizontalWhitespace(data[offset]) {
				offset++
			}
		case data[offset] == '\r':
			kind = syntax.TriviaLineTerminator
			offset++
			if offset < len(data) && data[offset] == '\n' {
				offset++
			}
		case data[offset] == '\n':
			kind = syntax.TriviaLineTerminator
			offset++
		case bytes.HasPrefix(data[offset:], []byte("//")):
			kind = syntax.TriviaLineComment
			offset += 2
			for offset < len(data) && data[offset] != '\r' && data[offset] != '\n' {
				offset++
			}
		case bytes.HasPrefix(data[offset:], []byte("/*")):
			kind = syntax.TriviaBlockComment
			if bytes.HasPrefix(data[offset:], []byte("/**")) {
				kind = syntax.TriviaDocumentationComment
			}
			offset += 2
			if closeOffset := bytes.Index(data[offset:], []byte("*/")); closeOffset >= 0 {
				offset += closeOffset + 2
			} else {
				offset = len(data)
			}
		default:
			offset++
			for offset < len(data) && !startsRecognizedTrivia(
				data,
				offset,
				start+uint32(offset),
			) {
				offset++
			}
		}

		pieces = append(pieces, triviaPiece{
			kind:  kind,
			start: start + uint32(itemStart),
			end:   start + uint32(offset),
		})
	}

	return pieces
}

func splitInterTokenTrivia(
	items []syntax.Trivia,
) (trailing, leading []syntax.Trivia) {
	for index := range items {
		if items[index].Kind() != syntax.TriviaLineTerminator {
			continue
		}
		return items[:index+1], items[index+1:]
	}

	return items, nil
}

func isHorizontalWhitespace(value byte) bool {
	switch value {
	case ' ', '\t', '\f':
		return true
	default:
		return false
	}
}

func startsRecognizedTrivia(data []byte, offset int, absoluteOffset uint32) bool {
	value := data[offset]
	return isHorizontalWhitespace(value) ||
		value == '\r' ||
		value == '\n' ||
		bytes.HasPrefix(data[offset:], []byte("//")) ||
		bytes.HasPrefix(data[offset:], []byte("/*")) ||
		(absoluteOffset == 0 && bytes.HasPrefix(data[offset:], utf8BOM))
}

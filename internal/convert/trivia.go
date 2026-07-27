package convert

import (
	"bytes"

	"git.alberto.engineer/alberto/java-cst-go/syntax"
)

var utf8BOM = []byte{0xef, 0xbb, 0xbf}

func classifyTrivia(source string, start, end uint32) []syntax.Trivia {
	if start == end {
		return nil
	}

	data := []byte(source[start:end])
	trivia := make([]syntax.Trivia, 0, 4)
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

		trivia = append(
			trivia,
			syntax.NewTrivia(kind, string(data[itemStart:offset])),
		)
	}

	return trivia
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

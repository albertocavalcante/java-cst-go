package diagnose

import (
	"git.alberto.engineer/alberto/java-cst-go/diagnostic"
	"git.alberto.engineer/alberto/java-cst-go/source"
)

const maxTemplateNesting = 256

// Lexical reports unterminated comments and literals in the Java
// Unicode-translated stream while preserving raw-source diagnostic spans.
func Lexical(translation *source.Translation) []diagnostic.Diagnostic {
	if translation == nil {
		return nil
	}
	scanner := lexicalScanner{
		translation: translation,
		logical:     translation.Logical(),
	}
	scanner.scanCode(0, 0)
	return diagnostic.Normalize(scanner.values)
}

type lexicalScanner struct {
	translation   *source.Translation
	logical       string
	values        []diagnostic.Diagnostic
	templateDepth int
}

func (s *lexicalScanner) scanCode(start, braceDepth int) (int, bool) {
	for offset := start; offset < len(s.logical); {
		switch {
		case hasPrefixAt(s.logical, offset, "//"):
			offset = s.scanLineComment(offset + 2)
		case hasPrefixAt(s.logical, offset, "/*"):
			offset = s.scanBlockComment(offset)
		case hasPrefixAt(s.logical, offset, `"""`):
			offset = s.scanTextBlock(offset, s.templateOpener(offset))
		case s.logical[offset] == '"':
			offset = s.scanString(offset, s.templateOpener(offset))
		case s.logical[offset] == '\'':
			offset = s.scanCharacter(offset)
		case braceDepth > 0 && s.logical[offset] == '{':
			braceDepth++
			offset++
		case braceDepth > 0 && s.logical[offset] == '}':
			braceDepth--
			offset++
			if braceDepth == 0 {
				return offset, true
			}
		default:
			offset++
		}
	}
	return len(s.logical), braceDepth == 0
}

func (s *lexicalScanner) scanLineComment(offset int) int {
	for offset < len(s.logical) && !isLineTerminator(s.logical[offset]) {
		offset++
	}
	return offset
}

func (s *lexicalScanner) scanBlockComment(start int) int {
	for offset := start + 2; offset < len(s.logical); offset++ {
		if hasPrefixAt(s.logical, offset, "*/") {
			return offset + 2
		}
	}
	s.report(
		diagnostic.CodeUnterminatedComment,
		start,
		len(s.logical),
		"unterminated block comment",
	)
	return len(s.logical)
}

func (s *lexicalScanner) scanString(start int, template bool) int {
	for offset := start + 1; offset < len(s.logical); {
		switch {
		case s.logical[offset] == '"':
			return offset + 1
		case isLineTerminator(s.logical[offset]):
			s.report(
				diagnostic.CodeUnterminatedLiteral,
				start,
				offset,
				"unterminated string literal",
			)
			return offset
		case s.logical[offset] == '\\':
			if template && hasPrefixAt(s.logical, offset, `\{`) {
				if s.templateDepth >= maxTemplateNesting {
					offset += 2
					continue
				}
				s.templateDepth++
				next, closed := s.scanCode(offset+2, 1)
				s.templateDepth--
				if !closed {
					s.report(
						diagnostic.CodeUnterminatedLiteral,
						start,
						len(s.logical),
						"unterminated string template",
					)
					return len(s.logical)
				}
				offset = next
				continue
			}
			if offset+1 < len(s.logical) &&
				!isLineTerminator(s.logical[offset+1]) {
				offset += 2
				continue
			}
			if offset+1 < len(s.logical) {
				s.report(
					diagnostic.CodeUnterminatedLiteral,
					start,
					offset+1,
					"unterminated string literal",
				)
				return offset + 1
			}
			offset++
		default:
			offset++
		}
	}
	s.report(
		diagnostic.CodeUnterminatedLiteral,
		start,
		len(s.logical),
		"unterminated string literal",
	)
	return len(s.logical)
}

func (s *lexicalScanner) scanCharacter(start int) int {
	for offset := start + 1; offset < len(s.logical); {
		switch {
		case s.logical[offset] == '\'':
			return offset + 1
		case isLineTerminator(s.logical[offset]):
			s.report(
				diagnostic.CodeUnterminatedLiteral,
				start,
				offset,
				"unterminated character literal",
			)
			return offset
		case s.logical[offset] == '\\':
			if offset+1 < len(s.logical) &&
				!isLineTerminator(s.logical[offset+1]) {
				offset += 2
				continue
			}
			if offset+1 < len(s.logical) {
				s.report(
					diagnostic.CodeUnterminatedLiteral,
					start,
					offset+1,
					"unterminated character literal",
				)
				return offset + 1
			}
			offset++
		default:
			offset++
		}
	}
	s.report(
		diagnostic.CodeUnterminatedLiteral,
		start,
		len(s.logical),
		"unterminated character literal",
	)
	return len(s.logical)
}

func (s *lexicalScanner) scanTextBlock(start int, template bool) int {
	for offset := start + 3; offset < len(s.logical); {
		switch {
		case hasPrefixAt(s.logical, offset, `"""`):
			return offset + 3
		case s.logical[offset] == '\\':
			if template && hasPrefixAt(s.logical, offset, `\{`) {
				if s.templateDepth >= maxTemplateNesting {
					offset += 2
					continue
				}
				s.templateDepth++
				next, closed := s.scanCode(offset+2, 1)
				s.templateDepth--
				if !closed {
					s.report(
						diagnostic.CodeUnterminatedLiteral,
						start,
						len(s.logical),
						"unterminated text block template",
					)
					return len(s.logical)
				}
				offset = next
				continue
			}
			if offset+1 < len(s.logical) {
				offset += 2
				continue
			}
			offset++
		default:
			offset++
		}
	}
	s.report(
		diagnostic.CodeUnterminatedLiteral,
		start,
		len(s.logical),
		"unterminated text block",
	)
	return len(s.logical)
}

func (s *lexicalScanner) templateOpener(offset int) bool {
	for offset--; offset >= 0; offset-- {
		switch s.logical[offset] {
		case ' ', '\t', '\r', '\n', '\f':
			continue
		default:
			return s.logical[offset] == '.'
		}
	}
	return false
}

func (s *lexicalScanner) report(
	code diagnostic.Code,
	logicalStart, logicalEnd int,
	message string,
) {
	raw, ok := s.translation.RawSpan(source.Span{
		Start: logicalStart,
		End:   logicalEnd,
	})
	if !ok {
		return
	}
	s.values = append(s.values, diagnostic.NewSource(
		code,
		diagnostic.SeverityError,
		raw,
		message,
	))
}

func hasPrefixAt(text string, offset int, prefix string) bool {
	return offset >= 0 &&
		offset+len(prefix) <= len(text) &&
		text[offset:offset+len(prefix)] == prefix
}

func isLineTerminator(value byte) bool {
	return value == '\r' || value == '\n'
}

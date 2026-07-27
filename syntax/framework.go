package syntax

import cst "github.com/albertocavalcante/cst-go"

// Kind is a repository-owned Java syntax kind.
//
// M0 uses stable, prefixed grammar names. A generated closed enum may replace
// this representation before the public syntax API is frozen.
type Kind string

// TriviaKind is a repository-owned Java trivia kind.
type TriviaKind uint8

const (
	TriviaWhitespace TriviaKind = iota + 1
	TriviaLineTerminator
	TriviaLineComment
	TriviaBlockComment
	TriviaDocumentationComment
	TriviaBOM
	TriviaSkippedTokens
	TriviaInvalidText
)

// DiagnosticCode is a stable Java diagnostic identifier.
type DiagnosticCode string

// TokenData carries immutable Java lexical metadata. Its fields will be added
// only when the M0 conversion demonstrates that they belong on green tokens.
type TokenData struct{}

type (
	Span         = cst.Span
	ElementID    = cst.ElementID
	Trivia       = cst.Trivia[TriviaKind]
	Token        = cst.Token[Kind, TriviaKind, TokenData]
	GreenNode    = cst.GreenNode[Kind, TriviaKind, TokenData]
	GreenElement = cst.GreenElement[Kind, TriviaKind, TokenData]
	Builder      = cst.Builder[Kind, TriviaKind, TokenData]
	RedNode      = cst.RedNode[Kind, TriviaKind, TokenData]
	RedToken     = cst.RedToken[Kind, TriviaKind, TokenData]
	Tree         = cst.Tree[Kind, TriviaKind, TokenData, DiagnosticCode]
)

// NodeKind maps one backend-neutral grammar name into the M0 node namespace.
func NodeKind(name string) Kind {
	return Kind("node:" + name)
}

// TokenKind maps one backend-neutral grammar name into the M0 token namespace.
func TokenKind(name string) Kind {
	return Kind("token:" + name)
}

// NewTrivia constructs immutable Java trivia.
func NewTrivia(kind TriviaKind, text string) Trivia {
	return cst.NewTrivia(kind, text)
}

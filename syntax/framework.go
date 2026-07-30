package syntax

import (
	cst "github.com/albertocavalcante/cst-go"

	"git.alberto.engineer/alberto/java-cst-go/diagnostic"
)

// Kind is a repository-owned Java syntax kind.
//
// During the staged M2 schema migration, kinds retain their stable prefixed
// spelling. Public consumers should use the generated KindNode... and
// KindToken... constants rather than constructing values.
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
type DiagnosticCode = diagnostic.Code

// TokenData carries immutable Java lexical metadata. Its fields will be added
// only when the M0 conversion demonstrates that they belong on green tokens.
type TokenData struct {
	// LogicalText is the Java Unicode-translated token spelling. Token.Text()
	// remains the exact raw spelling used for lossless emission.
	LogicalText string
}

type (
	Span         = cst.Span
	ElementID    = cst.ElementID
	Trivia       = cst.Trivia[TriviaKind]
	GreenToken   = cst.Token[Kind, TriviaKind, TokenData]
	Token        = cst.RedToken[Kind, TriviaKind, TokenData]
	GreenNode    = cst.GreenNode[Kind, TriviaKind, TokenData]
	GreenElement = cst.GreenElement[Kind, TriviaKind, TokenData]
	Builder      = cst.Builder[Kind, TriviaKind, TokenData]
	Node         = cst.RedNode[Kind, TriviaKind, TokenData]
	RedNode      = cst.RedNode[Kind, TriviaKind, TokenData]
	RedToken     = cst.RedToken[Kind, TriviaKind, TokenData]
	CoreTree     = cst.Tree[Kind, TriviaKind, TokenData, DiagnosticCode]
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

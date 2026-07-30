package syntax

import (
	cst "github.com/albertocavalcante/cst-go"

	"git.alberto.engineer/alberto/java-cst-go/diagnostic"
)

// Kind is a repository-owned Java syntax kind.
//
// Its stable prefixed value is independent of backend numeric symbols. Public
// consumers should use the generated KindNode... and KindToken... constants
// rather than constructing values.
type Kind string

// KindCategory classifies a stable kind as a node or token.
type KindCategory uint8

const (
	KindCategoryUnknown KindCategory = iota
	KindCategoryNode
	KindCategoryToken
)

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
	Element      = cst.RedElement[Kind, TriviaKind, TokenData]
	Node         = cst.RedNode[Kind, TriviaKind, TokenData]
	RedNode      = cst.RedNode[Kind, TriviaKind, TokenData]
	RedToken     = cst.RedToken[Kind, TriviaKind, TokenData]
	CoreTree     = cst.Tree[Kind, TriviaKind, TokenData, DiagnosticCode]
)

// NodeKind maps a registered backend-neutral grammar name to its stable node
// kind. Unknown names return KindUnknown.
func NodeKind(name string) Kind {
	kind, _ := LookupNodeKind(name)
	return kind
}

// TokenKind maps a registered backend-neutral grammar name to its stable token
// kind. Unknown names return KindUnknown.
func TokenKind(name string) Kind {
	kind, _ := LookupTokenKind(name)
	return kind
}

// NewTrivia constructs immutable Java trivia.
func NewTrivia(kind TriviaKind, text string) Trivia {
	return cst.NewTrivia(kind, text)
}

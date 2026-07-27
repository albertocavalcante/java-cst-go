package syntax

import cst "github.com/albertocavalcante/cst-go"

// Kind is a repository-owned Java syntax kind.
type Kind uint16

// TriviaKind is a repository-owned Java trivia kind.
type TriviaKind uint8

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

package lua

import "github.com/Lanterna-Games/defold-tools/internal/source"

type TokenKind int

const (
	TokenEndOfFile TokenKind = iota
	TokenName
	TokenKeyword
	TokenNumber
	TokenString
	TokenSymbol
)

type Token struct {
	Kind  TokenKind
	Text  string
	Start source.Position
	End   source.Position
}

func (t Token) Is(kind TokenKind, text string) bool {
	return t.Kind == kind && t.Text == text
}

type Comment struct {
	Text             string
	Start            source.Position
	End              source.Position
	IsAloneOnItsLine bool
}

type SyntaxError struct {
	Position source.Position
	Message  string
}

var keywords = map[string]bool{
	"and": true, "break": true, "do": true, "else": true, "elseif": true, "end": true,
	"false": true, "for": true, "function": true, "if": true, "in": true, "local": true,
	"nil": true, "not": true, "or": true, "repeat": true, "return": true, "then": true,
	"true": true, "until": true, "while": true,
}

var symbolsLongestFirst = []string{
	"...", "..", "==", "~=", "<=", ">=",
	"+", "-", "*", "/", "%", "^", "#", "<", ">", "=",
	"(", ")", "{", "}", "[", "]", ";", ":", ",", ".",
}

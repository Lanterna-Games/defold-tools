package lua

import (
	"strings"
	"testing"
)

func tokensOf(t *testing.T, content string) []Token {
	t.Helper()
	l := newLexer([]byte(content))
	var tokens []Token
	for {
		token := l.next()
		if token.Kind == TokenEndOfFile {
			break
		}
		tokens = append(tokens, token)
	}
	for _, err := range l.errors {
		t.Errorf("unexpected syntax error at %d:%d: %s", err.Position.Line, err.Position.Column, err.Message)
	}
	return tokens
}

func textsOf(tokens []Token) string {
	texts := make([]string, 0, len(tokens))
	for _, token := range tokens {
		texts = append(texts, token.Text)
	}
	return strings.Join(texts, " ")
}

func TestLexerSplitsNamesKeywordsNumbersAndSymbols(t *testing.T) {
	tokens := tokensOf(t, `local x = msg.post("#sprite", 3.5e-2) ~= nil`)

	if got, want := textsOf(tokens), "local x = msg . post ( #sprite , 3.5e-2 ) ~= nil"; got != want {
		t.Errorf("tokens = %q, want %q", got, want)
	}
	kinds := map[string]TokenKind{"local": TokenKeyword, "x": TokenName, "=": TokenSymbol, "#sprite": TokenString, "3.5e-2": TokenNumber, "nil": TokenKeyword}
	for _, token := range tokens {
		if want, checked := kinds[token.Text]; checked && token.Kind != want {
			t.Errorf("%q has kind %d, want %d", token.Text, token.Kind, want)
		}
	}
}

func TestLexerNumbers(t *testing.T) {
	for _, number := range []string{"0", "42", "3.14", ".5", "1e10", "1E+3", "2.5e-3", "0xFF", "0x1p-4", "100ULL"} {
		t.Run(number, func(t *testing.T) {
			tokens := tokensOf(t, number)
			if len(tokens) != 1 || tokens[0].Kind != TokenNumber || tokens[0].Text != number {
				t.Errorf("tokens = %+v, want one number %q", tokens, number)
			}
		})
	}
}

// A minus after a number is a subtraction unless the number is in the middle
// of an exponent.
func TestLexerDoesNotSwallowTheOperatorAfterANumber(t *testing.T) {
	if got, want := textsOf(tokensOf(t, "1-2 0xE-1 1e-2")), "1 - 2 0xE - 1 1e-2"; got != want {
		t.Errorf("tokens = %q, want %q", got, want)
	}
}

func TestLexerStrings(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{name: "double quotes", content: `"#sprite"`, want: "#sprite"},
		{name: "single quotes", content: `'main:/game'`, want: "main:/game"},
		{name: "escapes", content: `"a\tb\n\"c\"\\"`, want: "a\tb\n\"c\"\\"},
		{name: "decimal and hexadecimal bytes", content: `"\65\066\x43"`, want: "ABC"},
		{name: "unicode", content: `"\u{48}i"`, want: "Hi"},
		{name: "skipped whitespace", content: "\"one\\z  \n  two\"", want: "onetwo"},
		{name: "long bracket", content: "[[two\nlines]]", want: "two\nlines"},
		// The newline right after the opening bracket is not part of the string.
		{name: "long bracket drops the first newline", content: "[[\nfirst]]", want: "first"},
		{name: "long bracket with a level", content: "[==[a ]] b]==]", want: "a ]] b"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens := tokensOf(t, tt.content)
			if len(tokens) != 1 || tokens[0].Kind != TokenString || tokens[0].Text != tt.want {
				t.Errorf("tokens = %+v, want one string %q", tokens, tt.want)
			}
		})
	}
}

func TestLexerKeepsCommentsAside(t *testing.T) {
	l := newLexer([]byte("local x -- defold-vet: ignore\n--[[ block\ncomment ]] local y"))
	var texts []string
	for token := l.next(); token.Kind != TokenEndOfFile; token = l.next() {
		texts = append(texts, token.Text)
	}

	if got, want := strings.Join(texts, " "), "local x local y"; got != want {
		t.Errorf("tokens = %q, want %q", got, want)
	}
	if len(l.comments) != 2 || l.comments[0].Text != " defold-vet: ignore" || l.comments[0].Start.Line != 1 {
		t.Errorf("comments = %+v", l.comments)
	}
}

func TestLexerPositions(t *testing.T) {
	tokens := tokensOf(t, "local x\n  = \"ab\"")

	text := tokens[3]
	if text.Start.Line != 2 || text.Start.Column != 5 || text.End.Column != 9 {
		t.Errorf("string spans %d:%d to column %d, want 2:5 to column 9", text.Start.Line, text.Start.Column, text.End.Column)
	}
}

func TestLexerReportsWhatItCannotRead(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{name: "unclosed string", content: `x = "abc`, want: "string is not closed"},
		{name: "unclosed long bracket", content: "x = [[abc", want: "long bracket is not closed"},
		{name: "stray character", content: "x = 1 ? 2", want: `unexpected "?"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := newLexer([]byte(tt.content))
			for token := l.next(); token.Kind != TokenEndOfFile; token = l.next() {
				continue
			}
			if len(l.errors) != 1 || l.errors[0].Message != tt.want {
				t.Errorf("errors = %+v, want %q", l.errors, tt.want)
			}
		})
	}
}

package lua

import (
	"bytes"
	"fmt"
	"strconv"
	"unicode"
	"unicode/utf8"

	"github.com/Lanterna-Games/defold-tools/internal/source"
)

type lexer struct {
	content  []byte
	offset   int
	locate   source.Locator
	errors   []SyntaxError
	comments []Comment
}

func newLexer(content []byte) *lexer {
	return &lexer{content: content, locate: source.NewLocator(content)}
}

func (l *lexer) next() Token {
	l.skipWhitespaceAndComments()
	start := l.offset
	if l.atEnd() {
		return l.token(TokenEndOfFile, "", start)
	}

	character := l.current()
	switch {
	case isNameStart(character):
		return l.lexName(start)
	case isDigit(character) || (character == '.' && isDigit(l.peek(1))):
		return l.lexNumber(start)
	case character == '"' || character == '\'':
		return l.lexQuotedString(start)
	}
	if level, isLongBracket := l.longBracketLevelAt(l.offset); isLongBracket {
		return l.token(TokenString, l.lexLongBracket(level), start)
	}
	for _, symbol := range symbolsLongestFirst {
		if bytes.HasPrefix(l.content[l.offset:], []byte(symbol)) {
			l.offset += len(symbol)
			return l.token(TokenSymbol, symbol, start)
		}
	}

	_, width := utf8.DecodeRune(l.content[l.offset:])
	l.fail(start, fmt.Sprintf("unexpected %q", string(l.content[l.offset:l.offset+width])))
	l.offset += width
	return l.next()
}

func (l *lexer) lexName(start int) Token {
	for !l.atEnd() && isNamePart(l.current()) {
		l.offset++
	}
	text := string(l.content[start:l.offset])
	if keywords[text] {
		return l.token(TokenKeyword, text, start)
	}
	return l.token(TokenName, text, start)
}

func (l *lexer) lexNumber(start int) Token {
	for !l.atEnd() {
		character := l.current()
		isExponentSign := (character == '+' || character == '-') && isExponentMarker(l.content[l.offset-1], l.content[start:l.offset])
		if !isNamePart(character) && character != '.' && !isExponentSign {
			break
		}
		l.offset++
	}
	return l.token(TokenNumber, string(l.content[start:l.offset]), start)
}

func isExponentMarker(previous byte, numberSoFar []byte) bool {
	isHexadecimal := len(numberSoFar) > 1 && numberSoFar[0] == '0' && (numberSoFar[1] == 'x' || numberSoFar[1] == 'X')
	if isHexadecimal {
		return previous == 'p' || previous == 'P'
	}
	return previous == 'e' || previous == 'E'
}

func (l *lexer) lexQuotedString(start int) Token {
	quote := l.current()
	l.offset++
	var value []byte
	for {
		if l.atEnd() || l.current() == '\n' {
			l.fail(start, "string is not closed")
			break
		}
		character := l.current()
		if character == quote {
			l.offset++
			break
		}
		if character == '\\' {
			value = append(value, l.lexEscape()...)
			continue
		}
		value = append(value, character)
		l.offset++
	}
	return l.token(TokenString, string(value), start)
}

var simpleEscapes = map[byte]byte{
	'a': '\a', 'b': '\b', 'f': '\f', 'n': '\n', 'r': '\r', 't': '\t', 'v': '\v',
	'\\': '\\', '"': '"', '\'': '\'', '\n': '\n',
}

func (l *lexer) lexEscape() []byte {
	escapeStart := l.offset
	l.offset++
	if l.atEnd() {
		return nil
	}
	escaped := l.current()
	if replacement, isSimple := simpleEscapes[escaped]; isSimple {
		l.offset++
		return []byte{replacement}
	}
	switch {
	case isDigit(escaped):
		return l.lexNumericEscape(escapeStart, 3, 10)
	case escaped == 'x':
		l.offset++
		return l.lexNumericEscape(escapeStart, 2, 16)
	case escaped == 'z':
		l.offset++
		for !l.atEnd() && isWhitespace(l.current()) {
			l.offset++
		}
		return nil
	case escaped == 'u' && l.peek(1) == '{':
		return l.lexUnicodeEscape(escapeStart)
	default:
		l.fail(escapeStart, fmt.Sprintf("unknown escape %q", `\`+string(escaped)))
		l.offset++
		return []byte{escaped}
	}
}

func (l *lexer) lexNumericEscape(escapeStart, maxDigits, base int) []byte {
	digitsStart := l.offset
	for !l.atEnd() && l.offset-digitsStart < maxDigits && isDigitOfBase(l.current(), base) {
		l.offset++
	}
	value, err := strconv.ParseUint(string(l.content[digitsStart:l.offset]), base, 8)
	if err != nil {
		l.fail(escapeStart, "escape is not a byte")
		return nil
	}
	return []byte{byte(value)}
}

func (l *lexer) lexUnicodeEscape(escapeStart int) []byte {
	l.offset += len("u{")
	digitsStart := l.offset
	for !l.atEnd() && isDigitOfBase(l.current(), 16) {
		l.offset++
	}
	codePoint, err := strconv.ParseUint(string(l.content[digitsStart:l.offset]), 16, 32)
	if err != nil || codePoint > unicode.MaxRune || l.atEnd() || l.current() != '}' {
		l.fail(escapeStart, "malformed unicode escape")
		return nil
	}
	l.offset++
	return utf8.AppendRune(nil, rune(codePoint))
}

func (l *lexer) longBracketLevelAt(offset int) (level int, isLongBracket bool) {
	if offset >= len(l.content) || l.content[offset] != '[' {
		return 0, false
	}
	cursor := offset + 1
	for cursor < len(l.content) && l.content[cursor] == '=' {
		cursor++
	}
	if cursor < len(l.content) && l.content[cursor] == '[' {
		return cursor - offset - 1, true
	}
	return 0, false
}

func (l *lexer) lexLongBracket(level int) string {
	openingStart := l.offset
	l.offset += level + len("[[")
	if !l.atEnd() && l.current() == '\r' {
		l.offset++
	}
	if !l.atEnd() && l.current() == '\n' {
		l.offset++
	}
	contentStart := l.offset
	closing := []byte("]" + string(bytes.Repeat([]byte("="), level)) + "]")
	closingOffset := bytes.Index(l.content[l.offset:], closing)
	if closingOffset < 0 {
		l.fail(openingStart, "long bracket is not closed")
		l.offset = len(l.content)
		return string(l.content[contentStart:])
	}
	l.offset += closingOffset + len(closing)
	return string(l.content[contentStart : contentStart+closingOffset])
}

func (l *lexer) skipWhitespaceAndComments() {
	for !l.atEnd() {
		switch {
		case isWhitespace(l.current()):
			l.offset++
		case l.current() == '-' && l.peek(1) == '-':
			l.lexComment()
		default:
			return
		}
	}
}

func (l *lexer) lexComment() {
	start := l.offset
	isAlone := l.onlyBlanksPrecedeOnTheLine(start)
	l.offset += len("--")
	if level, isLongBracket := l.longBracketLevelAt(l.offset); isLongBracket {
		text := l.lexLongBracket(level)
		l.comments = append(l.comments, Comment{Text: text, Start: l.locate(start), End: l.locate(l.offset), IsAloneOnItsLine: isAlone})
		return
	}
	textStart := l.offset
	for !l.atEnd() && l.current() != '\n' {
		l.offset++
	}
	l.comments = append(l.comments, Comment{
		Text:             string(l.content[textStart:l.offset]),
		Start:            l.locate(start),
		End:              l.locate(l.offset),
		IsAloneOnItsLine: isAlone,
	})
}

func (l *lexer) onlyBlanksPrecedeOnTheLine(offset int) bool {
	for position := offset - 1; position >= 0 && l.content[position] != '\n'; position-- {
		if l.content[position] != ' ' && l.content[position] != '\t' && l.content[position] != '\r' {
			return false
		}
	}
	return true
}

func (l *lexer) token(kind TokenKind, text string, start int) Token {
	return Token{Kind: kind, Text: text, Start: l.locate(start), End: l.locate(l.offset)}
}

func (l *lexer) fail(offset int, message string) {
	l.errors = append(l.errors, SyntaxError{Position: l.locate(offset), Message: message})
}

func (l *lexer) atEnd() bool {
	return l.offset >= len(l.content)
}

func (l *lexer) current() byte {
	return l.content[l.offset]
}

func (l *lexer) peek(distance int) byte {
	if l.offset+distance >= len(l.content) {
		return 0
	}
	return l.content[l.offset+distance]
}

func isWhitespace(character byte) bool {
	return character == ' ' || character == '\t' || character == '\r' || character == '\n' || character == '\f' || character == '\v'
}

func isDigit(character byte) bool {
	return character >= '0' && character <= '9'
}

func isDigitOfBase(character byte, base int) bool {
	if isDigit(character) {
		return true
	}
	return base == 16 && ((character >= 'a' && character <= 'f') || (character >= 'A' && character <= 'F'))
}

func isNameStart(character byte) bool {
	return character == '_' || (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z')
}

func isNamePart(character byte) bool {
	return isNameStart(character) || isDigit(character)
}

func Tokens(content []byte) []Token {
	lexer := newLexer(content)
	var tokens []Token
	for {
		token := lexer.next()
		if token.Kind == TokenEndOfFile {
			return tokens
		}
		tokens = append(tokens, token)
	}
}

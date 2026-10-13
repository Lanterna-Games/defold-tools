package textproto

import (
	"fmt"

	"github.com/Lanterna-Games/defold-tools/internal/source"
)

type SyntaxError struct {
	Position source.Position
	Message  string
}

func (e SyntaxError) Error() string {
	return fmt.Sprintf("%d:%d: %s", e.Position.Line, e.Position.Column, e.Message)
}

func Parse(text []byte, locate source.Locator) (*Message, []SyntaxError) {
	p := &parser{source: text, locate: locate}
	message := p.parseFields(noClosingDelimiter)
	return message, p.errors
}

const noClosingDelimiter = 0

type parser struct {
	source []byte
	offset int
	locate source.Locator
	errors []SyntaxError
}

func (p *parser) parseFields(closingDelimiter byte) *Message {
	message := &Message{}
	for {
		p.skipInsignificant()
		if p.atEnd() {
			if closingDelimiter != noClosingDelimiter {
				p.fail(p.offset, fmt.Sprintf("missing %q", string(closingDelimiter)))
			}
			return message
		}
		if p.current() == closingDelimiter {
			p.offset++
			return message
		}
		message.Fields = append(message.Fields, p.parseField()...)
	}
}

func (p *parser) parseField() []Field {
	nameStart := p.offset
	for !p.atEnd() && isNameCharacter(p.current()) {
		p.offset++
	}
	if p.offset == nameStart {
		p.fail(p.offset, fmt.Sprintf("unexpected %q", string(p.current())))
		p.skipLine()
		return nil
	}
	field := Field{Name: string(p.source[nameStart:p.offset]), Position: p.locate(nameStart)}

	p.skipInsignificant()
	if !p.atEnd() && p.current() == ':' {
		p.offset++
		p.skipInsignificant()
	}
	if p.atEnd() {
		p.fail(p.offset, fmt.Sprintf("field %q has no value", field.Name))
		return nil
	}

	switch p.current() {
	case '{':
		p.offset++
		field.Message = p.parseFields('}')
		return []Field{field}
	case '<':
		p.offset++
		field.Message = p.parseFields('>')
		return []Field{field}
	case '[':
		return p.parseList(field)
	default:
		field.Scalar = p.parseScalar()
		return []Field{field}
	}
}

func (p *parser) parseList(template Field) []Field {
	p.offset++
	var fields []Field
	for {
		p.skipInsignificant()
		if p.atEnd() {
			p.fail(p.offset, `missing "]"`)
			return fields
		}
		if p.current() == ']' {
			p.offset++
			return fields
		}
		element := template
		if p.current() == '{' {
			p.offset++
			element.Message = p.parseFields('}')
		} else {
			element.Scalar = p.parseScalar()
		}
		fields = append(fields, element)
	}
}

func (p *parser) parseScalar() *Scalar {
	if isQuote(p.current()) {
		return p.parseString()
	}
	start := p.offset
	for !p.atEnd() && !isScalarTerminator(p.current()) {
		p.offset++
	}
	return &Scalar{Text: string(p.source[start:p.offset]), Position: p.locate(start), locate: p.locate}
}

func (p *parser) parseString() *Scalar {
	scalar := &Scalar{Quoted: true, Position: p.locate(p.offset), locate: p.locate}
	decoded := ""
	for !p.atEnd() && isQuote(p.current()) {
		quote := p.current()
		p.offset++
		rawStart := p.offset
		for !p.atEnd() && p.current() != quote && p.current() != '\n' {
			if p.current() == '\\' {
				p.offset++
			}
			p.offset++
		}
		rawEnd := min(p.offset, len(p.source))
		raw := string(p.source[rawStart:rawEnd])
		scalar.segments = append(scalar.segments, segment{rawStart: rawStart, raw: raw, decodedStart: len(decoded)})
		decoded += decode(raw)
		if p.atEnd() || p.current() != quote {
			p.fail(rawStart-1, "string is not closed")
			break
		}
		p.offset++
		p.skipWhitespaceAndComments()
	}
	scalar.Text = decoded
	return scalar
}

func (p *parser) skipInsignificant() {
	for {
		p.skipWhitespaceAndComments()
		if p.atEnd() || !isFieldSeparator(p.current()) {
			return
		}
		p.offset++
	}
}

func (p *parser) skipWhitespaceAndComments() {
	for !p.atEnd() {
		switch p.current() {
		case ' ', '\t', '\r', '\n':
			p.offset++
		case '#':
			p.skipLine()
		default:
			return
		}
	}
}

func (p *parser) skipLine() {
	for !p.atEnd() && p.current() != '\n' {
		p.offset++
	}
}

func (p *parser) atEnd() bool {
	return p.offset >= len(p.source)
}

func (p *parser) current() byte {
	return p.source[p.offset]
}

func (p *parser) fail(offset int, message string) {
	p.errors = append(p.errors, SyntaxError{Position: p.locate(offset), Message: message})
}

func isNameCharacter(character byte) bool {
	return character == '_' || character == '.' ||
		(character >= 'a' && character <= 'z') ||
		(character >= 'A' && character <= 'Z') ||
		(character >= '0' && character <= '9')
}

func isFieldSeparator(character byte) bool {
	return character == ',' || character == ';'
}

func isQuote(character byte) bool {
	return character == '"' || character == '\''
}

func isScalarTerminator(character byte) bool {
	switch character {
	case ' ', '\t', '\r', '\n', ',', ';', '}', '>', ']', '#':
		return true
	default:
		return false
	}
}

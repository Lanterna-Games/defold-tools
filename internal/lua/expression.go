package lua

import "fmt"

type precedence struct {
	left  int
	right int
}

var binaryPrecedences = map[string]precedence{
	"or":  {1, 1},
	"and": {2, 2},
	"<":   {3, 3}, ">": {3, 3}, "<=": {3, 3}, ">=": {3, 3}, "~=": {3, 3}, "==": {3, 3},
	"..": {5, 4},
	"+":  {6, 6}, "-": {6, 6},
	"*": {7, 7}, "/": {7, 7}, "%": {7, 7},
	"^": {10, 9},
}

const (
	unaryPrecedence  = 8
	lowestPrecedence = 0
)

func (p *parser) parseExpression() Expression {
	return p.parseExpressionAbove(lowestPrecedence)
}

func (p *parser) parseExpressionAbove(limit int) Expression {
	start := p.current.Start
	var left Expression
	if p.atUnaryOperator() {
		operator := p.advance().Text
		operand := p.parseExpressionAbove(unaryPrecedence)
		left = &UnaryExpression{Range: p.rangeFrom(start), Operator: operator, Operand: operand}
	} else {
		left = p.parseSimpleExpression()
	}
	for {
		operatorPrecedence, isBinaryOperator := p.currentBinaryPrecedence()
		if !isBinaryOperator || operatorPrecedence.left <= limit {
			return left
		}
		operator := p.advance().Text
		right := p.parseExpressionAbove(operatorPrecedence.right)
		left = &BinaryExpression{Range: p.rangeFrom(start), Operator: operator, Left: left, Right: right}
	}
}

func (p *parser) atUnaryOperator() bool {
	return p.atKeyword("not") || p.atSymbol("-") || p.atSymbol("#")
}

func (p *parser) currentBinaryPrecedence() (operatorPrecedence precedence, isBinaryOperator bool) {
	isOperatorToken := p.current.Kind == TokenSymbol || p.atKeyword("and") || p.atKeyword("or")
	if !isOperatorToken {
		return precedence{}, false
	}
	operatorPrecedence, isBinaryOperator = binaryPrecedences[p.current.Text]
	return operatorPrecedence, isBinaryOperator
}

func (p *parser) parseSimpleExpression() Expression {
	token := p.current
	span := Range{Start: token.Start, End: token.End}
	switch token.Kind {
	case TokenNumber:
		p.advance()
		return &NumberLiteral{Range: span, Text: token.Text}
	case TokenString:
		p.advance()
		return &StringLiteral{Range: span, Value: token.Text}
	case TokenKeyword:
		switch token.Text {
		case "nil":
			p.advance()
			return &NilLiteral{Range: span}
		case "true", "false":
			p.advance()
			return &BooleanLiteral{Range: span, Value: token.Text == "true"}
		case "function":
			p.advance()
			return p.parseFunctionBody(token.Start)
		}
	case TokenSymbol:
		switch token.Text {
		case "...":
			p.advance()
			return &VarargLiteral{Range: span}
		case "{":
			return p.parseTable()
		}
	case TokenName, TokenEndOfFile:
	}
	return p.parseSuffixedExpression()
}

func (p *parser) parseSuffixedExpression() Expression {
	start := p.current.Start
	expression := p.parsePrimaryExpression()
	for {
		switch {
		case p.atSymbol("."):
			p.advance()
			expression = p.fieldAccess(expression, p.expectName())
		case p.atSymbol("["):
			p.advance()
			index := p.parseExpression()
			p.expectSymbol("]")
			expression = &IndexExpression{Range: p.rangeFrom(start), Object: expression, Index: index}
		case p.atSymbol(":"):
			p.advance()
			method := p.expectName()
			arguments := p.parseArguments()
			expression = &CallExpression{Range: p.rangeFrom(start), Function: expression, Method: method, Arguments: arguments}
		case p.atArguments():
			arguments := p.parseArguments()
			expression = &CallExpression{Range: p.rangeFrom(start), Function: expression, Arguments: arguments}
		default:
			return expression
		}
	}
}

func (p *parser) parsePrimaryExpression() Expression {
	start := p.current.Start
	switch {
	case p.current.Kind == TokenName:
		token := p.advance()
		return &Identifier{Start: token.Start, End: token.End, Name: token.Text}
	case p.atSymbol("("):
		p.advance()
		inner := p.parseExpression()
		p.expectSymbol(")")
		return &ParenthesizedExpression{Range: p.rangeFrom(start), Inner: inner}
	default:
		p.failAtCurrent("unexpected " + describe(p.current))
		return &BadExpression{Start: start, End: start}
	}
}

func describe(token Token) string {
	if token.Kind == TokenEndOfFile {
		return "end of file"
	}
	return fmt.Sprintf("%q", token.Text)
}

func (p *parser) atArguments() bool {
	return p.atSymbol("(") || p.atSymbol("{") || p.current.Kind == TokenString
}

func (p *parser) parseArguments() []Expression {
	switch {
	case p.current.Kind == TokenString:
		token := p.advance()
		return []Expression{&StringLiteral{Start: token.Start, End: token.End, Value: token.Text}}
	case p.atSymbol("{"):
		return []Expression{p.parseTable()}
	case p.acceptSymbol("("):
		if p.acceptSymbol(")") {
			return nil
		}
		arguments := p.parseExpressionList()
		p.expectSymbol(")")
		return arguments
	default:
		p.failAtCurrent("expected arguments")
		return nil
	}
}

func (p *parser) parseTable() Expression {
	start := p.advance().Start
	table := &TableLiteral{}
	for !p.atSymbol("}") && p.current.Kind != TokenEndOfFile {
		offsetBefore := p.current.Start.Offset
		table.Fields = append(table.Fields, p.parseTableField())
		if !p.acceptSymbol(",") && !p.acceptSymbol(";") {
			break
		}
		if p.current.Start.Offset == offsetBefore {
			break
		}
	}
	p.expectSymbol("}")
	table.Range = p.rangeFrom(start)
	return table
}

func (p *parser) parseTableField() TableField {
	if p.acceptSymbol("[") {
		key := p.parseExpression()
		p.expectSymbol("]")
		p.expectSymbol("=")
		return TableField{Key: key, Value: p.parseExpression()}
	}
	value := p.parseExpression()
	name, isName := value.(*Identifier)
	if isName && p.acceptSymbol("=") {
		return TableField{Key: &StringLiteral{Range: name.Range, Value: name.Name}, Value: p.parseExpression()}
	}
	return TableField{Value: value}
}

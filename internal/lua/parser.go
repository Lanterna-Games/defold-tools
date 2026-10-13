package lua

import (
	"fmt"

	"github.com/Lanterna-Games/defold-tools/internal/source"
)

func Parse(content []byte) (*Chunk, []SyntaxError) {
	p := &parser{lexer: newLexer(content)}
	p.advance()
	start := p.current.Start
	body := p.parseBlock()
	for p.current.Kind != TokenEndOfFile {
		p.failAtCurrent(fmt.Sprintf("unexpected %q", p.current.Text))
		p.advance()
		body = append(body, p.parseBlock()...)
	}
	chunk := &Chunk{Start: start, End: p.current.End, Body: body, Comments: p.lexer.comments}
	return chunk, append(p.lexer.errors, p.errors...)
}

type parser struct {
	lexer       *lexer
	current     Token
	previousEnd source.Position
	errors      []SyntaxError
}

func (p *parser) advance() Token {
	consumed := p.current
	p.previousEnd = consumed.End
	p.current = p.lexer.next()
	return consumed
}

func (p *parser) at(kind TokenKind, text string) bool {
	return p.current.Is(kind, text)
}

func (p *parser) atKeyword(keyword string) bool {
	return p.at(TokenKeyword, keyword)
}

func (p *parser) atSymbol(symbol string) bool {
	return p.at(TokenSymbol, symbol)
}

func (p *parser) acceptKeyword(keyword string) bool {
	if !p.atKeyword(keyword) {
		return false
	}
	p.advance()
	return true
}

func (p *parser) acceptSymbol(symbol string) bool {
	if !p.atSymbol(symbol) {
		return false
	}
	p.advance()
	return true
}

func (p *parser) expectSymbol(symbol string) {
	if !p.acceptSymbol(symbol) {
		p.failAtCurrent(fmt.Sprintf("expected %q", symbol))
	}
}

func (p *parser) expectKeyword(keyword string) {
	if !p.acceptKeyword(keyword) {
		p.failAtCurrent(fmt.Sprintf("expected %q", keyword))
	}
}

func (p *parser) expectEndOf(construct string, opening source.Position) {
	if !p.acceptKeyword("end") {
		p.failAtCurrent(fmt.Sprintf("expected \"end\" to close the %s that starts on line %d", construct, opening.Line))
	}
}

func (p *parser) expectName() *Identifier {
	if p.current.Kind != TokenName {
		p.failAtCurrent("expected a name")
		return &Identifier{Start: p.current.Start, End: p.current.Start}
	}
	token := p.advance()
	return &Identifier{Start: token.Start, End: token.End, Name: token.Text}
}

func (p *parser) failAtCurrent(message string) {
	p.errors = append(p.errors, SyntaxError{Position: p.current.Start, Message: message})
}

func (p *parser) rangeFrom(start source.Position) Range {
	return Range{Start: start, End: p.previousEnd}
}

func (p *parser) atBlockEnd() bool {
	if p.current.Kind == TokenEndOfFile {
		return true
	}
	if p.current.Kind != TokenKeyword {
		return false
	}
	switch p.current.Text {
	case "end", "else", "elseif", "until":
		return true
	default:
		return false
	}
}

func (p *parser) parseBlock() []Statement {
	var statements []Statement
	for !p.atBlockEnd() {
		offsetBefore := p.current.Start.Offset
		statement := p.parseStatement()
		if statement != nil {
			statements = append(statements, statement)
		}
		p.acceptSymbol(";")
		madeNoProgress := p.current.Start.Offset == offsetBefore && p.current.Kind != TokenEndOfFile
		if madeNoProgress {
			p.advance()
		}
		if _, isReturn := statement.(*ReturnStatement); isReturn {
			break
		}
	}
	return statements
}

func (p *parser) parseStatement() Statement {
	start := p.current.Start
	if p.current.Kind == TokenKeyword {
		switch p.current.Text {
		case "local":
			return p.parseLocal()
		case "function":
			return p.parseFunctionStatement()
		case "if":
			return p.parseIf()
		case "while":
			return p.parseWhile()
		case "for":
			return p.parseFor()
		case "repeat":
			return p.parseRepeat()
		case "do":
			p.advance()
			body := p.parseBlock()
			p.expectEndOf("block", start)
			return &DoStatement{Range: p.rangeFrom(start), Body: body}
		case "return":
			return p.parseReturn()
		case "break":
			p.advance()
			return &BreakStatement{Range: p.rangeFrom(start)}
		}
	}
	return p.parseExpressionStatement()
}

func (p *parser) parseLocal() Statement {
	start := p.advance().Start
	if p.acceptKeyword("function") {
		name := p.expectName()
		function := p.parseFunctionBody(start)
		return &LocalFunctionStatement{Range: p.rangeFrom(start), Name: name, Function: function}
	}
	statement := &LocalStatement{Names: []*Identifier{p.expectName()}}
	for p.acceptSymbol(",") {
		statement.Names = append(statement.Names, p.expectName())
	}
	if p.acceptSymbol("=") {
		statement.Values = p.parseExpressionList()
	}
	statement.Range = p.rangeFrom(start)
	return statement
}

func (p *parser) parseFunctionStatement() Statement {
	start := p.advance().Start
	statement := &FunctionStatement{}
	var name Expression = p.expectName()
	for p.atSymbol(".") {
		p.advance()
		name = p.fieldAccess(name, p.expectName())
	}
	if p.acceptSymbol(":") {
		statement.Method = p.expectName()
	}
	statement.Name = name
	statement.Function = p.parseFunctionBody(start)
	statement.Range = p.rangeFrom(start)
	return statement
}

func (p *parser) fieldAccess(object Expression, field *Identifier) Expression {
	return &IndexExpression{
		Start: object.Span().Start, End: field.End,
		Object: object,
		Index:  &StringLiteral{Range: field.Range, Value: field.Name},
	}
}

func (p *parser) parseFunctionBody(functionStart source.Position) *FunctionLiteral {
	function := &FunctionLiteral{}
	p.expectSymbol("(")
	for !p.atSymbol(")") && p.current.Kind != TokenEndOfFile {
		if p.acceptSymbol("...") {
			function.IsVararg = true
			break
		}
		function.Parameters = append(function.Parameters, p.expectName())
		if !p.acceptSymbol(",") {
			break
		}
	}
	p.expectSymbol(")")
	function.Body = p.parseBlock()
	p.expectEndOf("function", functionStart)
	function.Range = p.rangeFrom(functionStart)
	return function
}

func (p *parser) parseIf() Statement {
	start := p.advance().Start
	statement := &IfStatement{}
	statement.Clauses = append(statement.Clauses, p.parseIfClause(start))
	for p.atKeyword("elseif") {
		statement.Clauses = append(statement.Clauses, p.parseIfClause(p.advance().Start))
	}
	if p.atKeyword("else") {
		statement.HasElse, statement.ElseStart = true, p.advance().Start
		statement.Else = p.parseBlock()
	}
	p.expectEndOf("if", start)
	statement.Range = p.rangeFrom(start)
	return statement
}

func (p *parser) parseIfClause(start source.Position) IfClause {
	condition := p.parseExpression()
	p.expectKeyword("then")
	return IfClause{Start: start, Condition: condition, Body: p.parseBlock()}
}

func (p *parser) parseWhile() Statement {
	start := p.advance().Start
	condition := p.parseExpression()
	p.expectKeyword("do")
	body := p.parseBlock()
	p.expectEndOf("while", start)
	return &WhileStatement{Range: p.rangeFrom(start), Condition: condition, Body: body}
}

func (p *parser) parseRepeat() Statement {
	start := p.advance().Start
	body := p.parseBlock()
	p.expectKeyword("until")
	condition := p.parseExpression()
	return &RepeatStatement{Range: p.rangeFrom(start), Body: body, Condition: condition}
}

func (p *parser) parseFor() Statement {
	start := p.advance().Start
	first := p.expectName()
	if p.acceptSymbol("=") {
		statement := &NumericForStatement{Variable: first, Start: p.parseExpression()}
		p.expectSymbol(",")
		statement.Stop = p.parseExpression()
		if p.acceptSymbol(",") {
			statement.Step = p.parseExpression()
		}
		statement.Body = p.parseLoopBody(start)
		statement.Range = p.rangeFrom(start)
		return statement
	}
	statement := &GenericForStatement{Names: []*Identifier{first}}
	for p.acceptSymbol(",") {
		statement.Names = append(statement.Names, p.expectName())
	}
	p.expectKeyword("in")
	statement.Values = p.parseExpressionList()
	statement.Body = p.parseLoopBody(start)
	statement.Range = p.rangeFrom(start)
	return statement
}

func (p *parser) parseLoopBody(loopStart source.Position) []Statement {
	p.expectKeyword("do")
	body := p.parseBlock()
	p.expectEndOf("for", loopStart)
	return body
}

func (p *parser) parseReturn() Statement {
	start := p.advance().Start
	statement := &ReturnStatement{}
	if !p.atBlockEnd() && !p.atSymbol(";") {
		statement.Values = p.parseExpressionList()
	}
	statement.Range = p.rangeFrom(start)
	return statement
}

func (p *parser) parseExpressionStatement() Statement {
	start := p.current.Start
	first := p.parseSuffixedExpression()
	if p.atSymbol("=") || p.atSymbol(",") {
		statement := &AssignmentStatement{Targets: []Expression{first}}
		for p.acceptSymbol(",") {
			statement.Targets = append(statement.Targets, p.parseSuffixedExpression())
		}
		p.expectSymbol("=")
		statement.Values = p.parseExpressionList()
		statement.Range = p.rangeFrom(start)
		return statement
	}
	if call, isCall := first.(*CallExpression); isCall {
		return &CallStatement{Range: call.Range, Call: call}
	}
	if _, isBad := first.(*BadExpression); !isBad {
		p.errors = append(p.errors, SyntaxError{Position: start, Message: "an expression is not a statement"})
	}
	return nil
}

func (p *parser) parseExpressionList() []Expression {
	expressions := []Expression{p.parseExpression()}
	for p.acceptSymbol(",") {
		expressions = append(expressions, p.parseExpression())
	}
	return expressions
}

package project

import (
	"slices"

	"github.com/Lanterna-Games/defold-tools/internal/lua"
)

const (
	nowhere                = -1
	messageParameter       = 2
	fieldsOfAPost          = 2
	operatorThatJoinsTests = "and"
	operatorThatWidens     = "or"
)

type FieldRead struct {
	Field    WrittenID
	Messages []string
}

func (s *Script) fieldsPostedIn(call *lua.CallExpression) (fields []string, areWritten bool) {
	if len(call.Arguments) <= fieldsOfAPost {
		return nil, true
	}
	table, isATable := s.Written(call.Arguments[fieldsOfAPost]).(*lua.TableLiteral)
	if !isATable {
		return nil, false
	}
	for _, field := range table.Fields {
		name, isNamed := field.Key.(*lua.StringLiteral)
		if !isNamed {
			return nil, false
		}
		fields = append(fields, name.Value)
	}
	return fields, true
}

type fieldScan struct {
	script         *Script
	idName         string
	messageName    string
	assigned       map[string]bool
	reads          []FieldRead
	cursor         int
	underTheCursor []string
}

func (s *Script) fieldScanOf(handler *lua.FunctionLiteral, cursor int) *fieldScan {
	scan := &fieldScan{
		script:      s,
		idName:      handler.Parameters[messageIDParameter].Name,
		messageName: handler.Parameters[messageParameter].Name,
		assigned:    map[string]bool{},
		cursor:      cursor,
	}
	scan.block(handler.Body, nil, holds(handler.Range, cursor))
	return scan
}

func (s *Script) collectFieldsRead(handler *lua.FunctionLiteral) {
	if len(handler.Parameters) <= messageParameter {
		return
	}
	scan := s.fieldScanOf(handler, nowhere)
	for _, read := range scan.reads {
		if !scan.assigned[read.Field.Name] {
			s.FieldsRead = append(s.FieldsRead, read)
		}
	}
}

func (s *Script) MessagesHandledAt(offset int) []string {
	for _, statement := range s.Chunk.Body {
		function, isFunction := statement.(*lua.FunctionStatement)
		if !isFunction || function.Method != nil || !holds(function.Range, offset) {
			continue
		}
		name, isPlainName := function.Name.(*lua.Identifier)
		if !isPlainName || name.Name != messageHandlerFunction || len(function.Function.Parameters) <= messageParameter {
			continue
		}
		messages := slices.Clone(s.fieldScanOf(function.Function, offset).underTheCursor)
		slices.Sort(messages)
		return slices.Compact(messages)
	}
	return nil
}

func holds(span lua.Range, offset int) bool {
	return offset != nowhere && span.Start.Offset <= offset && offset <= span.End.Offset
}

func (f *fieldScan) block(statements []lua.Statement, under []string, holdsTheCursor bool) {
	if holdsTheCursor {
		f.underTheCursor = under
	}
	for _, statement := range statements {
		f.visit(statement, under)
		if remaining, leavesOnEveryOtherMessage := f.messagesThatGetPast(statement); leavesOnEveryOtherMessage {
			under = remaining
			if holdsTheCursor && statement.Span().End.Offset <= f.cursor {
				f.underTheCursor = under
			}
		}
	}
}

func (f *fieldScan) clauseHoldingTheCursor(statement *lua.IfStatement) (clause int, isInTheElse bool) {
	if !holds(statement.Range, f.cursor) {
		return nowhere, false
	}
	if statement.HasElse && f.cursor >= statement.ElseStart.Offset {
		return nowhere, true
	}
	for index, each := range slices.Backward(statement.Clauses) {
		if f.cursor >= each.Start.Offset {
			return index, false
		}
	}
	return nowhere, false
}

func (f *fieldScan) messagesThatGetPast(statement lua.Statement) (messages []string, leavesOnEveryOtherMessage bool) {
	guard, isIf := statement.(*lua.IfStatement)
	if !isIf || len(guard.Clauses) != 1 || len(guard.Else) > 0 || !onlyReturns(guard.Clauses[0].Body) {
		return nil, false
	}
	messages = f.messagesTested(guard.Clauses[0].Condition, "~=", operatorThatWidens, operatorThatJoinsTests)
	return messages, len(messages) > 0
}

func (f *fieldScan) messagesTested(condition lua.Expression, comparison, keepsWithOneSide, needsBothSides string) []string {
	switch typed := condition.(type) {
	case *lua.ParenthesizedExpression:
		return f.messagesTested(typed.Inner, comparison, keepsWithOneSide, needsBothSides)
	case *lua.BinaryExpression:
		switch typed.Operator {
		case comparison:
			return f.messageComparedIn(typed)
		case keepsWithOneSide:
			left := f.messagesTested(typed.Left, comparison, keepsWithOneSide, needsBothSides)
			return append(left, f.messagesTested(typed.Right, comparison, keepsWithOneSide, needsBothSides)...)
		case needsBothSides:
			left := f.messagesTested(typed.Left, comparison, keepsWithOneSide, needsBothSides)
			right := f.messagesTested(typed.Right, comparison, keepsWithOneSide, needsBothSides)
			if len(left) == 0 || len(right) == 0 {
				return nil
			}
			return append(left, right...)
		}
	}
	return nil
}

func (f *fieldScan) messageComparedIn(comparison *lua.BinaryExpression) []string {
	other := comparison.Right
	switch {
	case isPlainlyNamed(comparison.Left, f.idName):
	case isPlainlyNamed(comparison.Right, f.idName):
		other = comparison.Left
	default:
		return nil
	}
	if name, isWritten := f.script.IDWrittenIn(other); isWritten {
		return []string{name}
	}
	return nil
}

func (f *fieldScan) visit(node lua.Node, under []string) {
	if lua.IsNil(node) {
		return
	}
	switch typed := node.(type) {
	case *lua.IfStatement:
		withTheCursor, cursorIsInTheElse := f.clauseHoldingTheCursor(typed)
		for index, clause := range typed.Clauses {
			inside := f.messagesTested(clause.Condition, "==", operatorThatJoinsTests, operatorThatWidens)
			if len(inside) == 0 {
				inside = under
			}
			f.visit(clause.Condition, inside)
			f.block(clause.Body, inside, index == withTheCursor)
		}
		f.block(typed.Else, under, cursorIsInTheElse)
	case *lua.AssignmentStatement:
		for _, target := range typed.Targets {
			if field, isAField := f.fieldOfTheMessage(target); isAField {
				f.assigned[field] = true
			} else {
				f.visit(target, under)
			}
		}
		for _, value := range typed.Values {
			f.visit(value, under)
		}
	case *lua.IndexExpression:
		if field, isAField := f.fieldOfTheMessage(typed); isAField {
			if len(under) > 0 {
				f.reads = append(f.reads, FieldRead{
					Field:    WrittenID{Name: field, Location: f.script.LocationOf(typed.Index)},
					Messages: under,
				})
			}
			return
		}
		f.visit(typed.Object, under)
		f.visit(typed.Index, under)
	default:
		for _, child := range lua.Children(node) {
			f.visit(child, under)
		}
	}
}

func (f *fieldScan) fieldOfTheMessage(expression lua.Expression) (field string, isAField bool) {
	index, isIndex := expression.(*lua.IndexExpression)
	if !isIndex || !isPlainlyNamed(index.Object, f.messageName) {
		return "", false
	}
	name, isNamed := index.Index.(*lua.StringLiteral)
	if !isNamed {
		return "", false
	}
	return name.Value, true
}

package project

import "github.com/Lanterna-Games/defold-tools/internal/lua"

const (
	postFunction           = "msg.post"
	messageHandlerFunction = "on_message"
	messageIDParameter     = 1
)

func (s *Script) collectMessages() {
	isTheFunctionOfAPost := map[lua.Node]bool{}
	lua.Inspect(s.Chunk, func(node lua.Node) bool {
		switch typed := node.(type) {
		case *lua.CallExpression:
			if post, isPost := s.postFrom(typed); isPost {
				s.Posts = append(s.Posts, post)
				isTheFunctionOfAPost[typed.Function] = true
			}
		case *lua.IndexExpression:
			if name, _ := lua.QualifiedName(typed); name == postFunction && !isTheFunctionOfAPost[typed] {
				s.Posts = append(s.Posts, MessagePost{ID: MessageID{Location: Location{File: s.File, Position: typed.Start}}})
				s.cannotRead(typed, "msg.post is used here as a value, so what is posted through it cannot be read")
			}
		case *lua.AssignmentStatement:
			for _, target := range typed.Targets {
				if name, isName := target.(*lua.Identifier); isName && name.Name == messageHandlerFunction {
					s.handlesEveryMessageBecauseOf(target, "on_message is assigned here and not declared")
				}
			}
		case *lua.FunctionStatement:
			name, isPlainName := typed.Name.(*lua.Identifier)
			if isPlainName && typed.Method == nil && name.Name == messageHandlerFunction {
				s.collectHandledMessages(typed.Function)
				s.collectFieldsRead(typed.Function)
			}
		}
		return true
	})
}

func (s *Script) postFrom(call *lua.CallExpression) (post MessagePost, isPost bool) {
	function, _ := lua.QualifiedName(call.Function)
	if function != postFunction || call.Method != nil || len(call.Arguments) < 2 {
		return MessagePost{}, false
	}
	post.ID.Location = s.LocationOf(call.Arguments[1])
	post.ID.Name, post.IDIsKnown = s.IDWrittenIn(call.Arguments[1])
	if !post.IDIsKnown {
		s.cannotRead(call.Arguments[1], "the id of the message posted here cannot be read")
	}
	if receiver, isWritten := s.AddressWrittenIn(call.Arguments[0]); isWritten {
		post.Receiver, post.ReceiverIsWritten = receiver.Text, true
	}
	post.Fields, post.FieldsAreWritten = s.fieldsPostedIn(call)
	return post, true
}

type handlerScan struct {
	script        *Script
	parameterName string
	idTests       int
}

func (s *Script) collectHandledMessages(handler *lua.FunctionLiteral) {
	scan := &handlerScan{script: s}
	if len(handler.Parameters) > messageIDParameter {
		scan.parameterName = handler.Parameters[messageIDParameter].Name
	}
	scan.visitAll(handler.Body)

	reactsWithoutLookingAtTheID := len(handler.Body) > 0 && scan.idTests == 0
	if reactsWithoutLookingAtTheID {
		s.handlesEveryMessageBecauseOf(handler.Body[0], "on_message acts without comparing the message id with one that can be read")
	}
}

func (h *handlerScan) visitAll(statements []lua.Statement) {
	for _, statement := range statements {
		h.visit(statement)
	}
}

func (h *handlerScan) visit(node lua.Node) {
	if lua.IsNil(node) {
		return
	}
	switch typed := node.(type) {
	case *lua.Identifier:
		if h.isTheID(typed) {
			h.script.handlesEveryMessageBecauseOf(typed, "the message id is used here for something other than a comparison")
		}
	case *lua.BinaryExpression:
		if _, comparesTheID := h.comparedWithTheID(typed); comparesTheID {
			h.script.handlesEveryMessageBecauseOf(typed, "the message id is compared here outside the condition of an if")
			return
		}
		h.visit(typed.Left)
		h.visit(typed.Right)
	case *lua.IfStatement:
		h.visitIf(typed)
	default:
		for _, child := range lua.Children(node) {
			h.visit(child)
		}
	}
}

func (h *handlerScan) visitIf(statement *lua.IfStatement) {
	idTestsBefore := h.idTests
	for _, clause := range statement.Clauses {
		h.visitCondition(clause.Condition, onlyReturns(clause.Body))
		h.visitAll(clause.Body)
	}
	everyOtherMessageFallsIntoTheElse := h.idTests > idTestsBefore && len(statement.Else) > 0
	if everyOtherMessageFallsIntoTheElse {
		h.script.handlesEveryMessageBecauseOf(statement.Else[0], "every other message falls into this else")
	}
	h.visitAll(statement.Else)
}

func onlyReturns(body []lua.Statement) bool {
	if len(body) != 1 {
		return false
	}
	_, isReturn := body[0].(*lua.ReturnStatement)
	return isReturn
}

func (h *handlerScan) visitCondition(condition lua.Expression, bodyOnlyReturns bool) {
	switch typed := condition.(type) {
	case *lua.ParenthesizedExpression:
		h.visitCondition(typed.Inner, bodyOnlyReturns)
	case *lua.BinaryExpression:
		if typed.Operator == "and" || typed.Operator == "or" {
			h.visitCondition(typed.Left, bodyOnlyReturns)
			h.visitCondition(typed.Right, bodyOnlyReturns)
			return
		}
		other, comparesTheID := h.comparedWithTheID(typed)
		if !comparesTheID {
			h.visit(typed)
			return
		}
		name, isKnown := h.script.IDWrittenIn(other)
		leavesEveryOtherMessageOut := typed.Operator == "==" || bodyOnlyReturns
		switch {
		case !isKnown:
			h.script.handlesEveryMessageBecauseOf(other, "the id compared here cannot be read")
			return
		case !leavesEveryOtherMessageOut:
			h.script.handlesEveryMessageBecauseOf(typed, "this branch runs for every message but one")
			return
		}
		h.idTests++
		if !h.script.Handles(name) {
			h.script.Handled = append(h.script.Handled, MessageID{Name: name, Location: h.script.LocationOf(other)})
		}
	default:
		h.visit(condition)
	}
}

func (h *handlerScan) comparedWithTheID(comparison *lua.BinaryExpression) (other lua.Expression, comparesTheID bool) {
	if comparison.Operator != "==" && comparison.Operator != "~=" {
		return nil, false
	}
	if h.isTheID(comparison.Left) {
		return comparison.Right, true
	}
	if h.isTheID(comparison.Right) {
		return comparison.Left, true
	}
	return nil, false
}

func (h *handlerScan) isTheID(expression lua.Expression) bool {
	name, isName := expression.(*lua.Identifier)
	return isName && h.parameterName != "" && name.Name == h.parameterName
}

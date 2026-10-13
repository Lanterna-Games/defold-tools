package project

import "github.com/Lanterna-Games/defold-tools/internal/lua"

const (
	inputHandlerFunction = "on_input"
	actionIDParameter    = 1
)

type WrittenID struct {
	Name     string
	Location Location
}

func (s *Script) collectActions() {
	lua.Inspect(s.Chunk, func(node lua.Node) bool {
		switch typed := node.(type) {
		case *lua.FunctionStatement:
			if isPlainlyNamed(typed.Name, inputHandlerFunction) && typed.Method == nil {
				s.collectComparedActions(typed.Function)
			}
		case *lua.CallExpression:
			if isPlainlyNamed(typed.Function, inputHandlerFunction) && typed.Method == nil {
				s.collectFedAction(typed)
			}
		}
		return true
	})
}

func isPlainlyNamed(expression lua.Expression, name string) bool {
	identifier, isIdentifier := expression.(*lua.Identifier)
	return isIdentifier && identifier.Name == name
}

func (s *Script) collectFedAction(call *lua.CallExpression) {
	if len(call.Arguments) <= actionIDParameter {
		return
	}
	if name, isWritten := s.IDWrittenIn(call.Arguments[actionIDParameter]); isWritten {
		s.ActionsItFeedsItself = append(s.ActionsItFeedsItself, name)
	} else {
		s.FeedsItselfActionsThatCannotBeRead = true
		s.cannotRead(call.Arguments[actionIDParameter], "the action passed to on_input here cannot be read, so the actions of this script are not judged")
	}
}

func (s *Script) collectComparedActions(handler *lua.FunctionLiteral) {
	if len(handler.Parameters) <= actionIDParameter {
		return
	}
	parameterName := handler.Parameters[actionIDParameter].Name
	isTheAction := func(expression lua.Expression) bool { return isPlainlyNamed(expression, parameterName) }
	for _, statement := range handler.Body {
		lua.Inspect(statement, func(node lua.Node) bool {
			comparison, isBinary := node.(*lua.BinaryExpression)
			if !isBinary || (comparison.Operator != "==" && comparison.Operator != "~=") {
				return true
			}
			other := comparison.Right
			switch {
			case isTheAction(comparison.Left):
			case isTheAction(comparison.Right):
				other = comparison.Left
			default:
				return true
			}
			_, isNil := other.(*lua.NilLiteral)
			if name, isWritten := s.IDWrittenIn(other); isWritten {
				s.ActionsCompared = append(s.ActionsCompared, WrittenID{Name: name, Location: s.LocationOf(other)})
			} else if !isNil {
				s.cannotRead(other, "the action compared here cannot be read")
			}
			return true
		})
	}
}

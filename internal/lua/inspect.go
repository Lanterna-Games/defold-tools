package lua

func Inspect(node Node, visit func(Node) (descend bool)) {
	if IsNil(node) || !visit(node) {
		return
	}
	for _, child := range Children(node) {
		Inspect(child, visit)
	}
}

func IsNil(node Node) bool {
	switch typed := node.(type) {
	case nil:
		return true
	case *Identifier:
		return typed == nil
	case *FunctionLiteral:
		return typed == nil
	case *CallExpression:
		return typed == nil
	default:
		return false
	}
}

func Children(node Node) []Node {
	switch typed := node.(type) {
	case *Chunk:
		return statements(typed.Body)
	case *FunctionLiteral:
		return append(identifiers(typed.Parameters), statements(typed.Body)...)
	case *IndexExpression:
		return []Node{typed.Object, typed.Index}
	case *CallExpression:
		return append([]Node{typed.Function, typed.Method}, expressions(typed.Arguments)...)
	case *BinaryExpression:
		return []Node{typed.Left, typed.Right}
	case *UnaryExpression:
		return []Node{typed.Operand}
	case *ParenthesizedExpression:
		return []Node{typed.Inner}
	case *TableLiteral:
		var children []Node
		for _, field := range typed.Fields {
			children = append(children, field.Key, field.Value)
		}
		return children
	case *LocalStatement:
		return append(identifiers(typed.Names), expressions(typed.Values)...)
	case *AssignmentStatement:
		return append(expressions(typed.Targets), expressions(typed.Values)...)
	case *CallStatement:
		return []Node{typed.Call}
	case *DoStatement:
		return statements(typed.Body)
	case *WhileStatement:
		return append([]Node{typed.Condition}, statements(typed.Body)...)
	case *RepeatStatement:
		return append(statements(typed.Body), typed.Condition)
	case *IfStatement:
		var children []Node
		for _, clause := range typed.Clauses {
			children = append(children, clause.Condition)
			children = append(children, statements(clause.Body)...)
		}
		return append(children, statements(typed.Else)...)
	case *NumericForStatement:
		return append([]Node{typed.Variable, typed.Start, typed.Stop, typed.Step}, statements(typed.Body)...)
	case *GenericForStatement:
		children := append(identifiers(typed.Names), expressions(typed.Values)...)
		return append(children, statements(typed.Body)...)
	case *FunctionStatement:
		return []Node{typed.Name, typed.Method, typed.Function}
	case *LocalFunctionStatement:
		return []Node{typed.Name, typed.Function}
	case *ReturnStatement:
		return expressions(typed.Values)
	default:
		return nil
	}
}

func statements(list []Statement) []Node {
	nodes := make([]Node, 0, len(list))
	for _, statement := range list {
		nodes = append(nodes, statement)
	}
	return nodes
}

func expressions(list []Expression) []Node {
	nodes := make([]Node, 0, len(list))
	for _, expression := range list {
		nodes = append(nodes, expression)
	}
	return nodes
}

func identifiers(list []*Identifier) []Node {
	nodes := make([]Node, 0, len(list))
	for _, identifier := range list {
		nodes = append(nodes, identifier)
	}
	return nodes
}

func QualifiedName(expression Expression) (name string, isQualifiedName bool) {
	switch typed := expression.(type) {
	case *Identifier:
		return typed.Name, true
	case *IndexExpression:
		field, isFieldName := typed.Index.(*StringLiteral)
		object, objectIsQualifiedName := QualifiedName(typed.Object)
		if !isFieldName || !objectIsQualifiedName {
			return "", false
		}
		return object + "." + field.Value, true
	default:
		return "", false
	}
}

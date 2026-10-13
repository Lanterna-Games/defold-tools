package project

import (
	"strings"

	"github.com/Lanterna-Games/defold-tools/internal/lua"
)

const (
	hashFunction = "hash"
	urlFunction  = "msg.url"
)

func constantsOf(chunk *lua.Chunk) map[string]lua.Expression {
	timesDeclared, isReassigned := map[string]int{}, map[string]bool{}
	lua.Inspect(chunk, func(node lua.Node) bool {
		switch typed := node.(type) {
		case *lua.LocalStatement:
			for _, name := range typed.Names {
				timesDeclared[name.Name]++
			}
		case *lua.AssignmentStatement:
			for _, target := range typed.Targets {
				if name, isName := target.(*lua.Identifier); isName {
					isReassigned[name.Name] = true
				}
			}
		}
		return true
	})

	constants := map[string]lua.Expression{}
	for _, statement := range chunk.Body {
		declaration, isLocal := statement.(*lua.LocalStatement)
		if !isLocal {
			continue
		}
		for index, name := range declaration.Names {
			standsForOneValue := index < len(declaration.Values) && timesDeclared[name.Name] == 1 && !isReassigned[name.Name]
			if standsForOneValue {
				constants[name.Name] = declaration.Values[index]
			}
		}
	}
	return constants
}

const (
	requireFunction       = "require"
	separatorOfModulePath = "."
	hopsThroughConstants  = 4
)

func tableFieldsOf(chunk *lua.Chunk, constants map[string]lua.Expression) map[string]map[string]lua.Expression {
	fields := map[string]map[string]lua.Expression{}
	timesSet := map[string]map[string]int{}
	set := func(table, field string, value lua.Expression) {
		if fields[table] == nil {
			fields[table], timesSet[table] = map[string]lua.Expression{}, map[string]int{}
		}
		fields[table][field] = value
		timesSet[table][field]++
	}
	for name, value := range constants {
		literal, isATable := value.(*lua.TableLiteral)
		if !isATable {
			continue
		}
		fields[name], timesSet[name] = map[string]lua.Expression{}, map[string]int{}
		for field, fieldValue := range namedFieldsOf(literal) {
			set(name, field, fieldValue)
		}
	}
	isSetAtTheTopLevel := map[lua.Node]bool{}
	for _, statement := range chunk.Body {
		isSetAtTheTopLevel[statement] = true
	}
	lua.Inspect(chunk, func(node lua.Node) bool {
		assignment, isAssignment := node.(*lua.AssignmentStatement)
		if !isAssignment {
			return true
		}
		for index, target := range assignment.Targets {
			table, field, isAField := fieldOfATable(target)
			if !isAField || fields[table] == nil {
				continue
			}
			var value lua.Expression
			if isSetAtTheTopLevel[assignment] && index < len(assignment.Values) {
				value = assignment.Values[index]
			}
			set(table, field, value)
		}
		return true
	})
	for table, tableFields := range fields {
		for field, value := range tableFields {
			if timesSet[table][field] != 1 || value == nil {
				delete(tableFields, field)
			}
		}
	}
	return fields
}

func namedFieldsOf(table *lua.TableLiteral) map[string]lua.Expression {
	fields := map[string]lua.Expression{}
	for _, field := range table.Fields {
		if name, isNamed := field.Key.(*lua.StringLiteral); isNamed {
			fields[name.Value] = field.Value
		}
	}
	return fields
}

func fieldOfATable(expression lua.Expression) (table, field string, isAField bool) {
	index, isIndex := expression.(*lua.IndexExpression)
	if !isIndex {
		return "", "", false
	}
	holder, isHeldByAName := index.Object.(*lua.Identifier)
	name, isNamed := index.Index.(*lua.StringLiteral)
	if !isHeldByAName || !isNamed {
		return "", "", false
	}
	return holder.Name, name.Value, true
}

func exportsOf(chunk *lua.Chunk, tableFields map[string]map[string]lua.Expression) map[string]lua.Expression {
	if len(chunk.Body) == 0 {
		return nil
	}
	returned, isReturn := chunk.Body[len(chunk.Body)-1].(*lua.ReturnStatement)
	if !isReturn || len(returned.Values) != 1 {
		return nil
	}
	switch typed := returned.Values[0].(type) {
	case *lua.Identifier:
		return tableFields[typed.Name]
	case *lua.TableLiteral:
		return namedFieldsOf(typed)
	default:
		return nil
	}
}

func (s *Script) moduleRequiredIn(expression lua.Expression) (module *Script, isRequired bool) {
	arguments, isRequire := argumentsOf(expression, requireFunction)
	if !isRequire || len(arguments) != 1 || s.moduleAt == nil {
		return nil, false
	}
	name, isLiteral := arguments[0].(*lua.StringLiteral)
	if !isLiteral {
		return nil, false
	}
	return s.moduleAt("/" + strings.ReplaceAll(name.Value, separatorOfModulePath, "/") + luaModuleExtension)
}

func (s *Script) written(expression lua.Expression, hopsLeft int) (value lua.Expression, writtenIn *Script) {
	for {
		parenthesized, isParenthesized := expression.(*lua.ParenthesizedExpression)
		if !isParenthesized {
			break
		}
		expression = parenthesized.Inner
	}
	if hopsLeft == 0 {
		return expression, s
	}
	switch typed := expression.(type) {
	case *lua.Identifier:
		if constant, isConstant := s.constants[typed.Name]; isConstant {
			return s.written(constant, hopsLeft-1)
		}
	case *lua.IndexExpression:
		field, isNamed := typed.Index.(*lua.StringLiteral)
		if !isNamed {
			break
		}
		holder, _ := s.written(typed.Object, hopsLeft-1)
		if module, isRequired := s.moduleRequiredIn(holder); isRequired {
			if exported, isExported := module.exports[field.Value]; isExported {
				return module.written(exported, hopsLeft-1)
			}
			break
		}
		if name, isHeldByAName := typed.Object.(*lua.Identifier); isHeldByAName {
			if constant, isConstant := s.tableFields[name.Name][field.Value]; isConstant {
				return s.written(constant, hopsLeft-1)
			}
		}
	}
	return expression, s
}

func (s *Script) Written(expression lua.Expression) lua.Expression {
	if value, writtenIn := s.written(expression, hopsThroughConstants); writtenIn == s {
		return value
	}
	return expression
}

func (s *Script) LocationOf(node lua.Node) Location {
	return Location{File: s.File, Position: node.Span().Start}
}

func argumentsOf(expression lua.Expression, function string) (arguments []lua.Expression, isACallToIt bool) {
	call, isCall := expression.(*lua.CallExpression)
	if !isCall || call.Method != nil {
		return nil, false
	}
	if name, _ := lua.QualifiedName(call.Function); name != function {
		return nil, false
	}
	return call.Arguments, true
}

func hashedLiteral(expression lua.Expression) (literal *lua.StringLiteral, isHashedLiteral bool) {
	arguments, isHash := argumentsOf(expression, hashFunction)
	if !isHash || len(arguments) != 1 {
		return nil, false
	}
	literal, isHashedLiteral = arguments[0].(*lua.StringLiteral)
	return literal, isHashedLiteral
}

func (s *Script) IDWrittenIn(expression lua.Expression) (name string, isWritten bool) {
	value, _ := s.written(expression, hopsThroughConstants)
	if literal, isLiteral := value.(*lua.StringLiteral); isLiteral {
		return literal.Value, true
	}
	if literal, isHashed := hashedLiteral(value); isHashed {
		return literal.Value, true
	}
	return "", false
}

type WrittenText struct {
	Text     string
	Location Location
}

func (s *Script) whereTheTextOf(literal *lua.StringLiteral) (written WrittenText, isBetweenQuotes bool) {
	opening := literal.Start.Offset
	if opening >= len(s.Source) || (s.Source[opening] != '"' && s.Source[opening] != '\'') {
		return WrittenText{}, false
	}
	start := literal.Start
	start.Offset++
	start.Column++
	return WrittenText{Text: literal.Value, Location: Location{File: s.File, Position: start}}, true
}

func (s *Script) TextWrittenIn(expression lua.Expression) (written WrittenText, isWritten bool) {
	value, writtenIn := s.written(expression, hopsThroughConstants)
	if literal, isLiteral := value.(*lua.StringLiteral); isLiteral {
		return writtenIn.whereTheTextOf(literal)
	}
	if literal, isHashed := hashedLiteral(value); isHashed {
		return writtenIn.whereTheTextOf(literal)
	}
	return WrittenText{}, false
}

type Address struct {
	Text     string
	Location Location
}

func (s *Script) AddressWrittenIn(expression lua.Expression) (address Address, isWritten bool) {
	value, writtenIn := s.written(expression, hopsThroughConstants)
	if literal, isLiteral := value.(*lua.StringLiteral); isLiteral {
		return Address{Text: literal.Value, Location: writtenIn.LocationOf(literal)}, true
	}
	if literal, isHashed := hashedLiteral(value); isHashed {
		namesAGameObject := strings.HasPrefix(literal.Value, "/") && !strings.ContainsAny(literal.Value, "#:")
		return Address{Text: literal.Value, Location: writtenIn.LocationOf(literal)}, namesAGameObject
	}
	if arguments, buildsAURL := argumentsOf(value, urlFunction); buildsAURL {
		return writtenIn.addressBuiltFrom(value, arguments)
	}
	return Address{}, false
}

func (s *Script) addressBuiltFrom(call lua.Expression, arguments []lua.Expression) (address Address, isWritten bool) {
	switch len(arguments) {
	case 1:
		literal, isLiteral := arguments[0].(*lua.StringLiteral)
		if !isLiteral {
			return Address{}, false
		}
		return Address{Text: literal.Value, Location: s.LocationOf(literal)}, true
	case 3:
		var parts [3]string
		for index, argument := range arguments {
			part, isWritten := urlPartWrittenIn(argument)
			if !isWritten {
				return Address{}, false
			}
			parts[index] = part
		}
		socket, text, fragment := parts[0], parts[1], parts[2]
		if socket != "" {
			text = socket + ":" + text
		}
		if fragment != "" {
			text += "#" + fragment
		}
		return Address{Text: text, Location: s.LocationOf(call)}, true
	default:
		return Address{}, false
	}
}

func urlPartWrittenIn(argument lua.Expression) (part string, isWritten bool) {
	switch typed := argument.(type) {
	case *lua.NilLiteral:
		return "", true
	case *lua.StringLiteral:
		return typed.Value, true
	default:
		literal, isHashed := hashedLiteral(argument)
		if !isHashed {
			return "", false
		}
		return literal.Value, true
	}
}

func (s *Script) URLsBuilt() []Address {
	var addresses []Address
	lua.Inspect(s.Chunk, func(node lua.Node) bool {
		call, isCall := node.(*lua.CallExpression)
		if !isCall {
			return true
		}
		if arguments, buildsAURL := argumentsOf(call, urlFunction); buildsAURL {
			if address, isWritten := s.addressBuiltFrom(call, arguments); isWritten {
				addresses = append(addresses, address)
			}
		}
		return true
	})
	return addresses
}

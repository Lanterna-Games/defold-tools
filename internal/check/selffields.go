package check

import (
	"fmt"
	"maps"
	"slices"
	"sort"

	"github.com/Lanterna-Games/defold-tools/internal/engine"
	"github.com/Lanterna-Games/defold-tools/internal/lua"
	"github.com/Lanterna-Games/defold-tools/internal/project"
)

const (
	unassignedSelfFieldCheck = "unassigned-self-field"
	instanceName             = "self"
)

var lifecycleFunctions = map[string]bool{
	"init": true, "final": true, "update": true, "fixed_update": true, "late_update": true,
	"on_message": true, "on_input": true, "on_reload": true,
}

func LifecycleFunctions() []string {
	return slices.Sorted(maps.Keys(lifecycleFunctions))
}

func unassignedSelfFields(inspected inspection) []Problem {
	var problems []Problem
	for _, script := range inspected.project.Scripts {
		problems = append(problems, unassignedSelfFieldsOf(script, inspected.reference)...)
	}
	return problems
}

type instanceUsage struct {
	reference         *engine.Reference
	functionsByName   map[string]*lua.FunctionLiteral
	assigned          map[string]bool
	reads             []*lua.StringLiteral
	pendingFunctions  []*lua.FunctionLiteral
	visitedFunctions  map[*lua.FunctionLiteral]bool
	instanceIsTracked bool
}

func unassignedSelfFieldsOf(script *project.Script, reference *engine.Reference) []Problem {
	if !script.DeclaresPropertiesByLiteral {
		return nil
	}
	usage := &instanceUsage{
		reference:         reference,
		functionsByName:   map[string]*lua.FunctionLiteral{},
		assigned:          map[string]bool{},
		visitedFunctions:  map[*lua.FunctionLiteral]bool{},
		instanceIsTracked: true,
	}
	for _, property := range script.Properties {
		usage.assigned[property.Name] = true
	}
	lua.Inspect(script.Chunk, usage.collectFunctionsAndAssignments)
	usage.followTheInstanceFromTheLifecycleFunctions()
	if !usage.instanceIsTracked {
		return nil
	}

	var problems []Problem
	for _, read := range usage.reads {
		if usage.assigned[read.Value] {
			continue
		}
		message := fmt.Sprintf("%s.%s is read, but this script never assigns it", instanceName, read.Value)
		message += hintTowards(read.Value, usage.assignedNames(), instanceName+".%s")
		problems = append(problems, Problem{
			Check:    unassignedSelfFieldCheck,
			Location: project.Location{File: script.File, Position: read.Start},
			Message:  message,
		})
	}
	return problems
}

func (u *instanceUsage) assignedNames() []string {
	names := make([]string, 0, len(u.assigned))
	for name := range u.assigned {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (u *instanceUsage) collectFunctionsAndAssignments(node lua.Node) bool {
	switch typed := node.(type) {
	case *lua.FunctionStatement:
		if name, isPlainName := typed.Name.(*lua.Identifier); isPlainName && typed.Method == nil {
			u.functionsByName[name.Name] = typed.Function
		}
	case *lua.LocalFunctionStatement:
		u.functionsByName[typed.Name.Name] = typed.Function
	case *lua.LocalStatement:
		for index, name := range typed.Names {
			if index >= len(typed.Values) {
				break
			}
			if function, isFunction := typed.Values[index].(*lua.FunctionLiteral); isFunction {
				u.functionsByName[name.Name] = function
			}
		}
	case *lua.AssignmentStatement:
		for _, target := range typed.Targets {
			if field, isField := fieldOfSomethingNamedSelf(target); isField {
				u.assigned[field.Value] = true
			}
		}
	}
	return true
}

func (u *instanceUsage) followTheInstanceFromTheLifecycleFunctions() {
	for name, function := range u.functionsByName {
		if lifecycleFunctions[name] && receivesTheInstanceAt(function, 0) {
			u.pendingFunctions = append(u.pendingFunctions, function)
		}
	}
	for len(u.pendingFunctions) > 0 {
		function := u.pendingFunctions[0]
		u.pendingFunctions = u.pendingFunctions[1:]
		if u.visitedFunctions[function] {
			continue
		}
		u.visitedFunctions[function] = true
		u.visitAll(statementNodes(function.Body))
	}
}

func receivesTheInstanceAt(function *lua.FunctionLiteral, index int) bool {
	return index < len(function.Parameters) && function.Parameters[index].Name == instanceName
}

func (u *instanceUsage) visitAll(nodes []lua.Node) {
	for _, node := range nodes {
		u.visit(node)
	}
}

func (u *instanceUsage) visit(node lua.Node) {
	if lua.IsNil(node) {
		return
	}
	switch typed := node.(type) {
	case *lua.Identifier:
		if typed.Name == instanceName {
			u.instanceIsTracked = false
		}
	case *lua.IndexExpression:
		u.visitIndex(typed)
	case *lua.AssignmentStatement:
		for _, target := range typed.Targets {
			if _, isField := fieldOfSomethingNamedSelf(target); !isField {
				u.visit(target)
			}
		}
		u.visitAll(expressionNodes(typed.Values))
	case *lua.CallExpression:
		u.visitCall(typed)
	case *lua.FunctionLiteral:
		u.visitAll(statementNodes(typed.Body))
	case *lua.FunctionStatement:
		if typed.Method == nil {
			u.visit(typed.Function)
		}
	case *lua.LocalFunctionStatement:
		u.visit(typed.Function)
	default:
		u.visitAll(lua.Children(node))
	}
}

func (u *instanceUsage) visitIndex(index *lua.IndexExpression) {
	object, isIdentifier := index.Object.(*lua.Identifier)
	if !isIdentifier || object.Name != instanceName {
		u.visit(index.Object)
		u.visit(index.Index)
		return
	}
	field, isLiteral := index.Index.(*lua.StringLiteral)
	if !isLiteral {
		u.instanceIsTracked = false
		return
	}
	u.reads = append(u.reads, field)
}

func (u *instanceUsage) visitCall(call *lua.CallExpression) {
	u.visit(call.Function)
	function, isQualifiedName := lua.QualifiedName(call.Function)
	_, isEngineFunction := u.reference.Functions[function]
	declared, isDeclaredHere := u.functionsByName[function]
	canBeFollowed := isQualifiedName && call.Method == nil

	for index, argument := range call.Arguments {
		instance, isIdentifier := argument.(*lua.Identifier)
		if !isIdentifier || instance.Name != instanceName {
			u.visit(argument)
			continue
		}
		switch {
		case canBeFollowed && isDeclaredHere && receivesTheInstanceAt(declared, index):
			u.pendingFunctions = append(u.pendingFunctions, declared)
		case canBeFollowed && isEngineFunction:
		default:
			u.instanceIsTracked = false
		}
	}
}

func fieldOfSomethingNamedSelf(expression lua.Expression) (field *lua.StringLiteral, isField bool) {
	index, isIndex := expression.(*lua.IndexExpression)
	if !isIndex {
		return nil, false
	}
	object, isIdentifier := index.Object.(*lua.Identifier)
	if !isIdentifier || object.Name != instanceName {
		return nil, false
	}
	field, isLiteral := index.Index.(*lua.StringLiteral)
	return field, isLiteral
}

func statementNodes(statements []lua.Statement) []lua.Node {
	nodes := make([]lua.Node, 0, len(statements))
	for _, statement := range statements {
		nodes = append(nodes, statement)
	}
	return nodes
}

func expressionNodes(expressions []lua.Expression) []lua.Node {
	nodes := make([]lua.Node, 0, len(expressions))
	for _, expression := range expressions {
		nodes = append(nodes, expression)
	}
	return nodes
}

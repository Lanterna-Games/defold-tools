package check

import (
	"github.com/Lanterna-Games/defold-tools/internal/engine"
	"github.com/Lanterna-Games/defold-tools/internal/lua"
	"github.com/Lanterna-Games/defold-tools/internal/project"
)

const (
	unverifiableCheck      = "unverifiable"
	operatorThatJoinsText  = ".."
	whyAPropertyIsUnread   = "the name of the property cannot be read"
	whyAnAnimationIsUnread = "the animation played here cannot be read"
	whyAGroupIsUnread      = "the collision group named here cannot be read"
	whyAnAddressIsUnread   = "this address is put together while the game runs"
)

var functionsThatBuildText = map[string]bool{"string.format": true, "tostring": true}

func unverifiable(inspected inspection) []Problem {
	var problems []Problem
	for _, scriptPath := range inspected.project.ScriptPaths() {
		script := inspected.project.Scripts[scriptPath]
		for _, spot := range script.BlindSpots {
			problems = append(problems, Problem{Check: unverifiableCheck, Location: spot.Location, Message: spot.Why})
		}
		scan := unreadScan{script: script, reference: inspected.reference}
		lua.Inspect(script.Chunk, scan.visit)
		problems = append(problems, scan.problems...)
	}
	return problems
}

type unreadScan struct {
	script    *project.Script
	reference *engine.Reference
	problems  []Problem
}

func (u *unreadScan) report(expression lua.Expression, why string) {
	u.problems = append(u.problems, Problem{Check: unverifiableCheck, Location: u.script.LocationOf(expression), Message: why})
}

func (u *unreadScan) expectAName(arguments []lua.Expression, index int, why string) {
	if index >= len(arguments) {
		return
	}
	if _, isWritten := u.script.IDWrittenIn(arguments[index]); !isWritten {
		u.report(arguments[index], why)
	}
}

func (u *unreadScan) visit(node lua.Node) bool {
	call, isCall := node.(*lua.CallExpression)
	if !isCall || call.Method != nil {
		return true
	}
	function, isQualifiedName := lua.QualifiedName(call.Function)
	if !isQualifiedName {
		return true
	}
	if _, property, takesAProperty := u.reference.PropertyParameterIndexes(function); takesAProperty {
		u.expectAName(call.Arguments, property, whyAPropertyIsUnread)
	}
	if function == spritePlayFunction || function == guiPlayFunction {
		u.expectAName(call.Arguments, 1, whyAnAnimationIsUnread)
	}
	if function == postFunction {
		if animation, isAPlayMessage := animationOfThePlayMessage(u.script, call); isAPlayMessage {
			u.expectAName([]lua.Expression{animation}, 0, whyAnAnimationIsUnread)
		}
	}
	ofOneGroup, ofManyGroups := u.reference.CollisionGroupParameterIndexes(function)
	for _, index := range ofOneGroup {
		u.expectAName(call.Arguments, index, whyAGroupIsUnread)
	}
	for _, index := range ofManyGroups {
		u.expectGroups(call.Arguments, index)
	}
	for _, index := range u.reference.URLParameterIndexes(function) {
		if index < len(call.Arguments) && isBuiltFromText(call.Arguments[index]) {
			u.report(call.Arguments[index], whyAnAddressIsUnread)
		}
	}
	return true
}

func (u *unreadScan) expectGroups(arguments []lua.Expression, index int) {
	if index >= len(arguments) {
		return
	}
	groups, isATable := u.script.Written(arguments[index]).(*lua.TableLiteral)
	if !isATable {
		u.report(arguments[index], whyAGroupIsUnread)
		return
	}
	for _, field := range groups.Fields {
		u.expectAName([]lua.Expression{field.Value}, 0, whyAGroupIsUnread)
	}
}

func isBuiltFromText(expression lua.Expression) bool {
	switch typed := expression.(type) {
	case *lua.ParenthesizedExpression:
		return isBuiltFromText(typed.Inner)
	case *lua.BinaryExpression:
		return typed.Operator == operatorThatJoinsText
	case *lua.CallExpression:
		function, _ := lua.QualifiedName(typed.Function)
		return typed.Method == nil && functionsThatBuildText[function]
	default:
		return false
	}
}

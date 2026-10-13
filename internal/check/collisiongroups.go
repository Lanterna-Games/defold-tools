package check

import (
	"fmt"

	"github.com/Lanterna-Games/defold-tools/internal/engine"
	"github.com/Lanterna-Games/defold-tools/internal/lua"
	"github.com/Lanterna-Games/defold-tools/internal/project"
)

const (
	unknownCollisionGroupCheck = "unknown-collision-group"
	messageHandlerName         = "on_message"
	messageParameter           = 2
	groupOfARayCastResponse    = "group"
)

var fieldsThatHoldACollisionGroup = map[string]bool{"other_group": true, "own_group": true}

func unknownCollisionGroups(inspected inspection) []Problem {
	loaded := inspected.project
	nothingDeclaresAGroup := len(loaded.CollisionGroups) == 0
	if loaded.RunsCodeOutOfSight || nothingDeclaresAGroup {
		return nil
	}
	known := loaded.CollisionGroupNames()

	var problems []Problem
	report := func(group project.WrittenID, subject string) {
		if loaded.CollisionGroups[group.Name] {
			return
		}
		problems = append(problems, Problem{
			Check:    unknownCollisionGroupCheck,
			Location: group.Location,
			Message:  fmt.Sprintf("%s %q is not the group of any collision object%s", subject, group.Name, hintTowards(group.Name, known, "%q")),
		})
	}
	for _, mask := range loaded.CollisionMasks {
		report(mask, "mask")
	}
	for _, scriptPath := range loaded.ScriptPaths() {
		for _, group := range collisionGroupsNamedIn(loaded.Scripts[scriptPath], inspected.reference) {
			report(group, "group")
		}
	}
	return problems
}

type groupScan struct {
	script    *project.Script
	reference *engine.Reference
	named     []project.WrittenID
}

func collisionGroupsNamedIn(script *project.Script, reference *engine.Reference) []project.WrittenID {
	scan := &groupScan{script: script, reference: reference}
	lua.Inspect(script.Chunk, func(node lua.Node) bool {
		switch typed := node.(type) {
		case *lua.BinaryExpression:
			scan.readComparison(typed, isAFieldThatHoldsACollisionGroup)
		case *lua.CallExpression:
			scan.readCall(typed)
		case *lua.FunctionStatement:
			scan.readMessageHandler(typed)
		}
		return true
	})
	return scan.named
}

func isAFieldThatHoldsACollisionGroup(expression lua.Expression) bool {
	_, field, isField := fieldOf(expression)
	return isField && fieldsThatHoldACollisionGroup[field]
}

func fieldOf(expression lua.Expression) (object lua.Expression, field string, isField bool) {
	index, isIndex := expression.(*lua.IndexExpression)
	if !isIndex {
		return nil, "", false
	}
	name, isNamed := index.Index.(*lua.StringLiteral)
	if !isNamed {
		return nil, "", false
	}
	return index.Object, name.Value, true
}

func (s *groupScan) name(expression lua.Expression) {
	if name, isWritten := s.script.IDWrittenIn(expression); isWritten {
		s.named = append(s.named, project.WrittenID{Name: name, Location: s.script.LocationOf(expression)})
	}
}

func (s *groupScan) readComparison(comparison *lua.BinaryExpression, holdsAGroup func(lua.Expression) bool) {
	if comparison.Operator != "==" && comparison.Operator != "~=" {
		return
	}
	switch {
	case holdsAGroup(comparison.Left):
		s.name(comparison.Right)
	case holdsAGroup(comparison.Right):
		s.name(comparison.Left)
	}
}

func (s *groupScan) readCall(call *lua.CallExpression) {
	if call.Method != nil {
		return
	}
	function, _ := lua.QualifiedName(call.Function)
	ofOneGroup, ofManyGroups := s.reference.CollisionGroupParameterIndexes(function)
	for _, index := range ofOneGroup {
		if index < len(call.Arguments) {
			s.name(call.Arguments[index])
		}
	}
	for _, index := range ofManyGroups {
		if index >= len(call.Arguments) {
			continue
		}
		groups, isATable := s.script.Written(call.Arguments[index]).(*lua.TableLiteral)
		if !isATable {
			continue
		}
		for _, field := range groups.Fields {
			s.name(field.Value)
		}
	}
}

func (s *groupScan) readMessageHandler(function *lua.FunctionStatement) {
	name, isPlainName := function.Name.(*lua.Identifier)
	if !isPlainName || function.Method != nil || name.Name != messageHandlerName || len(function.Function.Parameters) <= messageParameter {
		return
	}
	message := function.Function.Parameters[messageParameter].Name
	isTheGroupOfTheMessage := func(expression lua.Expression) bool {
		object, field, isField := fieldOf(expression)
		holder, isAName := object.(*lua.Identifier)
		return isField && isAName && field == groupOfARayCastResponse && holder.Name == message
	}
	for _, statement := range function.Function.Body {
		lua.Inspect(statement, func(node lua.Node) bool {
			if comparison, isComparison := node.(*lua.BinaryExpression); isComparison {
				s.readComparison(comparison, isTheGroupOfTheMessage)
			}
			return true
		})
	}
}

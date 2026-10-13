package check

import (
	"fmt"
	"slices"
)

const unboundActionCheck = "unbound-action"

type boundActions struct {
	names       []string
	description string
}

func (i inspection) boundActions() (bound boundActions, isInSight bool) {
	gameBinding := i.project.GameBinding
	_, isInTheProject := i.project.InputBindings[gameBinding]
	builtin, isBuiltin := i.reference.InputBindings[gameBinding]
	if !isInTheProject && !isBuiltin {
		return boundActions{}, false
	}

	bound.names = slices.Clone(builtin)
	for _, actions := range i.project.InputBindings {
		bound.names = append(bound.names, actions...)
	}
	slices.Sort(bound.names)
	bound.names = slices.Compact(bound.names)

	bound.description = gameBinding
	gameBindingIsTheOnlyOne := len(i.project.InputBindings) == 0 || (isInTheProject && len(i.project.InputBindings) == 1)
	if !gameBindingIsTheOnlyOne {
		bound.description = "any input binding of the project"
	}
	return bound, true
}

func unboundActions(inspected inspection) []Problem {
	bound, isInSight := inspected.boundActions()
	if !isInSight {
		return nil
	}
	var problems []Problem
	for _, scriptPath := range inspected.project.ScriptPaths() {
		script := inspected.project.Scripts[scriptPath]
		if script.FeedsItselfActionsThatCannotBeRead {
			continue
		}
		for _, action := range script.ActionsCompared {
			if slices.Contains(bound.names, action.Name) || slices.Contains(script.ActionsItFeedsItself, action.Name) {
				continue
			}
			problems = append(problems, Problem{
				Check:    unboundActionCheck,
				Location: action.Location,
				Message: fmt.Sprintf("%q is not bound in %s%s",
					action.Name, bound.description, hintTowards(action.Name, bound.names, "%q")),
			})
		}
	}
	return problems
}

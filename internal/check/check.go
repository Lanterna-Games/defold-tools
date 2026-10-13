package check

import (
	"regexp"
	"sort"

	"github.com/Lanterna-Games/defold-tools/internal/engine"
	"github.com/Lanterna-Games/defold-tools/internal/project"
)

type Problem struct {
	Check      string
	Location   project.Location
	Message    string
	Suggestion string

	RemovesTheLine bool
}

var suggestionInAMessage = regexp.MustCompile(`\(did you mean (?:"([^"]+)"|self\.([A-Za-z_][A-Za-z0-9_]*))\?\)`)

func suggestionIn(message string) string {
	found := suggestionInAMessage.FindStringSubmatch(message)
	if found == nil {
		return ""
	}
	return found[1] + found[2]
}

type inspection struct {
	project   *project.Project
	reference *engine.Reference
}

type checker func(inspection) []Problem

var checkers = []checker{
	syntaxErrors,
	missingResources,
	unknownChildren,
	unresolvedURLs,
	undeclaredProperties,
	unassignedSelfFields,
	unhandledMessages,
	unpostedMessages,
	unknownProperties,
	unboundActions,
	unknownCollisionGroups,
	unknownGUINodes,
	unknownAnimations,
	unknownMessageFields,
	unusedFiles,
	unknownSettings,
	unsetSettings,
	unverifiable,
}

func Run(loaded *project.Project, reference *engine.Reference, options Options) []Problem {
	inspected := inspection{project: loaded, reference: reference}
	var found []Problem
	for _, check := range checkers {
		for _, problem := range check(inspected) {
			if !loaded.IsOfALibrary(problem.Location.File) {
				found = append(found, problem)
			}
		}
	}
	problems := options.apply(loaded, found)
	for index := range problems {
		problems[index].Suggestion = suggestionIn(problems[index].Message)
	}
	sort.SliceStable(problems, func(i, j int) bool {
		a, b := problems[i].Location, problems[j].Location
		if a.File != b.File {
			return a.File < b.File
		}
		return a.Position.Offset < b.Position.Offset
	})
	return problems
}

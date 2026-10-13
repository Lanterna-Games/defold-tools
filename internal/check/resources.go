package check

import (
	"fmt"
	"path"

	"github.com/Lanterna-Games/defold-tools/internal/suggest"
)

const missingResourceCheck = "missing-resource"

func missingResources(inspected inspection) []Problem {
	loaded := inspected.project
	var problems []Problem
	for _, reference := range loaded.References {
		if loaded.HasFile(reference.Path) || !loaded.HasTopLevelDirectoryOf(reference.Path) {
			continue
		}
		message := fmt.Sprintf("%q does not exist", reference.Path)
		if closest, found := suggest.Closest(reference.Path, loaded.FilesWithExtension(path.Ext(reference.Path))); found {
			message += fmt.Sprintf(" (did you mean %q?)", closest)
		}
		problems = append(problems, Problem{Check: missingResourceCheck, Location: reference.Location, Message: message})
	}
	return problems
}

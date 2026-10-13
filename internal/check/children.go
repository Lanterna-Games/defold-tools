package check

import (
	"fmt"

	"github.com/Lanterna-Games/defold-tools/internal/suggest"
)

const unknownChildCheck = "unknown-child"

func unknownChildren(inspected inspection) []Problem {
	loaded := inspected.project
	var problems []Problem
	for _, collectionPath := range loaded.CollectionPaths() {
		collection := loaded.Collections[collectionPath]
		instanceIDs := collection.InstanceIDs()
		isInstance := map[string]bool{}
		for _, id := range instanceIDs {
			isInstance[id] = true
		}
		for _, instance := range collection.Instances {
			for _, child := range instance.Children {
				if isInstance[child.ID] {
					continue
				}
				message := fmt.Sprintf("%q lists the child %q, but collection %q has no game object with that id", instance.ID, child.ID, collection.Name)
				if closest, found := suggest.Closest(child.ID, instanceIDs); found {
					message += fmt.Sprintf(" (did you mean %q?)", closest)
				}
				problems = append(problems, Problem{Check: unknownChildCheck, Location: child.Location, Message: message})
			}
		}
	}
	return problems
}

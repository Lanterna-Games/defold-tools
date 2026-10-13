package check

import (
	"fmt"

	"github.com/Lanterna-Games/defold-tools/internal/project"
)

const undeclaredPropertyCheck = "undeclared-property"

func undeclaredProperties(inspected inspection) []Problem {
	loaded := inspected.project
	alreadyReported := map[project.Location]bool{}
	var problems []Problem

	judge := func(instancePath string, gameObject *project.GameObject, overrides []project.PropertyOverride) {
		if gameObject == nil {
			return
		}
		for _, override := range overrides {
			component, hasComponent := gameObject.Component(override.Component)
			if !hasComponent || !component.IsScript() || alreadyReported[override.NameLocation] {
				continue
			}
			script, isLoaded := loaded.Scripts[component.Resource]
			if !isLoaded || !script.DeclaresPropertiesByLiteral || script.DeclaresProperty(override.Property) {
				continue
			}
			alreadyReported[override.NameLocation] = true
			message := fmt.Sprintf("%q sets the property %q, but %s does not declare it", instancePath+"#"+override.Component, override.Property, component.Resource)
			if hint := hintTowards(override.Property, script.PropertyNames(), "%q"); hint != "" {
				message += hint
			} else {
				message += "; it declares " + quotedList(script.PropertyNames())
			}
			problems = append(problems, Problem{Check: undeclaredPropertyCheck, Location: override.NameLocation, Message: message})
		}
	}

	for _, mount := range loaded.Mounts() {
		for _, instance := range mount.Collection.Instances {
			judge(mount.Prefix+"/"+instance.ID, loaded.GameObjectOf(instance), instance.PropertyOverrides)
		}
		for _, subCollection := range mount.Collection.SubCollections {
			for instanceID, overrides := range subCollection.PropertyOverrides {
				instancePath := mount.Prefix + "/" + subCollection.ID + "/" + instanceID
				judge(instancePath, mount.Namespace.GameObjects[instancePath], overrides)
			}
		}
	}
	return problems
}

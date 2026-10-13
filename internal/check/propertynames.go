package check

import (
	"fmt"
	"path"
	"slices"

	"github.com/Lanterna-Games/defold-tools/internal/engine"
	"github.com/Lanterna-Games/defold-tools/internal/lua"
	"github.com/Lanterna-Games/defold-tools/internal/project"
)

const (
	unknownPropertyCheck = "unknown-property"
	guiScriptExtension   = ".gui_script"
	guiComponentType     = "gui"
)

type propertyUse struct {
	written         string
	name            string
	location        project.Location
	target          string
	targetIsWritten bool
}

func propertyUsesIn(script *project.Script, reference *engine.Reference) []propertyUse {
	var uses []propertyUse
	lua.Inspect(script.Chunk, func(node lua.Node) bool {
		call, isCall := node.(*lua.CallExpression)
		if !isCall || call.Method != nil {
			return true
		}
		function, _ := lua.QualifiedName(call.Function)
		addressIndex, propertyIndex, takesAProperty := reference.PropertyParameterIndexes(function)
		if !takesAProperty || propertyIndex >= len(call.Arguments) {
			return true
		}
		written, isWritten := script.IDWrittenIn(call.Arguments[propertyIndex])
		if !isWritten {
			return true
		}
		use := propertyUse{
			written:  written,
			name:     engine.PropertyWithoutItsElement(written),
			location: script.LocationOf(call.Arguments[propertyIndex]),
		}
		if target, targetIsWritten := script.AddressWrittenIn(call.Arguments[addressIndex]); targetIsWritten {
			use.target, use.targetIsWritten = target.Text, true
		}
		uses = append(uses, use)
		return true
	})
	return uses
}

type propertyHolder struct {
	description     string
	properties      []string
	script          *project.Script
	holdsMoreThanIs bool
}

type propertyJudge struct {
	inspection
	verdicts            verdicts
	everyProperty       []string
	everythingIsInSight bool
}

func unknownProperties(inspected inspection) []Problem {
	loaded := inspected.project
	judge := propertyJudge{
		inspection:          inspected,
		verdicts:            newVerdicts(unknownPropertyCheck),
		everyProperty:       inspected.reference.EveryProperty(),
		everythingIsInSight: !loaded.RunsCodeOutOfSight,
	}
	for name := range loaded.MaterialProperties {
		judge.everyProperty = append(judge.everyProperty, name)
	}
	for _, script := range loaded.Scripts {
		judge.everyProperty = append(judge.everyProperty, script.PropertyNames()...)
		if !declaresWhatCanBeRead(script) {
			judge.everythingIsInSight = false
		}
	}
	slices.Sort(judge.everyProperty)
	judge.everyProperty = slices.Compact(judge.everyProperty)

	hasAnOwner, usesIn := map[string]bool{}, map[string][]propertyUse{}
	for _, owner := range loaded.Owners() {
		for _, scriptPath := range loaded.ScriptPathsOf(owner.GameObject) {
			script, isLoaded := loaded.Scripts[scriptPath]
			if !isLoaded {
				continue
			}
			if !hasAnOwner[scriptPath] {
				usesIn[scriptPath] = propertyUsesIn(script, inspected.reference)
			}
			hasAnOwner[scriptPath] = true
			for _, use := range usesIn[scriptPath] {
				judge.judge(use, script, &owner)
			}
		}
	}
	for _, scriptPath := range loaded.ScriptPaths() {
		if hasAnOwner[scriptPath] {
			continue
		}
		for _, use := range propertyUsesIn(loaded.Scripts[scriptPath], inspected.reference) {
			judge.judge(use, loaded.Scripts[scriptPath], nil)
		}
	}
	return judge.verdicts.problems()
}

func declaresWhatCanBeRead(script *project.Script) bool {
	return script.IsReadInFull && script.DeclaresPropertiesByLiteral
}

func (j propertyJudge) judge(use propertyUse, user *project.Script, owner *project.Owner) {
	if use.targetIsWritten && owner != nil {
		if holder, isInSight := j.holderOf(project.ParseURL(use.target), user, *owner); isInSight {
			j.judgeAgainst(holder, use)
			return
		}
	}
	if !j.everythingIsInSight || engine.NamesTheProperty(j.everyProperty, use.name) {
		j.verdicts.accept(use.location)
		return
	}
	j.verdicts.reject(use.location, fmt.Sprintf("%q: nothing in the project has a property %q%s",
		use.written, use.name, hintTowards(use.name, j.everyProperty, "%q")))
}

func (j propertyJudge) judgeAgainst(holder propertyHolder, use propertyUse) {
	if holder.holdsMoreThanIs || engine.NamesTheProperty(holder.properties, use.name) {
		j.verdicts.accept(use.location)
		return
	}
	hint := hintTowards(use.name, holder.properties, "%q")
	if holder.script == nil {
		j.verdicts.reject(use.location, fmt.Sprintf("%q: %q is %s, which has no property %q%s",
			use.written, use.target, holder.description, use.name, hint))
		return
	}
	if hint == "" {
		hint = "; it declares " + quotedList(holder.properties)
	}
	j.verdicts.rejectAs(undeclaredPropertyCheck, use.location, fmt.Sprintf("%q: %s does not declare the property %q%s",
		use.written, holder.script.Path, use.name, hint))
}

func (j propertyJudge) holderOf(url project.URL, user *project.Script, owner project.Owner) (holder propertyHolder, isInSight bool) {
	addressesTheUserItself := url.Socket == "" && url.Path == "" && url.HasFragment && url.Fragment == ""
	if addressesTheUserItself {
		if path.Ext(user.Path) == guiScriptExtension {
			return j.holderOfType(guiComponentType), true
		}
		return holderThatIs(user), true
	}
	target, isKnown := j.project.TargetOf(owner, url)
	if !isKnown {
		return propertyHolder{}, false
	}
	if !url.HasFragment || url.Fragment == "" {
		properties, _ := j.reference.PropertiesOf(engine.GameObject)
		return propertyHolder{description: "a game object", properties: properties}, true
	}
	component, exists := target.GameObject.Component(url.Fragment)
	if !exists {
		return propertyHolder{}, false
	}
	if !component.IsScript() {
		return j.holderOfType(component.Type), true
	}
	script, isLoaded := j.project.Scripts[component.Resource]
	if !isLoaded {
		return propertyHolder{}, false
	}
	return holderThatIs(script), true
}

func holderThatIs(script *project.Script) propertyHolder {
	return propertyHolder{script: script, properties: script.PropertyNames(), holdsMoreThanIs: !declaresWhatCanBeRead(script)}
}

func (j propertyJudge) holderOfType(componentType string) propertyHolder {
	properties, isDocumented := j.reference.PropertiesOf(componentType)
	holder := propertyHolder{
		description:     fmt.Sprintf("a %s component", componentType),
		properties:      properties,
		holdsMoreThanIs: !isDocumented,
	}
	if j.reference.IsDrawnWithAMaterial(componentType) {
		holder.holdsMoreThanIs = holder.holdsMoreThanIs || j.project.RunsCodeOutOfSight
		holder.properties = append(holder.properties, j.reference.MaterialProperties...)
		for name := range j.project.MaterialProperties {
			holder.properties = append(holder.properties, name)
		}
		slices.Sort(holder.properties)
	}
	return holder
}

package insight

import (
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/Lanterna-Games/defold-tools/internal/docs"
	"github.com/Lanterna-Games/defold-tools/internal/engine"
	"github.com/Lanterna-Games/defold-tools/internal/project"
)

const (
	scriptComponentType = "script"
	typesOfAField       = " | "
)

type Documentation struct {
	Reference *docs.Reference
	Links     docs.Links
}

type kinds struct {
	names    []string
	areKnown bool
}

type documented struct {
	module      *docs.Module
	name        string
	brief       string
	description string
	examples    string
	fields      []docs.Value
	links       string
}

func (i Insight) documentation() (reference *docs.Reference, isAtHand bool) {
	reference = i.Documentation.Reference
	return reference, reference != nil && i.Reference != nil && reference.Version == i.Reference.Defold
}

func (k kinds) chosenAmong(found []documented, whenNoneIsOfTheKind func(found []documented) []documented) []documented {
	if !k.areKnown {
		return found
	}
	var ofTheKind []documented
	for _, candidate := range found {
		if slices.Contains(k.names, engine.ComponentTypeDocumentedIn(candidate.module.Namespace)) {
			ofTheKind = append(ofTheKind, candidate)
		}
	}
	if len(ofTheKind) > 0 {
		return ofTheKind
	}
	return whenNoneIsOfTheKind(found)
}

func thoseOfAnyGameObject(found []documented) []documented {
	var ofGameObjects []documented
	for _, candidate := range found {
		if candidate.module.Namespace == engine.GameObject {
			ofGameObjects = append(ofGameObjects, candidate)
		}
	}
	if len(ofGameObjects) > 0 {
		return ofGameObjects
	}
	return found
}

func none([]documented) []documented {
	return nil
}

func (i Insight) documentedMessages(name string, of kinds) []documented {
	reference, isAtHand := i.documentation()
	if !isAtHand {
		return nil
	}
	var found []documented
	for index := range reference.Modules {
		module := &reference.Modules[index]
		for _, message := range module.Messages {
			if message.Name == name {
				found = append(found, documented{
					module: module, name: name, brief: message.Brief, description: message.Description,
					examples: message.Examples, fields: message.Fields,
					links: i.Documentation.Links.ToTheMessage(reference.Version, module.Namespace, name),
				})
			}
		}
	}
	return of.chosenAmong(found, thoseOfAnyGameObject)
}

func (i Insight) documentedProperties(name string, of kinds) []documented {
	reference, isAtHand := i.documentation()
	if !isAtHand {
		return nil
	}
	whole := engine.PropertyWithoutItsElement(name)
	var found []documented
	for index := range reference.Modules {
		module := &reference.Modules[index]
		for _, property := range module.Properties {
			if engine.NamesTheProperty([]string{property.Name}, whole) {
				found = append(found, documented{
					module: module, name: property.Name, brief: property.Brief, description: property.Description,
					examples: property.Examples,
					links:    i.Documentation.Links.ToTheProperty(reference.Version, module.Namespace, property.Name),
				})
			}
		}
	}
	return of.chosenAmong(found, none)
}

func describedField(field docs.Value) string {
	var qualities []string
	if len(field.Types) > 0 {
		qualities = append(qualities, strings.Join(field.Types, typesOfAField))
	}
	if field.IsOptional {
		qualities = append(qualities, "optional")
	}
	line := "`" + field.Name + "`"
	if len(qualities) > 0 {
		line += " *" + strings.Join(qualities, ", ") + "*"
	}
	if doc := docs.OneLine(field.Doc); doc != "" {
		line += " — " + doc
	}
	return line
}

func (d documented) explained(withExamples bool) string {
	var parts []string
	if description := docs.Markdown(firstOf(d.description, d.brief)); description != "" {
		parts = append(parts, description)
	}
	if len(d.fields) > 0 {
		lines := []string{"**Fields:**", ""}
		for _, field := range d.fields {
			lines = append(lines, "- "+describedField(field))
		}
		parts = append(parts, strings.Join(lines, "\n"))
	}
	if examples := docs.Markdown(d.examples); withExamples && examples != "" {
		parts = append(parts, "**Examples:**\n\n"+examples)
	}
	return strings.Join(append(parts, d.links), "\n\n")
}

func (d documented) mentioned() string {
	line := "- **" + d.module.Name + "**"
	if brief := docs.OneLine(d.brief); brief != "" {
		line += " — " + brief
	}
	return line + " · " + d.links
}

func firstOf(texts ...string) string {
	for _, text := range texts {
		if strings.TrimSpace(text) != "" {
			return text
		}
	}
	return ""
}

func whatTheReferenceSays(found []documented, withExamples bool) string {
	switch len(found) {
	case 0:
		return ""
	case 1:
		return found[0].explained(withExamples)
	}
	lines := make([]string, 0, len(found))
	for _, each := range found {
		lines = append(lines, each.mentioned())
	}
	return strings.Join(lines, "\n")
}

func (i Insight) kindsPostedTo(about subject) kinds {
	if !about.place.TargetIsWritten {
		return kinds{}
	}
	url := project.ParseURL(about.place.Target)
	if url.Socket == "" && url.Path == "" && url.HasFragment && url.Fragment == "" {
		return kindOf(about.script)
	}
	components := i.componentsAt(about, about.place.Target)
	return kinds{names: typesOf(components), areKnown: len(components) > 0}
}

func kindOf(script *project.Script) kinds {
	if path.Ext(script.Path) == guiScriptExtension {
		return kinds{names: []string{guiComponentType}, areKnown: true}
	}
	return kinds{names: []string{scriptComponentType}, areKnown: true}
}

func (i Insight) kindsThatSendTo(about subject) kinds {
	if path.Ext(about.script.Path) == guiScriptExtension {
		return kinds{names: []string{guiComponentType}, areKnown: true}
	}
	var siblings []project.Component
	for _, owner := range about.owners {
		siblings = append(siblings, owner.GameObject.Components...)
	}
	return kinds{names: typesOf(siblings), areKnown: len(siblings) > 0}
}

func (i Insight) kindsHolding(about subject) kinds {
	if !about.place.TargetIsWritten {
		return kinds{}
	}
	url := project.ParseURL(about.place.Target)
	switch {
	case url.Socket == "" && url.Path == "" && url.HasFragment && url.Fragment == "":
		return kindOf(about.script)
	case !url.HasFragment || url.Fragment == "":
		return kinds{names: []string{engine.GameObject}, areKnown: true}
	}
	components := i.componentsAt(about, about.place.Target)
	return kinds{names: typesOf(components), areKnown: len(components) > 0}
}

func typesOf(components []project.Component) []string {
	types := make([]string, 0, len(components))
	for _, component := range components {
		types = append(types, component.Type)
	}
	return types
}

func (i Insight) kindsOf(about subject) kinds {
	switch about.place.Role {
	case RoleOfAMessagePosted:
		return i.kindsPostedTo(about)
	case RoleOfAMessageHandled, RoleOfAField:
		return i.kindsThatSendTo(about)
	case RoleOfAProperty:
		return i.kindsHolding(about)
	case RoleOfNothing, RoleOfAnAddress, RoleOfANode, RoleOfAnAnimation, RoleOfAnAnimationOfANode, RoleOfAnAction, RoleOfAGroup:
	}
	return kinds{}
}

type fieldOfAMessage struct {
	name          string
	message       string
	described     docs.Value
	isOfTheEngine bool
	setIn         []project.Location
}

func (i Insight) fieldsAt(about subject) []fieldOfAMessage {
	var fields []fieldOfAMessage
	for _, message := range about.script.MessagesHandledAt(about.place.Start) {
		names, isSentByTheEngine := i.Reference.FieldsOfWhatTheEngineSends(message)
		if isSentByTheEngine {
			var described []docs.Value
			if found := i.documentedMessages(message, i.kindsThatSendTo(about)); len(found) > 0 {
				described = found[0].fields
			}
			for _, name := range names {
				field := fieldOfAMessage{name: name, message: message, isOfTheEngine: true, described: docs.Value{Name: name}}
				if index := slices.IndexFunc(described, func(value docs.Value) bool { return value.Name == name }); index >= 0 {
					field.described = described[index]
				}
				fields = append(fields, field)
			}
		}
		setIn := map[string][]project.Location{}
		var inOrder []string
		for _, scriptPath := range i.Project.ScriptPaths() {
			for _, post := range i.Project.Scripts[scriptPath].Posts {
				if !post.IDIsKnown || post.ID.Name != message {
					continue
				}
				for _, name := range post.Fields {
					if _, isSeen := setIn[name]; !isSeen {
						inOrder = append(inOrder, name)
					}
					setIn[name] = append(setIn[name], post.ID.Location)
				}
			}
		}
		for _, name := range inOrder {
			if index := slices.IndexFunc(fields, func(field fieldOfAMessage) bool { return field.name == name && field.message == message }); index >= 0 {
				fields[index].setIn = setIn[name]
				continue
			}
			fields = append(fields, fieldOfAMessage{name: name, message: message, setIn: setIn[name]})
		}
	}
	return fields
}

func (f fieldOfAMessage) detail() string {
	if f.isOfTheEngine && len(f.described.Types) > 0 {
		return strings.Join(f.described.Types, typesOfAField) + " · field of " + f.message
	}
	return "field of " + f.message
}

func (f fieldOfAMessage) explained() string {
	lines := []string{fmt.Sprintf("Field `%s` of `%s`.", f.name, f.message)}
	if f.isOfTheEngine {
		lines = []string{fmt.Sprintf("Field of the engine message `%s`:", f.message), "", describedField(f.described)}
	}
	if len(f.setIn) > 0 {
		lines = append(lines, "", "Set where it is posted from:", "")
		for _, location := range f.setIn {
			lines = append(lines, fmt.Sprintf("- `%s` line %d", location.File, location.Position.Line))
		}
	}
	return strings.Join(lines, "\n")
}

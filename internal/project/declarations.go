package project

import (
	"slices"

	"github.com/Lanterna-Games/defold-tools/internal/lua"
	"github.com/Lanterna-Games/defold-tools/internal/textproto"
)

const (
	DeclaredNode      = "node"
	DeclaredAnimation = "animation"
	DeclaredAction    = "action"
	DeclaredGroup     = "group"
)

type declaration struct {
	kind  string
	owner string
	name  string
}

func (p *Project) declare(kind, owner, name string, location Location) {
	key := declaration{kind: kind, owner: owner, name: name}
	if _, isDeclared := p.declared[key]; !isDeclared && name != "" {
		p.declared[key] = location
	}
}

func (p *Project) mention(kind, owner, name string, location Location) {
	if name != "" {
		key := declaration{kind: kind, owner: owner, name: name}
		p.mentioned[key] = append(p.mentioned[key], location)
	}
}

func (p *Project) mentionGroup(filePath string, written *textproto.Scalar) {
	p.mention(DeclaredGroup, "", written.Text, Location{File: filePath, Position: written.TextPosition()})
}

func (p *Project) WhereWritten(kind, owner, name string) []Location {
	return slices.Clone(p.mentioned[declaration{kind: kind, owner: owner, name: name}])
}

func (p *Project) NamesWritten(kind, owner string) []string {
	var names []string
	for key := range p.mentioned {
		if key.kind == kind && key.owner == owner {
			names = append(names, key.name)
		}
	}
	slices.Sort(names)
	return names
}

func (p *Project) PropertyValuesWritten(text string) []Location {
	var locations []Location
	listed := func(overrides []PropertyOverride) {
		for _, override := range overrides {
			if override.Value == text {
				locations = append(locations, override.Location)
			}
		}
	}
	for _, collectionPath := range sortedKeys(p.Collections) {
		collection := p.Collections[collectionPath]
		for _, instance := range collection.Instances {
			listed(instance.PropertyOverrides)
		}
		for _, subCollection := range collection.SubCollections {
			for _, instanceID := range sortedKeys(subCollection.PropertyOverrides) {
				listed(subCollection.PropertyOverrides[instanceID])
			}
		}
	}
	return locations
}

func (p *Project) WhereDeclared(kind, owner, name string) (location Location, isDeclared bool) {
	location, isDeclared = p.declared[declaration{kind: kind, owner: owner, name: name}]
	return location, isDeclared
}

func (p *Project) WhereTheNodeIs(scene *GUIScene, id string) (location Location, isThere bool) {
	if location, isThere = p.WhereDeclared(DeclaredNode, scene.Path, id); isThere {
		return location, true
	}
	for _, template := range scene.Templates {
		inside, isUnderIt := cutPrefix(id, template.NodeID+separatorOfATemplateID)
		templateScene, isLoaded := p.GUIScenes[template.Scene]
		if !isUnderIt || !isLoaded || templateScene == scene {
			continue
		}
		if location, isThere = p.WhereTheNodeIs(templateScene, inside); isThere {
			return location, true
		}
	}
	return Location{}, false
}

func cutPrefix(text, prefix string) (rest string, hasIt bool) {
	if len(text) >= len(prefix) && text[:len(prefix)] == prefix {
		return text[len(prefix):], true
	}
	return text, false
}

func (p *Project) ReadScript(filePath string, content []byte) *Script {
	script := scriptFrom(filePath, "/"+filePath, content)
	script.readWithTheModulesOf(p)
	return script
}

func (s *Script) AddressKeptIn(constant string) (address Address, isWritten bool) {
	if _, isConstant := s.constants[constant]; !isConstant {
		return Address{}, false
	}
	return s.AddressWrittenIn(&lua.Identifier{Name: constant})
}

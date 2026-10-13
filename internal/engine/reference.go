package engine

import (
	"slices"
	"strings"
)

const (
	GameObject                 = "go"
	propertyParameter          = "property"
	propertyOfAMaterial        = "material"
	suffixOfANumberedProperty  = "N"
	digitsOfANumberedProperty  = "0123456789"
	elementsOfAVectorProperty  = "xyzw"
	separatorOfAVectorProperty = "."
)

type Reference struct {
	Defold             string                       `json:"defold"`
	SHA1               string                       `json:"sha1"`
	Functions          map[string][]Parameter       `json:"functions"`
	Messages           []string                     `json:"messages"`
	Properties         map[string][]string          `json:"properties"`
	MaterialProperties []string                     `json:"material_properties"`
	InputBindings      map[string][]string          `json:"input_bindings"`
	MessageFields      map[string][]string          `json:"message_fields"`
	Filled             []string                     `json:"filled,omitempty"`
	Settings           map[string]SettingsSection   `json:"settings,omitempty"`
	SettingsElsewhere  map[string][]string          `json:"settings_elsewhere,omitempty"`
	SettingsRemoved    map[string]map[string]string `json:"settings_removed,omitempty"`

	propertiesTheProjectVouchesFor []string
}

func (r *Reference) Knowing(messages, properties []string) *Reference {
	known := *r
	known.Messages = append(slices.Clone(r.Messages), messages...)
	known.propertiesTheProjectVouchesFor = append(slices.Clone(r.propertiesTheProjectVouchesFor), properties...)
	return &known
}

type Parameter struct {
	Name  string   `json:"name"`
	Types []string `json:"types"`
}

var documentedAsAddressesButAreNot = map[string][]int{
	"model.get_mesh_enabled": {1},
	"model.set_mesh_enabled": {1},
}

func (r *Reference) URLParameterIndexes(function string) []int {
	var indexes []int
	for index, parameter := range r.Functions[function] {
		if parameter.AcceptsOnlyAddresses() && !slices.Contains(documentedAsAddressesButAreNot[function], index) {
			indexes = append(indexes, index)
		}
	}
	return indexes
}

func (p Parameter) AcceptsOnlyAddresses() bool {
	if !slices.Contains(p.Types, "url") || !slices.Contains(p.Types, "string") {
		return false
	}
	for _, typeName := range p.Types {
		if !isAddressType(typeName) {
			return false
		}
	}
	return true
}

func isAddressType(typeName string) bool {
	switch typeName {
	case "string", "hash", "url", "nil":
		return true
	default:
		return strings.Contains(typeName, "url")
	}
}

func (r *Reference) HasMessage(name string) bool {
	return slices.Contains(r.Messages, name)
}

type leftOut struct {
	since Version
	names []string
}

var propertiesTheReferenceLeavesOut = map[string]leftOut{
	"mesh":  {since: OldestSupported, names: []string{"vertices", "material"}},
	"model": {since: OldestSupported, names: []string{"material"}},
}

var propertiesOfWhateverIsDrawnWithAMaterial = leftOut{since: OldestSupported, names: []string{"textureN"}}

func (r *Reference) isAtLeast(version Version) bool {
	own, isAVersion := ParseVersion(r.Defold)
	return !isAVersion || !own.IsBefore(version)
}

func (r *Reference) has(left leftOut) bool {
	return len(left.names) > 0 && r.isAtLeast(left.since)
}

var componentTypeOfNamespace = map[string]string{"physics": "collisionobject"}

func ComponentTypeDocumentedIn(namespace string) string {
	if componentType, isRenamed := componentTypeOfNamespace[namespace]; isRenamed {
		return componentType
	}
	return namespace
}

func (r *Reference) PropertiesOf(componentType string) (names []string, isDocumented bool) {
	documented, isDocumented := r.Properties[componentType]
	names = slices.Clone(documented)
	left := propertiesTheReferenceLeavesOut[componentType]
	isKnownAnyway := r.has(left)
	if isKnownAnyway {
		names = append(names, left.names...)
	}
	if slices.Contains(names, propertyOfAMaterial) && r.has(propertiesOfWhateverIsDrawnWithAMaterial) {
		names = append(names, propertiesOfWhateverIsDrawnWithAMaterial.names...)
	}
	names = append(names, r.propertiesTheProjectVouchesFor...)
	return names, isDocumented || isKnownAnyway
}

func (r *Reference) IsDrawnWithAMaterial(componentType string) bool {
	names, _ := r.PropertiesOf(componentType)
	return slices.Contains(names, propertyOfAMaterial)
}

func (r *Reference) EveryProperty() []string {
	var names []string
	for componentType := range r.Properties {
		names = append(names, r.Properties[componentType]...)
	}
	for _, left := range propertiesTheReferenceLeavesOut {
		if r.has(left) {
			names = append(names, left.names...)
		}
	}
	if r.has(propertiesOfWhateverIsDrawnWithAMaterial) {
		names = append(names, propertiesOfWhateverIsDrawnWithAMaterial.names...)
	}
	names = append(names, r.MaterialProperties...)
	names = append(names, r.propertiesTheProjectVouchesFor...)
	slices.Sort(names)
	return slices.Compact(names)
}

func NamesTheProperty(known []string, name string) bool {
	for _, candidate := range known {
		prefix, isNumbered := strings.CutSuffix(candidate, suffixOfANumberedProperty)
		if !isNumbered {
			if candidate == name {
				return true
			}
			continue
		}
		number, hasThePrefix := strings.CutPrefix(name, prefix)
		if hasThePrefix && number != "" && strings.Trim(number, digitsOfANumberedProperty) == "" {
			return true
		}
	}
	return false
}

func PropertyWithoutItsElement(name string) string {
	whole, element, hasElement := strings.Cut(name, separatorOfAVectorProperty)
	if hasElement && len(element) == 1 && strings.Contains(elementsOfAVectorProperty, element) {
		return whole
	}
	return name
}

func (r *Reference) PropertyParameterIndexes(function string) (address, property int, takesAProperty bool) {
	parameters := r.Functions[function]
	if len(parameters) == 0 || !parameters[0].AcceptsOnlyAddresses() {
		return 0, 0, false
	}
	for index, parameter := range parameters {
		if parameter.Name == propertyParameter {
			return 0, index, true
		}
	}
	return 0, 0, false
}

const (
	namespaceOfCollisions = "physics."
	oneGroupParameter     = "group"
	manyGroupsParameter   = "groups"
)

func (r *Reference) CollisionGroupParameterIndexes(function string) (ofOneGroup, ofManyGroups []int) {
	if !strings.HasPrefix(function, namespaceOfCollisions) {
		return nil, nil
	}
	for index, parameter := range r.Functions[function] {
		switch parameter.Name {
		case oneGroupParameter:
			ofOneGroup = append(ofOneGroup, index)
		case manyGroupsParameter:
			ofManyGroups = append(ofManyGroups, index)
		}
	}
	return ofOneGroup, ofManyGroups
}

var messagesOnlyTheEngineSends = []string{
	"collision_response", "contact_point_response", "trigger_response",
	"collision_event", "contact_point_event", "trigger_event",
	"ray_cast_response", "ray_cast_missed",
	"animation_done", "model_animation_done", "sound_done", "sound_stopped",
	"proxy_loaded", "proxy_unloaded", "proxy_loading", "proxy_ready", "proxy_error",
	"layout_changed", "window_resized",
}

var fieldsTheReferenceLeavesOut = map[string]leftOut{
	"collision_response":     {since: OldestSupported, names: []string{"group"}},
	"contact_point_response": {since: OldestSupported, names: []string{"group"}},
	"trigger_response":       {since: OldestSupported, names: []string{"group"}},
}

func (r *Reference) FieldsOfWhatTheEngineSends(message string) (fields []string, isSentByTheEngine bool) {
	if !slices.Contains(messagesOnlyTheEngineSends, message) || !r.HasMessage(message) {
		return nil, false
	}
	fields = slices.Clone(r.MessageFields[message])
	if left := fieldsTheReferenceLeavesOut[message]; r.has(left) {
		fields = append(fields, left.names...)
	}
	return fields, true
}

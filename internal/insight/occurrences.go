package insight

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/Lanterna-Games/defold-tools/internal/engine"
	"github.com/Lanterna-Games/defold-tools/internal/lua"
	"github.com/Lanterna-Games/defold-tools/internal/project"
)

const (
	charactersNoNameCanHold = "\"'\\\n\r\t"
	startOfAComponent       = "#"
	charactersOfAnAddress   = "#/:"
)

type kindOfThing int

const (
	noThing kindOfThing = iota
	aMessage
	anAction
	aGroup
	aNode
	aComponent
	aGameObject
	anAnimation
	aProperty
	aCollection
)

var kindsOfARole = map[Role][]kindOfThing{
	RoleOfAMessagePosted:     {aMessage},
	RoleOfAMessageHandled:    {aMessage},
	RoleOfAnAction:           {anAction},
	RoleOfAGroup:             {aGroup},
	RoleOfANode:              {aNode},
	RoleOfAnAddress:          {aComponent, aGameObject, aCollection},
	RoleOfAnAnimation:        {anAnimation},
	RoleOfAnAnimationOfANode: {anAnimation},
	RoleOfAProperty:          {aProperty},
}

var wordForAKind = map[kindOfThing]string{
	aMessage:    "message",
	anAction:    "action",
	aGroup:      "collision group",
	aNode:       "GUI node",
	aComponent:  "component",
	aGameObject: "game object",
	anAnimation: "animation",
	aProperty:   "script property",
	aCollection: "collection",
}

var kindInTheDataFiles = map[kindOfThing]string{
	anAction: project.DeclaredAction,
	aGroup:   project.DeclaredGroup,
}

type thing struct {
	kind       kindOfThing
	name       string
	file       string
	gameObject *project.GameObject
}

func (t thing) described() string {
	switch t.kind {
	case aNode:
		return fmt.Sprintf("the node %q of %s", t.name, t.file)
	case aComponent:
		return fmt.Sprintf("the component %q of %s", t.name, t.gameObject.Path)
	case aGameObject:
		return fmt.Sprintf("the game object %q of %s", t.name, t.file)
	case anAnimation:
		return fmt.Sprintf("the animation %q of %s", t.name, t.file)
	case aProperty:
		return fmt.Sprintf("the property %q of %s", t.name, t.file)
	case aCollection:
		return fmt.Sprintf("the collection %q placed in %s", t.name, t.file)
	case noThing, aMessage, anAction, aGroup:
	}
	return fmt.Sprintf("the %s %q", wordForAKind[t.kind], t.name)
}

type mention struct {
	thing
	where project.Location
}

func mentionOf(what thing, where project.Location) mention {
	var written mention
	written.thing, written.where = what, where
	return written
}

func (m mention) covers(file string, offset int) bool {
	start := m.where.Position.Offset
	return m.where.File == file && start <= offset && offset <= start+len(m.name)
}

func (m mention) overlaps(other mention) bool {
	return m.where.File == other.where.File &&
		m.where.Position.Offset < other.where.Position.Offset+len(other.name) &&
		other.where.Position.Offset < m.where.Position.Offset+len(m.name)
}

type dispute struct {
	one          mention
	other        mention
	isInOnePlace bool
}

type Occurrence struct {
	Place  Place
	Where  project.Location
	script *project.Script
	part   partOfAnAddress
	socket string
	path   string

	declaresAProperty bool
}

type partOfAnAddress int

const (
	theWholeAddress partOfAnAddress = iota
	thePathAlone
	theFragmentAlone
)

const (
	pathOfABuiltAddress     = 1
	fragmentOfABuiltAddress = 2
	wordForNothing          = "nil"
	functionThatDeclares    = "go.property"
	nameOfTheInstance       = "self"
)

var functionsThatSpawn = map[string]bool{"factory.create": true, "collectionfactory.create": true}

var nameOfAField = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func partWrittenIn(argument []lua.Token) (text string, isWritten bool) {
	if len(argument) != 1 {
		return "", false
	}
	switch {
	case argument[0].Kind == lua.TokenString:
		return argument[0].Text, true
	case argument[0].Is(lua.TokenKeyword, wordForNothing):
		return "", true
	}
	return "", false
}

func (s surroundings) partOfABuiltAddress() (part partOfAnAddress, socket, path string, isOne bool) {
	if !s.isInACall || s.isInATable || s.callee != urlFunction {
		return theWholeAddress, "", "", false
	}
	socket, socketIsWritten := partWrittenIn(s.firstArgument)
	switch {
	case s.argument == pathOfABuiltAddress && socketIsWritten:
		return thePathAlone, socket, "", true
	case s.argument == fragmentOfABuiltAddress && socketIsWritten:
		path, pathIsWritten := partWrittenIn(s.second)
		return theFragmentAlone, socket, path, pathIsWritten
	}
	return theWholeAddress, "", "", false
}

type Change struct {
	Where project.Location
	Old   string
	New   string
}

func constantStartingAt(tokens []lua.Token, index int) (constant lua.Expression, isOne bool) {
	if index > 0 && (isSymbol(tokens[index-1], ".") || isSymbol(tokens[index-1], ":")) {
		return nil, false
	}
	first := tokens[index]
	constant = &lua.Identifier{Start: first.Start, End: first.End, Name: first.Text}
	next := index + 1
	for next+1 < len(tokens) && isSymbol(tokens[next], ".") && tokens[next+1].Kind == lua.TokenName {
		field := tokens[next+1]
		constant = &lua.IndexExpression{
			Start: first.Start, End: field.End,
			Object: constant,
			Index:  &lua.StringLiteral{Start: field.Start, End: field.End, Value: field.Text},
		}
		next += 2
	}
	if next >= len(tokens) {
		return constant, true
	}
	after := tokens[next]
	isCalledIndexedOrAssigned := after.Kind == lua.TokenString || isSymbol(after, "(") || isSymbol(after, "[") || isSymbol(after, ":") || isSymbol(after, "=")
	return constant, !isCalledIndexedOrAssigned
}

func (i Insight) occurrencesIn(script *project.Script) []Occurrence {
	tokens := lua.Tokens(script.Source)
	var occurrences []Occurrence
	for index, token := range tokens {
		var written lua.Expression
		switch token.Kind {
		case lua.TokenString:
			written = &lua.StringLiteral{Start: token.Start, End: token.End, Value: token.Text}
		case lua.TokenName:
			constant, isOne := constantStartingAt(tokens, index)
			if !isOne {
				continue
			}
			written = constant
		case lua.TokenEndOfFile, lua.TokenKeyword, lua.TokenNumber, lua.TokenSymbol:
			continue
		}
		handler, parameters := handlerAround(script.Chunk, token.Start.Offset)
		around := surroundingsOf(tokens[:index])
		place := Place{Role: roleOf(around, handler, parameters, i.Reference)}
		part, socket, path, isAPart := around.partOfABuiltAddress()
		if isAPart {
			place.Role = RoleOfAnAddress
		}
		declaresAProperty := around.isInACall && !around.isInATable && around.callee == functionThatDeclares && around.argument == 0
		if declaresAProperty {
			place.Role = RoleOfAProperty
		}
		if place.Role == RoleOfNothing {
			continue
		}
		text, isWritten := script.TextWrittenIn(written)
		if !isWritten {
			continue
		}
		place.Text = text.Text
		around.targetInto(&place)
		if address, isKept := script.AddressKeptIn(place.TargetConstant); place.TargetConstant != "" && isKept {
			place.Target, place.TargetIsWritten = address.Text, true
		}
		occurrences = append(occurrences, Occurrence{
			Place: place, Where: text.Location, script: script,
			part: part, socket: socket, path: path, declaresAProperty: declaresAProperty,
		})
	}
	return occurrences
}

func movedBy(where project.Location, characters int) project.Location {
	where.Position.Offset += characters
	where.Position.Column += characters
	return where
}

func (i Insight) nodesNamedIn(scene *project.GUIScene, text string, where project.Location, ancestors map[*project.GUIScene]bool) []mention {
	if ancestors[scene] {
		return nil
	}
	ancestors[scene] = true
	defer delete(ancestors, scene)
	for _, template := range scene.Templates {
		inside, isUnderIt := strings.CutPrefix(text, template.NodeID+"/")
		if !isUnderIt || template.NodeID == "" {
			continue
		}
		mentions := []mention{mentionOf(thing{kind: aNode, name: template.NodeID, file: scene.Path}, where)}
		if templateScene, isLoaded := i.Project.GUIScenes[template.Scene]; isLoaded {
			mentions = append(mentions, i.nodesNamedIn(templateScene, inside, movedBy(where, len(template.NodeID)+1), ancestors)...)
		}
		return mentions
	}
	if !slices.Contains(scene.NodeIDs, text) {
		return nil
	}
	return []mention{mentionOf(thing{kind: aNode, name: text, file: scene.Path}, where)}
}

func componentNamedBy(target *project.GameObject, text string, where project.Location) []mention {
	_, fragment, hasAFragment := strings.Cut(text, startOfAComponent)
	if !hasAFragment || fragment == "" || target == nil {
		return nil
	}
	if _, exists := target.Component(fragment); !exists {
		return nil
	}
	return []mention{mentionOf(thing{kind: aComponent, name: fragment, gameObject: target}, movedBy(where, len(text)-len(fragment)))}
}

type everything struct {
	mentions []mention
	isListed map[mention]bool
	disputes []dispute
	isNamed  map[project.Location]bool
	uses     map[*project.Script][]Occurrence
}

func (e *everything) add(contexts [][]mention) {
	for index, context := range contexts {
		for position, named := range context {
			for _, beside := range context[position+1:] {
				if beside.thing != named.thing && beside.overlaps(named) {
					e.disputes = append(e.disputes, dispute{one: named, other: beside, isInOnePlace: true})
				}
			}
			if !e.isListed[named] {
				e.isListed[named] = true
				e.mentions = append(e.mentions, named)
			}
			for _, elsewhere := range contexts[index+1:] {
				for _, other := range elsewhere {
					if other.thing != named.thing && other.overlaps(named) {
						e.disputes = append(e.disputes, dispute{one: named, other: other})
					}
				}
			}
		}
	}
}

func (i Insight) ownersByScript() map[string][]project.Owner {
	running := map[string][]project.Owner{}
	for _, owner := range i.Project.Owners() {
		for _, scriptPath := range i.Project.ScriptPathsOf(owner.GameObject) {
			running[scriptPath] = append(running[scriptPath], owner)
		}
	}
	return running
}

type placed struct {
	collection *project.Collection
	instance   project.Instance
}

type reader struct {
	Insight
	owners   map[string][]project.Owner
	anywhere []project.Owner
	byPath   map[*project.Namespace]map[string]placed
	holders  map[*project.Namespace]map[string]placedCollection
}

type placedCollection struct {
	holder *project.Collection
	id     string
}

func (i *reader) ownersOf(script *project.Script) []project.Owner {
	if running := i.owners[script.Path]; len(running) > 0 {
		return running
	}
	return i.anywhere
}

func canBeFollowedFrom(owner project.Owner, url project.URL) bool {
	isAStandIn := owner.GameObject == nil
	return !isAStandIn || url.IsAbsolute()
}

func (i Insight) reader() *reader {
	reading := &reader{
		Insight: i, owners: i.ownersByScript(),
		byPath: map[*project.Namespace]map[string]placed{}, holders: map[*project.Namespace]map[string]placedCollection{},
	}
	for _, mount := range i.Project.Mounts() {
		if reading.byPath[mount.Namespace] == nil {
			reading.byPath[mount.Namespace] = map[string]placed{}
			reading.holders[mount.Namespace] = map[string]placedCollection{}
		}
		for _, nested := range mount.Collection.SubCollections {
			reading.holders[mount.Namespace][mount.Prefix+"/"+nested.ID] = placedCollection{holder: mount.Collection, id: nested.ID}
		}
		for _, instance := range mount.Collection.Instances {
			reading.byPath[mount.Namespace][mount.Prefix+"/"+instance.ID] = placed{collection: mount.Collection, instance: instance}
		}
		if mount.Collection == mount.Namespace.Root {
			reading.anywhere = append(reading.anywhere, project.Owner{Namespace: mount.Namespace})
		}
	}
	return reading
}

func (i *reader) gameObjectNamedBy(owner project.Owner, url project.URL, text string, where project.Location) []mention {
	if url.PointsAtItsOwnGameObject() {
		return nil
	}
	namespaces := i.Project.NamespacesAddressedBy(owner, url)
	if len(namespaces) != 1 {
		return nil
	}
	at, isPlaced := i.byPath[namespaces[0]][owner.PathAddressedBy(url)]
	pathStart := strings.Index(text, url.Path)
	lastSegment := strings.LastIndex(url.Path, "/") + 1
	if !isPlaced || pathStart < 0 || url.Path[lastSegment:] != at.instance.ID {
		return nil
	}
	return []mention{mentionOf(thing{kind: aGameObject, name: at.instance.ID, file: at.collection.Path}, movedBy(where, pathStart+lastSegment))}
}

func (i *reader) collectionsNamedBy(owner project.Owner, url project.URL, text string, where project.Location) []mention {
	if url.PointsAtItsOwnGameObject() {
		return nil
	}
	namespaces := i.Project.NamespacesAddressedBy(owner, url)
	if len(namespaces) != 1 {
		return nil
	}
	addressed := owner.PathAddressedBy(url)
	pathStart := strings.Index(text, url.Path)
	if pathStart < 0 || !strings.HasSuffix(addressed, url.Path) {
		return nil
	}
	var named []mention
	before := len(addressed) - len(url.Path)
	for start := 0; start < len(url.Path); {
		length := strings.Index(url.Path[start:], "/")
		if length < 0 {
			break
		}
		through, isOne := i.holders[namespaces[0]][addressed[:before+start+length]]
		if isOne && url.Path[start:start+length] == through.id {
			named = append(named, mentionOf(thing{kind: aCollection, name: through.id, file: through.holder.Path}, movedBy(where, pathStart+start)))
		}
		start += length + 1
	}
	return named
}

func (i *reader) animationsNamedIn(textures []string, name string, where project.Location) []mention {
	var mentions []mention
	for _, texture := range textures {
		if slices.Contains(i.Project.Animations[texture], name) {
			named := mentionOf(thing{kind: anAnimation, name: name, file: texture}, where)
			if !slices.Contains(mentions, named) {
				mentions = append(mentions, named)
			}
		}
	}
	return mentions
}

func (i *reader) whatAnAnimationNames(occurrence Occurrence) [][]mention {
	name, where := occurrence.Place.Text, occurrence.Where
	if occurrence.Place.Role == RoleOfAnAnimationOfANode {
		var contexts [][]mention
		for _, scene := range i.scenesRunning(occurrence.script) {
			contexts = append(contexts, theOnlyOneOf(i.animationsNamedIn(sortedValues(scene.Textures), name, where)))
		}
		return contexts
	}
	var contexts [][]mention
	if occurrence.Place.TargetIsWritten {
		url := project.ParseURL(occurrence.Place.Target)
		fragment := url.Fragment
		url.Fragment, url.HasFragment = "", false
		for _, owner := range i.ownersOf(occurrence.script) {
			target, isKnown := i.Project.TargetOf(owner, url)
			if !isKnown || !canBeFollowedFrom(owner, url) {
				continue
			}
			var textures []string
			for _, component := range target.GameObject.Components {
				if sprite, isRead := i.Project.SpriteOf(component); isRead && (fragment == "" || fragment == component.ID) {
					textures = append(textures, sprite.Textures...)
				}
			}
			contexts = append(contexts, i.animationsNamedIn(textures, name, where))
		}
	}
	if len(contexts) == 0 {
		contexts = append(contexts, theOnlyOneOf(i.animationsNamedIn(sortedKeys(i.Project.Animations), name, where)))
	}
	return contexts
}

func theOnlyOneOf(candidates []mention) []mention {
	if len(candidates) != 1 {
		return nil
	}
	return candidates
}

func sortedValues(values map[string]string) []string {
	sorted := make([]string, 0, len(values))
	for _, key := range sortedKeys(values) {
		sorted = append(sorted, values[key])
	}
	return sorted
}

func (i *reader) propertyOf(component project.Component, name string, where project.Location) []mention {
	script, isLoaded := i.Project.Scripts[component.Resource]
	whole := engine.PropertyWithoutItsElement(name)
	if !component.IsScript() || !isLoaded || !script.DeclaresProperty(whole) {
		return nil
	}
	return []mention{mentionOf(thing{kind: aProperty, name: whole, file: script.Path}, where)}
}

func (i *reader) whatAPropertyNames(occurrence Occurrence) [][]mention {
	name, where, script := occurrence.Place.Text, occurrence.Where, occurrence.script
	ofTheScriptItself := func() [][]mention {
		if whole := engine.PropertyWithoutItsElement(name); script.DeclaresProperty(whole) {
			return [][]mention{{mentionOf(thing{kind: aProperty, name: whole, file: script.Path}, where)}}
		}
		return nil
	}
	ofTheOnlyScriptThatHasIt := func() [][]mention {
		whole := engine.PropertyWithoutItsElement(name)
		var declaring []*project.Script
		for _, scriptPath := range i.Project.ScriptPaths() {
			if candidate := i.Project.Scripts[scriptPath]; candidate.DeclaresProperty(whole) {
				declaring = append(declaring, candidate)
			}
		}
		if len(declaring) != 1 || engine.NamesTheProperty(i.Reference.EveryProperty(), whole) {
			return nil
		}
		return [][]mention{{mentionOf(thing{kind: aProperty, name: whole, file: declaring[0].Path}, where)}}
	}
	if occurrence.declaresAProperty {
		return ofTheScriptItself()
	}
	if !occurrence.Place.TargetIsWritten {
		return ofTheOnlyScriptThatHasIt()
	}
	url := project.ParseURL(occurrence.Place.Target)
	if url.Socket == "" && url.Path == "" && url.HasFragment && url.Fragment == "" {
		return ofTheScriptItself()
	}
	if !url.HasFragment || url.Fragment == "" {
		return nil
	}
	fragment := url.Fragment
	url.Fragment, url.HasFragment = "", false
	var contexts [][]mention
	for _, owner := range i.ownersOf(script) {
		target, isKnown := i.Project.TargetOf(owner, url)
		if !isKnown || !canBeFollowedFrom(owner, url) {
			continue
		}
		if component, exists := target.GameObject.Component(fragment); exists {
			contexts = append(contexts, i.propertyOf(component, name, where))
		}
	}
	if len(contexts) == 0 {
		return ofTheOnlyScriptThatHasIt()
	}
	return contexts
}

func whereTheFieldIs(script *project.Script, field *lua.StringLiteral) project.Location {
	where := project.Location{File: script.File, Position: field.Start}
	if start := field.Start.Offset; start < len(script.Source) && (script.Source[start] == '"' || script.Source[start] == '\'') {
		return movedBy(where, 1)
	}
	return where
}

func fieldsOfTheInstance(script *project.Script) []*lua.StringLiteral {
	var fields []*lua.StringLiteral
	lua.Inspect(script.Chunk, func(node lua.Node) bool {
		index, isIndex := node.(*lua.IndexExpression)
		if !isIndex {
			return true
		}
		object, isAName := index.Object.(*lua.Identifier)
		field, isNamed := index.Index.(*lua.StringLiteral)
		if isAName && isNamed && object.Name == nameOfTheInstance {
			fields = append(fields, field)
		}
		return true
	})
	return fields
}

func (i *reader) whatTheInstanceHolds(script *project.Script, all *everything) {
	if len(script.Properties) == 0 {
		return
	}
	for _, field := range fieldsOfTheInstance(script) {
		if script.DeclaresProperty(field.Value) {
			all.add([][]mention{{mentionOf(thing{kind: aProperty, name: field.Value, file: script.Path}, whereTheFieldIs(script, field))}})
		}
	}
}

func (i *reader) whatAPartOfAnAddressNames(owners []project.Owner, occurrence Occurrence) [][]mention {
	contexts := make([][]mention, 0, len(owners))
	text, where := occurrence.Place.Text, occurrence.Where
	for _, owner := range owners {
		if alone := (project.URL{Socket: occurrence.socket, Path: text}); occurrence.part == thePathAlone {
			if canBeFollowedFrom(owner, alone) {
				contexts = append(contexts, append(i.gameObjectNamedBy(owner, alone, text, where), i.collectionsNamedBy(owner, alone, text, where)...))
			}
			continue
		}
		before := project.URL{Socket: occurrence.socket, Path: occurrence.path}
		target, isKnown := i.Project.TargetOf(owner, before)
		if !isKnown || text == "" || !canBeFollowedFrom(owner, before) {
			continue
		}
		if _, exists := target.GameObject.Component(text); exists {
			contexts = append(contexts, []mention{mentionOf(thing{kind: aComponent, name: text, gameObject: target.GameObject}, where)})
		}
	}
	return contexts
}

func (i *reader) whatAnAddressNames(owners []project.Owner, text string, where project.Location) [][]mention {
	contexts := make([][]mention, 0, len(owners))
	url := project.ParseURL(text)
	url.Fragment, url.HasFragment = "", false
	for _, owner := range owners {
		if !canBeFollowedFrom(owner, url) {
			continue
		}
		named := append(i.gameObjectNamedBy(owner, url, text, where), i.collectionsNamedBy(owner, url, text, where)...)
		if target, isKnown := i.Project.TargetOf(owner, url); isKnown {
			named = append(named, componentNamedBy(target.GameObject, text, where)...)
		}
		contexts = append(contexts, named)
	}
	return contexts
}

func (i *reader) whatIsNamedBy(occurrence Occurrence) [][]mention {
	text, where := occurrence.Place.Text, occurrence.Where
	switch occurrence.Place.Role {
	case RoleOfAMessagePosted, RoleOfAMessageHandled, RoleOfAnAction, RoleOfAGroup:
		return [][]mention{{mentionOf(thing{kind: kindsOfARole[occurrence.Place.Role][0], name: text}, where)}}
	case RoleOfANode:
		var contexts [][]mention
		for _, scene := range i.scenesRunning(occurrence.script) {
			contexts = append(contexts, i.nodesNamedIn(scene, text, where, map[*project.GUIScene]bool{}))
		}
		return contexts
	case RoleOfAnAddress:
		if occurrence.part != theWholeAddress {
			return i.whatAPartOfAnAddressNames(i.ownersOf(occurrence.script), occurrence)
		}
		return i.whatAnAddressNames(i.ownersOf(occurrence.script), text, where)
	case RoleOfAnAnimation, RoleOfAnAnimationOfANode:
		return i.whatAnAnimationNames(occurrence)
	case RoleOfAProperty:
		return i.whatAPropertyNames(occurrence)
	case RoleOfNothing, RoleOfAField:
	}
	return nil
}

func (i Insight) ownersOfEachInstance() map[*project.Collection]map[string][]project.Owner {
	placed := map[*project.Collection]map[string][]project.Owner{}
	for _, mount := range i.Project.Mounts() {
		if placed[mount.Collection] == nil {
			placed[mount.Collection] = map[string][]project.Owner{}
		}
		for _, instance := range mount.Collection.Instances {
			placed[mount.Collection][instance.ID] = append(placed[mount.Collection][instance.ID], project.Owner{
				Namespace:    mount.Namespace,
				InstancePath: mount.Prefix + "/" + instance.ID,
				BasePrefix:   mount.Prefix,
				GameObject:   i.Project.GameObjectOf(instance),
			})
		}
	}
	return placed
}

func (i *reader) namedByTheDataFiles(all *everything) {
	for kind, registered := range kindInTheDataFiles {
		for _, name := range i.Project.NamesWritten(registered, "") {
			for _, where := range i.Project.WhereWritten(registered, "", name) {
				all.add([][]mention{{mentionOf(thing{kind: kind, name: name}, where)}})
			}
		}
	}
	for _, scenePath := range sortedKeys(i.Project.GUIScenes) {
		scene := i.Project.GUIScenes[scenePath]
		for _, written := range scene.NodeNames {
			all.add([][]mention{i.nodesNamedIn(scene, written.Name, written.Location, map[*project.GUIScene]bool{})})
		}
		for _, written := range scene.TexturesUsed {
			texture, animation, namesAnAnimation := project.TextureAndAnimation(written.Name)
			if resource, isOfTheScene := scene.Textures[texture]; namesAnAnimation && isOfTheScene {
				all.add([][]mention{i.animationsNamedIn([]string{resource}, animation, movedBy(written.Location, len(texture)+1))})
			}
		}
	}
	for _, texture := range sortedKeys(i.Project.Animations) {
		for _, name := range i.Project.NamesWritten(project.DeclaredAnimation, texture) {
			for _, where := range i.Project.WhereWritten(project.DeclaredAnimation, texture, name) {
				all.add([][]mention{{mentionOf(thing{kind: anAnimation, name: name, file: texture}, where)}})
			}
		}
	}
	for _, sprite := range i.Project.Sprites {
		if sprite.DefaultAnimationWhere.File != "" {
			all.add([][]mention{i.animationsNamedIn(sprite.Textures, sprite.DefaultAnimation.Name, sprite.DefaultAnimationWhere)})
		}
	}
	for _, emitter := range i.Project.EmitterAnimations {
		all.add([][]mention{i.animationsNamedIn([]string{emitter.TileSource}, emitter.Name, emitter.Where)})
	}
	declares := func(gameObject *project.GameObject) {
		if gameObject == nil {
			return
		}
		for _, component := range gameObject.Components {
			if component.ID != "" && component.IDWhere.File != "" {
				all.add([][]mention{{mentionOf(thing{kind: aComponent, name: component.ID, gameObject: gameObject}, component.IDWhere)}})
			}
			for _, override := range component.PropertyOverrides {
				if override.NameWhere.File != "" {
					all.add([][]mention{i.propertyOf(component, override.Property, override.NameWhere)})
				}
			}
		}
	}
	for _, gameObjectPath := range i.Project.GameObjectPaths() {
		declares(i.Project.GameObjects[gameObjectPath])
	}
	placed := i.ownersOfEachInstance()
	overrides := func(gameObject *project.GameObject, owners []project.Owner, written []project.PropertyOverride) {
		for _, override := range written {
			component, exists := gameObject.Component(override.Component)
			if exists && override.ComponentWhere.File != "" {
				all.add([][]mention{{mentionOf(thing{kind: aComponent, name: override.Component, gameObject: gameObject}, override.ComponentWhere)}})
			}
			if exists && override.NameWhere.File != "" {
				all.add([][]mention{i.propertyOf(component, override.Property, override.NameWhere)})
			}
			if override.Type == project.PropertyTypeURL && override.ValueWhere.File != "" {
				all.add(i.whatAnAddressNames(owners, override.Value, override.ValueWhere))
			}
		}
	}
	for _, collectionPath := range sortedKeys(i.Project.Collections) {
		collection := i.Project.Collections[collectionPath]
		for _, instance := range collection.Instances {
			if instance.ID != "" && instance.IDWhere.File != "" {
				all.add([][]mention{{mentionOf(thing{kind: aGameObject, name: instance.ID, file: collection.Path}, instance.IDWhere)}})
			}
			for _, child := range instance.Children {
				if slices.ContainsFunc(collection.Instances, func(sibling project.Instance) bool { return sibling.ID == child.ID }) {
					all.add([][]mention{{mentionOf(thing{kind: aGameObject, name: child.ID, file: collection.Path}, child.Where)}})
				}
			}
			gameObject := i.Project.GameObjectOf(instance)
			if gameObject == nil {
				continue
			}
			if instance.EmbeddedObject != nil {
				declares(gameObject)
			}
			overrides(gameObject, placed[collection][instance.ID], instance.PropertyOverrides)
		}
		for _, subCollection := range collection.SubCollections {
			if subCollection.ID != "" && subCollection.IDWhere.File != "" {
				all.add([][]mention{{mentionOf(thing{kind: aCollection, name: subCollection.ID, file: collection.Path}, subCollection.IDWhere)}})
			}
			nested, isLoaded := i.Project.Collections[subCollection.Collection]
			if !isLoaded {
				continue
			}
			for _, named := range subCollection.InstancesNamed {
				if named.Location.File != "" && slices.ContainsFunc(nested.Instances, func(instance project.Instance) bool { return instance.ID == named.Name }) {
					all.add([][]mention{{mentionOf(thing{kind: aGameObject, name: named.Name, file: nested.Path}, named.Location)}})
				}
			}
			for _, instance := range nested.Instances {
				if gameObject := i.Project.GameObjectOf(instance); gameObject != nil {
					overrides(gameObject, placed[nested][instance.ID], subCollection.PropertyOverrides[instance.ID])
				}
			}
		}
	}
}

func (i Insight) everythingNamed(scripts []*project.Script) *everything {
	all := &everything{isListed: map[mention]bool{}, isNamed: map[project.Location]bool{}, uses: map[*project.Script][]Occurrence{}}
	reading := i.reader()
	for _, script := range scripts {
		all.uses[script] = i.occurrencesIn(script)
		for _, occurrence := range all.uses[script] {
			contexts := reading.whatIsNamedBy(occurrence)
			all.add(contexts)
			for _, context := range contexts {
				if len(context) > 0 {
					all.isNamed[occurrence.Where] = true
				}
			}
		}
		reading.whatTheInstanceHolds(script, all)
	}
	reading.namedByTheDataFiles(all)
	return all
}

func (e *everything) whereItIsWritten(sought thing) []project.Location {
	var locations []project.Location
	for _, named := range e.mentions {
		if named.thing == sought && !slices.Contains(locations, named.where) {
			locations = append(locations, named.where)
		}
	}
	return locations
}

type found struct {
	thing
	start   int
	end     int
	scripts []*project.Script
	all     *everything
}

func (i Insight) scriptsWith(current *project.Script) []*project.Script {
	scripts := make([]*project.Script, 0, len(i.Project.Scripts)+1)
	isAmongThem := false
	for _, scriptPath := range i.Project.ScriptPaths() {
		if scriptPath == current.Path {
			scripts, isAmongThem = append(scripts, current), true
			continue
		}
		scripts = append(scripts, i.Project.Scripts[scriptPath])
	}
	if !isAmongThem {
		scripts = append(scripts, current)
	}
	return scripts
}

func (i Insight) thingUnder(file string, content []byte, offset int) (under found, isOne bool) {
	_, _, isInAString := literalAround(content, offset)
	object, _, _, isAField := fieldAround(content, offset)
	if !isInAString && (!isAField || object != nameOfTheInstance) {
		return found{}, false
	}
	under = found{scripts: i.scriptsWith(i.Project.ReadScript(file, content))}
	under.all = i.everythingNamed(under.scripts)
	for _, named := range under.all.mentions {
		if named.covers(file, offset) {
			under.thing = named.thing
			under.start = named.where.Position.Offset
			under.end = under.start + len(named.name)
			return under, true
		}
	}
	return found{}, false
}

func (i Insight) References(file string, content []byte, offset int) (locations []project.Location, name string) {
	under, isOne := i.thingUnder(file, content, offset)
	if !isOne {
		return nil, ""
	}
	return under.all.whereItIsWritten(under.thing), under.name
}

var ErrNothingToRename = errors.New("there is no name of the project here to rename")

func (i Insight) whyItCannotBeRenamed(under found) error {
	sought := under.thing
	switch {
	case sought.kind == aMessage && i.Reference.HasMessage(sought.name):
		return fmt.Errorf("%q is a message of the engine and cannot be renamed", sought.name)
	case sought.kind == anAction && slices.Contains(i.Reference.InputBindings[i.Project.GameBinding], sought.name):
		return fmt.Errorf("%q is bound in %s, which is the engine's and cannot be changed", sought.name, i.Project.GameBinding)
	case sought.kind == anAction && len(i.Project.WhereWritten(project.DeclaredAction, "", sought.name)) == 0:
		return fmt.Errorf("%q is bound in no input binding of the project, so renaming it here would leave it unbound", sought.name)
	case sought.kind == anAnimation && len(i.Project.WhereWritten(project.DeclaredAnimation, sought.file, sought.name)) == 0:
		return fmt.Errorf("%q is an image of %s and takes its name from the file, so it cannot be renamed here", sought.name, sought.file)
	}
	for _, disputed := range under.all.disputes {
		isAboutIt := disputed.one.thing == sought || disputed.other.thing == sought
		if isAboutIt && disputed.isInOnePlace {
			return fmt.Errorf("%s:%d names %s and %s at once, so renaming one would break the other",
				disputed.one.where.File, disputed.one.where.Position.Line, disputed.one.described(), disputed.other.described())
		}
		if isAboutIt {
			return fmt.Errorf("%s:%d names %s where its script runs for one owner and %s for another, so renaming one would break the other",
				disputed.one.where.File, disputed.one.where.Position.Line, disputed.one.described(), disputed.other.described())
		}
	}
	changed := under.all.whereItIsWritten(sought)
	isChangedWithin := func(occurrence Occurrence) bool {
		return slices.ContainsFunc(changed, func(location project.Location) bool {
			start := occurrence.Where.Position.Offset
			return location.File == occurrence.Where.File && start <= location.Position.Offset && location.Position.Offset < start+max(len(occurrence.Place.Text), 1)
		})
	}
	for _, script := range under.scripts {
		read := map[Role]int{}
		for _, occurrence := range under.all.uses[script] {
			if !slices.Contains(kindsOfARole[occurrence.Place.Role], sought.kind) && isChangedWithin(occurrence) {
				return fmt.Errorf("%q at %s:%d is also used as something other than a %s, and renaming it would change that too",
					sought.name, occurrence.Where.File, occurrence.Where.Position.Line, wordForAKind[sought.kind])
			}
			if occurrence.Place.Text == sought.name {
				read[occurrence.Place.Role]++
			}
		}
		for role, expected := range usesTheChecksRead(script, sought) {
			if read[role] < expected {
				return fmt.Errorf("%s uses %q in %d places and only %d of them could be found to rename, so nothing was renamed",
					script.File, sought.name, expected, read[role])
			}
		}
	}
	return nil
}

func usesTheChecksRead(script *project.Script, sought thing) map[Role]int {
	read := map[Role]int{}
	switch sought.kind {
	case aMessage:
		for _, post := range script.Posts {
			if post.IDIsKnown && post.ID.Name == sought.name {
				read[RoleOfAMessagePosted]++
			}
		}
		for _, handled := range script.Handled {
			if handled.Name == sought.name {
				read[RoleOfAMessageHandled]++
			}
		}
	case anAction:
		for _, compared := range script.ActionsCompared {
			if compared.Name == sought.name {
				read[RoleOfAnAction]++
			}
		}
	case aGroup, aNode, aComponent, aGameObject, anAnimation, aProperty, aCollection, noThing:
	}
	return read
}

func (i Insight) Renamable(file string, content []byte, offset int) (name string, start, end int, err error) {
	under, isOne := i.thingUnder(file, content, offset)
	if !isOne {
		return "", 0, 0, ErrNothingToRename
	}
	if err := i.whyItCannotBeRenamed(under); err != nil {
		return "", 0, 0, err
	}
	return under.name, under.start, under.end, nil
}

func (i Insight) isTaken(under found, newName string) bool {
	wanted := under.thing
	wanted.name = newName
	switch wanted.kind {
	case aMessage:
		if i.Reference.HasMessage(newName) {
			return true
		}
	case aNode:
		if scene, isLoaded := i.Project.GUIScenes[wanted.file]; isLoaded && i.Project.NodesOf(scene).Has(newName) {
			return true
		}
	case aComponent:
		if _, exists := wanted.gameObject.Component(newName); exists {
			return true
		}
	case aGameObject:
		if collection, isLoaded := i.Project.Collections[wanted.file]; isLoaded && slices.ContainsFunc(collection.SubCollections, func(nested project.SubCollection) bool { return nested.ID == newName }) {
			return true
		}
	case aCollection:
		if collection, isLoaded := i.Project.Collections[wanted.file]; isLoaded && slices.ContainsFunc(collection.Instances, func(instance project.Instance) bool { return instance.ID == newName }) {
			return true
		}
	case anAnimation:
		if slices.Contains(i.Project.Animations[wanted.file], newName) {
			return true
		}
	case aProperty:
		script := i.Project.Scripts[wanted.file]
		for _, current := range under.scripts {
			if current.Path == wanted.file {
				script = current
			}
		}
		if script != nil && slices.ContainsFunc(fieldsOfTheInstance(script), func(field *lua.StringLiteral) bool { return field.Value == newName }) {
			return true
		}
	case anAction, aGroup, noThing:
	}
	return len(under.all.whereItIsWritten(wanted)) > 0
}

func holdsAsAPart(text string, sought thing) bool {
	switch sought.kind {
	case aNode:
		return text == sought.name || strings.HasSuffix(text, "/"+sought.name) || strings.HasPrefix(text, sought.name+"/")
	case aComponent:
		return strings.HasSuffix(text, startOfAComponent+sought.name)
	case aGameObject:
		path, _, _ := strings.Cut(text, startOfAComponent)
		return path == sought.name || strings.HasSuffix(path, "/"+sought.name)
	case aCollection:
		path, _, _ := strings.Cut(text, startOfAComponent)
		if _, afterTheSocket, hasASocket := strings.Cut(path, ":"); hasASocket {
			path = afterTheSocket
		}
		return strings.HasPrefix(path, sought.name+"/") || strings.Contains(path, "/"+sought.name+"/")
	case anAnimation:
		return text == sought.name || strings.HasSuffix(text, "/"+sought.name)
	case aProperty:
		return engine.PropertyWithoutItsElement(text) == sought.name
	case aMessage, anAction, aGroup, noThing:
	}
	return text == sought.name
}

func (i Insight) whereTheSameTextServesNoKnownPurpose(under found, changed []project.Location) []project.Location {
	var locations []project.Location
	for _, script := range under.scripts {
		for _, token := range lua.Tokens(script.Source) {
			where := movedBy(project.Location{File: script.File, Position: token.Start}, 1)
			if token.Kind == lua.TokenString && holdsAsAPart(token.Text, under.thing) && !under.all.isNamed[where] && !slices.Contains(changed, where) {
				locations = append(locations, where)
			}
		}
	}
	if under.kind == aMessage || under.kind == anAction || under.kind == aGroup || under.kind == anAnimation {
		locations = append(locations, i.Project.PropertyValuesWritten(under.name)...)
	}
	if under.kind == aProperty {
		locations = append(locations, i.whereAPropertyMayBeNamedOutOfSight(under, changed)...)
	}
	return locations
}

func (i Insight) whereAPropertyMayBeNamedOutOfSight(under found, changed []project.Location) []project.Location {
	var locations []project.Location
	for _, script := range under.scripts {
		for _, field := range fieldsOfTheInstance(script) {
			if where := whereTheFieldIs(script, field); field.Value == under.name && !slices.Contains(changed, where) {
				locations = append(locations, where)
			}
		}
		tokens := lua.Tokens(script.Source)
		for index, token := range tokens {
			setsAField := token.Kind == lua.TokenName && token.Text == under.name && index+1 < len(tokens) && isSymbol(tokens[index+1], "=")
			if around := surroundingsOf(tokens[:index]); setsAField && around.isInATable && functionsThatSpawn[around.callee] {
				locations = append(locations, project.Location{File: script.File, Position: token.Start})
			}
		}
	}
	return locations
}

type Renaming struct {
	Name      string
	Changes   []Change
	LeftAlone []project.Location
}

func charactersNotAllowedIn(kind kindOfThing) string {
	if kind == aNode || kind == aComponent || kind == aGameObject || kind == aCollection {
		return charactersNoNameCanHold + charactersOfAnAddress + " "
	}
	if kind == anAnimation {
		return charactersNoNameCanHold + "/"
	}
	return charactersNoNameCanHold
}

func canName(kind kindOfThing, newName string) bool {
	if kind == aProperty {
		return nameOfAField.MatchString(newName)
	}
	return !strings.ContainsAny(newName, charactersNotAllowedIn(kind))
}

func (i Insight) Rename(file string, content []byte, offset int, newName string) (Renaming, error) {
	under, isOne := i.thingUnder(file, content, offset)
	if !isOne {
		return Renaming{}, ErrNothingToRename
	}
	sought := under.thing
	if err := i.whyItCannotBeRenamed(under); err != nil {
		return Renaming{}, err
	}
	word := wordForAKind[sought.kind]
	switch {
	case newName == "":
		return Renaming{}, fmt.Errorf("the new name of the %s is empty", word)
	case !canName(sought.kind, newName):
		return Renaming{}, fmt.Errorf("%q cannot name a %s: it holds a character that would not survive where such names are written", newName, word)
	case newName == sought.name:
		return Renaming{Name: sought.name}, nil
	case sought.kind == aProperty && i.isTaken(under, newName):
		return Renaming{}, fmt.Errorf("%s already has a property or a field of self named %q, and renaming %q to it would merge the two", sought.file, newName, sought.name)
	case i.isTaken(under, newName):
		return Renaming{}, fmt.Errorf("another %s is already named %q, and renaming %q to it would merge the two", word, newName, sought.name)
	}
	locations := under.all.whereItIsWritten(sought)
	for _, location := range locations {
		if i.Project.IsOfALibrary(location.File) {
			return Renaming{}, fmt.Errorf("%q is also written in %s, which belongs to a library this project fetches and cannot change", sought.name, location.File)
		}
	}
	renaming := Renaming{Name: sought.name, LeftAlone: i.whereTheSameTextServesNoKnownPurpose(under, locations)}
	for _, location := range locations {
		renaming.Changes = append(renaming.Changes, Change{Where: location, Old: sought.name, New: newName})
	}
	return renaming, nil
}

func (c Change) Fits(content []byte) bool {
	start := c.Where.Position.Offset
	return start >= 0 && start+len(c.Old) <= len(content) && string(content[start:start+len(c.Old)]) == c.Old
}

func Apply(content []byte, changes []Change) ([]byte, error) {
	ordered := slices.Clone(changes)
	slices.SortFunc(ordered, func(left, right Change) int { return right.Where.Position.Offset - left.Where.Position.Offset })
	changed := slices.Clone(content)
	for _, change := range ordered {
		if !change.Fits(changed) {
			return nil, fmt.Errorf("%s:%d:%d does not hold %q any more", change.Where.File, change.Where.Position.Line, change.Where.Position.Column, change.Old)
		}
		start := change.Where.Position.Offset
		changed = slices.Concat(changed[:start], []byte(change.New), changed[start+len(change.Old):])
	}
	return changed, nil
}

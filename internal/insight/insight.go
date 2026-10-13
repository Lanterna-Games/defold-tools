package insight

import (
	"fmt"
	"path"
	"slices"
	"sort"
	"strings"

	"github.com/Lanterna-Games/defold-tools/internal/engine"
	"github.com/Lanterna-Games/defold-tools/internal/project"
)

const (
	guiScriptExtension = ".gui_script"
	guiComponentType   = "gui"
)

type Insight struct {
	Project       *project.Project
	Reference     *engine.Reference
	Documentation Documentation
}

type Candidate struct {
	Label         string
	Detail        string
	Documentation string
	Role          Role
}

type subject struct {
	place  Place
	script *project.Script
	owners []project.Owner
}

func (i Insight) subjectAt(file string, content []byte, offset int) (subject, bool) {
	place, isSomewhere := Locate(content, offset, i.Reference)
	if !isSomewhere {
		return subject{}, false
	}
	script := i.Project.ReadScript(file, content)
	if place.TargetConstant != "" {
		if address, isWritten := script.AddressKeptIn(place.TargetConstant); isWritten {
			place.Target, place.TargetIsWritten = address.Text, true
		}
	}
	return subject{place: place, script: script, owners: i.Project.OwnersRunning(script.Path)}, true
}

func (i Insight) targetsOf(about subject, address string) []project.Target {
	var targets []project.Target
	url := project.ParseURL(address)
	url.Fragment, url.HasFragment = "", false
	for _, owner := range about.owners {
		if target, isKnown := i.Project.TargetOf(owner, url); isKnown && !slices.ContainsFunc(targets, func(seen project.Target) bool {
			return seen.GameObject == target.GameObject && seen.Name == target.Name
		}) {
			targets = append(targets, target)
		}
	}
	return targets
}

func (i Insight) componentsAt(about subject, address string) []project.Component {
	url := project.ParseURL(address)
	var components []project.Component
	for _, target := range i.targetsOf(about, address) {
		if !url.HasFragment || url.Fragment == "" {
			components = append(components, target.GameObject.Components...)
			continue
		}
		if component, exists := target.GameObject.Component(url.Fragment); exists {
			components = append(components, component)
		}
	}
	return components
}

func (i Insight) scenesRunning(script *project.Script) []*project.GUIScene {
	var running, every []*project.GUIScene
	for _, scenePath := range sortedKeys(i.Project.GUIScenes) {
		scene := i.Project.GUIScenes[scenePath]
		every = append(every, scene)
		if scene.Script == script.Path {
			running = append(running, scene)
		}
	}
	if len(running) == 0 {
		return every
	}
	return running
}

func sortedKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func (i Insight) Complete(file string, content []byte, offset int) (candidates []Candidate, start, end int) {
	about, isSomewhere := i.subjectAt(file, content, offset)
	if !isSomewhere {
		return nil, 0, 0
	}
	start, end = about.place.Start, about.place.End
	switch about.place.Role {
	case RoleOfNothing:
	case RoleOfAnAddress:
		candidates, start = i.addresses(about)
	case RoleOfAProperty:
		candidates = i.properties(about)
	case RoleOfAMessagePosted:
		candidates = i.messagesToPost(about)
	case RoleOfAMessageHandled:
		candidates = i.messagesToHandle(about)
	case RoleOfANode:
		candidates = i.nodes(about)
	case RoleOfAnAnimation:
		candidates = i.animations(about)
	case RoleOfAnAnimationOfANode:
		candidates = i.animationsOfNodes(about)
	case RoleOfAnAction:
		candidates = i.actions()
	case RoleOfAGroup:
		for _, group := range i.Project.CollisionGroupNames() {
			candidates = append(candidates, Candidate{Label: group, Detail: "collision group"})
		}
	case RoleOfAField:
		for _, field := range i.fieldsAt(about) {
			candidates = append(candidates, Candidate{Label: field.name, Detail: field.detail(), Documentation: field.explained()})
		}
	}
	seen := map[string]bool{}
	unique := candidates[:0]
	for _, candidate := range candidates {
		if candidate.Label != "" && !seen[candidate.Label] {
			seen[candidate.Label] = true
			candidate.Role = about.place.Role
			unique = append(unique, candidate)
		}
	}
	return unique, start, end
}

func describedComponent(component project.Component) string {
	if component.Resource != "" {
		return component.Type + " · " + component.Resource
	}
	return component.Type
}

func (i Insight) addresses(about subject) (candidates []Candidate, start int) {
	typed := about.place.Typed
	if beforeTheFragment, _, hasFragment := strings.Cut(typed, "#"); hasFragment {
		for _, component := range i.componentsAt(about, beforeTheFragment) {
			candidates = append(candidates, Candidate{Label: component.ID, Detail: describedComponent(component)})
		}
		return candidates, about.place.Start + len(beforeTheFragment) + 1
	}
	url := project.ParseURL(typed)
	socket := ""
	if url.Socket != "" {
		socket = url.Socket + ":"
	}
	if typed == "" {
		for _, owner := range about.owners {
			for _, component := range owner.GameObject.Components {
				candidates = append(candidates, Candidate{Label: "#" + component.ID, Detail: describedComponent(component)})
			}
		}
		candidates = append(candidates, Candidate{Label: ".", Detail: "this game object"})
	}
	for _, namespace := range i.namespacesFor(about, url) {
		for _, gameObjectPath := range namespace.GameObjectPaths() {
			detail := "game object of " + namespace.Root.Name
			if gameObject := namespace.GameObjects[gameObjectPath]; gameObject != nil {
				detail = "game object · " + gameObject.Path
			}
			candidates = append(candidates, Candidate{Label: socket + gameObjectPath, Detail: detail})
			if socket != "" || strings.HasPrefix(typed, "/") {
				continue
			}
			for _, owner := range about.owners {
				if relative, isNextToIt := strings.CutPrefix(gameObjectPath, owner.BasePrefix+"/"); isNextToIt && owner.Namespace == namespace {
					candidates = append(candidates, Candidate{Label: relative, Detail: detail})
				}
			}
		}
	}
	return candidates, about.place.Start
}

func (i Insight) namespacesFor(about subject, url project.URL) []*project.Namespace {
	var namespaces []*project.Namespace
	add := func(namespace *project.Namespace) {
		if namespace != nil && !slices.Contains(namespaces, namespace) {
			namespaces = append(namespaces, namespace)
		}
	}
	if url.Socket != "" {
		for _, namespace := range i.Project.NamespacesNamed(url.Socket) {
			add(namespace)
		}
		return namespaces
	}
	for _, owner := range about.owners {
		add(owner.Namespace)
	}
	if len(namespaces) == 0 {
		for _, mount := range i.Project.Mounts() {
			add(mount.Namespace)
		}
	}
	return namespaces
}

func (i Insight) propertiesOf(about subject) (names []string, holder string, isKnown bool) {
	if !about.place.TargetIsWritten {
		return nil, "", false
	}
	url := project.ParseURL(about.place.Target)
	pointsAtTheScriptItself := url.Socket == "" && url.Path == "" && url.HasFragment && url.Fragment == ""
	switch {
	case pointsAtTheScriptItself && path.Ext(about.script.Path) == guiScriptExtension:
		names, _ = i.Reference.PropertiesOf(guiComponentType)
		return names, "a gui component", true
	case pointsAtTheScriptItself:
		return about.script.PropertyNames(), about.script.Path, true
	case !url.HasFragment || url.Fragment == "":
		if len(i.targetsOf(about, about.place.Target)) == 0 {
			return nil, "", false
		}
		names, _ = i.Reference.PropertiesOf(engine.GameObject)
		return names, "a game object", true
	}
	for _, component := range i.componentsAt(about, about.place.Target) {
		isKnown = true
		if component.IsScript() {
			if script, isLoaded := i.Project.Scripts[component.Resource]; isLoaded {
				names, holder = append(names, script.PropertyNames()...), script.Path
			}
			continue
		}
		ofTheKind, _ := i.Reference.PropertiesOf(component.Type)
		names, holder = append(names, ofTheKind...), "a "+component.Type+" component"
		if i.Reference.IsDrawnWithAMaterial(component.Type) {
			names = append(names, i.Reference.MaterialProperties...)
			names = append(names, sortedKeys(i.Project.MaterialProperties)...)
		}
	}
	return names, holder, isKnown
}

func (i Insight) properties(about subject) []Candidate {
	names, holder, isKnown := i.propertiesOf(about)
	if !isKnown {
		names, holder = i.Reference.EveryProperty(), "the engine"
		for _, scriptPath := range i.Project.ScriptPaths() {
			names = append(names, i.Project.Scripts[scriptPath].PropertyNames()...)
		}
	}
	of := i.kindsOf(about)
	var candidates []Candidate
	for _, name := range names {
		if !strings.HasSuffix(name, "N") {
			candidates = append(candidates, Candidate{
				Label: name, Detail: "property of " + holder,
				Documentation: whatTheReferenceSays(i.documentedProperties(name, of), false),
			})
		}
	}
	return candidates
}

func (i Insight) receiversOf(about subject) (scripts []*project.Script, takesEngineMessages, isKnown bool) {
	if !about.place.TargetIsWritten {
		return nil, false, false
	}
	url := project.ParseURL(about.place.Target)
	if url.Socket == "" && url.Path == "" && url.HasFragment && url.Fragment == "" {
		return []*project.Script{about.script}, false, true
	}
	components := i.componentsAt(about, about.place.Target)
	for _, component := range components {
		script, isInSight := i.Project.ScriptRunBy(component)
		switch {
		case script != nil:
			scripts = append(scripts, script)
		case isInSight:
			takesEngineMessages = true
		}
	}
	return scripts, takesEngineMessages, len(components) > 0
}

func (i Insight) messagesToPost(about subject) []Candidate {
	scripts, takesEngineMessages, isKnown := i.receiversOf(about)
	if !isKnown {
		takesEngineMessages = true
		for _, scriptPath := range i.Project.ScriptPaths() {
			scripts = append(scripts, i.Project.Scripts[scriptPath])
		}
	}
	var candidates []Candidate
	for _, script := range scripts {
		for _, handled := range script.Handled {
			candidates = append(candidates, Candidate{Label: handled.Name, Detail: "handled by " + script.Path})
		}
	}
	if takesEngineMessages {
		candidates = append(candidates, i.engineMessages(about)...)
	}
	return candidates
}

func (i Insight) engineMessages(about subject) []Candidate {
	of := i.kindsOf(about)
	candidates := make([]Candidate, 0, len(i.Reference.Messages))
	for _, message := range i.Reference.Messages {
		candidates = append(candidates, Candidate{
			Label: message, Detail: "engine message",
			Documentation: whatTheReferenceSays(i.documentedMessages(message, of), false),
		})
	}
	return candidates
}

func (i Insight) messagesToHandle(about subject) []Candidate {
	var candidates []Candidate
	for _, scriptPath := range i.Project.ScriptPaths() {
		for _, post := range i.Project.Scripts[scriptPath].Posts {
			if post.IDIsKnown && !i.Reference.HasMessage(post.ID.Name) {
				candidates = append(candidates, Candidate{Label: post.ID.Name, Detail: "posted by " + scriptPath})
			}
		}
	}
	return append(candidates, i.engineMessages(about)...)
}

func (i Insight) nodes(about subject) []Candidate {
	var candidates []Candidate
	for _, scene := range i.scenesRunning(about.script) {
		for _, id := range i.Project.NodesOf(scene).IDs() {
			candidates = append(candidates, Candidate{Label: id, Detail: "node of " + scene.Path})
		}
	}
	return candidates
}

func (i Insight) texturesPlayedBy(about subject) []string {
	var textures []string
	if about.place.TargetIsWritten {
		for _, component := range i.componentsAt(about, about.place.Target) {
			if sprite, isRead := i.Project.SpriteOf(component); isRead {
				textures = append(textures, sprite.Textures...)
			}
		}
	}
	if len(textures) == 0 {
		textures = sortedKeys(i.Project.Animations)
	}
	return textures
}

func (i Insight) animationsIn(textures []string) []Candidate {
	var candidates []Candidate
	for _, texture := range textures {
		for _, animation := range i.Project.Animations[texture] {
			candidates = append(candidates, Candidate{Label: animation, Detail: "animation of " + texture})
		}
	}
	return candidates
}

func (i Insight) animations(about subject) []Candidate {
	return i.animationsIn(i.texturesPlayedBy(about))
}

func (i Insight) texturesOfTheScenesOf(script *project.Script) []string {
	var textures []string
	for _, scene := range i.scenesRunning(script) {
		for _, name := range sortedKeys(scene.Textures) {
			textures = append(textures, scene.Textures[name])
		}
	}
	return textures
}

func (i Insight) animationsOfNodes(about subject) []Candidate {
	return i.animationsIn(i.texturesOfTheScenesOf(about.script))
}

func (i Insight) boundActions() (actions []string, binding string) {
	binding = i.Project.GameBinding
	actions = slices.Clone(i.Reference.InputBindings[binding])
	for _, bound := range i.Project.InputBindings {
		actions = append(actions, bound...)
	}
	sort.Strings(actions)
	return slices.Compact(actions), binding
}

func (i Insight) actions() []Candidate {
	actions, binding := i.boundActions()
	candidates := make([]Candidate, 0, len(actions))
	for _, action := range actions {
		candidates = append(candidates, Candidate{Label: action, Detail: "bound in " + binding})
	}
	return candidates
}

func (i Insight) Define(file string, content []byte, offset int) []project.Location {
	about, isSomewhere := i.subjectAt(file, content, offset)
	if !isSomewhere {
		return nil
	}
	name := about.place.Text
	var locations []project.Location
	add := func(location project.Location, isThere bool) {
		if isThere && !slices.Contains(locations, location) {
			locations = append(locations, location)
		}
	}
	switch about.place.Role {
	case RoleOfNothing:
	case RoleOfAnAddress:
		for _, location := range i.whereTheAddressLeads(about, name) {
			add(location, true)
		}
	case RoleOfAProperty:
		for _, component := range i.componentsAt(about, about.place.Target) {
			if script, isLoaded := i.Project.Scripts[component.Resource]; isLoaded && component.IsScript() && about.place.TargetIsWritten {
				for _, declared := range script.Properties {
					add(declared.Location, declared.Name == engine.PropertyWithoutItsElement(name))
				}
			}
		}
	case RoleOfAMessagePosted:
		scripts, _, isKnown := i.receiversOf(about)
		if !isKnown {
			for _, scriptPath := range i.Project.ScriptPaths() {
				scripts = append(scripts, i.Project.Scripts[scriptPath])
			}
		}
		for _, script := range scripts {
			for _, handled := range script.Handled {
				add(handled.Location, handled.Name == name)
			}
		}
	case RoleOfAMessageHandled:
		for _, scriptPath := range i.Project.ScriptPaths() {
			for _, post := range i.Project.Scripts[scriptPath].Posts {
				add(post.ID.Location, post.IDIsKnown && post.ID.Name == name)
			}
		}
	case RoleOfANode:
		for _, scene := range i.scenesRunning(about.script) {
			add(i.Project.WhereTheNodeIs(scene, name))
		}
	case RoleOfAnAnimation:
		for _, texture := range i.texturesPlayedBy(about) {
			add(i.Project.WhereDeclared(project.DeclaredAnimation, texture, name))
		}
	case RoleOfAnAnimationOfANode:
		for _, texture := range i.texturesOfTheScenesOf(about.script) {
			add(i.Project.WhereDeclared(project.DeclaredAnimation, texture, name))
		}
	case RoleOfAnAction:
		add(i.Project.WhereDeclared(project.DeclaredAction, "", name))
	case RoleOfAGroup:
		add(i.Project.WhereDeclared(project.DeclaredGroup, "", name))
	case RoleOfAField:
		for _, field := range i.fieldsAt(about) {
			for _, location := range field.setIn {
				add(location, field.name == name)
			}
		}
	}
	return locations
}

func (i Insight) whereTheAddressLeads(about subject, address string) []project.Location {
	url := project.ParseURL(address)
	var locations []project.Location
	if url.HasFragment && url.Fragment != "" {
		for _, component := range i.componentsAt(about, address) {
			locations = append(locations, component.Location)
		}
		return locations
	}
	for _, target := range i.targetsOf(about, address) {
		for _, mount := range i.Project.Mounts() {
			for _, instance := range mount.Collection.Instances {
				if mount.Prefix+"/"+instance.ID == target.Name && i.Project.GameObjectOf(instance) == target.GameObject {
					locations = append(locations, instance.Location)
				}
			}
		}
	}
	return locations
}

func (i Insight) Describe(file string, content []byte, offset int) (markdown string, start, end int, isSomewhere bool) {
	about, isSomewhere := i.subjectAt(file, content, offset)
	if !isSomewhere {
		return "", 0, 0, false
	}
	name := about.place.Text
	switch about.place.Role {
	case RoleOfNothing:
	case RoleOfAnAddress:
		markdown = i.describedAddress(about, name)
	case RoleOfAProperty:
		_, holder, isKnown := i.propertiesOf(about)
		switch {
		case isKnown:
			markdown = fmt.Sprintf("Property `%s` of %s.", name, holder)
		case about.place.TargetIsWritten:
			markdown = fmt.Sprintf("Property `%s`. `%s` leads to nothing that can be followed from here, so where it belongs cannot be told.", name, about.place.Target)
		default:
			markdown = fmt.Sprintf("Property `%s`. The address it is asked of is not written, so where it belongs cannot be told.", name)
		}
		if said := whatTheReferenceSays(i.documentedProperties(name, i.kindsOf(about)), true); said != "" {
			markdown += "\n\n" + said
		}
	case RoleOfAMessagePosted, RoleOfAMessageHandled:
		markdown = i.describedMessage(about, name)
	case RoleOfANode:
		markdown = i.describedByWhereItIs("GUI node", name, i.Define(file, content, offset))
	case RoleOfAnAnimation, RoleOfAnAnimationOfANode:
		markdown = i.describedByWhereItIs("Animation", name, i.Define(file, content, offset))
	case RoleOfAnAction:
		_, binding := i.boundActions()
		markdown = i.describedByWhereItIs("Input action", name, i.Define(file, content, offset))
		if actions, _ := i.boundActions(); slices.Contains(actions, name) && !strings.Contains(markdown, "\n") {
			markdown = fmt.Sprintf("Input action `%s`, bound in `%s`.", name, binding)
		}
	case RoleOfAGroup:
		markdown = i.describedByWhereItIs("Collision group", name, i.Define(file, content, offset))
	case RoleOfAField:
		var said []string
		for _, field := range i.fieldsAt(about) {
			if field.name == name {
				said = append(said, field.explained())
			}
		}
		markdown = strings.Join(said, "\n\n")
	}
	return markdown, about.place.Start, about.place.End, markdown != ""
}

func (i Insight) describedByWhereItIs(what, name string, locations []project.Location) string {
	if len(locations) == 0 {
		return fmt.Sprintf("%s `%s`. Nothing in the project declares it.", what, name)
	}
	lines := []string{fmt.Sprintf("%s `%s`, declared in:", what, name), ""}
	for _, location := range locations {
		lines = append(lines, fmt.Sprintf("- `%s` line %d", location.File, location.Position.Line))
	}
	return strings.Join(lines, "\n")
}

func (i Insight) describedAddress(about subject, address string) string {
	url := project.ParseURL(address)
	targets := i.targetsOf(about, address)
	if len(targets) == 0 {
		return fmt.Sprintf("`%s` leads to nothing that can be followed from here.", address)
	}
	var lines []string
	for _, target := range targets {
		if url.HasFragment && url.Fragment != "" {
			component, exists := target.GameObject.Component(url.Fragment)
			if !exists {
				lines = append(lines, fmt.Sprintf("`%s` has no component `%s`.", target.Name, url.Fragment))
				continue
			}
			lines = append(lines, fmt.Sprintf("**%s** component `%s` of `%s`", component.Type, component.ID, target.Name))
			if component.Resource != "" {
				lines = append(lines, "", "`"+component.Resource+"`")
			}
			if script, _ := i.Project.ScriptRunBy(component); script != nil {
				if handled := script.HandledNames(); len(handled) > 0 {
					lines = append(lines, "", "Handles: `"+strings.Join(handled, "`, `")+"`")
				}
				if properties := script.PropertyNames(); len(properties) > 0 && component.IsScript() {
					lines = append(lines, "", "Properties: `"+strings.Join(properties, "`, `")+"`")
				}
			}
			continue
		}
		lines = append(lines, fmt.Sprintf("Game object `%s`, from `%s`", target.Name, target.GameObject.Path), "")
		for _, component := range target.GameObject.Components {
			lines = append(lines, fmt.Sprintf("- `#%s` %s", component.ID, describedComponent(component)))
		}
	}
	return strings.Join(lines, "\n")
}

func (i Insight) describedMessage(about subject, name string) string {
	var lines []string
	if i.Reference.HasMessage(name) {
		lines = append(lines, fmt.Sprintf("Engine message `%s`.", name))
		if said := whatTheReferenceSays(i.documentedMessages(name, i.kindsOf(about)), true); said != "" {
			lines = append(lines, "", said)
		} else if fields := i.Reference.MessageFields[name]; len(fields) > 0 {
			lines = append(lines, "", "Fields: `"+strings.Join(fields, "`, `")+"`")
		}
	}
	var handlers, posts []string
	for _, scriptPath := range i.Project.ScriptPaths() {
		script := i.Project.Scripts[scriptPath]
		for _, handled := range script.Handled {
			if handled.Name == name {
				handlers = append(handlers, fmt.Sprintf("- `%s` line %d", handled.Location.File, handled.Location.Position.Line))
			}
		}
		for _, post := range script.Posts {
			if post.IDIsKnown && post.ID.Name == name {
				fields := ""
				if len(post.Fields) > 0 {
					fields = " with `" + strings.Join(post.Fields, "`, `") + "`"
				}
				posts = append(posts, fmt.Sprintf("- `%s` line %d%s", post.ID.Location.File, post.ID.Location.Position.Line, fields))
			}
		}
	}
	if len(lines) == 0 {
		lines = append(lines, fmt.Sprintf("Message `%s`.", name))
	}
	if len(handlers) > 0 {
		lines = append(append(lines, "", "Handled in:", ""), handlers...)
	} else if about.place.Role == RoleOfAMessagePosted && !i.Reference.HasMessage(name) {
		lines = append(lines, "", "No script in the project handles it.")
	}
	if len(posts) > 0 {
		lines = append(append(lines, "", "Posted from:", ""), posts...)
	} else if about.place.Role == RoleOfAMessageHandled && !i.Reference.HasMessage(name) {
		lines = append(lines, "", "Nothing in the project posts it.")
	}
	return strings.Join(lines, "\n")
}

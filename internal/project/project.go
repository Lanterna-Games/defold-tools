package project

import (
	"path"

	"github.com/Lanterna-Games/defold-tools/internal/lua"
	"github.com/Lanterna-Games/defold-tools/internal/source"
	"github.com/Lanterna-Games/defold-tools/internal/textproto"
)

type Location struct {
	File     string
	Position source.Position
}

type Project struct {
	GameObjects        map[string]*GameObject
	Collections        map[string]*Collection
	Scripts            map[string]*Script
	GUIScenes          map[string]*GUIScene
	MaterialProperties map[string]bool
	InputBindings      map[string][]string
	CollisionGroups    map[string]bool
	CollisionMasks     []WrittenID
	Animations         map[string][]string
	Sprites            []*Sprite
	EmitterAnimations  []EmitterAnimation
	GameBinding        string
	References         []ResourceReference
	SyntaxErrors       []SyntaxError

	RunsCodeOutOfSight bool
	IsALibrary         bool
	Dependencies       []string
	PathsInSettings    []string
	Settings           []SettingWritten

	libraries           Libraries
	hasANativeExtension bool
	mounts              []Mount
	mountsAreBuilt      bool
	spawnedByAFactory   map[string]bool
	declared            map[declaration]Location
	mentioned           map[declaration][]Location
	spriteFiles         map[string]*Sprite
	files               map[string]bool
	filesOfAnExtension  map[string][]string
	parsed              *Parsed
	topLevelDirectories map[string]bool
	embeddedDocuments   map[*textproto.Scalar]*textproto.Message
}

type SyntaxError struct {
	Location Location
	Message  string
}

type Script struct {
	Path                        string
	File                        string
	Chunk                       *lua.Chunk
	Source                      []byte
	IsReadInFull                bool
	Properties                  []PropertyDeclaration
	DeclaresPropertiesByLiteral bool

	Posts                        []MessagePost
	Handled                      []MessageID
	HandlesMessagesItDoesNotName bool
	FieldsRead                   []FieldRead

	ActionsCompared                    []WrittenID
	ActionsItFeedsItself               []string
	FeedsItselfActionsThatCannotBeRead bool

	GUINodesLookedUp              []WrittenID
	GUINodesItNames               []string
	NamesGUINodesThatCannotBeRead bool

	BlindSpots []BlindSpot

	constants   map[string]lua.Expression
	tableFields map[string]map[string]lua.Expression
	exports     map[string]lua.Expression
	moduleAt    func(resourcePath string) (module *Script, isRead bool)
}

type BlindSpot struct {
	Location Location
	Why      string
}

func (s *Script) cannotRead(node lua.Node, why string) {
	s.BlindSpots = append(s.BlindSpots, BlindSpot{Location: s.LocationOf(node), Why: why})
}

func (s *Script) handlesEveryMessageBecauseOf(node lua.Node, why string) {
	if !s.HandlesMessagesItDoesNotName {
		s.cannotRead(node, why+", so this script counts as handling every message")
	}
	s.HandlesMessagesItDoesNotName = true
}

type MessageID struct {
	Name     string
	Location Location
}

type MessagePost struct {
	ID                MessageID
	IDIsKnown         bool
	Receiver          string
	ReceiverIsWritten bool
	Fields            []string
	FieldsAreWritten  bool
}

func (s *Script) Handles(message string) bool {
	for _, handled := range s.Handled {
		if handled.Name == message {
			return true
		}
	}
	return false
}

func (s *Script) HandledNames() []string {
	names := make([]string, 0, len(s.Handled))
	for _, handled := range s.Handled {
		names = append(names, handled.Name)
	}
	return names
}

type PropertyDeclaration struct {
	Name     string
	Location Location
}

func (s *Script) DeclaresProperty(name string) bool {
	for _, declaration := range s.Properties {
		if declaration.Name == name {
			return true
		}
	}
	return false
}

func (s *Script) PropertyNames() []string {
	names := make([]string, 0, len(s.Properties))
	for _, declaration := range s.Properties {
		names = append(names, declaration.Name)
	}
	return names
}

type GameObject struct {
	Path       string
	Components []Component
}

type Component struct {
	ID                string
	Type              string
	Resource          string
	Location          Location
	IDWhere           Location
	PropertyOverrides []PropertyOverride

	sprite *Sprite
}

func (c Component) IsScript() bool {
	return c.Type == scriptComponentType
}

func (g *GameObject) Component(id string) (component Component, found bool) {
	for _, candidate := range g.Components {
		if candidate.ID == id {
			return candidate, true
		}
	}
	return Component{}, false
}

func (g *GameObject) ComponentIDs() []string {
	ids := make([]string, 0, len(g.Components))
	for _, component := range g.Components {
		ids = append(ids, component.ID)
	}
	return ids
}

type Collection struct {
	Path           string
	Name           string
	Instances      []Instance
	SubCollections []SubCollection
}

type Instance struct {
	ID                string
	Location          Location
	IDWhere           Location
	Prototype         string
	EmbeddedObject    *GameObject
	Children          []Child
	PropertyOverrides []PropertyOverride
}

type Child struct {
	ID       string
	Location Location
	Where    Location
}

type SubCollection struct {
	ID                string
	Location          Location
	IDWhere           Location
	Collection        string
	PropertyOverrides map[string][]PropertyOverride
	InstancesNamed    []WrittenID
}

type PropertyOverride struct {
	Component      string
	Property       string
	Type           string
	Value          string
	Location       Location
	NameLocation   Location
	ComponentWhere Location
	NameWhere      Location
	ValueWhere     Location
}

const PropertyTypeURL = "PROPERTY_TYPE_URL"

type ResourceReference struct {
	Path     string
	Location Location
}

func (p *Project) IsSpawnedByAFactory(resourcePath string) bool {
	return p.spawnedByAFactory[resourcePath]
}

func (p *Project) HasFile(resourcePath string) bool {
	return p.files[resourcePath]
}

func (p *Project) HasTopLevelDirectoryOf(resourcePath string) bool {
	return p.topLevelDirectories[topLevelDirectoryOf(resourcePath)]
}

func (p *Project) IsOfALibrary(filePath string) bool {
	return p.libraries.Owns(filePath)
}

func (p *Project) Files() []string {
	return sortedKeys(p.files)
}

func (p *Project) FilesWithExtension(extension string) []string {
	if p.filesOfAnExtension == nil {
		p.filesOfAnExtension = map[string][]string{}
		for _, resourcePath := range sortedKeys(p.files) {
			p.filesOfAnExtension[path.Ext(resourcePath)] = append(p.filesOfAnExtension[path.Ext(resourcePath)], resourcePath)
		}
	}
	return p.filesOfAnExtension[extension]
}

func (p *Project) CollectionPaths() []string {
	return sortedKeys(p.Collections)
}

func (c *Collection) InstanceIDs() []string {
	ids := make([]string, 0, len(c.Instances))
	for _, instance := range c.Instances {
		ids = append(ids, instance.ID)
	}
	return ids
}

func (p *Project) ScriptPathsOf(gameObject *GameObject) []string {
	var paths []string
	for _, component := range gameObject.Components {
		if scriptPath, hasScript := p.ScriptPathOf(component); hasScript {
			paths = append(paths, scriptPath)
		}
	}
	return paths
}

func (p *Project) ScriptPathOf(component Component) (scriptPath string, hasScript bool) {
	switch component.Type {
	case scriptComponentType:
		return component.Resource, component.Resource != ""
	case guiComponentType:
		scene, isLoaded := p.GUIScenes[component.Resource]
		if !isLoaded || scene.Script == "" {
			return "", false
		}
		return scene.Script, true
	default:
		return "", false
	}
}

func (p *Project) ScriptRunBy(component Component) (script *Script, isInSight bool) {
	scriptPath := component.Resource
	switch component.Type {
	case scriptComponentType:
	case guiComponentType:
		scene, isLoaded := p.GUIScenes[component.Resource]
		if !isLoaded {
			return nil, false
		}
		if scene.Script == "" {
			return nil, true
		}
		scriptPath = scene.Script
	default:
		return nil, true
	}
	script, isLoaded := p.Scripts[scriptPath]
	return script, isLoaded
}

func (p *Project) ScriptPaths() []string {
	return sortedKeys(p.Scripts)
}

func (p *Project) GameObjectPaths() []string {
	return sortedKeys(p.GameObjects)
}

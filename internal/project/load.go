package project

import (
	"fmt"
	"io/fs"
	"path"
	"strings"

	"github.com/Lanterna-Games/defold-tools/internal/inputbinding"
	"github.com/Lanterna-Games/defold-tools/internal/lua"
	"github.com/Lanterna-Games/defold-tools/internal/material"
	"github.com/Lanterna-Games/defold-tools/internal/source"
	"github.com/Lanterna-Games/defold-tools/internal/textproto"
)

const (
	gameObjectExtension     = ".go"
	collectionExtension     = ".collection"
	guiSceneExtension       = ".gui"
	scriptComponentType     = "script"
	guiComponentType        = "gui"
	nativeExtensionManifest = "ext.manifest"
	settingsFile            = "game.project"
)

var scriptExtensions = map[string]bool{".script": true, ".gui_script": true, ".render_script": true, ".lua": true}

var textFormatExtensions = map[string]bool{
	gameObjectExtension: true, collectionExtension: true,
	".animationset": true, ".atlas": true, ".camera": true, ".collectionfactory": true,
	".collectionproxy": true, ".collisionobject": true, ".cubemap": true, ".display_profiles": true,
	".factory": true, ".font": true, ".gamepads": true, ".gui": true, ".input_binding": true,
	".label": true, ".material": true, ".mesh": true, ".model": true, ".particlefx": true,
	".render": true, ".sound": true, ".spinemodel": true, ".spinescene": true, ".sprite": true,
	".texture_profiles": true, ".tilemap": true, ".tilesource": true,
}

var directoriesNotPartOfTheSources = map[string]bool{"build": true}

type Libraries interface {
	Owns(filePath string) bool
	HasAll(addresses []string) bool
}

type noLibraries struct{}

func (noLibraries) Owns(string) bool     { return false }
func (noLibraries) HasAll([]string) bool { return false }

func Load(files fs.FS) (*Project, error) {
	return LoadWithLibraries(files, noLibraries{})
}

func LoadWithLibraries(files fs.FS, libraries Libraries) (*Project, error) {
	return LoadRemembering(files, libraries, nil)
}

func LoadRemembering(files fs.FS, libraries Libraries, parsed *Parsed) (*Project, error) {
	loaded := &Project{
		libraries:           libraries,
		parsed:              parsed,
		GameObjects:         map[string]*GameObject{},
		Collections:         map[string]*Collection{},
		Scripts:             map[string]*Script{},
		GUIScenes:           map[string]*GUIScene{},
		MaterialProperties:  map[string]bool{},
		InputBindings:       map[string][]string{},
		CollisionGroups:     map[string]bool{},
		Animations:          map[string][]string{},
		spriteFiles:         map[string]*Sprite{},
		GameBinding:         defaultGameBinding,
		spawnedByAFactory:   map[string]bool{},
		declared:            map[declaration]Location{},
		mentioned:           map[declaration][]Location{},
		files:               map[string]bool{},
		topLevelDirectories: map[string]bool{},
		embeddedDocuments:   map[*textproto.Scalar]*textproto.Message{},
	}
	err := fs.WalkDir(files, ".", func(filePath string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if isSkippedDirectory(filePath, entry.Name()) {
				return fs.SkipDir
			}
			return nil
		}
		return loaded.loadFile(files, filePath)
	})
	if err != nil {
		return nil, fmt.Errorf("reading the project: %w", err)
	}
	parsed.forgetAllBut(loaded.files)
	for _, script := range loaded.Scripts {
		script.readWithTheModulesOf(loaded)
	}
	dependsOnWhatWasNotRead := len(loaded.Dependencies) > 0 && !libraries.HasAll(loaded.Dependencies)
	loaded.RunsCodeOutOfSight = loaded.IsALibrary || loaded.hasANativeExtension || dependsOnWhatWasNotRead
	return loaded, nil
}

func isSkippedDirectory(directoryPath, name string) bool {
	if directoryPath == "." {
		return false
	}
	isAtTopLevel := !strings.Contains(directoryPath, "/")
	return strings.HasPrefix(name, ".") || (isAtTopLevel && directoriesNotPartOfTheSources[name])
}

func (p *Project) loadFile(files fs.FS, filePath string) error {
	resourcePath := "/" + filePath
	p.files[resourcePath] = true
	p.topLevelDirectories[topLevelDirectoryOf(resourcePath)] = true

	if path.Base(filePath) == nativeExtensionManifest {
		p.hasANativeExtension = true
	}
	extension := path.Ext(filePath)
	if !scriptExtensions[extension] && !textFormatExtensions[extension] && !extensionsOfSettings[extension] {
		return nil
	}
	content, err := fs.ReadFile(files, filePath)
	if err != nil {
		return fmt.Errorf("reading %s: %w", filePath, err)
	}
	if extensionsOfSettings[extension] {
		projectSettings := settingsFrom(content)
		p.PathsInSettings = append(p.PathsInSettings, projectSettings.pathsNamed()...)
		if filePath == settingsFile {
			p.Settings = settingsWrittenIn(filePath, content)
			for _, written := range p.Settings {
				if written.Section == sectionOfTheProject && written.Key == keyOfTheDependencies {
					p.Dependencies = append(p.Dependencies, strings.FieldsFunc(written.Value, separatesAddresses)...)
				}
			}
			p.IsALibrary = projectSettings.isOfALibrary()
			p.GameBinding = projectSettings.gameBinding()
			if projectSettings["bootstrap"]["main_collection"] == "" {
				p.PathsInSettings = append(p.PathsInSettings, defaultMainCollection)
			}
		}
		return nil
	}
	if scriptExtensions[extension] {
		chunk, isReadInFull := p.parsed.script(filePath, content, func() (*lua.Chunk, bool) { return parsedLua(content) })
		p.Scripts[resourcePath] = scriptReadAs(filePath, resourcePath, content, chunk, isReadInFull)
		return nil
	}
	document := p.parse(filePath, content)

	p.References = append(p.References, p.resourceReferencesIn(filePath, document)...)
	switch extension {
	case gameObjectExtension:
		p.GameObjects[resourcePath] = p.gameObjectFrom(filePath, resourcePath, document)
	case collectionExtension:
		p.Collections[resourcePath] = p.collectionFrom(filePath, resourcePath, document)
	case guiSceneExtension:
		p.GUIScenes[resourcePath] = p.guiSceneFrom(filePath, resourcePath, document)
	case collisionObjectExtension:
		p.collectCollisionObject(filePath, document)
	case tileSourceExtension:
		for _, group := range document.Scalars("collision_groups") {
			p.CollisionGroups[group.Text] = true
			p.mentionGroup(filePath, group)
		}
		for _, hull := range document.Messages("convex_hulls") {
			if group, isSet := hull.Scalar("collision_group"); isSet {
				p.mentionGroup(filePath, group)
			}
		}
		p.Animations[resourcePath] = p.animationIDsIn(filePath, resourcePath, document)
	case atlasExtension:
		p.Animations[resourcePath] = p.animationsOfAnAtlas(filePath, resourcePath, document)
	case particleEffectExtension:
		p.collectEmitters(filePath, document)
	case spriteExtension:
		p.spriteFiles[resourcePath] = p.addSprite(spriteFrom(filePath, document))
	case inputbinding.Extension:
		p.InputBindings[resourcePath] = inputbinding.Actions(document)
		for _, trigger := range inputbinding.Triggers(document) {
			p.declare(DeclaredAction, "", trigger.Action, Location{File: filePath, Position: trigger.Position})
			p.mention(DeclaredAction, "", trigger.Action, Location{File: filePath, Position: trigger.TextPosition})
		}
	case material.Extension:
		for _, name := range material.PropertyNames(document) {
			p.MaterialProperties[name] = true
		}
	}
	return nil
}

func (p *Project) parse(filePath string, content []byte) *textproto.Message {
	document, syntaxErrors := p.parsed.document(filePath, content, func() (*textproto.Message, []textproto.SyntaxError) {
		return textproto.Parse(content, source.NewLocator(content))
	})
	p.recordSyntaxErrors(filePath, syntaxErrors)
	return document
}

func (p *Project) parseEmbedded(filePath string, data *textproto.Scalar) *textproto.Message {
	if document, alreadyParsed := p.embeddedDocuments[data]; alreadyParsed {
		return document
	}
	document, syntaxErrors := data.Embedded()
	p.recordSyntaxErrors(filePath, syntaxErrors)
	p.embeddedDocuments[data] = document
	return document
}

func (p *Project) recordSyntaxErrors(filePath string, syntaxErrors []textproto.SyntaxError) {
	for _, syntaxError := range syntaxErrors {
		p.SyntaxErrors = append(p.SyntaxErrors, SyntaxError{
			Location: Location{File: filePath, Position: syntaxError.Position},
			Message:  syntaxError.Message,
		})
	}
}

func topLevelDirectoryOf(resourcePath string) string {
	withoutLeadingSlash := strings.TrimPrefix(resourcePath, "/")
	directory, _, isInsideDirectory := strings.Cut(withoutLeadingSlash, "/")
	if !isInsideDirectory {
		return ""
	}
	return directory
}

package insight

import (
	"bytes"
	"fmt"
	"path"
	"regexp"
	"slices"
	"strings"

	"github.com/Lanterna-Games/defold-tools/internal/source"
	"github.com/Lanterna-Games/defold-tools/internal/textproto"
)

const (
	fieldOfADefaultAnimation = "default_animation"
	fieldOfTheChildren       = "children"
	startOfAResourcePath     = "/"
	mostNamesInADescription  = 12
)

var (
	extensionsAFieldTakes = map[string][]string{
		"component":        {".script", ".sprite", ".label", ".collisionobject", ".factory", ".collectionfactory", ".collectionproxy", ".gui", ".sound", ".tilemap", ".particlefx", ".model", ".mesh", ".camera", ".spinemodel"},
		"prototype":        {".go"},
		"collection":       {".collection"},
		"image":            {".png", ".jpg", ".jpeg"},
		"tile_set":         {".atlas", ".tilesource"},
		"texture":          {".atlas", ".tilesource"},
		"material":         {".material"},
		"font":             {".font", ".ttf", ".otf", ".fnt"},
		"script":           {".gui_script", ".render_script"},
		"template":         {".gui"},
		"sound":            {".wav", ".ogg"},
		"collision":        {".png", ".jpg", ".jpeg"},
		"collision_shape":  {".tilemap", ".convexshape"},
		"vertex_program":   {".vp", ".glsl"},
		"fragment_program": {".fp", ".glsl"},
		"mesh":             {".gltf", ".glb", ".dae", ".buffer"},
		"skeleton":         {".gltf", ".glb", ".dae"},
		"animations":       {".gltf", ".glb", ".dae", ".animationset"},
		"vertices":         {".buffer"},
		"particlefx":       {".particlefx"},
		"spine_scene":      {".spinescene"},
		"spine_json":       {".spinejson", ".json"},
		"atlas":            {".atlas"},
	}
	textureNamedNearby      = regexp.MustCompile(`/[^"\\\r\n]+\.(?:atlas|tilesource)`)
	collectionFactoryNearby = regexp.MustCompile(`type:\s*\\*"collectionfactory\\*"`)
	factoryNearby           = regexp.MustCompile(`type:\s*\\*"factory\\*"`)
)

const (
	fieldOfAPrototype            = "prototype"
	blockOfAPlacedGameObject     = "instances"
	extensionOfACollectionSpawn  = ".collectionfactory"
	extensionOfAGameObjectSpawn  = ".factory"
	extensionOfAGameObject       = ".go"
	extensionOfAPlacedCollection = ".collection"
)

func whatAPrototypeIs(file string, content []byte, offset int) []string {
	around := blockAround(content, offset)
	spawnsCollections, spawnsGameObjects := collectionFactoryNearby.Match(around), factoryNearby.Match(around)
	switch {
	case bytes.HasPrefix(bytes.TrimSpace(around), []byte(blockOfAPlacedGameObject)), path.Ext(file) == extensionOfAGameObjectSpawn:
		return []string{extensionOfAGameObject}
	case path.Ext(file) == extensionOfACollectionSpawn, spawnsCollections && !spawnsGameObjects:
		return []string{extensionOfAPlacedCollection}
	case spawnsCollections && spawnsGameObjects:
		return []string{extensionOfAGameObject, extensionOfAPlacedCollection}
	}
	return []string{extensionOfAGameObject}
}

type placeInData struct {
	field string
	start int
	end   int
}

func isOfAFieldName(character byte) bool {
	return character == '_' || (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') || (character >= '0' && character <= '9')
}

func placeInDataAt(content []byte, offset int) (place placeInData, isInAValue bool) {
	if offset < 0 || offset > len(content) {
		return placeInData{}, false
	}
	place.start, place.end = ValueAround(content, offset)
	before := place.start - 1
	if before < 0 || content[before] != '"' {
		return placeInData{}, false
	}
	for before--; before >= 0 && content[before] == '\\'; before-- {
	}
	for ; before >= 0 && content[before] == ' '; before-- {
	}
	if before < 0 || content[before] != ':' {
		return placeInData{}, false
	}
	nameEnd := before
	for before--; before >= 0 && isOfAFieldName(content[before]); before-- {
	}
	place.field = string(content[before+1 : nameEnd])
	return place, place.field != ""
}

func blockAround(content []byte, offset int) []byte {
	locate := source.NewLocator(content)
	line := locate(offset).Line
	document, _ := textproto.Parse(content, locate)
	lines := strings.SplitAfter(string(content), "\n")
	for index, field := range document.Fields {
		if field.Message == nil || field.Position.Line > line {
			continue
		}
		last := len(lines)
		if index+1 < len(document.Fields) {
			last = document.Fields[index+1].Position.Line - 1
		}
		if line <= last {
			return []byte(strings.Join(lines[field.Position.Line-1:min(last, len(lines))], ""))
		}
	}
	return content
}

func idsOfTheInstancesIn(content []byte) []string {
	document, _ := textproto.Parse(content, source.NewLocator(content))
	var ids []string
	for _, field := range document.Fields {
		if field.Message != nil && (field.Name == "instances" || field.Name == "embedded_instances") {
			ids = append(ids, field.Message.Text("id"))
		}
	}
	return ids
}

func (i Insight) CompleteInData(file string, content []byte, offset int) (candidates []Candidate, start, end int) {
	place, isInAValue := placeInDataAt(content, offset)
	if !isInAValue {
		return nil, 0, 0
	}
	typed := string(content[place.start:place.end])
	extensions, takesAFile := extensionsAFieldTakes[place.field]
	if place.field == fieldOfAPrototype {
		extensions = whatAPrototypeIs(file, content, offset)
	}
	switch {
	case place.field == fieldOfADefaultAnimation:
		names := i.Project.EveryAnimation()
		if texture := textureNamedNearby.Find(blockAround(content, offset)); texture != nil {
			if ofTheTexture, isRead := i.Project.AnimationsOf([]string{string(texture)}); isRead {
				names = ofTheTexture
			}
		}
		for _, name := range names {
			candidates = append(candidates, Candidate{Label: name, Detail: "animation"})
		}
	case place.field == fieldOfTheChildren:
		for _, id := range idsOfTheInstancesIn(content) {
			candidates = append(candidates, Candidate{Label: id, Detail: "game object of this collection"})
		}
	case takesAFile || strings.HasPrefix(typed, startOfAResourcePath):
		for _, file := range i.Project.Files() {
			if !takesAFile || slices.Contains(extensions, path.Ext(file)) {
				candidates = append(candidates, Candidate{Label: file, Detail: strings.TrimPrefix(path.Ext(file), ".")})
			}
		}
	}
	return candidates, place.start, place.end
}

func listed(names []string) string {
	if len(names) > mostNamesInADescription {
		return strings.Join(names[:mostNamesInADescription], ", ") + fmt.Sprintf(" and %d more", len(names)-mostNamesInADescription)
	}
	return strings.Join(names, ", ")
}

func (i Insight) DescribeFile(content []byte, offset int) (markdown string, start, end int, isSomewhere bool) {
	named, isAFile := i.FileNamedAt(content, offset)
	if !isAFile {
		return "", 0, 0, false
	}
	start, end = ValueAround(content, offset)
	said := []string{"**" + named + "**"}
	switch {
	case i.Project.GameObjects[named] != nil:
		var components []string
		for _, component := range i.Project.GameObjects[named].Components {
			components = append(components, fmt.Sprintf("`%s` (%s)", component.ID, component.Type))
		}
		said = append(said, "", fmt.Sprintf("Game object with %d components: %s", len(components), listed(components)))
	case i.Project.Collections[named] != nil:
		collection := i.Project.Collections[named]
		said = append(said, "", fmt.Sprintf("Collection `%s` with %d game objects and %d collections inside it", collection.Name, len(collection.Instances), len(collection.SubCollections)))
	case i.Project.Animations[named] != nil:
		said = append(said, "", fmt.Sprintf("%d animations: %s", len(i.Project.Animations[named]), listed(i.Project.Animations[named])))
	case i.Project.GUIScenes[named] != nil:
		said = append(said, "", fmt.Sprintf("GUI scene with %d nodes", len(i.Project.GUIScenes[named].NodeIDs)))
	case i.Project.Scripts[named] != nil:
		var properties []string
		for _, property := range i.Project.Scripts[named].Properties {
			properties = append(properties, "`"+property.Name+"`")
		}
		if len(properties) > 0 {
			said = append(said, "", "Script with the properties "+listed(properties))
		} else {
			said = append(said, "", "Script")
		}
	default:
		said = append(said, "", "File of the project")
	}
	return strings.Join(said, "\n"), start, end, true
}

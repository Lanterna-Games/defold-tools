package scaffold

import (
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/Lanterna-Games/defold-tools/internal/atlas"
)

const (
	extensionOfAScript     = ".script"
	extensionOfAGUIScript  = ".gui_script"
	extensionOfAModule     = ".lua"
	extensionOfAGameObject = ".go"
	extensionOfACollection = ".collection"
	extensionOfAScene      = ".gui"

	script = `function init(self)
end

function final(self)
end

function update(self, dt)
end

function on_message(self, message_id, message, sender)
end

function on_input(self, action_id, action)
end

function on_reload(self)
end
`
	module = `local M = {}

return M
`
	renderScript = `function init(self)
end

function update(self)
end

function on_message(self, message_id, message)
end
`
	label = `size {
  x: 128.0
  y: 32.0
}
text: "Label"
font: "/builtins/fonts/default.font"
material: "/builtins/fonts/label-df.material"
`
	collisionObject = `type: COLLISION_OBJECT_TYPE_KINEMATIC
mass: 0.0
friction: 0.1
restitution: 0.5
group: "default"
mask: "default"
embedded_collision_shape {
  shapes {
    shape_type: TYPE_BOX
    position {
    }
    rotation {
    }
    index: 0
    count: 3
  }
  data: 16.0
  data: 16.0
  data: 10.0
}
`
)

var madeFromAnother = map[string]struct {
	content string
	takes   []string
}{
	".tilesource":        {content: "image: %q\ntile_width: 16\ntile_height: 16\n", takes: []string{".png", ".jpg", ".jpeg"}},
	".tilemap":           {content: "tile_set: %q\nlayers {\n  id: \"layer1\"\n  z: 0.0\n  is_visible: 1\n}\n", takes: []string{".tilesource"}},
	".sprite":            {content: "tile_set: %q\ndefault_animation: \"\"\n", takes: []string{".atlas", ".tilesource"}},
	".factory":           {content: "prototype: %q\n", takes: []string{extensionOfAGameObject}},
	".collectionfactory": {content: "prototype: %q\n", takes: []string{extensionOfACollection}},
	".collectionproxy":   {content: "collection: %q\n", takes: []string{extensionOfACollection}},
	".sound":             {content: "sound: %q\n", takes: []string{".wav", ".ogg"}},
	".font":              {content: "font: %q\nmaterial: \"/builtins/fonts/font.material\"\nsize: 15\n", takes: []string{".ttf", ".otf"}},
}

var madeAsTheyAre = map[string]string{
	extensionOfAScript:    script,
	extensionOfAGUIScript: script,
	".render_script":      renderScript,
	extensionOfAModule:    module,
	".label":              label,
	".collisionobject":    collisionObject,
	".input_binding":      "",
}

var Kinds = slices.Sorted(func(yield func(string) bool) {
	for _, kind := range []string{extensionOfAGameObject, extensionOfACollection, extensionOfAScene, atlas.Extension} {
		if !yield(kind) {
			return
		}
	}
	for kind := range madeAsTheyAre {
		if !yield(kind) {
			return
		}
	}
	for kind := range madeFromAnother {
		if !yield(kind) {
			return
		}
	}
})

type Asked struct {
	File       string
	WithScript bool
	Images     []string
	From       string
}

func TakenBy(kind string) []string {
	return slices.Clone(madeFromAnother[kind].takes)
}

func TakesAScript(kind string) bool {
	return kind == extensionOfAGameObject || kind == extensionOfAScene
}

func Files(asked Asked) (files map[string]string, err error) {
	file := path.Clean(strings.TrimPrefix(asked.File, "/"))
	extension := path.Ext(file)
	name := strings.TrimSuffix(path.Base(file), extension)
	beside := func(otherExtension string) string { return strings.TrimSuffix(file, extension) + otherExtension }
	if name == "" || strings.ContainsAny(name, "\"\\ \t") {
		return nil, fmt.Errorf("%q cannot name a file of a game", asked.File)
	}
	if asked.WithScript && !TakesAScript(extension) {
		return nil, fmt.Errorf("a script can be made along with a game object or a GUI scene, not with %s", extension)
	}
	if len(asked.Images) > 0 && extension != atlas.Extension {
		return nil, fmt.Errorf("images go into an atlas, not into %s", extension)
	}
	from, isMadeFromAnother := madeFromAnother[extension]
	switch {
	case asked.From != "" && !isMadeFromAnother:
		return nil, fmt.Errorf("%s is not made from another file", extension)
	case asked.From != "" && !slices.Contains(from.takes, strings.ToLower(path.Ext(asked.From))):
		return nil, fmt.Errorf("%s is made from %s, not from %s", extension, strings.Join(from.takes, " or "), asked.From)
	case isMadeFromAnother:
		return map[string]string{file: fmt.Sprintf(from.content, asked.From)}, nil
	}
	if content, isMadeAsItIs := madeAsTheyAre[extension]; isMadeAsItIs {
		return map[string]string{file: content}, nil
	}
	switch extension {
	case extensionOfACollection:
		return map[string]string{file: fmt.Sprintf("name: %q\n", name)}, nil
	case extensionOfAGameObject:
		if !asked.WithScript {
			return map[string]string{file: ""}, nil
		}
		return map[string]string{
			file:                       fmt.Sprintf("components {\n  id: %q\n  component: %q\n}\n", name, "/"+beside(extensionOfAScript)),
			beside(extensionOfAScript): script,
		}, nil
	case extensionOfAScene:
		if !asked.WithScript {
			return map[string]string{file: "script: \"\"\n"}, nil
		}
		return map[string]string{
			file:                          fmt.Sprintf("script: %q\n", "/"+beside(extensionOfAGUIScript)),
			beside(extensionOfAGUIScript): script,
		}, nil
	case atlas.Extension:
		var content []byte
		for _, image := range asked.Images {
			content = atlas.With(content, image)
		}
		return map[string]string{file: string(content)}, nil
	}
	return nil, fmt.Errorf("%s is not a kind of file this makes: it makes %s", extension, strings.Join(Kinds, ", "))
}

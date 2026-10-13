package scaffold

import (
	"maps"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/Lanterna-Games/defold-tools/internal/check"
	"github.com/Lanterna-Games/defold-tools/internal/engine"
	"github.com/Lanterna-Games/defold-tools/internal/project"
)

func made(t *testing.T, asked Asked) map[string]string {
	t.Helper()
	files, err := Files(asked)
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func TestEachKindOfFileIsMadeTheWayTheEditorWritesIt(t *testing.T) {
	cases := map[string]struct {
		asked Asked
		want  map[string]string
	}{
		"a game object":      {asked: Asked{File: "main/enemy.go"}, want: map[string]string{"main/enemy.go": ""}},
		"a collection":       {asked: Asked{File: "/levels/cave.collection"}, want: map[string]string{"levels/cave.collection": "name: \"cave\"\n"}},
		"a module":           {asked: Asked{File: "main/physics.lua"}, want: map[string]string{"main/physics.lua": "local M = {}\n\nreturn M\n"}},
		"a GUI scene":        {asked: Asked{File: "main/hud.gui"}, want: map[string]string{"main/hud.gui": "script: \"\"\n"}},
		"an atlas of images": {asked: Asked{File: "main/ball.atlas", Images: []string{"/main/images/a.png", "/main/images/b.png"}}, want: map[string]string{"main/ball.atlas": "images {\n  image: \"/main/images/a.png\"\n}\nimages {\n  image: \"/main/images/b.png\"\n}\n"}},
	}
	for name, testCase := range cases {
		if got := made(t, testCase.asked); !maps.Equal(got, testCase.want) {
			t.Errorf("%s: made %q, want %q", name, got, testCase.want)
		}
	}

	withAScript := made(t, Asked{File: "main/enemy.go", WithScript: true})
	if withAScript["main/enemy.go"] != "components {\n  id: \"enemy\"\n  component: \"/main/enemy.script\"\n}\n" || !strings.HasPrefix(withAScript["main/enemy.script"], "function init(self)\nend\n") || len(withAScript) != 2 {
		t.Errorf("a game object with its script: %q", withAScript)
	}
	scene := made(t, Asked{File: "main/hud.gui", WithScript: true})
	if scene["main/hud.gui"] != "script: \"/main/hud.gui_script\"\n" || !strings.Contains(scene["main/hud.gui_script"], "function on_message(self, message_id, message, sender)") {
		t.Errorf("a GUI scene with its script: %q", scene)
	}
}

// What is made has to be read back as what was meant, and has to bring no problem of
// its own into the project it is added to.
func TestWhatIsMadeReadsBackWithoutAProblem(t *testing.T) {
	tree := fstest.MapFS{
		"game.project":         {Data: []byte("[project]\ntitle = Pong\n\n[bootstrap]\nmain_collection = /main/main.collectionc\n\n[input]\ngame_binding = /builtins/input/all.input_bindingc\n")},
		"main/main.collection": {Data: []byte("name: \"main\"\ninstances {\n  id: \"enemy\"\n  prototype: \"/main/enemy.go\"\n}\ninstances {\n  id: \"hud\"\n  prototype: \"/main/hud.go\"\n}\n")},
		"main/hud.go":          {Data: []byte("components {\n  id: \"hud\"\n  component: \"/main/hud.gui\"\n}\n")},
		"main/images/a.png":    {Data: []byte("")},
	}
	for _, asked := range []Asked{{File: "main/enemy.go", WithScript: true}, {File: "main/hud.gui", WithScript: true}, {File: "main/cave.collection"}, {File: "main/ball.atlas", Images: []string{"/main/images/a.png"}}} {
		for name, content := range made(t, asked) {
			tree[name] = &fstest.MapFile{Data: []byte(content)}
		}
	}

	loaded, err := project.Load(tree)
	if err != nil {
		t.Fatal(err)
	}
	reference, err := engine.Bundled()
	if err != nil {
		t.Fatal(err)
	}
	enemy := loaded.GameObjects["/main/enemy.go"]
	if enemy == nil || len(enemy.Components) != 1 || enemy.Components[0].Resource != "/main/enemy.script" || loaded.Scripts["/main/enemy.script"] == nil {
		t.Errorf("the game object does not read back with its script: %+v", enemy)
	}
	if scene := loaded.GUIScenes["/main/hud.gui"]; scene == nil || scene.Script != "/main/hud.gui_script" {
		t.Errorf("the scene does not read back with its script: %+v", scene)
	}
	if !slices.Equal(loaded.Animations["/main/ball.atlas"], []string{"a"}) || loaded.Collections["/main/cave.collection"].Name != "cave" {
		t.Errorf("the atlas or the collection does not read back: %q", loaded.Animations["/main/ball.atlas"])
	}
	for _, problem := range check.Run(loaded, reference, check.Options{}) {
		if problem.Check != "unused-file" {
			t.Errorf("what was made brings a problem: %s:%d: %s [%s]", problem.Location.File, problem.Location.Position.Line, problem.Message, problem.Check)
		}
	}
}

func TestWhatCannotBeMadeIsRefused(t *testing.T) {
	cases := map[string]struct {
		asked Asked
		want  string
	}{
		"a kind of file it does not make":  {asked: Asked{File: "main/ball.png"}, want: ".png is not a kind of file this makes"},
		"a script along with a collection": {asked: Asked{File: "main/cave.collection", WithScript: true}, want: "a script can be made along with a game object or a GUI scene"},
		"images in a game object":          {asked: Asked{File: "main/enemy.go", Images: []string{"/a.png"}}, want: "images go into an atlas"},
		"a name with a space":              {asked: Asked{File: "main/my enemy.go"}, want: "cannot name a file of a game"},
	}
	for name, testCase := range cases {
		if _, err := Files(testCase.asked); err == nil || !strings.Contains(err.Error(), testCase.want) {
			t.Errorf("%s: error = %v, want %q", name, err, testCase.want)
		}
	}
}

// The kinds that are one line pointing at another file are made from that file:
// a tile source from its image, a tile map from its tile source, a factory from
// the game object it makes. Each of these was built by the build tool of Defold
// 1.14.0 in a game made of nothing else. Without the file to make it from, the
// line is left to be filled in, where the names are completed.
func TestAFileIsMadeFromTheFileItPointsAt(t *testing.T) {
	cases := map[string]struct {
		asked Asked
		want  string
	}{
		"a tile source from an image":                {asked: Asked{File: "main/tiles.tilesource", From: "/main/tiles.png"}, want: "image: \"/main/tiles.png\"\ntile_width: 16\ntile_height: 16\n"},
		"a tile map from a tile source":              {asked: Asked{File: "main/level.tilemap", From: "/main/tiles.tilesource"}, want: "tile_set: \"/main/tiles.tilesource\"\nlayers {\n  id: \"layer1\"\n  z: 0.0\n  is_visible: 1\n}\n"},
		"a sprite from an atlas":                     {asked: Asked{File: "main/ball.sprite", From: "/main/sprites.atlas"}, want: "tile_set: \"/main/sprites.atlas\"\ndefault_animation: \"\"\n"},
		"a factory from a game object":               {asked: Asked{File: "main/bullets.factory", From: "/main/bullet.go"}, want: "prototype: \"/main/bullet.go\"\n"},
		"a collection factory":                       {asked: Asked{File: "main/levels.collectionfactory", From: "/main/cave.collection"}, want: "prototype: \"/main/cave.collection\"\n"},
		"a collection proxy":                         {asked: Asked{File: "main/cave.collectionproxy", From: "/main/cave.collection"}, want: "collection: \"/main/cave.collection\"\n"},
		"a sound":                                    {asked: Asked{File: "main/beep.sound", From: "/main/beep.WAV"}, want: "sound: \"/main/beep.WAV\"\n"},
		"a font":                                     {asked: Asked{File: "main/text.font", From: "/main/arial.ttf"}, want: "font: \"/main/arial.ttf\"\nmaterial: \"/builtins/fonts/font.material\"\nsize: 15\n"},
		"a tile source with nothing to make it from": {asked: Asked{File: "main/tiles.tilesource"}, want: "image: \"\"\ntile_width: 16\ntile_height: 16\n"},
		"an input binding, which starts empty":       {asked: Asked{File: "input/game.input_binding"}, want: ""},
	}
	for name, testCase := range cases {
		files := made(t, testCase.asked)
		if content, isMade := files[strings.TrimPrefix(testCase.asked.File, "/")]; !isMade || content != testCase.want || len(files) != 1 {
			t.Errorf("%s: made %q, want %q", name, files, testCase.want)
		}
	}
	for file, start := range map[string]string{"main/name.label": "size {\n  x: 128.0", "main/body.collisionobject": "type: COLLISION_OBJECT_TYPE_KINEMATIC\n", "main/custom.render_script": "function init(self)\nend\n\nfunction update(self)\nend\n"} {
		if content := made(t, Asked{File: file})[file]; !strings.HasPrefix(content, start) {
			t.Errorf("%s starts with %.40q, want %q", file, content, start)
		}
	}
	if !slices.Equal(TakenBy(".tilemap"), []string{".tilesource"}) || len(TakenBy(".label")) != 0 {
		t.Errorf("a tile map is made from %q and a label from %q", TakenBy(".tilemap"), TakenBy(".label"))
	}
}

func TestAFileIsNotMadeFromTheWrongKindOfFile(t *testing.T) {
	cases := map[string]struct {
		asked Asked
		want  string
	}{
		"a tile map from an image":   {asked: Asked{File: "main/level.tilemap", From: "/main/tiles.png"}, want: ".tilemap is made from .tilesource, not from /main/tiles.png"},
		"a label from anything":      {asked: Asked{File: "main/name.label", From: "/main/tiles.png"}, want: ".label is not made from another file"},
		"a game object from another": {asked: Asked{File: "main/hero.go", From: "/main/enemy.go"}, want: ".go is not made from another file"},
		"a kind this does not make":  {asked: Asked{File: "main/smoke.particlefx"}, want: ".particlefx is not a kind of file this makes"},
	}
	for name, testCase := range cases {
		if files, err := Files(testCase.asked); err == nil || !strings.Contains(err.Error(), testCase.want) {
			t.Errorf("%s: made %q, %v; want it refused with %q", name, files, err, testCase.want)
		}
	}
}

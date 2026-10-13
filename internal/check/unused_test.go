package check

import (
	"maps"
	"slices"
	"testing"
	"testing/fstest"

	"github.com/Lanterna-Games/defold-tools/internal/engine"
	"github.com/Lanterna-Games/defold-tools/internal/project"
)

func unusedIn(t *testing.T, files map[string]string) []string {
	t.Helper()
	tree := fstest.MapFS{}
	for name, content := range files {
		tree[name] = &fstest.MapFile{Data: []byte(content)}
	}
	loaded, err := project.Load(tree)
	if err != nil {
		t.Fatal(err)
	}
	reference, err := engine.Bundled()
	if err != nil {
		t.Fatal(err)
	}
	var unused []string
	for _, problem := range Run(loaded, reference, Options{}) {
		if problem.Check == unusedFileCheck {
			unused = append(unused, problem.Location.File)
		}
	}
	return unused
}

const gameWithItsOwnBinding = "[project]\ntitle = Pong\n\n[bootstrap]\nmain_collection = /main/main.collectionc\n\n[input]\ngame_binding = /input/game.input_bindingc\n"

func with(files, others map[string]string) map[string]string {
	together := maps.Clone(files)
	maps.Copy(together, others)
	return together
}

var gameThatUsesAllItHas = map[string]string{
	"game.project":             gameWithItsOwnBinding,
	"input/game.input_binding": "key_trigger {\n  input: KEY_SPACE\n  action: \"jump\"\n}\n",
	"main/main.collection":     "name: \"main\"\ninstances {\n  id: \"ball\"\n  prototype: \"/main/ball.go\"\n}\n",
	"main/ball.go":             "components {\n  id: \"ball\"\n  component: \"/main/ball.script\"\n}\ncomponents {\n  id: \"sprite\"\n  component: \"/main/ball.sprite\"\n}\n",
	"main/ball.sprite":         "tile_set: \"/main/ball.atlas\"\ndefault_animation: \"ball\"\n",
	"main/ball.atlas":          "animations {\n  id: \"ball\"\n}\n",
	"main/ball.script":         "local physics = require(\"main.physics\")\n\nfunction init(self)\n    physics.start()\nend\n",
	"main/physics.lua":         "local M = {}\n\nfunction M.start()\nend\n\nreturn M\n",
}

func TestAFileNothingUsesIsReported(t *testing.T) {
	cases := map[string]struct {
		files map[string]string
		want  []string
	}{
		"everything is used": {},
		"a script no game object runs": {
			files: map[string]string{"main/old.script": "function init(self)\nend\n"},
			want:  []string{"main/old.script"},
		},
		"a game object in no collection, with what only it uses": {
			files: map[string]string{
				"main/ghost.go":     "components {\n  id: \"ghost\"\n  component: \"/main/ghost.script\"\n}\n",
				"main/ghost.script": "function init(self)\nend\n",
			},
			want: []string{"main/ghost.go"},
		},
		"a module nothing requires": {
			files: map[string]string{"main/unused.lua": "return {}\n"},
			want:  []string{"main/unused.lua"},
		},
		"an atlas no sprite draws from": {
			files: map[string]string{"main/old.atlas": "animations {\n  id: \"old\"\n}\n"},
			want:  []string{"main/old.atlas"},
		},
	}
	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			if got := unusedIn(t, with(gameThatUsesAllItHas, testCase.files)); !slices.Equal(got, testCase.want) {
				t.Errorf("unused = %q, want %q", got, testCase.want)
			}
		})
	}
}

// A file can be used without a data file naming it. Each of these is a way a real
// project does that, and a report there would be wrong.
func TestAFileUsedInAWayThatIsNotADataFileIsLeftAlone(t *testing.T) {
	cases := map[string]map[string]string{
		"a collection nothing loads is where a game can start": {
			"main/level2.collection": "name: \"level2\"\n",
		},
		"a game object a script names for a factory": {
			"main/enemy.go":    "",
			"main/ball.script": gameThatUsesAllItHas["main/ball.script"] + "\nfunction update(self)\n    factory.set_prototype(\"#factory\", \"/main/enemy.goc\")\nend\n",
		},
		"an atlas a script property holds": {
			"main/skin.atlas":  "animations {\n  id: \"skin\"\n}\n",
			"main/ball.script": "go.property(\"skin\", resource.atlas(\"/main/skin.atlas\"))\n" + gameThatUsesAllItHas["main/ball.script"],
		},
		"a module required behind pcall": {
			"main/optional.lua": "return {}\n",
			"main/ball.script":  gameThatUsesAllItHas["main/ball.script"] + "\nlocal ok, optional = pcall(require, \"main.optional\")\n",
		},
		"modules, when one is required by a name that is built": {
			"main/level1.lua":  "return {}\n",
			"main/ball.script": gameThatUsesAllItHas["main/ball.script"] + "\nlocal level = require(\"main.level\" .. 1)\n",
		},
		"a file the settings name": {
			"main/default.render":        "script: \"/main/default.render_script\"\n",
			"main/default.render_script": "function init(self)\nend\n",
			"game.project":               gameWithItsOwnBinding + "\n[bootstrap]\nrender = /main/default.renderc\n",
		},
		"a file under a directory the settings bundle": {
			"data/tables/enemies.lua": "return {}\n",
			"game.project":            gameWithItsOwnBinding + "\n[project]\ncustom_resources = data/tables\n",
		},
		"a file another settings file names": {
			"main/test.collection": "name: \"test\"\ninstances {\n  id: \"probe\"\n  prototype: \"/main/probe.go\"\n}\n",
			"main/probe.go":        "",
			"test.settings":        "[bootstrap]\nmain_collection = /main/test.collectionc\n",
		},
		"the files of a library, which are for other projects": {
			"shared/helper.lua": "return {}\n",
			"game.project":      gameWithItsOwnBinding + "\n[library]\ninclude_dirs = shared\n",
		},
		"a file that is no part of the game": {
			"README.md":      "# Pong\n",
			"main/notes.txt": "todo\n",
			"main/ball.png":  "",
		},
	}
	for name, files := range cases {
		t.Run(name, func(t *testing.T) {
			if got := unusedIn(t, with(gameThatUsesAllItHas, files)); len(got) != 0 {
				t.Errorf("unused = %q, want none", got)
			}
		})
	}
}

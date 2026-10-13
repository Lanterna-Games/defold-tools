package insight

import (
	"errors"
	"io/fs"
	"slices"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/Lanterna-Games/defold-tools/internal/engine"
)

func treeOf(files map[string]string) fstest.MapFS {
	tree := fstest.MapFS{}
	for name, content := range files {
		tree[name] = &fstest.MapFile{Data: []byte(content)}
	}
	return tree
}

func bundledReference(t *testing.T) *engine.Reference {
	t.Helper()
	reference, err := engine.Bundled()
	if err != nil {
		t.Fatal(err)
	}
	return reference
}

func gameToMove() map[string]string {
	return map[string]string{
		"game.project":          "[project]\ntitle = Pong\ncustom_resources = /main/data\n\n[bootstrap]\nmain_collection = /main/main.collectionc\n\n[input]\ngame_binding = /builtins/input/all.input_bindingc\n",
		"main/main.collection":  "name: \"main\"\ninstances {\n  id: \"ball\"\n  prototype: \"/main/ball.go\"\n}\nembedded_instances {\n  id: \"wall\"\n  data: \"embedded_components {\\n\"\n  \"  id: \\\"sprite\\\"\\n\"\n  \"  type: \\\"sprite\\\"\\n\"\n  \"  data: \\\"tile_set: \\\\\\\"/main/sprites.atlas\\\\\\\"\\\\n\"\n  \"default_animation: \\\\\\\"ball\\\\\\\"\\\\n\"\n  \"\\\"\\n\"\n  \"}\\n\"\n  \"\"\n}\n",
		"main/ball.go":          "components {\n  id: \"ball\"\n  component: \"/main/ball.script\"\n}\nembedded_components {\n  id: \"sprite\"\n  type: \"sprite\"\n  data: \"default_animation: \\\"ball\\\"\\n\"\n  \"textures {\\n\"\n  \"  sampler: \\\"texture_sampler\\\"\\n\"\n  \"  texture: \\\"/main/sprites.atlas\\\"\\n\"\n  \"}\\n\"\n  \"\"\n}\n",
		"main/ball.script":      "local ids = require(\"main.ids\")\nlocal again = require 'main.ids'\nlocal levels = sys.load_resource(\"/main/data/levels.json\")\n-- the ball of /main/ball.go\nfunction init(self)\nend\n",
		"main/ids.lua":          "return {}\n",
		"main/idsmore.lua":      "return require(\"main.ids\")\n",
		"main/sprites.atlas":    "images {\n  image: \"/main/images/ball.png\"\n}\n",
		"main/images/ball.png":  "",
		"main/data/levels.json": "{}",
	}
}

func movedIn(t *testing.T, files map[string]string, moves ...Move) (after map[string]string, changed []string) {
	t.Helper()
	known := insightInto(t, files)
	changes, err := known.ChangesToMove(moves, func(file string) ([]byte, error) {
		content, isThere := files[file]
		if !isThere {
			return nil, fs.ErrNotExist
		}
		return []byte(content), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	after = map[string]string{}
	byFile := map[string][]Change{}
	for _, change := range changes {
		byFile[change.Where.File] = append(byFile[change.Where.File], change)
	}
	for name, content := range files {
		applied, err := Apply([]byte(content), byFile[name])
		if err != nil {
			t.Fatal(err)
		}
		if string(applied) != content {
			changed = append(changed, name)
		}
		target := name
		for _, move := range moves {
			from, to := strings.TrimPrefix(move.From, "/"), strings.TrimPrefix(move.To, "/")
			if name == from || strings.HasPrefix(name, from+"/") {
				target = to + strings.TrimPrefix(name, from)
			}
		}
		after[target] = string(applied)
	}
	slices.Sort(changed)
	return after, changed
}

// A file is named by its path in every file that uses it, so moving one means
// writing the new path in all of them: plainly, inside a component written in
// place with its quotes escaped once or twice, under the compiled name the
// settings use, and, for a Lua module, as the dotted name scripts require it
// by. A directory takes all it holds with it.
func TestMovingAFileRewritesEveryPlaceThatNamesIt(t *testing.T) {
	cases := map[string]struct {
		move    Move
		changed []string
		holds   map[string][]string
	}{
		"a game object": {
			move: Move{From: "/main/ball.go", To: "/objects/ball.go"}, changed: []string{"main/ball.script", "main/main.collection"},
			holds: map[string][]string{"main/main.collection": {`prototype: "/objects/ball.go"`}, "main/ball.script": {"-- the ball of /objects/ball.go"}},
		},
		"an atlas, named inside components written in place": {
			move: Move{From: "/main/sprites.atlas", To: "/art/sprites.atlas"}, changed: []string{"main/ball.go", "main/main.collection"},
			holds: map[string][]string{"main/ball.go": {`texture: \"/art/sprites.atlas\"\n`}, "main/main.collection": {`tile_set: \\\"/art/sprites.atlas\\\"\\n`}},
		},
		"the collection the game starts from, named by its compiled name": {
			move: Move{From: "/main/main.collection", To: "/levels/first.collection"}, changed: []string{"game.project"},
			holds: map[string][]string{"game.project": {"main_collection = /levels/first.collectionc\n"}},
		},
		"a Lua module, required by its dotted name": {
			move: Move{From: "/main/ids.lua", To: "/shared/names.lua"}, changed: []string{"main/ball.script", "main/idsmore.lua"},
			holds: map[string][]string{"main/ball.script": {`require("shared.names")`, `require 'shared.names'`}, "main/idsmore.lua": {`require("shared.names")`}},
		},
		"a directory of images": {
			move: Move{From: "/main/images", To: "/art/pictures"}, changed: []string{"main/sprites.atlas"},
			holds: map[string][]string{"main/sprites.atlas": {`image: "/art/pictures/ball.png"`}},
		},
		"a directory the settings name": {
			move: Move{From: "/main/data", To: "/content"}, changed: []string{"game.project", "main/ball.script"},
			holds: map[string][]string{"game.project": {"custom_resources = /content\n"}, "main/ball.script": {`sys.load_resource("/content/levels.json")`}},
		},
		"the directory that holds nearly everything": {
			move: Move{From: "/main", To: "/game"}, changed: []string{"game.project", "main/ball.go", "main/ball.script", "main/idsmore.lua", "main/main.collection", "main/sprites.atlas"},
			holds: map[string][]string{
				"game.project":          {"main_collection = /game/main.collectionc\n", "custom_resources = /game/data\n", "game_binding = /builtins/input/all.input_bindingc"},
				"game/ball.script":      {`require("game.ids")`, `require 'game.ids'`, `"/game/data/levels.json"`},
				"game/main.collection":  {`prototype: "/game/ball.go"`, `\\\"/game/sprites.atlas\\\"`},
				"game/sprites.atlas":    {`"/game/images/ball.png"`},
				"game/images/ball.png":  {""},
				"game/data/levels.json": {"{}"},
			},
		},
	}
	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			before := gameToMove()
			_, problemsBefore := judged(t, treeOf(before), bundledReference(t))

			after, changed := movedIn(t, before, testCase.move)

			if !slices.Equal(changed, testCase.changed) {
				t.Errorf("files changed = %q, want %q", changed, testCase.changed)
			}
			for file, texts := range testCase.holds {
				target := file
				if _, isThere := after[target]; !isThere {
					target = strings.TrimPrefix(testCase.move.To, "/") + strings.TrimPrefix(file, strings.TrimPrefix(testCase.move.From, "/"))
				}
				for _, text := range texts {
					if !strings.Contains(after[target], text) {
						t.Errorf("%s does not hold %q after the move:\n%s", target, text, after[target])
					}
				}
			}
			if _, problemsAfter := judged(t, treeOf(after), bundledReference(t)); len(problemsAfter) != len(problemsBefore) {
				t.Errorf("the checks found %q before the move and %q after it", problemsBefore, problemsAfter)
			}
		})
	}
}

// A name that only begins like the one that moved is another name, and a file
// is not moved onto another, or from where nothing is.
func TestWhatIsNotTheFileThatMovedIsLeftAlone(t *testing.T) {
	files := gameToMove()
	files["main/ball.gone"] = ""
	files["main/extra.collection"] = "name: \"extra\"\ninstances {\n  id: \"gone\"\n  prototype: \"/main/ball.gone\"\n}\ninstances {\n  id: \"more\"\n  prototype: \"/main/ball.go.bak\"\n}\n"

	after, changed := movedIn(t, files, Move{From: "/main/ball.go", To: "/objects/ball.go"})

	if !slices.Equal(changed, []string{"main/ball.script", "main/main.collection"}) || !strings.Contains(after["main/extra.collection"], `"/main/ball.gone"`) || !strings.Contains(after["main/extra.collection"], `"/main/ball.go.bak"`) {
		t.Errorf("changed %q; the collection that names other files now holds:\n%s", changed, after["main/extra.collection"])
	}
	if strings.Contains(after["main/idsmore.lua"], "objects") {
		t.Error("a module whose name only begins like another was rewritten")
	}

	known := insightInto(t, files)
	read := func(file string) ([]byte, error) { return []byte(files[file]), nil }
	if _, err := known.ChangesToMove([]Move{{From: "/main/ball.go", To: "/main/ids.lua"}}, read); err == nil || !strings.Contains(err.Error(), "/main/ids.lua is already there") {
		t.Errorf("moving onto a file gave %v", err)
	}
	if _, err := known.ChangesToMove([]Move{{From: "/main/nothing.go", To: "/objects/nothing.go"}}, read); !errors.Is(err, ErrNothingToMove) {
		t.Errorf("moving what is not there gave %v", err)
	}
}

// The other use of the same search: every place that names a file, which is
// what uses it.
func TestThePlacesThatNameAFileAreWhatUsesIt(t *testing.T) {
	files := gameToMove()
	known := insightInto(t, files)
	read := func(file string) ([]byte, error) { return []byte(files[file]), nil }

	cases := map[string][]string{
		"/main/sprites.atlas":   {"main/ball.go:11", "main/main.collection:11"},
		"/main/ids.lua":         {"main/ball.script:1", "main/ball.script:2", "main/idsmore.lua:1"},
		"/main/main.collection": {"game.project:6"},
		"/main/images/ball.png": {"main/sprites.atlas:2"},
		"/main/ball.script":     {"main/ball.go:3"},
		"/main/idsmore.lua":     nil,
	}
	for file, want := range cases {
		var got []string
		for _, place := range known.PlacesThatName(file, read) {
			got = append(got, place.Where.File+":"+strconv.Itoa(place.Where.Position.Line))
		}
		slices.Sort(got)
		if !slices.Equal(got, want) {
			t.Errorf("%s is named in %q, want %q", file, got, want)
		}
	}
}

// An atlas has no names of its own for its images: an image plays under the
// name of its file. So giving an image another name gives its animation another
// name, which the move cannot follow into every sprite and script that plays
// it. It says so, and says nothing when an image only changes directory.
func TestAnImageUnderANewNameIsAnAnimationUnderANewName(t *testing.T) {
	files := gameToMove()
	known := insightInto(t, files)
	read := func(file string) ([]byte, error) { return []byte(files[file]), nil }

	renamed, _ := known.ChangesToMove([]Move{{From: "/main/images/ball.png", To: "/main/images/sphere.png"}}, read)
	moved, _ := known.ChangesToMove([]Move{{From: "/main/images/ball.png", To: "/art/ball.png"}}, read)

	if got := AnimationsRenamedBy(renamed); !slices.Equal(got, []string{`"ball" of /main/sprites.atlas becomes "sphere"`}) {
		t.Errorf("an image given another name: %q", got)
	}
	if got := AnimationsRenamedBy(moved); len(moved) != 1 || len(got) != 0 {
		t.Errorf("an image moved under its own name: %d changes, said %q", len(moved), got)
	}
}

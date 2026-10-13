package pls

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func gameToMoveOnDisk(t *testing.T) string {
	t.Helper()
	return writeProject(t, map[string]string{
		"game.project":         "[project]\ntitle = Pong\n\n[bootstrap]\nmain_collection = /main/main.collectionc\n\n[input]\ngame_binding = /builtins/input/all.input_bindingc\n",
		"main/main.collection": "name: \"main\"\ninstances {\n  id: \"ball\"\n  prototype: \"/main/ball.go\"\n}\n",
		"main/ball.go":         "components {\n  id: \"ball\"\n  component: \"/main/ball.script\"\n}\n",
		"main/ball.script":     "local ids = require(\"main.ids\")\nfunction init(self)\nend\n",
		"main/ids.lua":         "return {}\n",
		"main/notes.txt":       "nothing names this\n",
	})
}

func contentOnDisk(t *testing.T, root, name string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}

func ranIn(t *testing.T, directory string, arguments ...string) (exitCode int, stdout, stderr string) {
	t.Helper()
	t.Chdir(directory)
	var said, complained bytes.Buffer
	exitCode = run(arguments, strings.NewReader(""), &said, &complained, workshopWithoutANetwork(t))
	return exitCode, said.String(), complained.String()
}

// Moving a file of a game from the command line: the file goes where it was
// sent, a directory made for it if need be, and every file that named it names
// it where it now is. What changed is said, a line for each file.
func TestAFileIsMovedWithEveryPlaceThatNamesIt(t *testing.T) {
	cases := map[string]struct {
		arguments []string
		said      string
		holds     map[string]string
		gone      string
	}{
		"a game object": {
			arguments: []string{"move", "main/ball.go", "objects/ball.go"}, said: "~ main/main.collection\nmain/ball.go -> objects/ball.go\n",
			holds: map[string]string{"main/main.collection": `prototype: "/objects/ball.go"`, "objects/ball.go": `component: "/main/ball.script"`}, gone: "main/ball.go",
		},
		"a module": {
			arguments: []string{"move", "main/ids.lua", "shared/names.lua"}, said: "~ main/ball.script\nmain/ids.lua -> shared/names.lua\n",
			holds: map[string]string{"main/ball.script": `require("shared.names")`, "shared/names.lua": "return {}"}, gone: "main/ids.lua",
		},
		"a directory, with files that name each other": {
			arguments: []string{"move", "main", "game"}, said: "~ game.project\n~ main/ball.go\n~ main/ball.script\n~ main/main.collection\nmain -> game\n",
			holds: map[string]string{
				"game.project": "main_collection = /game/main.collectionc", "game/main.collection": `prototype: "/game/ball.go"`,
				"game/ball.go": `component: "/game/ball.script"`, "game/ball.script": `require("game.ids")`, "game/notes.txt": "nothing names this",
			},
			gone: "main",
		},
		"a file nothing names": {
			arguments: []string{"move", "main/notes.txt", "docs/notes.txt"}, said: "main/notes.txt -> docs/notes.txt\n",
			holds: map[string]string{"docs/notes.txt": "nothing names this"}, gone: "main/notes.txt",
		},
	}
	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			root := gameToMoveOnDisk(t)

			exitCode, said, complained := ranIn(t, root, testCase.arguments...)

			if exitCode != exitSuccess || said != testCase.said {
				t.Fatalf("exit code = %d, said %q, complained %q; want %q", exitCode, said, complained, testCase.said)
			}
			for file, text := range testCase.holds {
				if content := contentOnDisk(t, root, file); !strings.Contains(content, text) {
					t.Errorf("%s holds %q, want %q in it", file, content, text)
				}
			}
			if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(testCase.gone))); err == nil {
				t.Errorf("%s is still there", testCase.gone)
			}
		})
	}
}

// A move that cannot be done whole is not done at all: nothing is rewritten and
// nothing moves.
func TestAMoveThatCannotBeDoneChangesNothing(t *testing.T) {
	cases := map[string]struct {
		arguments []string
		want      string
	}{
		"onto a file that is there":     {arguments: []string{"move", "main/ball.go", "main/ids.lua"}, want: "main/ids.lua is already there"},
		"of what is not there":          {arguments: []string{"move", "main/gone.go", "main/here.go"}, want: "main/gone.go is not there to move"},
		"out of the project":            {arguments: []string{"move", "main/ball.go", "../elsewhere/ball.go"}, want: "is not inside the project"},
		"with only one place said":      {arguments: []string{"move", "main/ball.go"}, want: "move takes what to move and where to"},
		"uses, with no file said":       {arguments: []string{"uses"}, want: "uses takes one file"},
		"uses, of a file of no project": {arguments: []string{"uses", "/nowhere/at/all.go"}, want: "is in no Defold project"},
	}
	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			root := gameToMoveOnDisk(t)
			before := contentOnDisk(t, root, "main/main.collection")

			exitCode, said, complained := ranIn(t, root, testCase.arguments...)

			if exitCode != exitCouldNotRun || said != "" || !strings.Contains(complained, testCase.want) {
				t.Errorf("exit code = %d, said %q, complained %q; want %q", exitCode, said, complained, testCase.want)
			}
			if _, err := os.Stat(filepath.Join(root, "main", "ball.go")); err != nil || contentOnDisk(t, root, "main/main.collection") != before {
				t.Error("something changed")
			}
		})
	}
}

func TestWhatUsesAFileIsPrinted(t *testing.T) {
	root := gameToMoveOnDisk(t)

	exitCode, said, complained := ranIn(t, root, "uses", "main/ball.go")
	if exitCode != exitSuccess || said != "main/main.collection:4:15\n" {
		t.Errorf("exit code = %d, said %q, complained %q", exitCode, said, complained)
	}
	if _, said, _ := ranIn(t, root, "uses", "main/ids.lua"); said != "main/ball.script:1:22\n" {
		t.Errorf("a module is used by %q", said)
	}
	if _, said, _ := ranIn(t, root, "uses", "main/notes.txt"); said != "" {
		t.Errorf("a file nothing names is used by %q", said)
	}
}

// An editor asks which kinds of file can be made, and what each is made from,
// instead of keeping a list of its own that would drift from this one.
func TestTheKindsOfFileThatCanBeMadeAreSaid(t *testing.T) {
	var stdout, stderr bytes.Buffer

	exitCode := run([]string{"new", "--kinds"}, strings.NewReader(""), &stdout, &stderr, workshopWithoutANetwork(t))

	var kinds []kindOfFile
	if err := json.Unmarshal(stdout.Bytes(), &kinds); exitCode != exitSuccess || err != nil {
		t.Fatalf("exit code = %d, stdout = %q, stderr = %q, %v", exitCode, stdout.String(), stderr.String(), err)
	}
	said := map[string]kindOfFile{}
	for _, kind := range kinds {
		said[kind.Extension] = kind
	}
	if len(said) < 17 || !said[".go"].TakesAScript || said[".atlas"].TakesAScript || !slices.Equal(said[".tilemap"].MadeFrom, []string{".tilesource"}) || said[".label"].MadeFrom == nil || len(said[".label"].MadeFrom) != 0 {
		t.Errorf("kinds = %+v", kinds)
	}
}

// From the command line, a file is made from another by naming that other, as
// it is on disk, and comes out pointing at it by its path in the project.
func TestAFileIsMadeFromAnotherOnDisk(t *testing.T) {
	root := gameToMoveOnDisk(t)

	exitCode, said, complained := ranIn(t, root, "new", "--from", "main/ball.go", "main/balls.factory")
	if exitCode != exitSuccess || said != "main/balls.factory\n" || contentOnDisk(t, root, "main/balls.factory") != "prototype: \"/main/ball.go\"\n" {
		t.Errorf("exit code = %d, said %q, complained %q", exitCode, said, complained)
	}
	if exitCode, _, complained := ranIn(t, root, "new", "--from", "main/gone.go", "main/gone.factory"); exitCode != exitCouldNotRun || !strings.Contains(complained, "main/gone.go is not there to make main/gone.factory from") {
		t.Errorf("made from a file that is not there: exit code = %d, complained %q", exitCode, complained)
	}
}

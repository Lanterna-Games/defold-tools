package pls

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func gameWithAScene(t *testing.T) string {
	t.Helper()
	files := map[string]string{
		"game.project":   "[project]\ntitle = Pong\n",
		"main/ball.png":  pictureOf(t, 32, 16),
		"main/wall.png":  pictureOf(t, 64, 20),
		"main/tiles.png": pictureOf(t, 32, 16),
		"main/wide.png":  pictureOf(t, 48, 16),
		"main/tall.png":  pictureOf(t, 16, 40),
		"server/main.go": "package main\n\nfunc main() {}\n",
	}
	written, err := os.ReadDir(filepath.Join("..", "scene", "testdata", "main"))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range written {
		content, err := os.ReadFile(filepath.Join("..", "scene", "testdata", "main", entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		files["main/"+strings.TrimSuffix(entry.Name(), ".txt")] = string(content)
	}
	return writeProject(t, files)
}

type sceneSaid struct {
	Kind    string `json:"kind"`
	File    string `json:"file"`
	Root    string `json:"root"`
	Objects int    `json:"objects"`
	Images  []struct {
		Path string `json:"path"`
	} `json:"images"`
	Items []struct {
		Kind   string     `json:"kind"`
		Owner  string     `json:"owner"`
		Across [6]float64 `json:"across"`
		Line   int        `json:"line"`
		Map    int        `json:"map"`
		Width  float64    `json:"width"`
		Alpha  float64    `json:"alpha"`
	} `json:"items"`
	Uses       []string    `json:"uses"`
	IsCutShort bool        `json:"cutShort"`
	Display    *[2]float64 `json:"display"`
}

func sceneSaidFor(t *testing.T, stdin string, arguments ...string) sceneSaid {
	t.Helper()
	var stdout, stderr bytes.Buffer
	exitCode := run(append([]string{"scene"}, arguments...), strings.NewReader(stdin), &stdout, &stderr, workshopWithoutANetwork(t))
	var said sceneSaid
	if err := json.Unmarshal(stdout.Bytes(), &said); exitCode != exitSuccess || err != nil {
		t.Fatalf("exit code = %d, stdout = %.200q, stderr = %q, %v", exitCode, stdout.String(), stderr.String(), err)
	}
	return said
}

// What an editor needs to draw a collection: every component that has a
// picture, where it ends up, and the line of the collection that brought it
// there. The places are checked against the engine where the scene is built;
// here it is the command that is held to them.
func TestACollectionIsSaidAsWhatItPlaces(t *testing.T) {
	root := gameWithAScene(t)

	said := sceneSaidFor(t, "", filepath.Join(root, "main", "main.collection"))

	if said.Kind != "scene" || said.File != "/main/main.collection" || said.Root != root || said.Objects != 8 || len(said.Items) != 11 || len(said.Images) != 3 || said.IsCutShort {
		t.Fatalf("said = %+v", said)
	}
	var deepest bool
	for _, item := range said.Items {
		if item.Owner == "/sub/deep/leaf" && item.Kind == "sprite" && item.Across[4] == -205 && item.Across[5] == 32.5 {
			deepest = true
		}
	}
	if !deepest {
		t.Errorf("the object three collections down is not where the engine has it: %+v", said.Items)
	}
}

// A GUI scene comes the same way, with the display it is laid out in. The
// display is the first thing drawn, and the node the tests of the scene hold
// to the engine is where the engine has it.
func TestAGUISceneIsSaidAsItsNodes(t *testing.T) {
	root := gameWithAScene(t)

	said := sceneSaidFor(t, "", filepath.Join(root, "main", "study.gui"))

	if said.Kind != "scene" || said.Display == nil || *said.Display != [2]float64{960, 640} || said.Objects != 24 || len(said.Items) != 23 || said.IsCutShort {
		t.Fatalf("said = %+v", said)
	}
	if first := said.Items[0]; first.Kind != "display" || first.Width != 960 || first.Alpha != 1 {
		t.Errorf("the first thing drawn is %+v, want the display", first)
	}
	var overridden bool
	for _, item := range said.Items {
		if item.Owner == "holder/leaf" && item.Kind == "panel" && item.Across[4] == 542 && item.Across[5] == 234 {
			overridden = true
		}
	}
	if !overridden || strings.Join(said.Uses, " ") != "/builtins/fonts/default.font /game.project /main/inner.gui /main/part.gui /main/tall.png /main/ui.atlas /main/wide.png" {
		t.Errorf("the node of a template is not where the engine has it, or the scene uses %v", said.Uses)
	}
}

// The file is taken as the editor holds it, and a game object is laid out on
// its own.
func TestASceneFollowsWhatIsTyped(t *testing.T) {
	root := gameWithAScene(t)
	typed := "embedded_components {\n  id: \"typed\"\n  type: \"sprite\"\n  data: \"default_animation: \\\"ball\\\"\\n\"\n  \"tile_set: \\\"/main/sprites.atlas\\\"\\n\"\n  position {\n    x: 3.0\n    y: 4.0\n  }\n}\n"

	said := sceneSaidFor(t, typed, "--stdin", filepath.Join(root, "main", "hero.go"))

	if said.Objects != 1 || len(said.Items) != 1 || said.Items[0].Kind != "sprite" || said.Items[0].Across[4] != 3 || said.Items[0].Across[5] != 4 || said.Items[0].Line != 1 {
		t.Errorf("said = %+v, want the one sprite that was typed", said)
	}
}

func TestAFileThatIsNoSceneSaysSo(t *testing.T) {
	root := gameWithAScene(t)
	cases := map[string]struct {
		arguments []string
		want      string
	}{
		"no file":              {arguments: nil, want: "scene takes one game object, collection or GUI scene"},
		"a Go source file":     {arguments: []string{filepath.Join(root, "server", "main.go")}, want: "this is not a game object of Defold: /server/main.go"},
		"another kind of file": {arguments: []string{filepath.Join(root, "main", "sprites.atlas")}, want: "/main/sprites.atlas is not a game object, a collection or a GUI scene"},
		"a file that is gone":  {arguments: []string{filepath.Join(root, "main", "gone.collection")}, want: "/main/gone.collection is not in the project"},
	}
	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer

			exitCode := run(append([]string{"scene"}, testCase.arguments...), strings.NewReader(""), &stdout, &stderr, workshopWithoutANetwork(t))

			if exitCode != exitCouldNotRun || !strings.Contains(stderr.String(), testCase.want) {
				t.Errorf("exit code = %d, stderr = %q; want %q", exitCode, stderr.String(), testCase.want)
			}
		})
	}
}

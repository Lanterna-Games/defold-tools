package analysis

import (
	"archive/zip"
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"slices"
	"testing"
	"testing/fstest"

	"github.com/Lanterna-Games/defold-tools/internal/engine"
)

const addressOfTheLibrary = "https://example.com/menus/archive/main.zip"

func archiveOf(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var written bytes.Buffer
	archive := zip.NewWriter(&written)
	for name, content := range files {
		entry, err := archive.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	return written.Bytes()
}

func nameOfTheArchiveOf(address string) string {
	digest := sha1.Sum([]byte(address))
	return ".internal/lib/" + hex.EncodeToString(digest[:]) + "-IjEyMyI=.zip"
}

var libraryOfMenus = map[string]string{
	"menus-main/game.project":            "[project]\ntitle = Menus\n\n[library]\ninclude_dirs = menus\n",
	"menus-main/README.md":               "# Menus\n",
	"menus-main/example/example.script":  "function init(self)\n    msg.post(\"#nowhere\", \"enable\")\nend\n",
	"menus-main/menus/menu.go":           "components {\n  id: \"menu\"\n  component: \"/menus/menu.script\"\n}\n",
	"menus-main/menus/menu.script":       "function on_message(self, message_id, message, sender)\n    if message_id == hash(\"show\") then\n        msg.post(\"#missing\", \"enable\")\n    end\nend\n",
	"menus-main/menus/unused_helper.lua": "return {}\n",
}

func gameThatUses(t *testing.T, archives map[string][]byte, posted string) fstest.MapFS {
	t.Helper()
	tree := fstest.MapFS{
		"game.project":         {Data: []byte("[project]\ntitle = Pong\ndependencies#0 = " + addressOfTheLibrary + "\n\n[bootstrap]\nmain_collection = /main/main.collectionc\n\n[input]\ngame_binding = /builtins/input/all.input_bindingc\n")},
		"defold-vet.toml":      {Data: []byte("strict = false\n")},
		"main/main.collection": {Data: []byte("name: \"main\"\ninstances {\n  id: \"game\"\n  prototype: \"/main/game.go\"\n}\ninstances {\n  id: \"menu\"\n  prototype: \"/menus/menu.go\"\n}\n")},
		"main/game.go":         {Data: []byte("components {\n  id: \"game\"\n  component: \"/main/game.script\"\n}\n")},
		"main/game.script":     {Data: []byte("function init(self)\n    msg.post(\"/menu#menu\", \"" + posted + "\")\nend\n")},
	}
	for name, content := range archives {
		tree[name] = &fstest.MapFile{Data: content}
	}
	return tree
}

func problemsOf(t *testing.T, tree fstest.MapFS) []string {
	t.Helper()
	result, err := Run(tree, engine.Versions{}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	found := make([]string, 0, len(result.Problems))
	for _, problem := range result.Problems {
		found = append(found, fmt.Sprintf("%s:%d: %s [%s]", problem.Location.File, problem.Location.Position.Line, problem.Message, problem.Check))
	}
	return found
}

// A project that depends on a library runs code that is not in its files, and is judged
// less for it. Once the library was fetched, by the Defold editor or by bob, its files
// are in the project's own directory and can be read like any other.
func TestALibraryThatWasFetchedIsRead(t *testing.T) {
	fetched := map[string][]byte{nameOfTheArchiveOf(addressOfTheLibrary): archiveOf(t, libraryOfMenus)}
	cases := map[string]struct {
		archives map[string][]byte
		posted   string
		want     []string
	}{
		"a message the library handles":           {archives: fetched, posted: "show"},
		"a message the library does not handle":   {archives: fetched, posted: "shwo", want: []string{`main/game.script:2: "shwo" is posted to "/menu#menu", but /menus/menu.script does not handle it (did you mean "show"?) [unhandled-message]`}},
		"the same, before the library is fetched": {posted: "shwo"},
		"the same, when another archive is there and not this one": {
			archives: map[string][]byte{nameOfTheArchiveOf("https://example.com/other.zip"): archiveOf(t, libraryOfMenus)},
			posted:   "shwo",
			want:     nil,
		},
	}
	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			if got := problemsOf(t, gameThatUses(t, testCase.archives, testCase.posted)); !slices.Equal(got, testCase.want) {
				t.Errorf("problems = %q, want %q", got, testCase.want)
			}
		})
	}
}

package insight

import (
	"slices"
	"strings"
	"testing"
)

const objectThatNamesFiles = `components {
  id: "script"
  component: "/pickups/feedback.script"
}
components {
  id: "gone"
  component: "/pickups/gone.script"
}
embedded_components {
  id: "sprite"
  type: "sprite"
  data: "default_animation: \"coin\"\n"
  "material: \"/builtins/materials/sprite.material\"\n"
  "textures {\n"
  "  sampler: \"texture_sampler\"\n"
  "  texture: \"/pickups/pickups.atlas\"\n"
  "}\n"
  ""
}
`

const collectionThatWritesAnObjectInPlace = `name: "level"
embedded_instances {
  id: "coin"
  data: "embedded_components {\n"
  "  id: \"sprite\"\n"
  "  type: \"sprite\"\n"
  "  data: \"tile_set: \\\"/pickups/pickups.atlas\\\"\\n"
  "default_animation: \\\"coin\\\"\\n"
  "\"\n"
  "}\n"
  ""
}
`

// A data file names other files by their path from the project, and that is all
// the link there is between them. With the cursor anywhere on such a path, the
// file is the one to go to. The path may be written plainly or inside the text
// of a component written in place, where its quotes are escaped once, or twice
// for a component of an object that is itself written in place.
func TestAFileNamedInADataFileIsFoundUnderTheCursor(t *testing.T) {
	known := insightInto(t, map[string]string{
		"game.project":             "[project]\ntitle = Pickups\n",
		"pickups/coin.go":          objectThatNamesFiles,
		"pickups/level.collection": collectionThatWritesAnObjectInPlace,
		"pickups/feedback.script":  "function init(self)\nend\n",
		"pickups/pickups.atlas":    "images {\n  image: \"/pickups/coin.png\"\n}\n",
		"pickups/coin.png":         "",
	})
	cases := map[string]struct {
		content string
		before  string
		want    string
	}{
		"at the start of a path":                      {content: objectThatNamesFiles, before: "/pickups/feedback.script", want: "/pickups/feedback.script"},
		"in the middle of it":                         {content: objectThatNamesFiles, before: "feedback.script", want: "/pickups/feedback.script"},
		"at its very end":                             {content: objectThatNamesFiles, before: "\"\n}\ncomponents {\n  id: \"gone\"", want: "/pickups/feedback.script"},
		"on the name of the field":                    {content: objectThatNamesFiles, before: "component: \"/pickups/feedback"},
		"on a file that is not there":                 {content: objectThatNamesFiles, before: "gone.script"},
		"on a file of the engine, which has no place": {content: objectThatNamesFiles, before: "sprite.material"},
		"on a name that is no path":                   {content: objectThatNamesFiles, before: "coin\\\""},
		"inside a component written in place":         {content: objectThatNamesFiles, before: "pickups.atlas", want: "/pickups/pickups.atlas"},
		"inside one written in place twice over":      {content: collectionThatWritesAnObjectInPlace, before: "pickups.atlas", want: "/pickups/pickups.atlas"},
		"an image an atlas names":                     {content: "images {\n  image: \"/pickups/coin.png\"\n}\n", before: "coin.png", want: "/pickups/coin.png"},
	}
	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			offset := strings.Index(testCase.content, testCase.before)
			if offset < 0 {
				t.Fatalf("%q is not in the content", testCase.before)
			}

			got, isAFile := known.FileNamedAt([]byte(testCase.content), offset)

			if got != testCase.want || isAFile != (testCase.want != "") {
				t.Errorf("file named = %q, %v; want %q", got, isAFile, testCase.want)
			}
		})
	}
	for _, outside := range []int{-1, len(objectThatNamesFiles) + 1} {
		if got, isAFile := known.FileNamedAt([]byte(objectThatNamesFiles), outside); isAFile {
			t.Errorf("an offset outside the file gave %q", got)
		}
	}
}

// Writing a data file by hand is mostly writing the names of other things: the
// file of a component, the animation of a sprite, the children of an object.
// Inside the quotes of a value, what the field takes is offered, and the whole
// value is what a choice replaces, written plainly or with its quotes escaped.
func TestTheValuesOfADataFileAreCompleted(t *testing.T) {
	known := insightInto(t, map[string]string{
		"game.project":             "[project]\ntitle = Pickups\n",
		"pickups/coin.go":          objectThatNamesFiles,
		"pickups/level.collection": "name: \"level\"\ninstances {\n  id: \"coin\"\n  prototype: \"/pickups/coin.go\"\n  children: \"\"\n}\ninstances {\n  id: \"star\"\n  prototype: \"\"\n}\n",
		"pickups/feedback.script":  "function init(self)\nend\n",
		"pickups/pickups.atlas":    "images {\n  image: \"/pickups/coin.png\"\n}\nanimations {\n  id: \"spin\"\n  images {\n    image: \"/pickups/coin.png\"\n  }\n}\n",
		"pickups/other.atlas":      "images {\n  image: \"/pickups/star.png\"\n}\n",
		"pickups/coin.png":         "",
		"pickups/star.png":         "",
	})
	level := "name: \"level\"\ninstances {\n  id: \"coin\"\n  prototype: \"/pickups/coin.go\"\n  children: \"\"\n}\ninstances {\n  id: \"star\"\n  prototype: \"\"\n}\n"
	spawner := "embedded_components {\n  id: \"spawner\"\n  type: \"collectionfactory\"\n  data: \"prototype: \\\"\\\"\\n\"\n  \"\"\n}\n"
	cases := map[string]struct {
		file     string
		content  string
		before   string
		want     []string
		replaced string
	}{
		"the file of a component":                            {content: objectThatNamesFiles, before: "feedback.script\"", want: []string{"/pickups/feedback.script"}, replaced: "/pickups/feedback.script"},
		"the game object a collection places":                {file: "pickups/level.collection", content: "name: \"level\"\ninstances {\n  id: \"star\"\n  prototype: \"\"\n}\n", before: "\"\n}\n", want: []string{"/pickups/coin.go"}, replaced: ""},
		"the game object a factory makes":                    {file: "pickups/coins.factory", content: "prototype: \"\"\n", before: "\"\n", want: []string{"/pickups/coin.go"}, replaced: ""},
		"the collection a collection factory makes":          {file: "pickups/levels.collectionfactory", content: "prototype: \"\"\n", before: "\"\n", want: []string{"/pickups/level.collection"}, replaced: ""},
		"the collection of a factory written in place":       {file: "pickups/door.go", content: spawner, before: "\\\"\\n", want: []string{"/pickups/level.collection"}, replaced: ""},
		"the children of a game object":                      {content: level, before: "\"\n}\ninstances {\n  id: \"star\"", want: []string{"coin", "star"}, replaced: ""},
		"the animation of a sprite, from the atlas it names": {content: objectThatNamesFiles, before: "coin\\\"", want: []string{"coin", "spin"}, replaced: "coin"},
		"the atlas of a sprite written in place":             {content: objectThatNamesFiles, before: "pickups.atlas", want: []string{"/pickups/other.atlas", "/pickups/pickups.atlas"}, replaced: "/pickups/pickups.atlas"},
		"an image of an atlas":                               {content: "images {\n  image: \"\"\n}\n", before: "\"\n}", want: []string{"/pickups/coin.png", "/pickups/star.png"}, replaced: ""},
		"a field that takes neither a file nor a name":       {content: objectThatNamesFiles, before: "script\"\n  component"},
		"outside the quotes of a value":                      {content: objectThatNamesFiles, before: "component: \"/pickups/feedback"},
	}
	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			offset := strings.Index(testCase.content, testCase.before)
			if offset < 0 {
				t.Fatalf("%q is not in the content", testCase.before)
			}

			candidates, start, end := known.CompleteInData(testCase.file, []byte(testCase.content), offset)

			if got := labelsOf(candidates); !slices.Equal(got, testCase.want) {
				t.Errorf("offered %q, want %q", got, testCase.want)
			}
			if len(testCase.want) > 0 && testCase.content[start:end] != testCase.replaced {
				t.Errorf("a choice replaces %q, want %q", testCase.content[start:end], testCase.replaced)
			}
		})
	}
}

// Over the path of a file, what the file is: enough to know what a line of a
// game object brings in without opening it.
func TestAFileNamedInADataFileIsDescribed(t *testing.T) {
	known := insightInto(t, map[string]string{
		"game.project":             "[project]\ntitle = Pickups\n",
		"pickups/coin.go":          objectThatNamesFiles,
		"pickups/level.collection": "name: \"level\"\ninstances {\n  id: \"coin\"\n  prototype: \"/pickups/coin.go\"\n}\n",
		"pickups/feedback.script":  "go.property(\"points\", 10)\nfunction init(self)\nend\n",
		"pickups/pickups.atlas":    "images {\n  image: \"/pickups/coin.png\"\n}\n",
		"pickups/coin.png":         "",
	})
	cases := map[string]struct {
		content string
		before  string
		want    string
	}{
		"a script":      {content: objectThatNamesFiles, before: "feedback.script", want: "Script with the properties `points`"},
		"an atlas":      {content: objectThatNamesFiles, before: "pickups.atlas", want: "1 animations: coin"},
		"a game object": {content: "prototype: \"/pickups/coin.go\"\n", before: "coin.go", want: "Game object with 3 components: `script` (script), `gone` (script), `sprite` (sprite)"},
		"a collection":  {content: "collection: \"/pickups/level.collection\"\n", before: "level.collection", want: "Collection `level` with 1 game objects and 0 collections inside it"},
		"an image":      {content: "image: \"/pickups/coin.png\"\n", before: "coin.png", want: "File of the project"},
	}
	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			said, start, end, isSomewhere := known.DescribeFile([]byte(testCase.content), strings.Index(testCase.content, testCase.before))

			if !isSomewhere || !strings.Contains(said, testCase.want) || !strings.HasPrefix(testCase.content[start:end], "/pickups/") {
				t.Errorf("said %q over %q, want %q", said, testCase.content[start:end], testCase.want)
			}
		})
	}
	if said, _, _, isSomewhere := known.DescribeFile([]byte(objectThatNamesFiles), strings.Index(objectThatNamesFiles, "gone.script")); isSomewhere {
		t.Errorf("a file that is not there was described: %q", said)
	}
}

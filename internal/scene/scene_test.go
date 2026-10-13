package scene

import (
	"bytes"
	"errors"
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"
)

func pictureOf(t *testing.T, width, height int) []byte {
	t.Helper()
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, image.NewNRGBA(image.Rect(0, 0, width, height))); err != nil {
		t.Fatal(err)
	}
	return encoded.Bytes()
}

// The game objects of the fixture end in .go.txt on disk, since a file that
// ends in .go is taken for Go source by everything that walks this repository.
func studyProject(t *testing.T) fstest.MapFS {
	t.Helper()
	project := fstest.MapFS{
		"main/ball.png":  {Data: pictureOf(t, 32, 16)},
		"main/wall.png":  {Data: pictureOf(t, 64, 20)},
		"main/tiles.png": {Data: pictureOf(t, 32, 16)},
	}
	written, err := os.ReadDir(filepath.Join("testdata", "main"))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range written {
		content, err := os.ReadFile(filepath.Join("testdata", "main", entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		project["main/"+strings.TrimSuffix(entry.Name(), ".txt")] = &fstest.MapFile{Data: content}
	}
	return project
}

func sceneOf(t *testing.T, project fstest.MapFS, resourcePath string) *Scene {
	t.Helper()
	laidOut, err := Of(Files{All: project, OnDisk: project}, resourcePath)
	if err != nil {
		t.Fatal(err)
	}
	return laidOut
}

func itemOf(t *testing.T, laidOut *Scene, owner, component, kind string) Item {
	t.Helper()
	for _, item := range laidOut.Items {
		if item.Owner == owner && item.Component == component && item.Kind == kind {
			return item
		}
	}
	t.Fatalf("no %s %s#%s among %d items", kind, owner, component, len(laidOut.Items))
	return Item{}
}

func isClose(got, want [6]float64) bool {
	for index := range got {
		if math.Abs(got[index]-want[index]) > 1e-4 {
			return false
		}
	}
	return true
}

// Where a game object ends up is where its parents put it: each one turns,
// scales and moves what it holds, a collection placed in another does the same
// to everything in it, and an object is named by the collections it came
// through. The numbers are the engine's own, printed by a script in this very
// project, for the objects whose sprite sits squarely on them. Written across,
// up and then the place, as two columns and a point.
func TestObjectsAreWhereTheEngineSaysTheyAre(t *testing.T) {
	laidOut := sceneOf(t, studyProject(t), "/main/main.collection")

	fromTheEngine := map[string][6]float64{
		"/child":         {0.517638, 1.931852, -5.795555, 1.552915, 117.320511, 60},
		"/emb":           {1.409539, -0.513030, 0.513030, 1.409539, 0, -80},
		"/sub/leaf":      {0, 0.5, -0.5, 0, -200, 40},
		"/sub/tip":       {0, 1, -0.5, 0, -215, 40},
		"/sub/deep/leaf": {0, -1.5, 1.5, 0, -205, 32.5},
	}
	for owner, want := range fromTheEngine {
		if sprite := itemOf(t, laidOut, owner, "sprite", KindOfASprite); !isClose(sprite.Across, want) {
			t.Errorf("%s is placed by %v, and the engine says %v", owner, sprite.Across, want)
		}
	}
	if laidOut.Objects != 8 {
		t.Errorf("objects = %d, want the 8 the collections place", laidOut.Objects)
	}
	wantUsed := []string{"/main/ball.png", "/main/deep.collection", "/main/ground.go", "/main/hero.go", "/main/level.tilemap", "/main/prop.go", "/main/sprites.atlas", "/main/sub.collection", "/main/tiles.png", "/main/tiles.tilesource", "/main/wall.png"}
	if !slices.Equal(laidOut.Uses, wantUsed) {
		t.Errorf("files the scene was read from = %q, want %q, so that a change to any of them shows it again", laidOut.Uses, wantUsed)
	}
}

// A component has a place of its own on its object, and a size that is its
// image's: the engine said 32 by 16 for the sprite that shows the ball, and 64
// by 20 for the one that plays an animation, which is its first frame.
func TestAComponentIsPlacedOnItsObject(t *testing.T) {
	laidOut := sceneOf(t, studyProject(t), "/main/main.collection")

	ball := itemOf(t, laidOut, "/parent", "sprite", KindOfASprite)
	if laidOut.Images[ball.Image].Path != "/main/ball.png" || ball.Width != 32 || ball.Height != 16 || ball.From != [4]int{0, 0, 32, 16} {
		t.Errorf("the sprite of the parent = %+v", ball)
	}
	if want := [6]float64{-2, 3.464102, -1.732051, -1, 112.320508, 68.660254}; !isClose(ball.Across, want) || ball.Depth != 0.1 {
		t.Errorf("the sprite of the parent is placed by %v at depth %v, want %v at 0.1", ball.Across, ball.Depth, want)
	}
	frame := itemOf(t, laidOut, "/child", "sprite", KindOfASprite)
	if laidOut.Images[frame.Image].Path != "/main/wall.png" || frame.Width != 64 || frame.Height != 20 {
		t.Errorf("the sprite of the child = %+v, want the first frame of its animation", frame)
	}
	ground := itemOf(t, laidOut, "/ground", "map", KindOfAMap)
	if want := [6]float64{1, 0, 0, 1, -58, -48}; !isClose(ground.Across, want) || ground.Depth != -0.5 || len(laidOut.Maps) != 1 || len(laidOut.Maps[ground.Map].Cells) != 2 || laidOut.Maps[ground.Map].TileWidth != 16 {
		t.Errorf("the tile map = %+v with %+v, want it placed by %v", ground, laidOut.Maps, want)
	}
}

// A game object on its own is drawn around its origin, each component with the
// lines that write it: the shapes of a collision object one by one, a label as
// the box it was given, and a component with nothing to draw as a mark.
func TestAGameObjectIsItsComponents(t *testing.T) {
	laidOut := sceneOf(t, studyProject(t), "/main/hero.go")

	box := itemOf(t, laidOut, "", "body", KindOfABox)
	sphere := itemOf(t, laidOut, "", "body", KindOfASphere)
	if box.Width != 32 || box.Height != 16 || !isClose(box.Across, [6]float64{1, 0, 0, 1, 0, -3}) || sphere.Radius != 6 || !isClose(sphere.Across, [6]float64{1, 0, 0, 1, 20, 0}) {
		t.Errorf("shapes = %+v and %+v", box, sphere)
	}
	label := itemOf(t, laidOut, "", "name", KindOfALabel)
	if label.Text != "Hero" || label.Width != 128 || label.Height != 32 || !isClose(label.Across, [6]float64{1, 0, 0, 1, 0, 30}) {
		t.Errorf("label = %+v", label)
	}
	sprite := itemOf(t, laidOut, "", "sprite", KindOfASprite)
	if sprite.Line != 1 || sprite.LastLine <= sprite.Line || box.Line <= sprite.LastLine-1 || label.Line <= box.Line {
		t.Errorf("lines: sprite %d-%d, box %d-%d, label %d-%d", sprite.Line, sprite.LastLine, box.Line, box.LastLine, label.Line, label.LastLine)
	}
	if laidOut.MinX > -64 || laidOut.MaxX < 64 || laidOut.MinY > -11 || laidOut.MaxY < 46 {
		t.Errorf("the scene reaches from %v, %v to %v, %v, which leaves the label out", laidOut.MinX, laidOut.MinY, laidOut.MaxX, laidOut.MaxY)
	}
}

// What is drawn is drawn from the back: Defold orders by depth, and so does
// the list.
func TestItemsComeFromTheBackToTheFront(t *testing.T) {
	laidOut := sceneOf(t, studyProject(t), "/main/main.collection")

	depths := make([]float64, 0, len(laidOut.Items))
	for _, item := range laidOut.Items {
		depths = append(depths, item.Depth)
	}
	if !slices.IsSorted(depths) || depths[0] != -0.5 || depths[len(depths)-1] != 0.5 {
		t.Errorf("depths = %v, want them rising from the tile map to the child", depths)
	}
}

func objectWith(component string) []byte {
	return []byte("embedded_components {\n  id: \"shown\"\n  " + component + "\n}\n")
}

// What cannot be drawn from the text is marked where it is, and nothing is
// made up for it: a component of a kind that has no picture here, a sprite
// whose image cannot be found, a shape that is not one of the plain ones, and
// an object whose file is missing.
func TestWhatCannotBeDrawnIsMarkedWhereItIs(t *testing.T) {
	cases := map[string]struct {
		file    string
		content []byte
		kind    string
	}{
		"a script":                           {file: "main/shown.go", content: []byte("components {\n  id: \"shown\"\n  component: \"/main/any.script\"\n  position {\n    x: 7.0\n    y: 9.0\n  }\n}\n"), kind: "script"},
		"a model":                            {file: "main/shown.go", content: objectWith("type: \"model\"\n  data: \"mesh: \\\"/main/ship.gltf\\\"\\n\"\n  position {\n    x: 7.0\n    y: 9.0\n  }"), kind: "model"},
		"a sprite of an atlas not there":     {file: "main/shown.go", content: objectWith("type: \"sprite\"\n  data: \"default_animation: \\\"ball\\\"\\n\"\n  \"tile_set: \\\"/lost/sprites.atlas\\\"\\n\"\n  position {\n    x: 7.0\n    y: 9.0\n  }"), kind: "sprite"},
		"a sprite of an animation not there": {file: "main/shown.go", content: objectWith("type: \"sprite\"\n  data: \"default_animation: \\\"gone\\\"\\n\"\n  \"tile_set: \\\"/main/sprites.atlas\\\"\\n\"\n  position {\n    x: 7.0\n    y: 9.0\n  }"), kind: "sprite"},
		"a collision object of a hull":       {file: "main/shown.go", content: objectWith("type: \"collisionobject\"\n  data: \"embedded_collision_shape {\\n\"\n  \"  shapes {\\n\"\n  \"    shape_type: TYPE_HULL\\n\"\n  \"  }\\n\"\n  \"}\\n\"\n  position {\n    x: 7.0\n    y: 9.0\n  }"), kind: "collisionobject"},
	}
	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			project := studyProject(t)
			project[testCase.file] = &fstest.MapFile{Data: testCase.content}

			laidOut := sceneOf(t, project, "/"+testCase.file)

			if len(laidOut.Items) != 1 || laidOut.Items[0].Kind != KindOfAMarker || laidOut.Items[0].Type != testCase.kind || !isClose(laidOut.Items[0].Across, [6]float64{1, 0, 0, 1, 7, 9}) {
				t.Errorf("items = %+v, want one mark of a %s at 7, 9", laidOut.Items, testCase.kind)
			}
		})
	}
	t.Run("an object whose file is missing", func(t *testing.T) {
		project := studyProject(t)
		project["main/shown.collection"] = &fstest.MapFile{Data: []byte("name: \"shown\"\ninstances {\n  id: \"ghost\"\n  prototype: \"/main/ghost.go\"\n  position {\n    x: 7.0\n    y: 9.0\n  }\n}\n")}

		laidOut := sceneOf(t, project, "/main/shown.collection")

		if len(laidOut.Items) != 1 || laidOut.Items[0].Kind != KindOfAMarker || laidOut.Items[0].Owner != "/ghost" || laidOut.Items[0].Type != "missing" {
			t.Errorf("items = %+v", laidOut.Items)
		}
	})
}

// A sprite can take its picture from a tile source, where an animation is a run
// of tiles and what is shown is the first of them, and it can be given a size
// by hand, which is the one it is drawn at.
func TestASpriteOfATileSourceAndASpriteSizedByHand(t *testing.T) {
	project := studyProject(t)
	project["main/tiles.tilesource"] = &fstest.MapFile{Data: []byte("image: \"/main/tiles.png\"\ntile_width: 16\ntile_height: 16\nanimations {\n  id: \"second\"\n  start_tile: 2\n  end_tile: 2\n}\n")}
	project["main/shown.go"] = &fstest.MapFile{Data: slices.Concat(
		objectWith("type: \"sprite\"\n  data: \"default_animation: \\\"second\\\"\\n\"\n  \"tile_set: \\\"/main/tiles.tilesource\\\"\\n\""),
		[]byte("embedded_components {\n  id: \"wide\"\n  type: \"sprite\"\n  data: \"default_animation: \\\"ball\\\"\\n\"\n  \"tile_set: \\\"/main/sprites.atlas\\\"\\n\"\n  \"size_mode: SIZE_MODE_MANUAL\\n\"\n  \"size {\\n\"\n  \"  x: 200.0\\n\"\n  \"  y: 10.0\\n\"\n  \"}\\n\"\n}\n"),
	)}

	laidOut := sceneOf(t, project, "/main/shown.go")

	tile := itemOf(t, laidOut, "", "shown", KindOfASprite)
	wide := itemOf(t, laidOut, "", "wide", KindOfASprite)
	if tile.From != [4]int{16, 0, 16, 16} || tile.Width != 16 || wide.Width != 200 || wide.Height != 10 || wide.From != [4]int{0, 0, 32, 16} {
		t.Errorf("the sprite of a tile = %+v, the sprite sized by hand = %+v", tile, wide)
	}
}

// A collection may be written to hold itself, by way of another or by a child
// that names its parent. Reading one must end.
func TestACollectionThatHoldsItselfIsStillRead(t *testing.T) {
	project := studyProject(t)
	project["main/loop.collection"] = &fstest.MapFile{Data: []byte("name: \"loop\"\ninstances {\n  id: \"first\"\n  prototype: \"/main/prop.go\"\n  children: \"second\"\n}\ninstances {\n  id: \"second\"\n  prototype: \"/main/prop.go\"\n  children: \"first\"\n}\ninstances {\n  id: \"alone\"\n  prototype: \"/main/prop.go\"\n}\ncollection_instances {\n  id: \"again\"\n  collection: \"/main/loop.collection\"\n}\n")}

	laidOut := sceneOf(t, project, "/main/loop.collection")

	if laidOut.Objects == 0 || laidOut.Objects > 64 {
		t.Errorf("objects = %d, want the reading to end with a few", laidOut.Objects)
	}
}

// An editor shows the file as it is typed. Only the file handed over is read
// from there.
func TestTheFileAsTheEditorHoldsItIsWhatIsLaidOut(t *testing.T) {
	project := studyProject(t)
	typed := "name: \"main\"\ninstances {\n  id: \"only\"\n  prototype: \"/main/prop.go\"\n  position {\n    x: 5.0\n    y: 6.0\n  }\n}\n"

	laidOut, err := Of(Files{All: project, OnDisk: project, Held: map[string][]byte{"main/main.collection": []byte(typed)}}, "/main/main.collection")
	if err != nil {
		t.Fatal(err)
	}

	if len(laidOut.Items) != 1 || laidOut.Items[0].Owner != "/only" || !isClose(laidOut.Items[0].Across, [6]float64{1, 0, 0, 1, 5, 6}) {
		t.Errorf("items = %+v, want what was typed", laidOut.Items)
	}
}

// To Defold a file that ends in .go is a game object, and to everyone else it
// is a source file of the Go language. One is not taken for the other, and an
// empty file is a game object with nothing in it.
func TestAGoSourceFileIsNotAGameObject(t *testing.T) {
	project := studyProject(t)
	project["server/main.go"] = &fstest.MapFile{Data: []byte("package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"serving\")\n}\n")}
	project["main/empty.go"] = &fstest.MapFile{Data: nil}

	if laidOut, err := Of(Files{All: project, OnDisk: project}, "/server/main.go"); !errors.Is(err, ErrNotAGameObject) {
		t.Errorf("a Go source file gave %+v, %v", laidOut, err)
	}
	if empty := sceneOf(t, project, "/main/empty.go"); empty.Objects != 1 || len(empty.Items) != 1 || empty.Items[0].Type != "empty" {
		t.Errorf("an empty game object gave %+v", empty)
	}
	if _, err := Of(Files{All: project, OnDisk: project}, "/main/ball.png"); err == nil || !strings.Contains(err.Error(), "is not a game object, a collection or a GUI scene") {
		t.Errorf("an image gave %v", err)
	}
}

func nestedCollections(levels, cellsOfTheMap int) fstest.MapFS {
	project := fstest.MapFS{
		"main/tiles.png":        {Data: nil},
		"main/tiles.tilesource": {Data: []byte("image: \"/main/tiles.png\"\ntile_width: 16\ntile_height: 16\n")},
		"main/prop.go":          {Data: []byte("components {\n  id: \"map\"\n  component: \"/main/level.tilemap\"\n}\n")},
	}
	var cells strings.Builder
	for index := range cellsOfTheMap {
		cells.WriteString("  cell {\n    x: " + strconv.Itoa(index%20) + "\n    y: " + strconv.Itoa(index/20) + "\n    tile: 0\n  }\n")
	}
	project["main/level.tilemap"] = &fstest.MapFile{Data: []byte("tile_set: \"/main/tiles.tilesource\"\nlayers {\n  id: \"ground\"\n  z: 0.0\n" + cells.String() + "}\n")}
	for level := range levels {
		var content strings.Builder
		content.WriteString("name: \"level\"\ninstances {\n  id: \"prop\"\n  prototype: \"/main/prop.go\"\n}\n")
		if level+1 < levels {
			for _, id := range []string{"left", "right"} {
				content.WriteString("collection_instances {\n  id: \"" + id + "\"\n  collection: \"/main/level" + strconv.Itoa(level+1) + ".collection\"\n}\n")
			}
		}
		project["main/level"+strconv.Itoa(level)+".collection"] = &fstest.MapFile{Data: []byte(content.String())}
	}
	return project
}

// A collection placed in another is laid out again each time, so a few files
// can stand for a great many objects: twelve levels that each hold the next one
// twice are four thousand objects, and one more level doubles it. The first
// version of this copied the cells of a tile map for every object that showed
// it, and a project made to stress Defold, of 134 thousand objects, took all
// the memory of the machine it was tried on. A tile map is now kept once,
// however often it is placed.
func TestATileMapPlacedManyTimesIsKeptOnce(t *testing.T) {
	project := nestedCollections(12, 100)
	project["main/tiles.png"] = &fstest.MapFile{Data: pictureOf(t, 32, 16)}

	laidOut := sceneOf(t, project, "/main/level0.collection")

	if laidOut.Objects != 4095 || len(laidOut.Items) != 4095 || laidOut.IsCutShort {
		t.Fatalf("objects = %d, items = %d, cut short = %v; want the 4095 the twelve levels hold", laidOut.Objects, len(laidOut.Items), laidOut.IsCutShort)
	}
	if len(laidOut.Maps) != 1 || len(laidOut.Maps[0].Cells) != 100 {
		t.Errorf("tile maps kept = %d, want the one they all show", len(laidOut.Maps))
	}
	for _, item := range laidOut.Items {
		if item.Kind != KindOfAMap || item.Map != 0 {
			t.Fatalf("item = %+v, want it to show the one tile map", item)
		}
	}
}

// And there is an end to it. Past a number of objects, or of cells to draw, the
// rest is left out and the scene says so: what could not be drawn in a view
// anyway is not worth the memory of the machine.
func TestASceneTooLargeToShowIsCutShort(t *testing.T) {
	t.Run("too many objects", func(t *testing.T) {
		project := nestedCollections(20, 1)
		project["main/prop.go"] = &fstest.MapFile{Data: []byte("components {\n  id: \"script\"\n  component: \"/main/prop.script\"\n}\n")}

		laidOut := sceneOf(t, project, "/main/level0.collection")

		if !laidOut.IsCutShort || laidOut.Objects > MostObjects || laidOut.Objects < MostObjects/2 || len(laidOut.Items) > MostObjects {
			t.Errorf("objects = %d, items = %d, cut short = %v; want it stopped at %d", laidOut.Objects, len(laidOut.Items), laidOut.IsCutShort, MostObjects)
		}
	})
	t.Run("too many cells", func(t *testing.T) {
		project := nestedCollections(14, 400)
		project["main/tiles.png"] = &fstest.MapFile{Data: pictureOf(t, 32, 16)}

		laidOut := sceneOf(t, project, "/main/level0.collection")

		if !laidOut.IsCutShort || laidOut.Objects > MostCells/400+1 || len(laidOut.Maps) != 1 {
			t.Errorf("objects = %d, cut short = %v, tile maps = %d; want it stopped once %d cells are placed", laidOut.Objects, laidOut.IsCutShort, len(laidOut.Maps), MostCells)
		}
	})
}

// A collection that holds itself, straight or by way of another, would never
// end. One that is already being laid out is not laid out again inside itself.
func TestACollectionIsNotLaidOutInsideItself(t *testing.T) {
	project := studyProject(t)
	project["main/first.collection"] = &fstest.MapFile{Data: []byte("name: \"first\"\ninstances {\n  id: \"prop\"\n  prototype: \"/main/prop.go\"\n}\ncollection_instances {\n  id: \"a\"\n  collection: \"/main/second.collection\"\n}\ncollection_instances {\n  id: \"b\"\n  collection: \"/main/second.collection\"\n}\n")}
	project["main/second.collection"] = &fstest.MapFile{Data: []byte("name: \"second\"\ninstances {\n  id: \"prop\"\n  prototype: \"/main/prop.go\"\n}\ncollection_instances {\n  id: \"back\"\n  collection: \"/main/first.collection\"\n}\ncollection_instances {\n  id: \"again\"\n  collection: \"/main/second.collection\"\n}\n")}

	laidOut := sceneOf(t, project, "/main/first.collection")

	if laidOut.Objects != 3 || laidOut.IsCutShort {
		t.Errorf("objects = %d, cut short = %v; want the one of the first and the one of each of the two seconds", laidOut.Objects, laidOut.IsCutShort)
	}
}

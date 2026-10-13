package pls

import (
	"archive/zip"
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func pictureOf(t *testing.T, width, height int) string {
	t.Helper()
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, image.NewNRGBA(image.Rect(0, 0, width, height))); err != nil {
		t.Fatal(err)
	}
	return encoded.String()
}

func gameWithTiles(t *testing.T) string {
	t.Helper()
	source, err := os.ReadFile(filepath.Join("..", "tiles", "testdata", "tiles.tilesource"))
	if err != nil {
		t.Fatal(err)
	}
	level, err := os.ReadFile(filepath.Join("..", "tiles", "testdata", "level.tilemap"))
	if err != nil {
		t.Fatal(err)
	}
	return writeProject(t, map[string]string{
		"game.project":          "[project]\ntitle = Pong\n",
		"main/tiles.tilesource": string(source),
		"main/level.tilemap":    string(level),
		"main/tiles.png":        pictureOf(t, 78, 58),
	})
}

func tilesSaidFor(t *testing.T, stdin string, arguments ...string) shownTiles {
	t.Helper()
	var stdout, stderr bytes.Buffer
	exitCode := run(append([]string{"tiles"}, arguments...), strings.NewReader(stdin), &stdout, &stderr, workshopWithoutANetwork(t))
	var said shownTiles
	if err := json.Unmarshal(stdout.Bytes(), &said); exitCode != exitSuccess || err != nil {
		t.Fatalf("exit code = %d, stdout = %q, stderr = %q, %v", exitCode, stdout.String(), stderr.String(), err)
	}
	return said
}

// What an editor needs to draw a tile source: the image, its size, and where
// each tile lies in it, with the animations as the tiles they run over.
func TestATileSourceIsSaidAsTheImageAndItsTiles(t *testing.T) {
	root := gameWithTiles(t)

	said := tilesSaidFor(t, "", filepath.Join(root, "main", "tiles.tilesource"))

	source := said.Source
	if said.Kind != "tilesource" || said.File != "/main/tiles.tilesource" || said.Root != root || source.Image != "/main/tiles.png" || source.ImageData != "" {
		t.Errorf("said = %+v", said)
	}
	if source.Width != 78 || source.Height != 58 || source.Columns != 4 || source.Rows != 3 || len(source.Tiles) != 12 || source.Tiles[5] != [2]int{21, 21} {
		t.Errorf("source = %+v", source)
	}
	if want := []shownAnimation{{ID: "blink", First: 1, Last: 3, Line: 6, LastLine: 12}, {ID: "last", First: 11, Last: 11, Line: 13, LastLine: 17}}; !slices.Equal(source.Animations, want) {
		t.Errorf("animations = %+v, want %+v", source.Animations, want)
	}
}

// A tile map is said with the tile source it draws from, read from the project,
// and each cell as six numbers: where it is, its tile, what was done to the
// tile, and the lines that write it. What was done is the mask the engine uses:
// one for mirrored across, two for mirrored up and down, four for a quarter
// turn, which the engine makes clockwise after the mirroring.
func TestATileMapIsSaidAsItsCells(t *testing.T) {
	root := gameWithTiles(t)

	said := tilesSaidFor(t, "", filepath.Join(root, "main", "level.tilemap"))

	if said.Kind != "tilemap" || said.Source.File != "/main/tiles.tilesource" || len(said.Source.Tiles) != 12 || len(said.Layers) != 1 {
		t.Fatalf("said = %+v", said)
	}
	wantCells := [][6]int{{0, 0, 0, 0, 6, 12}, {1, 0, 5, 1, 13, 19}, {-1, 2, 11, 6, 20, 27}}
	if layer := said.Layers[0]; layer.ID != "ground" || !layer.IsVisible || !slices.Equal(layer.Cells, wantCells) {
		t.Errorf("layer = %+v, want the cells %v", layer, wantCells)
	}
	if said.Bounds == nil || *said.Bounds != (shownBounds{MinX: -1, MinY: 0, MaxX: 1, MaxY: 2}) || said.CellsWithoutATile != 0 {
		t.Errorf("bounds = %+v, cells without a tile = %d", said.Bounds, said.CellsWithoutATile)
	}
}

// An editor shows the file as it is being typed, which is not what is on disk.
// It hands the text over, and only the file named is read from there: the tile
// source and the image a tile map leads to still come from the project.
func TestTheFileCanBeHandedOverInsteadOfReadFromDisk(t *testing.T) {
	root := gameWithTiles(t)
	typed := "tile_set: \"/main/tiles.tilesource\"\nlayers {\n  id: \"sky\"\n  z: 1.0\n  cell {\n    x: 7\n    y: 7\n    tile: 3\n  }\n  cell {\n    x: 8\n    y: 7\n    tile: 40\n  }\n}\n"

	said := tilesSaidFor(t, typed, "--stdin", filepath.Join(root, "main", "level.tilemap"))

	if len(said.Layers) != 1 || said.Layers[0].ID != "sky" || !slices.Equal(said.Layers[0].Cells, [][6]int{{7, 7, 3, 0, 5, 9}}) {
		t.Errorf("layers = %+v, want what was typed", said.Layers)
	}
	if said.CellsWithoutATile != 1 {
		t.Errorf("cells without a tile = %d, want the one that names tile 40 of 12", said.CellsWithoutATile)
	}
}

// A tile source may come from a library the project fetched, where its image is
// inside an archive and has no path an editor could load. It is then handed
// over whole.
func TestAnImageThatIsNotAFileIsHandedOverWhole(t *testing.T) {
	root := gameWithTiles(t)
	picture := pictureOf(t, 32, 16)
	library := archiveForTests(t, map[string]string{
		"tiles-main/game.project":             "[project]\ntitle = Tiles\n\n[library]\ninclude_dirs = shared\n",
		"tiles-main/shared/ground.tilesource": "image: \"/shared/ground.png\"\ntile_width: 16\ntile_height: 16\n",
		"tiles-main/shared/ground.png":        picture,
	})
	address := "https://example.com/tiles/archive/main.zip"
	for name, content := range map[string]string{
		"game.project":                    "[project]\ntitle = Pong\ndependencies#0 = " + address + "\n",
		nameOfTheArchiveForTests(address): library,
		"main/level.tilemap":              "tile_set: \"/shared/ground.tilesource\"\nlayers {\n  id: \"ground\"\n  z: 0.0\n  cell {\n    x: 0\n    y: 0\n    tile: 1\n  }\n}\n",
	} {
		target := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	said := tilesSaidFor(t, "", filepath.Join(root, "main", "level.tilemap"))

	if said.Source.Columns != 2 || said.Source.Rows != 1 || !strings.HasPrefix(said.Source.ImageData, "data:image/png;base64,") || len(said.Layers[0].Cells) != 1 {
		t.Errorf("source = %+v, layers = %+v", said.Source, said.Layers)
	}
}

func TestTilesThatCannotBeShownSayWhy(t *testing.T) {
	root := gameWithTiles(t)
	write := func(name, content string) string {
		target := filepath.Join(root, filepath.FromSlash(name))
		if err := os.WriteFile(target, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		return target
	}
	cases := map[string]struct {
		arguments []string
		want      string
	}{
		"no file":                      {arguments: nil, want: "tiles takes one tile source or tile map"},
		"another kind of file":         {arguments: []string{write("main/ball.script", "")}, want: "main/ball.script is not a tile source or a tile map"},
		"a tile source with no image":  {arguments: []string{write("main/empty.tilesource", "tile_width: 16\ntile_height: 16\n")}, want: "/main/empty.tilesource names no image"},
		"an image that is not there":   {arguments: []string{write("main/lost.tilesource", "image: \"/main/lost.png\"\ntile_width: 16\ntile_height: 16\n")}, want: "names /main/lost.png, which is not in the project"},
		"an image outside the project": {arguments: []string{write("main/out.tilesource", "image: \"/../secret.png\"\ntile_width: 16\ntile_height: 16\n")}, want: "which is not in the project"},
		"a file that is no image":      {arguments: []string{write("main/text.tilesource", "image: \"/main/level.tilemap\"\ntile_width: 16\ntile_height: 16\n")}, want: "/main/level.tilemap cannot be read as an image"},
		"tiles larger than the image":  {arguments: []string{write("main/big.tilesource", "image: \"/main/tiles.png\"\ntile_width: 512\ntile_height: 512\n")}, want: "the image holds no tile of that size"},
		"a map with no tile source":    {arguments: []string{write("main/bare.tilemap", "layers {\n  id: \"ground\"\n}\n")}, want: "/main/bare.tilemap names no tile source"},
		"a map of a lost tile source":  {arguments: []string{write("main/lost.tilemap", "tile_set: \"/main/gone.tilesource\"\n")}, want: "names /main/gone.tilesource, which is not in the project"},
	}
	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer

			exitCode := run(append([]string{"tiles"}, testCase.arguments...), strings.NewReader(""), &stdout, &stderr, workshopWithoutANetwork(t))

			if exitCode != exitCouldNotRun || !strings.Contains(stderr.String(), testCase.want) {
				t.Errorf("exit code = %d, stderr = %q; want %q", exitCode, stderr.String(), testCase.want)
			}
		})
	}
}

func archiveForTests(t *testing.T, files map[string]string) string {
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
	return written.String()
}

func nameOfTheArchiveForTests(address string) string {
	digest := sha1.Sum([]byte(address))
	return ".internal/lib/" + hex.EncodeToString(digest[:]) + "-IjEyMyI=.zip"
}

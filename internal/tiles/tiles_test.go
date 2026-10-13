package tiles

import (
	"errors"
	"os"
	"slices"
	"testing"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	content, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return content
}

// A tile source cuts one image into a grid, and the text says how: the size of a
// tile, the margin around each one and the spacing between them. Where each tile
// lies follows from that and from the size of the image, and Defold counts how
// many fit the same way for its editor and its build tool. An animation names
// its tiles counting from one, both ends included.
func TestATileSourceIsCutIntoItsTiles(t *testing.T) {
	read := SourceOf(fixture(t, "tiles.tilesource"))

	if read.Image != "/main/tiles.png" || read.TileWidth != 16 || read.TileHeight != 16 || read.Margin != 1 || read.Spacing != 2 {
		t.Errorf("source = %+v", read)
	}
	wantAnimations := []Animation{{ID: "blink", First: 1, Last: 3, Line: 6, LastLine: 12}, {ID: "last", First: 11, Last: 11, Line: 13, LastLine: 17}}
	if !slices.Equal(read.Animations, wantAnimations) {
		t.Errorf("animations = %+v, want %+v", read.Animations, wantAnimations)
	}

	grid, err := read.GridOver(78, 58)
	if err != nil {
		t.Fatal(err)
	}

	if grid.Columns != 4 || grid.Rows != 3 || len(grid.Tiles) != 12 {
		t.Fatalf("grid = %d columns, %d rows, %d tiles; want 4, 3 and 12", grid.Columns, grid.Rows, len(grid.Tiles))
	}
	for tile, want := range map[int]Point{0: {X: 1, Y: 1}, 1: {X: 21, Y: 1}, 5: {X: 21, Y: 21}, 11: {X: 61, Y: 41}} {
		if grid.Tiles[tile] != want {
			t.Errorf("tile %d is at %+v, want %+v", tile, grid.Tiles[tile], want)
		}
	}
}

// How many tiles fit is counted as Defold counts it: what is left over at the
// right and at the bottom is no tile, and the spacing is not owed after the
// last one.
func TestHowManyTilesFitAnImage(t *testing.T) {
	cases := map[string]struct {
		source      Source
		width       int
		height      int
		wantColumns int
		wantRows    int
	}{
		"tiles edge to edge":           {source: Source{TileWidth: 16, TileHeight: 16}, width: 64, height: 32, wantColumns: 4, wantRows: 2},
		"a strip left over":            {source: Source{TileWidth: 16, TileHeight: 16}, width: 70, height: 47, wantColumns: 4, wantRows: 2},
		"spacing, none after the last": {source: Source{TileWidth: 16, TileHeight: 16, Spacing: 2}, width: 70, height: 34, wantColumns: 4, wantRows: 2},
		"a margin around each":         {source: Source{TileWidth: 16, TileHeight: 16, Margin: 1}, width: 72, height: 36, wantColumns: 4, wantRows: 2},
		"tiles that are not square":    {source: Source{TileWidth: 32, TileHeight: 8}, width: 64, height: 32, wantColumns: 2, wantRows: 4},
		"one tile, the whole image":    {source: Source{TileWidth: 64, TileHeight: 64}, width: 64, height: 64, wantColumns: 1, wantRows: 1},
	}
	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			grid, err := testCase.source.GridOver(testCase.width, testCase.height)

			if err != nil || grid.Columns != testCase.wantColumns || grid.Rows != testCase.wantRows {
				t.Errorf("grid = %d by %d, %v; want %d by %d", grid.Columns, grid.Rows, err, testCase.wantColumns, testCase.wantRows)
			}
		})
	}
}

func TestAnImageWithoutATileOfThatSizeHasNoGrid(t *testing.T) {
	cases := map[string]Source{
		"a tile larger than the image": {TileWidth: 128, TileHeight: 16},
		"no size written":              {},
		"a size below nothing":         {TileWidth: -16, TileHeight: 16},
	}
	for name, read := range cases {
		t.Run(name, func(t *testing.T) {
			if grid, err := read.GridOver(64, 64); !errors.Is(err, ErrNoTiles) {
				t.Errorf("grid = %+v, %v; want none", grid, err)
			}
		})
	}
}

// A tile map is layers of cells, each a place on the grid and the number of a
// tile, counted from nothing here, with what was done to it: mirrored across,
// mirrored up and down, turned a quarter. A cell says where it is written, from
// its first line to its last, so that an editor can lead from one to the other.
func TestATileMapIsReadLayerByLayer(t *testing.T) {
	read := MapOf(fixture(t, "level.tilemap"))

	if read.TileSource != "/main/tiles.tilesource" || len(read.Layers) != 1 {
		t.Fatalf("map = %+v", read)
	}
	ground := read.Layers[0]
	if ground.ID != "ground" || ground.Z != 0 || !ground.IsVisible || ground.Line != 2 {
		t.Errorf("layer = %+v", ground)
	}
	want := []Cell{
		{X: 0, Y: 0, Tile: 0, Line: 6, LastLine: 12},
		{X: 1, Y: 0, Tile: 5, IsFlippedAcross: true, Line: 13, LastLine: 19},
		{X: -1, Y: 2, Tile: 11, IsFlippedUpDown: true, IsTurned: true, Line: 20, LastLine: 27},
	}
	if !slices.Equal(ground.Cells, want) {
		t.Errorf("cells = %+v, want %+v", ground.Cells, want)
	}
}

// A layer shows unless it says it does not: the setting is not written by every
// version of Defold's editor.
func TestALayerShowsUnlessItSaysOtherwise(t *testing.T) {
	read := MapOf([]byte("tile_set: \"/main/tiles.tilesource\"\nlayers {\n  id: \"shown\"\n  z: 0.5\n}\nlayers {\n  id: \"hidden\"\n  z: -1.0\n  is_visible: 0\n}\n"))

	if len(read.Layers) != 2 || !read.Layers[0].IsVisible || read.Layers[0].Z != 0.5 || read.Layers[1].IsVisible || read.Layers[1].Z != -1 {
		t.Errorf("layers = %+v", read.Layers)
	}
}

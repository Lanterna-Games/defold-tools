package tiles

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/Lanterna-Games/defold-tools/internal/source"
	"github.com/Lanterna-Games/defold-tools/internal/textproto"
)

const (
	SourceExtension = ".tilesource"
	MapExtension    = ".tilemap"
	wordForYes      = "1"
)

var ErrNoTiles = errors.New("the image holds no tile of that size")

type Animation struct {
	ID              string
	First           int
	Last            int
	IsFlippedAcross bool
	IsFlippedUpDown bool
	Line            int
	LastLine        int
}

type Source struct {
	Image      string
	TileWidth  int
	TileHeight int
	Margin     int
	Spacing    int
	Animations []Animation
}

type Point struct {
	X int
	Y int
}

type Grid struct {
	Width   int
	Height  int
	Columns int
	Rows    int
	Tiles   []Point
}

type Cell struct {
	X               int
	Y               int
	Tile            int
	IsFlippedAcross bool
	IsFlippedUpDown bool
	IsTurned        bool
	Line            int
	LastLine        int
}

type Layer struct {
	ID        string
	Z         float64
	IsVisible bool
	Line      int
	Cells     []Cell
}

type Map struct {
	TileSource string
	Layers     []Layer
}

func numberIn(message *textproto.Message, name string) int {
	number, _ := strconv.Atoi(message.Text(name))
	return number
}

func lastLineOf(field textproto.Field) int {
	last := field.Position.Line
	for _, inner := range field.Message.Fields {
		last = max(last, inner.Position.Line)
	}
	return last + 1
}

func SourceOf(content []byte) Source {
	document, _ := textproto.Parse(content, source.NewLocator(content))
	read := Source{
		Image:      document.Text("image"),
		TileWidth:  numberIn(document, "tile_width"),
		TileHeight: numberIn(document, "tile_height"),
		Margin:     numberIn(document, "tile_margin"),
		Spacing:    numberIn(document, "tile_spacing"),
	}
	for _, field := range document.Fields {
		if field.Name != "animations" || field.Message == nil {
			continue
		}
		read.Animations = append(read.Animations, Animation{
			ID:              field.Message.Text("id"),
			First:           numberIn(field.Message, "start_tile") - 1,
			Last:            numberIn(field.Message, "end_tile") - 1,
			IsFlippedAcross: numberIn(field.Message, "flip_horizontal") != 0,
			IsFlippedUpDown: numberIn(field.Message, "flip_vertical") != 0,
			Line:            field.Position.Line,
			LastLine:        lastLineOf(field),
		})
	}
	return read
}

func tilesAlong(tileSize, imageSize, margin, spacing int) int {
	taken := 2*margin + spacing + tileSize
	if tileSize <= 0 || taken <= 0 {
		return 0
	}
	return (imageSize + spacing) / taken
}

func (s Source) GridOver(imageWidth, imageHeight int) (Grid, error) {
	grid := Grid{
		Width:   imageWidth,
		Height:  imageHeight,
		Columns: tilesAlong(s.TileWidth, imageWidth, s.Margin, s.Spacing),
		Rows:    tilesAlong(s.TileHeight, imageHeight, s.Margin, s.Spacing),
	}
	if grid.Columns <= 0 || grid.Rows <= 0 {
		return Grid{}, fmt.Errorf("%w: the image is %d by %d and a tile %d by %d", ErrNoTiles, imageWidth, imageHeight, s.TileWidth, s.TileHeight)
	}
	grid.Tiles = make([]Point, 0, grid.Columns*grid.Rows)
	for row := range grid.Rows {
		for column := range grid.Columns {
			grid.Tiles = append(grid.Tiles, Point{
				X: s.Margin + column*(s.TileWidth+2*s.Margin+s.Spacing),
				Y: s.Margin + row*(s.TileHeight+2*s.Margin+s.Spacing),
			})
		}
	}
	return grid, nil
}

func MapOf(content []byte) Map {
	document, _ := textproto.Parse(content, source.NewLocator(content))
	read := Map{TileSource: document.Text("tile_set")}
	for _, field := range document.Fields {
		if field.Name != "layers" || field.Message == nil {
			continue
		}
		depth, _ := strconv.ParseFloat(field.Message.Text("z"), 64)
		_, saysWhetherItShows := field.Message.Scalar("is_visible")
		layer := Layer{
			ID:        field.Message.Text("id"),
			Z:         depth,
			IsVisible: !saysWhetherItShows || field.Message.Text("is_visible") == wordForYes,
			Line:      field.Position.Line,
		}
		for _, inner := range field.Message.Fields {
			if inner.Name != "cell" || inner.Message == nil {
				continue
			}
			layer.Cells = append(layer.Cells, Cell{
				X:               numberIn(inner.Message, "x"),
				Y:               numberIn(inner.Message, "y"),
				Tile:            numberIn(inner.Message, "tile"),
				IsFlippedAcross: inner.Message.Text("h_flip") == wordForYes,
				IsFlippedUpDown: inner.Message.Text("v_flip") == wordForYes,
				IsTurned:        inner.Message.Text("rotate90") == wordForYes,
				Line:            inner.Position.Line,
				LastLine:        lastLineOf(inner),
			})
		}
		read.Layers = append(read.Layers, layer)
	}
	return read
}

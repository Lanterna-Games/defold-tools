package pls

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/Lanterna-Games/defold-tools/internal/libraries"
	"github.com/Lanterna-Games/defold-tools/internal/tiles"
)

const (
	commandThatShowsTiles = "tiles"
	kindOfATileSource     = "tilesource"
	kindOfATileMap        = "tilemap"
	flippedAcross         = 1
	flippedUpDown         = 2
	turnedAQuarter        = 4
	startOfAnImageInline  = "data:%s;base64,%s"
)

type shownAnimation struct {
	ID       string `json:"id"`
	First    int    `json:"first"`
	Last     int    `json:"last"`
	Line     int    `json:"line"`
	LastLine int    `json:"lastLine"`
}

type shownTileSource struct {
	File       string           `json:"file"`
	Image      string           `json:"image"`
	ImageData  string           `json:"imageData,omitempty"`
	Width      int              `json:"width"`
	Height     int              `json:"height"`
	TileWidth  int              `json:"tileWidth"`
	TileHeight int              `json:"tileHeight"`
	Columns    int              `json:"columns"`
	Rows       int              `json:"rows"`
	Tiles      [][2]int         `json:"tiles"`
	Animations []shownAnimation `json:"animations"`
}

type shownLayer struct {
	ID        string   `json:"id"`
	Z         float64  `json:"z"`
	IsVisible bool     `json:"visible"`
	Line      int      `json:"line"`
	Cells     [][6]int `json:"cells"`
}

type shownBounds struct {
	MinX int `json:"minX"`
	MinY int `json:"minY"`
	MaxX int `json:"maxX"`
	MaxY int `json:"maxY"`
}

type shownTiles struct {
	Kind              string          `json:"kind"`
	File              string          `json:"file"`
	Root              string          `json:"root"`
	Source            shownTileSource `json:"source"`
	Layers            []shownLayer    `json:"layers,omitempty"`
	Bounds            *shownBounds    `json:"bounds,omitempty"`
	CellsWithoutATile int             `json:"cellsWithoutATile,omitempty"`
}

type projectFiles struct {
	all    fs.FS
	onDisk fs.FS
}

func tileSourceToShow(project projectFiles, resourcePath string, content []byte) (shownTileSource, error) {
	read := tiles.SourceOf(content)
	if read.Image == "" {
		return shownTileSource{}, fmt.Errorf("%s names no image", resourcePath)
	}
	nameOfTheImage := strings.TrimPrefix(read.Image, "/")
	picture, err := fs.ReadFile(project.all, nameOfTheImage)
	if err != nil {
		return shownTileSource{}, fmt.Errorf("%s names %s, which is not in the project", resourcePath, read.Image)
	}
	size, _, err := image.DecodeConfig(bytes.NewReader(picture))
	if err != nil {
		return shownTileSource{}, fmt.Errorf("%s cannot be read as an image: %w", read.Image, err)
	}
	grid, err := read.GridOver(size.Width, size.Height)
	if err != nil {
		return shownTileSource{}, fmt.Errorf("%s: %w", resourcePath, err)
	}
	shown := shownTileSource{
		File: resourcePath, Image: read.Image, Width: grid.Width, Height: grid.Height,
		TileWidth: read.TileWidth, TileHeight: read.TileHeight, Columns: grid.Columns, Rows: grid.Rows,
		Tiles: make([][2]int, 0, len(grid.Tiles)), Animations: []shownAnimation{},
	}
	if _, err := fs.Stat(project.onDisk, nameOfTheImage); err != nil {
		shown.ImageData = fmt.Sprintf(startOfAnImageInline, http.DetectContentType(picture), base64.StdEncoding.EncodeToString(picture))
	}
	for _, tile := range grid.Tiles {
		shown.Tiles = append(shown.Tiles, [2]int{tile.X, tile.Y})
	}
	for _, animation := range read.Animations {
		shown.Animations = append(shown.Animations, shownAnimation{ID: animation.ID, First: animation.First, Last: animation.Last, Line: animation.Line, LastLine: animation.LastLine})
	}
	return shown, nil
}

func tileMapToShow(project projectFiles, resourcePath string, content []byte) (shownTiles, error) {
	read := tiles.MapOf(content)
	if read.TileSource == "" {
		return shownTiles{}, fmt.Errorf("%s names no tile source", resourcePath)
	}
	sourceContent, err := fs.ReadFile(project.all, strings.TrimPrefix(read.TileSource, "/"))
	if err != nil {
		return shownTiles{}, fmt.Errorf("%s names %s, which is not in the project", resourcePath, read.TileSource)
	}
	source, err := tileSourceToShow(project, read.TileSource, sourceContent)
	if err != nil {
		return shownTiles{}, err
	}
	shown := shownTiles{Kind: kindOfATileMap, File: resourcePath, Source: source}
	for _, layer := range read.Layers {
		drawn := shownLayer{ID: layer.ID, Z: layer.Z, IsVisible: layer.IsVisible, Line: layer.Line, Cells: make([][6]int, 0, len(layer.Cells))}
		for _, cell := range layer.Cells {
			if cell.Tile < 0 || cell.Tile >= len(source.Tiles) {
				shown.CellsWithoutATile++
				continue
			}
			transform := 0
			if cell.IsFlippedAcross {
				transform |= flippedAcross
			}
			if cell.IsFlippedUpDown {
				transform |= flippedUpDown
			}
			if cell.IsTurned {
				transform |= turnedAQuarter
			}
			drawn.Cells = append(drawn.Cells, [6]int{cell.X, cell.Y, cell.Tile, transform, cell.Line, cell.LastLine})
			if shown.Bounds == nil {
				shown.Bounds = &shownBounds{MinX: cell.X, MinY: cell.Y, MaxX: cell.X, MaxY: cell.Y}
			}
			shown.Bounds.MinX, shown.Bounds.MaxX = min(shown.Bounds.MinX, cell.X), max(shown.Bounds.MaxX, cell.X)
			shown.Bounds.MinY, shown.Bounds.MaxY = min(shown.Bounds.MinY, cell.Y), max(shown.Bounds.MaxY, cell.Y)
		}
		shown.Layers = append(shown.Layers, drawn)
	}
	return shown, nil
}

func showTiles(args []string, stdin io.Reader, stdout io.Writer, fail func(error) int) int {
	flags := flag.NewFlagSet(commandThatShowsTiles, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	isOnStandardInput := flags.Bool("stdin", false, "")
	if err := flags.Parse(args); err != nil {
		return fail(fmt.Errorf("%s: %w", commandThatShowsTiles, err))
	}
	if flags.NArg() != 1 {
		return fail(fmt.Errorf("%s takes one tile source or tile map, as in %q", commandThatShowsTiles, "defoldpls tiles main/level.tilemap"))
	}
	root, file, err := rootOfTheProjectHolding(flags.Arg(0))
	if err != nil {
		return fail(err)
	}
	onDisk := os.DirFS(root)
	project := projectFiles{all: libraries.Mount(onDisk).FS, onDisk: onDisk}
	var content []byte
	if *isOnStandardInput {
		content, err = io.ReadAll(stdin)
	} else {
		content, err = fs.ReadFile(project.all, file)
	}
	if err != nil {
		return fail(err)
	}
	resourcePath := "/" + file
	var shown shownTiles
	switch path.Ext(file) {
	case tiles.SourceExtension:
		shown.Kind, shown.File = kindOfATileSource, resourcePath
		shown.Source, err = tileSourceToShow(project, resourcePath, content)
	case tiles.MapExtension:
		shown, err = tileMapToShow(project, resourcePath, content)
	default:
		err = errors.New(file + " is not a tile source or a tile map")
	}
	if err != nil {
		return fail(err)
	}
	shown.Root = root
	if err := json.NewEncoder(stdout).Encode(shown); err != nil {
		return fail(err)
	}
	return exitSuccess
}

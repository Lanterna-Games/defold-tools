package pls

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path"
	"path/filepath"

	"github.com/Lanterna-Games/defold-tools/internal/atlas"
	"github.com/Lanterna-Games/defold-tools/internal/build"
)

const commandThatPacksAnAtlas = "pack"

type packedImage struct {
	Image     string `json:"image"`
	Name      string `json:"name"`
	Animation string `json:"animation,omitempty"`
	Line      int    `json:"line"`
	Page      int    `json:"page"`
	X         int    `json:"x"`
	Y         int    `json:"y"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	IsRotated bool   `json:"rotated"`
}

type packedAtlas struct {
	Atlas  string        `json:"atlas"`
	Root   string        `json:"root"`
	Defold string        `json:"defold"`
	Width  int           `json:"width"`
	Height int           `json:"height"`
	Pages  int           `json:"pages"`
	Images []packedImage `json:"images"`
}

func packAnAtlas(args []string, stdout, stderr io.Writer, shop *workshop, fail func(error) int) int {
	flags := flag.NewFlagSet(commandThatPacksAnAtlas, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	home := flags.String("defold-home", "", "")
	if err := flags.Parse(args); err != nil {
		return fail(fmt.Errorf("%s: %w", commandThatPacksAnAtlas, err))
	}
	if flags.NArg() != 1 {
		return fail(fmt.Errorf("%s takes one atlas, as in %q", commandThatPacksAnAtlas, "defoldpls pack main/sprites.atlas"))
	}
	root, file, err := rootOfTheProjectHolding(flags.Arg(0))
	if err != nil {
		return fail(err)
	}
	if path.Ext(file) != atlas.Extension {
		return fail(fmt.Errorf("%s is not an atlas", file))
	}
	if shop.cache == "" {
		return fail(errors.New("this machine has no cache directory to pack the atlas in"))
	}
	userHome, _ := os.UserHomeDir()
	installed, err := build.FindInstallation(*home, os.Getenv, userHome)
	if err != nil {
		return fail(err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	atlasPath := "/" + file
	packed, err := installed.PackAnAtlas(ctx, root, atlasPath, shop.cache, stderr)
	if err != nil {
		return fail(err)
	}
	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(file))) //nolint:gosec // the atlas named on the command line
	if err != nil {
		return fail(err)
	}
	layout, err := atlas.LayoutOf(content, packed)
	if err != nil {
		return fail(err)
	}
	said := packedAtlas{Atlas: atlasPath, Root: root, Defold: installed.Version, Width: layout.Width, Height: layout.Height, Pages: layout.Pages}
	for _, placed := range layout.Images {
		said.Images = append(said.Images, packedImage{
			Image: placed.Image, Name: placed.Name, Animation: placed.Animation, Line: placed.Line,
			Page: placed.Place.Page, X: placed.Place.X, Y: placed.Place.Y, Width: placed.Place.Width, Height: placed.Place.Height, IsRotated: placed.Place.IsRotated,
		})
	}
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(said); err != nil {
		return fail(err)
	}
	return exitSuccess
}

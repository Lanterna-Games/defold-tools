package atlas

import (
	"errors"
	"fmt"
	"path"
	"strings"

	"github.com/Lanterna-Games/defold-tools/internal/source"
	"github.com/Lanterna-Games/defold-tools/internal/textproto"
	"github.com/Lanterna-Games/defold-tools/internal/textureset"
)

const (
	separatorOfRenamePatterns = ","
	separatorOfARenamePattern = "="
)

var ErrNoImages = errors.New("the atlas has no images")

type Entry struct {
	Image     string
	Name      string
	Animation string
	Line      int
}

type PlacedImage struct {
	Entry

	Place textureset.Frame
}

type Layout struct {
	Width  int
	Height int
	Pages  int
	Images []PlacedImage
}

func NameOfAnImage(imagePath, renamePatterns string) string {
	name := strings.TrimSuffix(path.Base(imagePath), path.Ext(imagePath))
	for pattern := range strings.SplitSeq(renamePatterns, separatorOfRenamePatterns) {
		search, replacement, isAPattern := strings.Cut(strings.TrimSpace(pattern), separatorOfARenamePattern)
		if isAPattern && search != "" {
			name = strings.ReplaceAll(name, search, replacement)
		}
	}
	return name
}

func EntriesOf(content []byte) []Entry {
	document, _ := textproto.Parse(content, source.NewLocator(content))
	patterns := document.Text("rename_patterns")
	entryOf := func(image *textproto.Message, animation string) (Entry, bool) {
		written, isWritten := image.Scalar("image")
		if !isWritten || written.Text == "" {
			return Entry{}, false
		}
		return Entry{Image: written.Text, Name: NameOfAnImage(written.Text, patterns), Animation: animation, Line: written.Position.Line}, true
	}
	var entries []Entry
	for _, image := range document.Messages("images") {
		if entry, isWhole := entryOf(image, ""); isWhole {
			entries = append(entries, entry)
		}
	}
	for _, animation := range document.Messages("animations") {
		for _, image := range animation.Messages("images") {
			if entry, isWhole := entryOf(image, animation.Text("id")); isWhole {
				entries = append(entries, entry)
			}
		}
	}
	return entries
}

func LayoutOf(content []byte, packed *textureset.Set) (Layout, error) {
	entries := EntriesOf(content)
	if len(entries) == 0 {
		return Layout{}, ErrNoImages
	}
	layout := Layout{Width: packed.Width, Height: packed.Height, Pages: packed.Pages}
	frameOfAnimation := map[string]int{}
	for _, entry := range entries {
		id, frame := entry.Name, 0
		if entry.Animation != "" {
			id, frame = entry.Animation, frameOfAnimation[entry.Animation]
			frameOfAnimation[entry.Animation]++
		}
		animation, isThere := packed.Animation(id)
		if !isThere || frame >= len(animation.Frames) {
			return Layout{}, fmt.Errorf("the build has no place for %s: it was made from another state of the atlas", entry.Image)
		}
		layout.Images = append(layout.Images, PlacedImage{Entry: entry, Place: animation.Frames[frame]})
	}
	return layout, nil
}

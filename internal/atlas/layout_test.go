package atlas

import (
	"errors"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/Lanterna-Games/defold-tools/internal/textureset"
)

func studyAtlas(t *testing.T) (content []byte, packed *textureset.Set) {
	t.Helper()
	content, err := os.ReadFile("../textureset/testdata/props.atlas")
	if err != nil {
		t.Fatal(err)
	}
	compiled, err := os.ReadFile("../textureset/testdata/props.a.texturesetc")
	if err != nil {
		t.Fatal(err)
	}
	packed, err = textureset.Read(compiled)
	if err != nil {
		t.Fatal(err)
	}
	return content, packed
}

// The build knows an image by the name it plays under, and an atlas by the
// file it is written in. Each line of the atlas that names an image is given
// the place the build found for that image: an image on its own by its name,
// and an image of an animation by the animation and its turn in it.
func TestEachImageOfTheAtlasGetsThePlaceTheBuildGaveIt(t *testing.T) {
	content, packed := studyAtlas(t)

	layout, err := LayoutOf(content, packed)
	if err != nil {
		t.Fatal(err)
	}

	if layout.Width != 128 || layout.Height != 128 || layout.Pages != 1 {
		t.Errorf("page = %dx%d, %d of them", layout.Width, layout.Height, layout.Pages)
	}
	want := []PlacedImage{
		{Image: "/main/images/house.png", Name: "house", Line: 2, Place: textureset.Frame{X: 4, Y: 40, Width: 48, Height: 64, IsRotated: true}},
		{Image: "/main/images/tree.png", Name: "tree", Line: 5, Place: textureset.Frame{X: 58, Y: 40, Width: 32, Height: 80}},
		{Image: "/main/images/coin.png", Name: "coin", Line: 8, Place: textureset.Frame{X: 96, Y: 100, Width: 16, Height: 16}},
		{Image: "/main/images/sign.png", Name: "sign", Line: 11, Place: textureset.Frame{X: 4, Y: 4, Width: 100, Height: 30}},
		{Image: "/main/frames/walk_1.png", Name: "walk_1", Animation: "walk", Line: 16, Place: textureset.Frame{X: 96, Y: 40, Width: 24, Height: 24}},
		{Image: "/main/frames/walk_2.png", Name: "walk_2", Animation: "walk", Line: 19, Place: textureset.Frame{X: 96, Y: 70, Width: 24, Height: 24}},
	}
	if !slices.Equal(layout.Images, want) {
		t.Errorf("images =\n%+v\nwant\n%+v", layout.Images, want)
	}
}

// The build that is read may be older than the atlas: an image was added since,
// or renamed. A layout with holes in it would pass for the atlas as it is, so
// none is given.
func TestABuildOfAnotherStateOfTheAtlasIsRefused(t *testing.T) {
	content, packed := studyAtlas(t)
	cases := map[string]string{
		"an image added since":                "images {\n  image: \"/main/images/door.png\"\n}\n" + string(content),
		"an image added to an animation":      strings.Replace(string(content), "  playback:", "  images {\n    image: \"/main/frames/walk_3.png\"\n  }\n  playback:", 1),
		"names changed by a pattern since":    string(content) + "rename_patterns: \"house=home\"\n",
		"an animation given another id since": strings.Replace(string(content), `id: "walk"`, `id: "run"`, 1),
	}
	for name, changed := range cases {
		t.Run(name, func(t *testing.T) {
			if layout, err := LayoutOf([]byte(changed), packed); err == nil {
				t.Errorf("layout = %+v, want it refused", layout)
			}
		})
	}
}

func TestAnAtlasWithoutImagesHasNoLayout(t *testing.T) {
	_, packed := studyAtlas(t)

	if _, err := LayoutOf([]byte("margin: 2\n"), packed); !errors.Is(err, ErrNoImages) {
		t.Errorf("err = %v, want it said that the atlas has no images", err)
	}
}

// An image plays under the name of its file, without the directory and the
// extension, after the atlas has changed it by its patterns.
func TestAnImageIsNamedAfterItsFileAndThePatternsOfTheAtlas(t *testing.T) {
	entries := EntriesOf([]byte("images {\n  image: \"/main/images/hero_idle_nrm.png\"\n}\nimages {\n}\nrename_patterns: \"_nrm=,hero_=\"\n"))

	if want := []Entry{{Image: "/main/images/hero_idle_nrm.png", Name: "idle", Line: 2}}; !slices.Equal(entries, want) {
		t.Errorf("entries = %+v, want %+v", entries, want)
	}
}

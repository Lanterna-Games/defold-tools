package atlas

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const (
	samplesVariable   = "DEFOLD_TOOLS_SAMPLES"
	asTheEditorWrites = `images {
  image: "/main/images/ball.png"
}
images {
  image: "/main/images/paddle.png"
}
animations {
  id: "roll"
  images {
    image: "/main/images/roll_1.png"
  }
  images {
    image: "/main/images/roll_2.png"
  }
  playback: PLAYBACK_LOOP_FORWARD
  fps: 12
}
margin: 0
extrude_borders: 2
inner_padding: 0
`
)

func TestOnlyTheImagesOfTheAtlasItselfAreListed(t *testing.T) {
	if got, want := ImagesOf([]byte(asTheEditorWrites)), []string{"/main/images/ball.png", "/main/images/paddle.png"}; !slices.Equal(got, want) {
		t.Errorf("images = %q, want %q: those of an animation belong to the animation", got, want)
	}
}

func TestAnImageIsAddedAfterTheLastOne(t *testing.T) {
	cases := map[string]struct {
		before string
		want   string
	}{
		"between the images and the animations": {
			before: asTheEditorWrites,
			want:   strings.Replace(asTheEditorWrites, "animations {", "images {\n  image: \"/main/images/wall.png\"\n}\nanimations {", 1),
		},
		"in an atlas that has none": {
			before: "margin: 0\n",
			want:   "images {\n  image: \"/main/images/wall.png\"\n}\nmargin: 0\n",
		},
		"in an empty file": {
			before: "",
			want:   "images {\n  image: \"/main/images/wall.png\"\n}\n",
		},
		"in a file written on Windows": {
			before: "images {\r\n  image: \"/a.png\"\r\n}\r\nmargin: 0\r\n",
			want:   "images {\r\n  image: \"/a.png\"\r\n}\r\nimages {\r\n  image: \"/main/images/wall.png\"\r\n}\r\nmargin: 0\r\n",
		},
		"in a file that does not end its last line": {
			before: "images {\n  image: \"/a.png\"\n}",
			want:   "images {\n  image: \"/a.png\"\n}\nimages {\n  image: \"/main/images/wall.png\"\n}",
		},
	}
	for name, testCase := range cases {
		if got := string(With([]byte(testCase.before), "/main/images/wall.png")); got != testCase.want {
			t.Errorf("%s:\n%q\nwant\n%q", name, got, testCase.want)
		}
	}
}

func TestAnImageIsTakenOutWithItsBlockAndNothingElse(t *testing.T) {
	want := strings.Replace(asTheEditorWrites, "images {\n  image: \"/main/images/ball.png\"\n}\n", "", 1)
	if got := string(Without([]byte(asTheEditorWrites), "/main/images/ball.png")); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
	if got := string(Without([]byte(asTheEditorWrites), "/main/images/roll_1.png")); got != asTheEditorWrites {
		t.Errorf("an image of an animation was taken out:\n%s", got)
	}
	trimmed := "images {\n  image: \"/a.png\"\n  sprite_trim_mode: SPRITE_TRIM_MODE_6\n  pivot {\n    x: 0.5\n  }\n}\nmargin: 0\n"
	if got := string(Without([]byte(trimmed), "/a.png")); got != "margin: 0\n" {
		t.Errorf("a block with more than its image was not taken out whole:\n%s", got)
	}
}

func TestAnAtlasIsBroughtInLineWithWhatIsOnDisk(t *testing.T) {
	there := []string{"/main/images/ball.png", "/main/images/net.png", "/main/images/wall.png"}
	isStillThere := func(image string) bool { return image != "/main/images/paddle.png" }

	synced, change := Synced([]byte(asTheEditorWrites), there, isStillThere)

	if !slices.Equal(change.Added, []string{"/main/images/net.png", "/main/images/wall.png"}) || !slices.Equal(change.Removed, []string{"/main/images/paddle.png"}) {
		t.Errorf("change = %+v", change)
	}
	if got, want := ImagesOf(synced), []string{"/main/images/ball.png", "/main/images/net.png", "/main/images/wall.png"}; !slices.Equal(got, want) {
		t.Errorf("images = %q, want %q", got, want)
	}
	if _, afterTheImages, _ := strings.Cut(asTheEditorWrites, "animations {"); !strings.HasSuffix(string(synced), "animations {"+afterTheImages) {
		t.Errorf("what follows the images was touched:\n%s", synced)
	}
	if again, nothing := Synced(synced, there, isStillThere); string(again) != string(synced) || len(nothing.Added)+len(nothing.Removed) != 0 {
		t.Errorf("a second time changes it again: %+v", nothing)
	}
}

// The frames of an animation are files in the same directory as the other
// images, more often than not. They are in the atlas already, inside their
// animation, and adding each of them again as an image of its own would give
// the atlas a second copy and the game an animation nobody asked for.
func TestAFrameOfAnAnimationIsNotAddedAsAnImageOfItsOwn(t *testing.T) {
	there := []string{"/main/images/ball.png", "/main/images/paddle.png", "/main/images/roll_1.png", "/main/images/roll_2.png", "/main/images/net.png"}

	synced, change := Synced([]byte(asTheEditorWrites), there, func(string) bool { return true })

	if !slices.Equal(change.Added, []string{"/main/images/net.png"}) || len(change.Removed) != 0 {
		t.Errorf("change = %+v, want the one image that is in no animation added", change)
	}
	if got, want := ImagesOf(synced), []string{"/main/images/ball.png", "/main/images/paddle.png", "/main/images/net.png"}; !slices.Equal(got, want) {
		t.Errorf("images = %q, want %q", got, want)
	}
}

// Run with DEFOLD_TOOLS_SAMPLES set to a directory of real Defold projects. In every
// atlas found there, the last image is taken out and put back: when its block holds
// nothing but the image, the file has to come back byte for byte, which is what says
// that an image is added the way the Defold editor writes it.
func TestRealAtlasesComeBackAsTheyWere(t *testing.T) {
	root := os.Getenv(samplesVariable)
	if root == "" {
		t.Skipf("%s is not set", samplesVariable)
	}
	var paths []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err == nil && !entry.IsDir() && filepath.Ext(path) == Extension {
			paths = append(paths, path)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	compared := 0
	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		blocks := blocksOf(content)
		images := ImagesOf(content)
		if len(blocks) == 0 || slices.Index(images, images[len(images)-1]) != len(images)-1 {
			continue
		}
		last := blocks[len(blocks)-1]
		without := Without(content, last.image)
		if got := ImagesOf(without); !slices.Equal(got, images[:len(images)-1]) {
			t.Errorf("%s: taking out %q leaves %d images of %d", path, last.image, len(got), len(images))
			continue
		}
		lines := strings.Count(string(content[last.start:last.end]), "\n")
		holdsOnlyItsImage := lines == 3 || (last.end == len(content) && lines == 2)
		if holdsOnlyItsImage {
			compared++
			if back := With(without, last.image); string(back) != string(content) {
				t.Errorf("%s does not come back as it was", path)
			}
		}
	}
	t.Logf("%d atlases, %d of them compared byte for byte", len(paths), compared)
}

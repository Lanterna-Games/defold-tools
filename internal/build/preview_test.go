package build

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/Lanterna-Games/defold-tools/internal/atlas"
)

const atlasOfProps = "images {\n  image: \"/main/images/house.png\"\n}\nimages {\n  image: \"/main/images/tree.png\"\n}\nanimations {\n  id: \"walk\"\n  images {\n    image: \"/main/frames/walk_1.png\"\n  }\n}\nmargin: 2\n"

func projectWithAnAtlas(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "game")
	for name, content := range map[string]string{
		"game.project":            "[project]\ntitle = Game\ndependencies#0 = https://example.com/library.zip\n\n[bootstrap]\nmain_collection = /main/main.collectionc\nrender = /library/own.renderc\n",
		"main/main.collection":    "name: \"main\"\n",
		"main/broken.script":      "function init(self\n",
		"main/props.atlas":        atlasOfProps,
		"main/images/house.png":   "house",
		"main/images/tree.png":    "tree",
		"main/images/unused.png":  "unused",
		"main/frames/walk_1.png":  "walk",
		"main/other/sounds.atlas": "images {\n  image: \"/main/images/unused.png\"\n}\n",
	} {
		target := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func filesUnder(t *testing.T, directory string) []string {
	t.Helper()
	var names []string
	err := filepath.WalkDir(directory, func(file string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			relative, _ := filepath.Rel(directory, file)
			names = append(names, filepath.ToSlash(relative))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return names
}

// Defold's build tool builds what a game reaches from the collection it starts
// with, so an atlas nothing uses yet is not packed, and one mistake anywhere in
// the project stops the build. To have one atlas packed whatever the state of
// the rest, it is given a project of its own, away from the game: the atlas and
// its images where they are in the game, and a sprite that shows it. Nothing of
// the rest of the game goes along, and nothing is written in the game.
func TestAnAtlasIsLaidOutAloneAwayFromTheGame(t *testing.T) {
	root := projectWithAnAtlas(t)
	before := filesUnder(t, root)
	directory := filepath.Join(t.TempDir(), "packing")

	if err := laidOutToPack(root, directory, "/main/props.atlas"); err != nil {
		t.Fatal(err)
	}

	want := []string{"defoldpls_preview.collection", "defoldpls_preview.go", "game.project", "main/frames/walk_1.png", "main/images/house.png", "main/images/tree.png", "main/props.atlas"}
	if got := filesUnder(t, directory); !slices.Equal(got, want) {
		t.Errorf("files laid out = %q, want %q", got, want)
	}
	if after := filesUnder(t, root); !slices.Equal(after, before) {
		t.Errorf("the game has %q, and had %q", after, before)
	}
	settings, _ := os.ReadFile(filepath.Join(directory, "game.project"))
	sprite, _ := os.ReadFile(filepath.Join(directory, "defoldpls_preview.go"))
	if strings.Contains(string(settings), "dependencies") || strings.Contains(string(settings), "/library/") {
		t.Errorf("the settings of the game went along:\n%s", settings)
	}
	if !strings.Contains(string(sprite), `tile_set: \"/main/props.atlas\"`) || !strings.Contains(string(sprite), `default_animation: \"house\"`) {
		t.Errorf("the sprite does not show the atlas by its first image:\n%s", sprite)
	}
}

// The directory is kept between runs, since the build tool only builds again
// what changed. So it has to follow the atlas: an image that is no longer named
// goes, one that changed is taken again, and what the build wrote stays.
func TestWhatIsLaidOutFollowsTheAtlas(t *testing.T) {
	root := projectWithAnAtlas(t)
	directory := filepath.Join(t.TempDir(), "packing")
	if err := laidOutToPack(root, directory, "/main/props.atlas"); err != nil {
		t.Fatal(err)
	}
	built := filepath.Join(directory, "build", "default", "main", "props.a.texturesetc")
	if err := os.MkdirAll(filepath.Dir(built), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(built, []byte("built"), 0o600); err != nil {
		t.Fatal(err)
	}
	copiedHouse := filepath.Join(directory, "main", "images", "house.png")
	asCopied, _ := os.Stat(copiedHouse)
	if err := os.WriteFile(copiedHouse, []byte("HOUSE"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(copiedHouse, asCopied.ModTime(), asCopied.ModTime()); err != nil {
		t.Fatal(err)
	}

	withoutTheTree := strings.Replace(atlasOfProps, "images {\n  image: \"/main/images/tree.png\"\n}\n", "", 1)
	if err := os.WriteFile(filepath.Join(root, "main", "props.atlas"), []byte(withoutTheTree), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main", "frames", "walk_1.png"), []byte("walk, drawn again"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := laidOutToPack(root, directory, "/main/props.atlas"); err != nil {
		t.Fatal(err)
	}

	want := []string{"build/default/main/props.a.texturesetc", "defoldpls_preview.collection", "defoldpls_preview.go", "game.project", "main/frames/walk_1.png", "main/images/house.png", "main/props.atlas"}
	if got := filesUnder(t, directory); !slices.Equal(got, want) {
		t.Errorf("files laid out = %q, want %q", got, want)
	}
	if walk, _ := os.ReadFile(filepath.Join(directory, "main", "frames", "walk_1.png")); string(walk) != "walk, drawn again" {
		t.Errorf("the image that changed is still %q", walk)
	}
	if house, _ := os.ReadFile(copiedHouse); string(house) != "HOUSE" {
		t.Error("an image of the same size and time as its copy was copied again, which would cost a large atlas its whole size at every look")
	}
}

func TestAnAtlasThatCannotBePackedSaysWhy(t *testing.T) {
	root := projectWithAnAtlas(t)
	cases := map[string]struct {
		content string
		want    string
	}{
		"an image that is not there": {content: atlasOfProps + "images {\n  image: \"/main/images/door.png\"\n}\n", want: "/main/images/door.png"},
		"no image at all":            {content: "margin: 2\n", want: atlas.ErrNoImages.Error()},
	}
	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			if err := os.WriteFile(filepath.Join(root, "main", "props.atlas"), []byte(testCase.content), 0o600); err != nil {
				t.Fatal(err)
			}

			err := laidOutToPack(root, filepath.Join(t.TempDir(), "packing"), "/main/props.atlas")

			if err == nil || !strings.Contains(err.Error(), testCase.want) {
				t.Errorf("err = %v, want it to name %q", err, testCase.want)
			}
		})
	}
}

// An editor whose Java is a shell script: it writes down what it was asked and,
// where the build tool would, the texture set of the study atlas.
func editorThatPacksTheStudyAtlas(t *testing.T) (installed Installation, asked string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the Java that stands in for the editor's is a shell script")
	}
	fixture, err := filepath.Abs(filepath.Join("..", "textureset", "testdata", "props.a.texturesetc"))
	if err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	asked = filepath.Join(home, "asked")
	java := filepath.Join(home, "java")
	script := "#!/bin/sh\necho \"$@\" >> " + asked + "\nmkdir -p build/default/main && cp " + fixture + " build/default/main/props.a.texturesetc\n"
	if err := os.WriteFile(java, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	return Installation{Home: home, Version: "1.14.0", SHA1: "abc", Java: java, Jar: filepath.Join(home, "defold.jar")}, asked
}

func TestAnAtlasIsPackedByTheBuildToolInTheCache(t *testing.T) {
	installed, asked := editorThatPacksTheStudyAtlas(t)
	root, cache := projectWithAnAtlas(t), t.TempDir()
	before := filesUnder(t, root)

	packed, err := installed.PackAnAtlas(t.Context(), root, "/main/props.atlas", cache, io.Discard)
	if err != nil {
		t.Fatal(err)
	}

	if house, isThere := packed.Animation("house"); packed.Width != 128 || !isThere || !house.Frames[0].IsRotated {
		t.Errorf("packed = %+v, want what the build wrote", packed)
	}
	said, _ := os.ReadFile(asked)
	directory := directoryToPackIn(cache, root, "/main/props.atlas")
	if !strings.HasPrefix(directory, cache) || !strings.Contains(string(said), "--root "+directory+" ") || !strings.HasSuffix(strings.TrimSpace(string(said)), " build") {
		t.Errorf("the build tool was asked %q, want it to build the directory in the cache", said)
	}
	if after := filesUnder(t, root); !slices.Equal(after, before) {
		t.Errorf("the game has %q, and had %q", after, before)
	}
}

func TestABuildThatWritesNoLayoutIsNotTakenForOne(t *testing.T) {
	installed, _ := editorThatPacksTheStudyAtlas(t)
	root := projectWithAnAtlas(t)
	if err := os.Rename(filepath.Join(root, "main", "props.atlas"), filepath.Join(root, "main", "others.atlas")); err != nil {
		t.Fatal(err)
	}

	_, err := installed.PackAnAtlas(t.Context(), root, "/main/others.atlas", t.TempDir(), io.Discard)

	if err == nil || errors.Is(err, fs.ErrNotExist) || !strings.Contains(err.Error(), "wrote no layout for /main/others.atlas") {
		t.Errorf("err = %v, want it said that the build wrote no layout", err)
	}
}

// An atlas is a file anyone may have written. A path in it that climbs out of
// the project would have its target copied into the cache, and one that climbs
// out of the packing directory would be written wherever it leads. Neither is
// followed: what is read stays inside the game and what is written inside the
// directory.
func TestAnImageOutsideTheProjectIsNotReached(t *testing.T) {
	root := projectWithAnAtlas(t)
	outside := filepath.Join(filepath.Dir(root), "secret.png")
	if err := os.WriteFile(outside, []byte("not of the game"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main", "props.atlas"), []byte("images {\n  image: \"/../secret.png\"\n}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(t.TempDir(), "inside", "packing")

	err := laidOutToPack(root, directory, "/main/props.atlas")

	if err == nil {
		t.Error("an image outside the project was accepted")
	}
	if _, errOfTheCopy := os.Stat(filepath.Join(filepath.Dir(directory), "secret.png")); errOfTheCopy == nil {
		t.Error("the image outside the project was copied outside the packing directory")
	}
}

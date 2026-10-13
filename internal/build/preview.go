package build

import (
	"context"
	"crypto/sha1" //nolint:gosec // names a directory in the cache, nothing is protected by it
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/Lanterna-Games/defold-tools/internal/atlas"
	"github.com/Lanterna-Games/defold-tools/internal/textureset"
)

const (
	directoryOfPackedAtlases = "atlas"
	stepThatBuilds           = "build"
	settingsOfAPreview       = "game.project"
	collectionOfAPreview     = "defoldpls_preview.collection"
	gameObjectOfAPreview     = "defoldpls_preview.go"
	permissionsOfAPreview    = 0o600
)

func filesThatShowAnAtlas(atlasPath, animation string) map[string]string {
	return map[string]string{
		settingsOfAPreview:   "[project]\ntitle = Atlas\n\n[bootstrap]\nmain_collection = /" + collectionOfAPreview + "c\n\n[input]\ngame_binding = /builtins/input/all.input_bindingc\n",
		collectionOfAPreview: "name: \"preview\"\ninstances {\n  id: \"atlas\"\n  prototype: \"/" + gameObjectOfAPreview + "\"\n}\n",
		gameObjectOfAPreview: "embedded_components {\n  id: \"sprite\"\n  type: \"sprite\"\n  data: \"tile_set: \\\"" + atlasPath + "\\\"\\n\"\n  \"default_animation: \\\"" + animation + "\\\"\\n\"\n  \"material: \\\"/builtins/materials/sprite.material\\\"\\n\"\n  \"\"\n}\n",
	}
}

func directoryToPackIn(cache, root, atlasPath string) string {
	absolute, err := filepath.Abs(root)
	if err != nil {
		absolute = root
	}
	digest := sha1.Sum([]byte(filepath.ToSlash(absolute) + atlasPath)) //nolint:gosec // as above
	return filepath.Join(cache, directoryOfPackedAtlases, hex.EncodeToString(digest[:]))
}

func isAsItWasCopied(source, copied fs.FileInfo) bool {
	return copied.Size() == source.Size() && copied.ModTime().Equal(source.ModTime())
}

func copyIfItChanged(game, packing *os.Root, name string) error {
	source, err := game.Stat(name)
	if err != nil {
		return err
	}
	if copied, err := packing.Stat(name); err == nil && isAsItWasCopied(source, copied) {
		return nil
	}
	content, err := game.ReadFile(name)
	if err != nil {
		return err
	}
	if err := packing.MkdirAll(path.Dir(name), 0o750); err != nil {
		return err
	}
	if err := packing.WriteFile(name, content, permissionsOfAPreview); err != nil {
		return err
	}
	return packing.Chtimes(name, source.ModTime(), source.ModTime())
}

func removeWhatIsNoLongerWanted(packing *os.Root, isWanted map[string]bool) error {
	outputOfTheBuild, _, _ := strings.Cut(OutputOfABuild, "/")
	var stale []string
	err := fs.WalkDir(packing.FS(), ".", func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		isOfTheBuild := name != "." && (strings.HasPrefix(entry.Name(), ".") || name == outputOfTheBuild)
		switch {
		case entry.IsDir() && isOfTheBuild:
			return fs.SkipDir
		case !entry.IsDir() && !isOfTheBuild && !isWanted[name]:
			stale = append(stale, name)
		}
		return nil
	})
	for _, name := range stale {
		err = errors.Join(err, packing.Remove(name))
	}
	return err
}

func laidOutToPack(root, directory, atlasPath string) error {
	game, err := os.OpenRoot(root)
	if err != nil {
		return err
	}
	defer game.Close()
	nameOfTheAtlas := strings.TrimPrefix(atlasPath, "/")
	content, err := game.ReadFile(nameOfTheAtlas)
	if err != nil {
		return err
	}
	entries := atlas.EntriesOf(content)
	if len(entries) == 0 {
		return atlas.ErrNoImages
	}
	animation := entries[0].Name
	if entries[0].Animation != "" {
		animation = entries[0].Animation
	}
	generated := filesThatShowAnAtlas(atlasPath, animation)
	copied := []string{nameOfTheAtlas}
	var missing []string
	for _, entry := range entries {
		name := strings.TrimPrefix(entry.Image, "/")
		if _, err := game.Stat(name); err != nil {
			missing = append(missing, entry.Image)
		}
		copied = append(copied, name)
	}
	if len(missing) > 0 {
		return fmt.Errorf("the atlas names images that are not in the project: %s", strings.Join(missing, ", "))
	}
	isWanted := map[string]bool{}
	for name := range generated {
		isWanted[name] = true
	}
	for _, name := range copied {
		isWanted[name] = true
	}
	if err := os.MkdirAll(directory, 0o750); err != nil {
		return err
	}
	packing, err := os.OpenRoot(directory)
	if err != nil {
		return err
	}
	defer packing.Close()
	if err := removeWhatIsNoLongerWanted(packing, isWanted); err != nil {
		return err
	}
	for name, content := range generated {
		if written, err := packing.ReadFile(name); err == nil && string(written) == content {
			continue
		}
		if err := packing.WriteFile(name, []byte(content), permissionsOfAPreview); err != nil {
			return err
		}
	}
	for _, name := range copied {
		if err := copyIfItChanged(game, packing, name); err != nil {
			return err
		}
	}
	return nil
}

func (i Installation) PackAnAtlas(ctx context.Context, root, atlasPath, cache string, output io.Writer) (*textureset.Set, error) {
	directory := directoryToPackIn(cache, root, atlasPath)
	if err := laidOutToPack(root, directory, atlasPath); err != nil {
		return nil, err
	}
	if err := i.Bob(ctx, directory, output, stepThatBuilds); err != nil {
		return nil, err
	}
	compiledPath := strings.TrimSuffix(atlasPath, path.Ext(atlasPath)) + textureset.CompiledSuffixOfAnAtlas
	compiled, err := os.ReadFile(filepath.Join(directory, filepath.FromSlash(OutputOfABuild), filepath.FromSlash(compiledPath))) //nolint:gosec // what the build just wrote, in the cache
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("the build wrote no layout for %s", atlasPath)
	}
	if err != nil {
		return nil, err
	}
	return textureset.Read(compiled)
}

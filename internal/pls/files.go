package pls

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Lanterna-Games/defold-tools/internal/analysis"
	"github.com/Lanterna-Games/defold-tools/internal/atlas"
	"github.com/Lanterna-Games/defold-tools/internal/scaffold"
)

const (
	commandThatMakesAFile  = "new"
	commandThatSyncsAtlas  = "atlas"
	flagThatListsTheKinds  = "--kinds"
	markOfAnAddedImage     = "+ "
	markOfARemovedImage    = "- "
	permissionsOfADataFile = 0o644
)

type fileArguments struct {
	root       string
	file       string
	withScript bool
	from       string
	madeFrom   string
}

func rootOfTheProjectHolding(file string) (root, relative string, err error) {
	absolute, err := filepath.Abs(file)
	if err != nil {
		return "", "", err
	}
	for directory := filepath.Dir(absolute); ; directory = filepath.Dir(directory) {
		if analysis.IsAProject(os.DirFS(directory)) {
			inside, err := filepath.Rel(directory, absolute)
			return directory, filepath.ToSlash(inside), err
		}
		if directory == filepath.Dir(directory) {
			return "", "", fmt.Errorf("%s is in no Defold project: no directory above it holds a %s", file, analysis.ProjectFile)
		}
	}
}

func fileArgumentsFrom(command string, args []string) (fileArguments, error) {
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var parsed fileArguments
	flags.BoolVar(&parsed.withScript, "script", false, "")
	flags.StringVar(&parsed.from, "from", "", "")
	if err := flags.Parse(args); err != nil {
		return fileArguments{}, fmt.Errorf("%s: %w", command, err)
	}
	if flags.NArg() != 1 {
		return fileArguments{}, fmt.Errorf("%s takes one file, as in %q", command, "defoldpls "+command+" main/enemy"+map[string]string{commandThatMakesAFile: ".go", commandThatSyncsAtlas: ".atlas"}[command])
	}
	root, relative, err := rootOfTheProjectHolding(flags.Arg(0))
	if err != nil {
		return fileArguments{}, err
	}
	parsed.root, parsed.file = root, relative
	if parsed.from != "" && command == commandThatMakesAFile && path.Ext(relative) != atlas.Extension {
		if _, err := os.Stat(parsed.from); err != nil {
			return fileArguments{}, fmt.Errorf("%s is not there to make %s from", parsed.from, relative)
		}
		if parsed.madeFrom, err = placeInTheProject(root, parsed.from); err != nil {
			return fileArguments{}, err
		}
		parsed.from = ""
	}
	if parsed.from != "" {
		if _, parsed.from, err = rootOfTheProjectHolding(filepath.Join(parsed.from, "x")); err != nil {
			return fileArguments{}, err
		}
		parsed.from = path.Dir(parsed.from)
	}
	return parsed, nil
}

func imagesIn(root, directory string, isDeep bool) []string {
	var images []string
	_ = fs.WalkDir(os.DirFS(root), directory, func(file string, entry fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return fs.SkipDir
		case entry.IsDir() && file != directory && !isDeep:
			return fs.SkipDir
		case !entry.IsDir() && slices.Contains(atlas.ExtensionsOfImages, strings.ToLower(path.Ext(file))):
			images = append(images, "/"+file)
		}
		return nil
	})
	slices.Sort(images)
	return images
}

type kindOfFile struct {
	Extension    string   `json:"extension"`
	TakesAScript bool     `json:"takesAScript"`
	MadeFrom     []string `json:"madeFrom"`
}

func sayTheKindsOfFile(stdout io.Writer, fail func(error) int) int {
	kinds := make([]kindOfFile, 0, len(scaffold.Kinds))
	for _, kind := range scaffold.Kinds {
		kinds = append(kinds, kindOfFile{Extension: kind, TakesAScript: scaffold.TakesAScript(kind), MadeFrom: append([]string{}, scaffold.TakenBy(kind)...)})
	}
	if err := json.NewEncoder(stdout).Encode(kinds); err != nil {
		return fail(err)
	}
	return exitSuccess
}

func makeAFile(args []string, stdout io.Writer, fail func(error) int) int {
	if len(args) == 1 && args[0] == flagThatListsTheKinds {
		return sayTheKindsOfFile(stdout, fail)
	}
	asked, err := fileArgumentsFrom(commandThatMakesAFile, args)
	if err != nil {
		return fail(err)
	}
	wanted := scaffold.Asked{File: asked.file, WithScript: asked.withScript, From: asked.madeFrom}
	if asked.from != "" {
		wanted.Images = imagesIn(asked.root, asked.from, true)
	}
	files, err := scaffold.Files(wanted)
	if err != nil {
		return fail(err)
	}
	names := slices.Sorted(func(yield func(string) bool) {
		for name := range files {
			if !yield(name) {
				return
			}
		}
	})
	for _, name := range names {
		if _, err := os.Stat(filepath.Join(asked.root, filepath.FromSlash(name))); err == nil {
			return fail(fmt.Errorf("%s is there already, and nothing was made", name))
		}
	}
	for _, name := range names {
		target := filepath.Join(asked.root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
			return fail(err)
		}
		if err := os.WriteFile(target, []byte(files[name]), permissionsOfADataFile); err != nil {
			return fail(err)
		}
		fmt.Fprintln(stdout, name)
	}
	return exitSuccess
}

func syncAnAtlas(args []string, stdout io.Writer, fail func(error) int) int {
	asked, err := fileArgumentsFrom(commandThatSyncsAtlas, args)
	if err != nil {
		return fail(err)
	}
	if path.Ext(asked.file) != atlas.Extension {
		return fail(fmt.Errorf("%s is not an atlas", asked.file))
	}
	target := filepath.Join(asked.root, filepath.FromSlash(asked.file))
	content, err := os.ReadFile(target) //nolint:gosec // the atlas named on the command line
	if err != nil {
		return fail(err)
	}
	var there []string
	if asked.from != "" {
		there = imagesIn(asked.root, asked.from, true)
	} else {
		var directories []string
		for _, image := range atlas.ImagesOf(content) {
			if directory := path.Dir(strings.TrimPrefix(image, "/")); !slices.Contains(directories, directory) {
				directories = append(directories, directory)
			}
		}
		if len(directories) == 0 {
			directories = []string{path.Dir(asked.file)}
		}
		for _, directory := range directories {
			there = append(there, imagesIn(asked.root, directory, false)...)
		}
	}
	isStillThere := func(image string) bool {
		_, err := os.Stat(filepath.Join(asked.root, filepath.FromSlash(image)))
		top, _, _ := strings.Cut(strings.TrimPrefix(image, "/"), "/")
		_, errOfTheDirectory := os.Stat(filepath.Join(asked.root, top))
		return err == nil || errors.Is(errOfTheDirectory, fs.ErrNotExist)
	}
	synced, change := atlas.Synced(content, there, isStillThere)
	for _, image := range change.Removed {
		fmt.Fprintln(stdout, markOfARemovedImage+image)
	}
	for _, image := range change.Added {
		fmt.Fprintln(stdout, markOfAnAddedImage+image)
	}
	if len(change.Added)+len(change.Removed) == 0 {
		return exitSuccess
	}
	if err := os.WriteFile(target, synced, permissionsOfADataFile); err != nil { //nolint:gosec // as for a new file
		return fail(err)
	}
	return exitSuccess
}

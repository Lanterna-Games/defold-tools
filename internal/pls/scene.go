package pls

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/Lanterna-Games/defold-tools/internal/libraries"
	"github.com/Lanterna-Games/defold-tools/internal/scene"
)

const (
	commandThatShowsAScene = "scene"
	kindOfAScene           = "scene"
)

type shownScene struct {
	Kind string `json:"kind"`
	File string `json:"file"`
	Root string `json:"root"`
	*scene.Scene
}

func showAScene(args []string, stdin io.Reader, stdout io.Writer, fail func(error) int) int {
	flags := flag.NewFlagSet(commandThatShowsAScene, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	isOnStandardInput := flags.Bool("stdin", false, "")
	if err := flags.Parse(args); err != nil {
		return fail(fmt.Errorf("%s: %w", commandThatShowsAScene, err))
	}
	if flags.NArg() != 1 {
		return fail(fmt.Errorf("%s takes one game object, collection or GUI scene, as in %q", commandThatShowsAScene, "defoldpls scene main/main.collection"))
	}
	root, file, err := rootOfTheProjectHolding(flags.Arg(0))
	if err != nil {
		return fail(err)
	}
	onDisk := os.DirFS(root)
	files := scene.Files{All: libraries.Mount(onDisk).FS, OnDisk: onDisk, Held: map[string][]byte{}}
	if *isOnStandardInput {
		held, err := io.ReadAll(stdin)
		if err != nil {
			return fail(err)
		}
		files.Held[file] = held
	}
	laidOut, err := scene.Of(files, "/"+file)
	if err != nil {
		return fail(err)
	}
	if err := json.NewEncoder(stdout).Encode(shownScene{Kind: kindOfAScene, File: "/" + file, Root: root, Scene: laidOut}); err != nil {
		return fail(err)
	}
	return exitSuccess
}

package pls

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync"

	"github.com/Lanterna-Games/defold-tools/internal/build"
)

const (
	exitBuildFailed    = 1
	enginesDirectory   = "engine"
	stepThatBuilds     = "build"
	stepThatFetches    = "resolve"
	stepThatCleans     = "distclean"
	commandThatBuilds  = "build"
	commandThatRuns    = "run"
	commandThatReloads = "reload"
	commandThatFetches = "resolve"
	commandThatCleans  = "clean"
)

var stepsInsteadOfBuilding = map[string]string{
	commandThatFetches: stepThatFetches,
	commandThatCleans:  stepThatCleans,
}

type gameArguments struct {
	home      string
	engine    string
	directory string
	isBuilt   bool
	files     []string
}

func gameArgumentsFrom(command string, args []string) (gameArguments, error) {
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	parsed := gameArguments{directory: currentDirectory}
	flags.StringVar(&parsed.home, "defold-home", "", "")
	flags.StringVar(&parsed.engine, "engine", "", "")
	flags.BoolVar(&parsed.isBuilt, "built", false, "")
	project := flags.String("project", "", "")
	if err := flags.Parse(args); err != nil {
		return gameArguments{}, fmt.Errorf("%s: %w", command, err)
	}
	switch {
	case command == commandThatReloads:
		parsed.files = flags.Args()
		if *project != "" {
			parsed.directory = *project
		}
		if len(parsed.files) == 0 {
			return gameArguments{}, fmt.Errorf("reload needs the files that changed, as in %q", "defoldpls reload main/ball.script")
		}
	case flags.NArg() == 1:
		parsed.directory = flags.Arg(0)
	case flags.NArg() > 1:
		return gameArguments{}, fmt.Errorf("%s takes one directory, got %d arguments", command, flags.NArg())
	}
	return parsed, nil
}

func (w *workshop) installation(asked gameArguments, announce func(string)) (build.Installation, error) {
	home, _ := os.UserHomeDir()
	installed, err := build.FindInstallation(asked.home, os.Getenv, home)
	if err != nil {
		return build.Installation{}, err
	}
	if declared, _ := w.versionOf(asked.directory); declared != "" && declared != installed.Version {
		announce(fmt.Sprintf("the project declares Defold %s and the editor in %s is %s: the game is built with %s", declared, installed.Home, installed.Version, installed.Version))
	}
	return installed, nil
}

func (w *workshop) engineFor(root, named string, announce func(string), editor func() (build.Installation, error)) (string, error) {
	if named != "" {
		return named, nil
	}
	if engine, wasBuilt := build.EngineBuiltFor(root); wasBuilt {
		announce("the game runs on the engine built for the native extensions of this project, " + engine)
		return engine, nil
	}
	installed, err := editor()
	if err != nil {
		return "", err
	}
	return installed.Engine(filepath.Join(w.cache, enginesDirectory))
}

func playTheGame(command string, args []string, stdout io.Writer, announce func(string), shop *workshop, fail func(error) int) int {
	asked, err := gameArgumentsFrom(command, args)
	if err != nil {
		return fail(err)
	}
	if shop.cache == "" {
		return fail(errors.New("this machine has no cache directory to keep the engine and the running game in"))
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	editor := sync.OnceValues(func() (build.Installation, error) { return shop.installation(asked, announce) })
	step, doesNotBuild := stepsInsteadOfBuilding[command]
	if !doesNotBuild {
		step = stepThatBuilds
	}
	if !asked.isBuilt || doesNotBuild {
		installed, err := editor()
		if err != nil {
			return fail(err)
		}
		if err := installed.Bob(ctx, asked.directory, stdout, step); err != nil {
			fmt.Fprintf(stdout, "%s: %v\n", programName, err)
			return exitBuildFailed
		}
	}
	switch command {
	case commandThatRuns:
		engine, err := shop.engineFor(asked.directory, asked.engine, announce, editor)
		if err != nil {
			return fail(err)
		}
		if err := build.Run(ctx, engine, asked.directory, shop.cache, stdout); err != nil {
			return fail(err)
		}
	case commandThatReloads:
		compiled := make([]string, 0, len(asked.files))
		for _, file := range asked.files {
			name, err := build.CompiledNameOf(asked.directory, file)
			if err != nil {
				return fail(err)
			}
			compiled = append(compiled, name)
		}
		if err := build.Reload(ctx, http.DefaultClient, shop.cache, asked.directory, compiled); err != nil {
			return fail(err)
		}
		fmt.Fprintf(stdout, "reloaded %v\n", compiled)
	}
	return exitSuccess
}

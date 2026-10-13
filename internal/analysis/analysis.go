package analysis

import (
	"io/fs"
	"slices"

	"github.com/Lanterna-Games/defold-tools/internal/check"
	"github.com/Lanterna-Games/defold-tools/internal/config"
	"github.com/Lanterna-Games/defold-tools/internal/engine"
	"github.com/Lanterna-Games/defold-tools/internal/ignore"
	"github.com/Lanterna-Games/defold-tools/internal/libraries"
	"github.com/Lanterna-Games/defold-tools/internal/project"
)

const (
	ProjectFile      = "game.project"
	rootOfTheProject = "/"
)

func IsAProject(files fs.FS) bool {
	_, err := fs.Stat(files, ProjectFile)
	return err == nil
}

type Options struct {
	DefoldInsteadOfTheDeclared string
	ReadBefore                 *project.Parsed
}

type Result struct {
	Problems  []check.Problem
	Project   *project.Project
	Reference *engine.Reference
}

func Run(files fs.FS, versions engine.Versions, options Options) (Result, error) {
	settings, err := config.Load(files)
	if err != nil {
		return Result{}, err
	}
	if options.DefoldInsteadOfTheDeclared != "" {
		settings.Defold = options.DefoldInsteadOfTheDeclared
	}
	reference, err := versions.For(settings.Defold)
	if err != nil {
		return Result{}, err
	}
	withLibraries := libraries.Mount(files)
	loaded, err := project.LoadRemembering(withLibraries.FS, withLibraries, options.ReadBefore)
	if err != nil {
		return Result{}, err
	}
	reference = reference.Knowing(settings.Known.Messages, settings.Known.Properties)
	defold, _ := engine.ParseVersion(reference.Defold)
	ignored := ignore.RulesOf(files, defold)
	problems := slices.DeleteFunc(check.Run(loaded, reference, settings.Options()), func(problem check.Problem) bool {
		return ignored.Hides(rootOfTheProject + problem.Location.File)
	})
	return Result{Problems: problems, Project: loaded, Reference: reference}, nil
}

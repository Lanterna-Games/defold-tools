package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/Lanterna-Games/defold-tools/internal/analysis"
)

const currentDirectory = "."

func runCheck(args []string, stdout, stderr io.Writer, versions versionsWith) (exitCode int) {
	flags := flag.NewFlagSet("check", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	defold := flags.String("defold", "", "")
	if err := flags.Parse(args); err != nil {
		fmt.Fprintf(stderr, "defold-vet: check: %v\n", err)
		return exitCouldNotRun
	}
	directory := currentDirectory
	switch flags.NArg() {
	case 0:
	case 1:
		directory = flags.Arg(0)
	default:
		fmt.Fprintf(stderr, "defold-vet: check takes one directory, got %d arguments\n", flags.NArg())
		return exitCouldNotRun
	}

	files := os.DirFS(directory)
	if !analysis.IsAProject(files) {
		fmt.Fprintf(stderr, "defold-vet: %s is not a Defold project: it has no %s\n", directory, analysis.ProjectFile)
		return exitCouldNotRun
	}
	announce := func(message string) { fmt.Fprintf(stderr, "defold-vet: %s\n", message) }
	analysed, err := analysis.Run(files, versions(announce), analysis.Options{DefoldInsteadOfTheDeclared: *defold})
	problems := analysed.Problems
	if err != nil {
		fmt.Fprintf(stderr, "defold-vet: %v\n", err)
		return exitCouldNotRun
	}
	for _, problem := range problems {
		fmt.Fprintf(stdout, "%s:%d:%d: %s [%s]\n",
			filepath.Join(directory, filepath.FromSlash(problem.Location.File)),
			problem.Location.Position.Line,
			problem.Location.Position.Column,
			problem.Message,
			problem.Check,
		)
	}
	if len(problems) > 0 {
		return exitProblemsFound
	}
	return exitSuccess
}

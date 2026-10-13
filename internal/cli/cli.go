package cli

import (
	"fmt"
	"io"
	"runtime/debug"

	"github.com/Lanterna-Games/defold-tools/internal/engine"
)

const (
	exitSuccess       = 0
	exitProblemsFound = 1
	exitCouldNotRun   = 2
)

const usage = `defold-vet checks Defold projects for references that point at nothing.

Usage:
  defold-vet check [directory]   check the Defold project in a directory (default: the current one)
      --defold <version>         check against this Defold version instead of the one the project declares
  defold-vet lsp                 serve an editor over the Language Server Protocol, on standard input and output
  defold-vet version             print the version
  defold-vet help                print this text

check exits with 0 when it finds nothing, 1 when it finds problems and 2 when it
could not run. It follows the settings in defold-vet.toml, if the project has one.
`

func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) (exitCode int) {
	return run(args, stdin, stdout, stderr, engine.VersionsOfThisMachine)
}

type versionsWith func(announce func(message string)) engine.Versions

func run(args []string, stdin io.Reader, stdout, stderr io.Writer, versions versionsWith) (exitCode int) {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return exitCouldNotRun
	}
	switch command := args[0]; command {
	case "check":
		return runCheck(args[1:], stdout, stderr, versions)
	case "lsp":
		return runServer(stdin, stdout, stderr, versions)
	case "version", "--version":
		fmt.Fprintf(stdout, "defold-vet %s\n", resolveVersion(versionSetByLinker, debug.ReadBuildInfo))
		return exitSuccess
	case "help", "--help", "-h":
		fmt.Fprint(stdout, usage)
		return exitSuccess
	default:
		fmt.Fprintf(stderr, "defold-vet: unknown command %q\n\n%s", command, usage)
		return exitCouldNotRun
	}
}

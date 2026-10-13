package cli

import (
	"fmt"
	"io"
	"runtime/debug"

	"github.com/Lanterna-Games/defold-tools/internal/lsp"
)

const programName = "defold-vet"

func runServer(stdin io.Reader, stdout, stderr io.Writer, versions versionsWith) (exitCode int) {
	server := &lsp.Server{
		Name:     programName,
		Version:  resolveVersion(versionSetByLinker, debug.ReadBuildInfo),
		Versions: versions,
	}
	if err := server.Serve(stdin, stdout); err != nil {
		fmt.Fprintf(stderr, "defold-vet: %v\n", err)
		return exitCouldNotRun
	}
	return exitSuccess
}

package pls

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"runtime/debug"

	"github.com/Lanterna-Games/defold-tools/internal/docs"
	"github.com/Lanterna-Games/defold-tools/internal/insight"
	"github.com/Lanterna-Games/defold-tools/internal/lsp"
)

const (
	programName       = "defoldpls"
	exitSuccess       = 0
	exitCouldNotRun   = 2
	currentDirectory  = "."
	versionWhenUnset  = "(devel)"
	annotationsMethod = "defold/annotations"
)

var versionSetByLinker string

const usage = `defoldpls serves editors that work on Defold projects.

Usage:
  defoldpls                            serve an editor over the Language Server Protocol, on standard input and output
  defoldpls docs [directory]           serve the offline reference of the project's Defold version and print its address
  defoldpls annotations [directory]    write the annotations for the Lua language server and print where they are
  defoldpls fetch <version>...         fetch the documentation of Defold versions, to have it without a network later
  defoldpls build [directory]          build the game with the Defold editor installed on this machine
  defoldpls run [directory]            build the game and run it, printing what it logs
  defoldpls reload <file>...           build again and have the running game take the files that changed
  defoldpls resolve [directory]        fetch the libraries the project depends on
  defoldpls clean [directory]          delete what was built, so that the next build starts from nothing
  defoldpls bundle [directory]         build the game for a platform and pack it, ready to hand out
  defoldpls dap                        serve an editor that debugs the game, over the Debug Adapter Protocol
  defoldpls new <file>                 make a file of a game: a script, a game object, a collection, an atlas, a tile map and more
  defoldpls atlas <file>               add the images that are on disk to an atlas, and take out those that are gone
  defoldpls pack <file>                pack an atlas as the build does and print where each image lands, as JSON
  defoldpls tiles <file>               print the tiles of a tile source, or the cells of a tile map, as JSON
  defoldpls scene <file>               print what a game object, a collection or a GUI scene places, and where, as JSON
  defoldpls move <from> <to>           move a file or a directory of the game, and write its new path wherever it is named
  defoldpls uses <file>                print every place that names a file
  defoldpls version                    print the version
  defoldpls help                       print this text

docs and annotations take --defold <version> to use that version instead of the one the project declares.
annotations takes --output <directory> to write somewhere else than the cache.
bundle takes --platform <platform> (this machine's by default), --variant release or debug (release by default),
--output <directory> (dist/<platform> by default), and after -- anything else for Defold's build tool.
build, run, reload, resolve, clean, bundle and pack take --defold-home <directory> to say where the editor is; DEFOLD_HOME says it too.
run and reload take --built to use what is already built, and run takes --engine <program> to run another engine.
reload takes --project <directory> when it is not run from the project.
tiles and scene take --stdin to read the file from standard input, as an editor holds it, instead of from disk.
new takes --kinds alone to print the kinds of file it makes, as JSON.
new takes --script to make a game object or a GUI scene along with its script, and --from to say what a file is made from:
a directory of images for an atlas, an image for a tile source, a tile source for a tile map, a game object for a factory.
atlas takes --from <directory> to look for images there, and below, instead of next to the ones the atlas has.
`

type annotationsReady struct {
	Defold    string `json:"defold"`
	Directory string `json:"directory"`
	Reference string `json:"reference"`
}

func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) (exitCode int) {
	return run(args, stdin, stdout, stderr, workshopOfThisMachine())
}

func versionOfThisProgram() string {
	if versionSetByLinker != "" {
		return versionSetByLinker
	}
	if info, hasInfo := debug.ReadBuildInfo(); hasInfo && info.Main.Version != "" {
		return info.Main.Version
	}
	return versionWhenUnset
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer, shop *workshop) (exitCode int) {
	command := "serve"
	if len(args) > 0 {
		command, args = args[0], args[1:]
	}
	announce := func(message string) { fmt.Fprintf(stderr, "%s: %s\n", programName, message) }
	fail := func(err error) int {
		fmt.Fprintf(stderr, "%s: %v\n", programName, err)
		return exitCouldNotRun
	}
	switch command {
	case "serve":
		return serve(stdin, stdout, stderr, shop)
	case "docs":
		declared, _, err := projectArguments("docs", args, shop)
		if err != nil {
			return fail(err)
		}
		return serveTheReference(declared, stdout, announce, shop, fail)
	case "annotations":
		declared, output, err := projectArguments("annotations", args, shop)
		if err != nil {
			return fail(err)
		}
		return writeTheAnnotations(declared, output, stdout, announce, shop, fail)
	case "fetch":
		if len(args) == 0 {
			return fail(fmt.Errorf("fetch needs at least one version, as in %q", "defoldpls fetch 1.12.0"))
		}
		for _, version := range args {
			reference, err := shop.documentationOf(version, announce)
			if err != nil {
				return fail(err)
			}
			fmt.Fprintf(stdout, "Defold %s: the documentation of %d modules is on this machine\n", reference.Version, len(reference.Modules))
		}
		return exitSuccess
	case commandThatBuilds, commandThatRuns, commandThatReloads, commandThatFetches, commandThatCleans:
		return playTheGame(command, args, stdout, announce, shop, fail)
	case commandThatDebugs:
		return debugAdapter(stdin, stdout, shop, fail)
	case commandThatMakesAFile:
		return makeAFile(args, stdout, fail)
	case commandThatSyncsAtlas:
		return syncAnAtlas(args, stdout, fail)
	case commandThatPacksAnAtlas:
		return packAnAtlas(args, stdout, stderr, shop, fail)
	case commandThatShowsTiles:
		return showTiles(args, stdin, stdout, fail)
	case commandThatShowsAScene:
		return showAScene(args, stdin, stdout, fail)
	case commandThatBundles:
		return bundleTheGame(args, stdout, announce, shop, fail)
	case commandThatMoves:
		return moveAFile(args, stdout, announce, shop, fail)
	case commandThatSaysUses:
		return sayWhatUses(args, stdout, announce, shop, fail)
	case "version", "--version":
		fmt.Fprintf(stdout, "%s %s\n", programName, versionOfThisProgram())
		return exitSuccess
	case "help", "--help", "-h":
		fmt.Fprint(stdout, usage)
		return exitSuccess
	default:
		fmt.Fprintf(stderr, "%s: unknown command %q\n\n%s", programName, command, usage)
		return exitCouldNotRun
	}
}

func projectArguments(command string, args []string, shop *workshop) (declared, output string, err error) {
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	defold := flags.String("defold", "", "")
	target := flags.String("output", "", "")
	if err := flags.Parse(args); err != nil {
		return "", "", fmt.Errorf("%s: %w", command, err)
	}
	directory := currentDirectory
	switch flags.NArg() {
	case 0:
	case 1:
		directory = flags.Arg(0)
	default:
		return "", "", fmt.Errorf("%s takes one directory, got %d arguments", command, flags.NArg())
	}
	declared = *defold
	if declared == "" {
		declared, err = shop.versionOf(directory)
		if err != nil {
			return "", "", err
		}
	}
	return declared, *target, nil
}

func serveTheReference(declared string, stdout io.Writer, announce func(string), shop *workshop, fail func(error) int) int {
	reference, err := shop.documentationOf(declared, announce)
	if err != nil {
		return fail(err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	address, err := shop.startTheReference(ctx, announce)
	if err != nil {
		return fail(err)
	}
	fmt.Fprintf(stdout, "%s/%s/\n", address, reference.Version)
	<-ctx.Done()
	return exitSuccess
}

func writeTheAnnotations(declared, output string, stdout io.Writer, announce func(string), shop *workshop, fail func(error) int) int {
	reference, err := shop.documentationOf(declared, announce)
	if err != nil {
		return fail(err)
	}
	if output == "" {
		if output, err = shop.annotationsDirectoryOf(reference.Version); err != nil {
			return fail(err)
		}
	}
	if err := writeAnnotations(reference, docs.Links{}, output); err != nil {
		return fail(err)
	}
	fmt.Fprintln(stdout, output)
	return exitSuccess
}

func serve(stdin io.Reader, stdout, stderr io.Writer, shop *workshop) int {
	ctx, stop := context.WithCancel(context.Background())
	defer stop()
	server := &lsp.Server{
		Name:     programName,
		Version:  versionOfThisProgram(),
		Versions: shop.versions,

		AnswersAboutNames: true,

		Ready: func(session lsp.Session) {
			ready, err := prepare(ctx, session.Root, session.Say, session.Consult, shop)
			if err != nil {
				session.Complain(err)
				return
			}
			session.Say(fmt.Sprintf("the annotations of Defold %s are in %s and its reference is at %s", ready.Defold, ready.Directory, ready.Reference))
			session.Notify(annotationsMethod, ready)
		},
	}
	if err := server.Serve(stdin, stdout); err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", programName, err)
		return exitCouldNotRun
	}
	return exitSuccess
}

func prepare(ctx context.Context, root string, announce func(string), consult func(insight.Documentation), shop *workshop) (annotationsReady, error) {
	declared, err := shop.versionOf(root)
	if err != nil {
		return annotationsReady{}, err
	}
	reference, err := shop.documentationOf(declared, announce)
	if err != nil {
		return annotationsReady{}, err
	}
	consult(insight.Documentation{Reference: reference})
	directory, err := shop.annotationsDirectoryOf(reference.Version)
	if err != nil {
		return annotationsReady{}, err
	}
	address, err := shop.startTheReference(ctx, announce)
	if err != nil {
		return annotationsReady{}, err
	}
	consult(insight.Documentation{Reference: reference, Links: docs.Links{OfflineReference: address}})
	if err := writeAnnotations(reference, docs.Links{OfflineReference: address}, directory); err != nil {
		return annotationsReady{}, err
	}
	return annotationsReady{Defold: reference.Version, Directory: directory, Reference: address + "/" + reference.Version + "/"}, nil
}

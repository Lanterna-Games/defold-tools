package pls

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"slices"

	"github.com/Lanterna-Games/defold-tools/internal/build"
)

const (
	commandThatBundles  = "bundle"
	endOfOurOwnFlags    = "--"
	platformsOfThisTool = "x86_64-win32, x86_64-linux, arm64-linux, x86_64-macos, arm64-macos, js-web, wasm-web, arm64-android, armv7-android, arm64-ios"
)

func bundleTheGame(args []string, stdout io.Writer, announce func(string), shop *workshop, fail func(error) int) int {
	forBob := []string{}
	if at := slices.Index(args, endOfOurOwnFlags); at >= 0 {
		args, forBob = args[:at], args[at+1:]
	}
	flags := flag.NewFlagSet(commandThatBundles, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	home := flags.String("defold-home", "", "")
	platform := flags.String("platform", "", "")
	variant := flags.String("variant", build.VariantToRelease, "")
	into := flags.String("output", "", "")
	if err := flags.Parse(args); err != nil {
		return fail(fmt.Errorf("%s: %w", commandThatBundles, err))
	}
	directory := currentDirectory
	switch {
	case flags.NArg() == 1:
		directory = flags.Arg(0)
	case flags.NArg() > 1:
		return fail(fmt.Errorf("%s takes one directory, got %d arguments", commandThatBundles, flags.NArg()))
	}
	if *platform == "" {
		host, isKnown := build.PlatformOfThisMachine()
		if !isKnown {
			return fail(errors.New("say which platform to bundle for with --platform: " + platformsOfThisTool))
		}
		*platform = host
	}
	if *into == "" {
		*into = filepath.Join(filepath.FromSlash(build.DirectoryOfBundles), *platform)
	}
	installed, err := shop.installation(gameArguments{home: *home, directory: directory}, announce)
	if err != nil {
		return fail(err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := installed.Bundle(ctx, directory, *platform, *variant, *into, forBob, stdout); err != nil {
		fmt.Fprintf(stdout, "%s: %v\n", programName, err)
		return exitBuildFailed
	}
	fmt.Fprintf(stdout, "bundled for %s into %s\n", *platform, *into)
	return exitSuccess
}

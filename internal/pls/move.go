package pls

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Lanterna-Games/defold-tools/internal/analysis"
	"github.com/Lanterna-Games/defold-tools/internal/insight"
)

const (
	commandThatMoves     = "move"
	commandThatSaysUses  = "uses"
	markOfAChangedFile   = "~ "
	markOfTheMove        = " -> "
	permissionsOfAFolder = 0o750
)

func (w *workshop) insightInto(root string, announce func(string)) (insight.Insight, error) {
	analysed, err := analysis.Run(os.DirFS(root), w.versions(announce), analysis.Options{})
	if err != nil {
		return insight.Insight{}, err
	}
	return insight.Insight{Project: analysed.Project, Reference: analysed.Reference}, nil
}

func readerOf(root string) func(file string) ([]byte, error) {
	return func(file string) ([]byte, error) {
		return os.ReadFile(filepath.Join(root, filepath.FromSlash(file))) //nolint:gosec // a file of the project the command was run on
	}
}

func placeInTheProject(root, named string) (resourcePath string, err error) {
	absolute, err := filepath.Abs(named)
	if err != nil {
		return "", err
	}
	relative, err := filepath.Rel(root, absolute)
	if err != nil || relative == "." || strings.HasPrefix(relative, "..") {
		return "", fmt.Errorf("%s is not inside the project in %s", named, root)
	}
	return "/" + filepath.ToSlash(relative), nil
}

func moveAFile(args []string, stdout io.Writer, announce func(string), shop *workshop, fail func(error) int) int {
	if len(args) != 2 {
		return fail(fmt.Errorf("%s takes what to move and where to, as in %q", commandThatMoves, "defoldpls move main/ball.go objects/ball.go"))
	}
	if _, err := os.Stat(args[0]); err != nil {
		return fail(fmt.Errorf("%s is not there to move", args[0]))
	}
	if _, err := os.Stat(args[1]); err == nil {
		return fail(fmt.Errorf("%s is already there", args[1]))
	}
	root, from, err := rootOfTheProjectHolding(args[0])
	if err != nil {
		return fail(err)
	}
	to, err := placeInTheProject(root, args[1])
	if err != nil {
		return fail(err)
	}
	known, err := shop.insightInto(root, announce)
	if err != nil {
		return fail(err)
	}
	read := readerOf(root)
	changes, err := known.ChangesToMove([]insight.Move{{From: "/" + from, To: to}}, read)
	if err != nil && !errors.Is(err, insight.ErrNothingToMove) {
		return fail(err)
	}
	byFile := map[string][]insight.Change{}
	var order []string
	for _, change := range changes {
		if _, isListed := byFile[change.Where.File]; !isListed {
			order = append(order, change.Where.File)
		}
		byFile[change.Where.File] = append(byFile[change.Where.File], change)
	}
	rewritten := map[string][]byte{}
	for _, file := range order {
		content, err := read(file)
		if err != nil {
			return fail(err)
		}
		if rewritten[file], err = insight.Apply(content, byFile[file]); err != nil {
			return fail(err)
		}
	}
	for _, file := range order {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(file)), rewritten[file], permissionsOfADataFile); err != nil {
			return fail(err)
		}
		fmt.Fprintln(stdout, markOfAChangedFile+file)
	}
	target := filepath.Join(root, filepath.FromSlash(to))
	if err := os.MkdirAll(filepath.Dir(target), permissionsOfAFolder); err != nil {
		return fail(err)
	}
	if err := os.Rename(filepath.Join(root, filepath.FromSlash(from)), target); err != nil {
		return fail(err)
	}
	fmt.Fprintln(stdout, from+markOfTheMove+strings.TrimPrefix(to, "/"))
	for _, renamed := range insight.AnimationsRenamedBy(changes) {
		announce("the animation " + renamed + ", and what plays it was left as it is")
	}
	return exitSuccess
}

func sayWhatUses(args []string, stdout io.Writer, announce func(string), shop *workshop, fail func(error) int) int {
	if len(args) != 1 {
		return fail(fmt.Errorf("%s takes one file, as in %q", commandThatSaysUses, "defoldpls uses main/sprites.atlas"))
	}
	root, file, err := rootOfTheProjectHolding(args[0])
	if err != nil {
		return fail(err)
	}
	known, err := shop.insightInto(root, announce)
	if err != nil {
		return fail(err)
	}
	for _, place := range known.PlacesThatName("/"+file, readerOf(root)) {
		fmt.Fprintf(stdout, "%s:%d:%d\n", place.Where.File, place.Where.Position.Line, place.Where.Position.Column)
	}
	return exitSuccess
}

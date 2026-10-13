package insight

import (
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/Lanterna-Games/defold-tools/internal/engine"
)

const (
	movesOfEachKind     = 1
	candidatesOfAKind   = 6
	suffixOfAMovedFile  = "_moved_by_the_test"
	mostExamplesOfAMove = 12
)

var kindsOfFileToMove = []string{".go", ".collection", ".atlas", ".lua", ".script", ".gui", ".png", ".tilesource", ".font", ".material"}

type entryUnderAnotherName struct {
	fs.DirEntry

	name string
}

func (e entryUnderAnotherName) Name() string { return e.name }

type directoryWithAMovedFile struct {
	fs.ReadDirFile

	from string
	to   string
}

func (d directoryWithAMovedFile) ReadDir(count int) ([]fs.DirEntry, error) {
	entries, err := d.ReadDirFile.ReadDir(count)
	for index, entry := range entries {
		if entry.Name() == d.from {
			entries[index] = entryUnderAnotherName{DirEntry: entry, name: d.to}
		}
	}
	return entries, err
}

// A project with one of its files under another name in the same directory, and
// some of its files changed, without a byte of it written anywhere.
type projectWithAMovedFile struct {
	base    fs.FS
	from    string
	to      string
	changed fstest.MapFS
}

func (p projectWithAMovedFile) Open(name string) (fs.File, error) {
	if _, isChanged := p.changed[name]; isChanged {
		return p.changed.Open(name)
	}
	switch name {
	case p.from:
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	case p.to:
		return p.base.Open(p.from)
	}
	opened, err := p.base.Open(name)
	if directory, isADirectory := opened.(fs.ReadDirFile); err == nil && isADirectory && name == path.Dir(p.from) {
		return directoryWithAMovedFile{ReadDirFile: directory, from: path.Base(p.from), to: path.Base(p.to)}, nil
	}
	return opened, err
}

// Run with DEFOLD_TOOLS_SAMPLES set to a directory of real Defold projects. In
// each, a file of every kind that something names is given another name in
// memory, the edits that follow it are applied, and the project is read again:
// the checks have to find what they found before, the file being under its new
// name. The projects are not part of the repository, so without the variable
// the test is skipped.
func TestMovesLeaveRealProjectsAsTheyWere(t *testing.T) {
	root := os.Getenv(samplesVariable)
	if root == "" {
		t.Skipf("%s is not set", samplesVariable)
	}
	reference, err := engine.Bundled()
	if err != nil {
		t.Fatal(err)
	}
	var directories []string
	_ = filepath.WalkDir(root, func(file string, entry fs.DirEntry, err error) error {
		if err == nil && !entry.IsDir() && entry.Name() == projectFileOfAGame {
			directories = append(directories, filepath.Dir(file))
		}
		return nil
	})
	projects, moves, edits, differences, animations := 0, 0, 0, 0, 0
	for _, directory := range directories {
		tree := os.DirFS(directory)
		known, before := judged(t, tree, reference)
		read := func(file string) ([]byte, error) { return fs.ReadFile(tree, file) }
		movedHere := 0
		for _, kind := range kindsOfFileToMove {
			tried, done := 0, 0
			for _, file := range known.Project.FilesWithExtension(kind) {
				if tried >= candidatesOfAKind || done >= movesOfEachKind {
					break
				}
				if known.Project.IsOfALibrary(strings.TrimPrefix(file, "/")) || strings.HasPrefix(file, "/builtins/") {
					continue
				}
				tried++
				if len(known.PlacesThatName(file, read)) == 0 {
					continue
				}
				to := strings.TrimSuffix(file, kind) + suffixOfAMovedFile + kind
				changes, err := known.ChangesToMove([]Move{{From: file, To: to}}, read)
				if err != nil {
					t.Errorf("%s: moving %s: %v", directory, file, err)
					continue
				}
				if len(AnimationsRenamedBy(changes)) > 0 {
					animations++
					continue
				}
				done++
				moves++
				edits += len(changes)
				from, target := strings.TrimPrefix(file, "/"), strings.TrimPrefix(to, "/")
				after := projectWithAMovedFile{base: tree, from: from, to: target, changed: fstest.MapFS{}}
				byFile := map[string][]Change{}
				for _, change := range changes {
					byFile[change.Where.File] = append(byFile[change.Where.File], change)
				}
				for name, ofTheFile := range byFile {
					content, err := read(name)
					if err != nil {
						t.Fatalf("%s: %v", directory, err)
					}
					changed, err := Apply(content, ofTheFile)
					if err != nil {
						t.Errorf("%s: moving %s: %v", directory, file, err)
						continue
					}
					if name == from {
						name = target
					}
					after.changed[name] = &fstest.MapFile{Data: changed}
				}
				_, found := judged(t, after, reference)
				expected := make([]string, 0, len(before))
				for _, problem := range before {
					expected = append(expected, strings.Replace(problem, from+":", target+":", 1))
				}
				slices.Sort(expected)
				if !slices.Equal(found, expected) {
					if differences++; differences <= mostExamplesOfAMove {
						t.Errorf("%s: moving %s, named in %d places: only before: %q, only after: %q", directory, file, len(changes), onlyIn(expected, found), onlyIn(found, expected))
					}
				}
				movedHere++
			}
		}
		if movedHere > 0 {
			projects++
		}
	}
	t.Logf("%d files moved in %d of %d projects, by %d edits; %d left the checks saying something else; %d images were left where they were, since a new name would rename their animation",
		moves, projects, len(directories), edits, differences, animations)
	if moves == 0 {
		t.Error("nothing was moved under " + root)
	}
}

package insight

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"testing"
	"testing/fstest"

	"github.com/Lanterna-Games/defold-tools/internal/check"
	"github.com/Lanterna-Games/defold-tools/internal/engine"
	"github.com/Lanterna-Games/defold-tools/internal/project"
)

const (
	samplesVariable       = "DEFOLD_TOOLS_SAMPLES"
	renamesOfEachKind     = 3
	examplesOfEachRefusal = 2
	suffixOfANewName      = "_renamed_by_the_test"
	projectFileOfAGame    = "game.project"
)

var whyRefused = regexp.MustCompile(`message of the engine|which is the engine's|no input binding of the project|also used as something other|could be found to rename|is already named|already has a property|for one owner and|at once|takes its name from the file`)

func onlyIn(these, those []string) []string {
	var only []string
	for _, each := range these {
		if !slices.Contains(those, each) {
			only = append(only, each)
		}
	}
	return only
}

type changedFiles struct {
	base    fs.FS
	changed fstest.MapFS
}

func (c changedFiles) Open(name string) (fs.File, error) {
	if _, isChanged := c.changed[name]; isChanged {
		return c.changed.Open(name)
	}
	return c.base.Open(name)
}

func judged(t *testing.T, tree fs.FS, reference *engine.Reference) (Insight, []string) {
	t.Helper()
	loaded, err := project.Load(tree)
	if err != nil {
		t.Fatal(err)
	}
	var found []string
	for _, problem := range check.Run(loaded, reference, check.Options{}) {
		found = append(found, fmt.Sprintf("%s:%d [%s]", problem.Location.File, problem.Location.Position.Line, problem.Check))
	}
	slices.Sort(found)
	return Insight{Project: loaded, Reference: reference}, found
}

// Run with DEFOLD_TOOLS_SAMPLES set to a directory of real Defold projects. In each of
// them, a few names of every kind are renamed in memory and the project is read again:
// the checks have to find what they found before, on the same lines, and the new name
// has to be written in as many places as the old one was. The projects are not part of
// the repository, so without the variable the test is skipped.
func TestRenamesLeaveRealProjectsAsTheyWere(t *testing.T) {
	root := os.Getenv(samplesVariable)
	if root == "" {
		t.Skipf("%s is not set", samplesVariable)
	}
	reference, err := engine.Bundled()
	if err != nil {
		t.Fatal(err)
	}
	var directories []string
	_ = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err == nil && !entry.IsDir() && entry.Name() == projectFileOfAGame {
			directories = append(directories, filepath.Dir(path))
		}
		return nil
	})
	projects, renames, leftAlone, refusals := 0, 0, 0, map[string]int{}
	for _, directory := range directories {
		tree := os.DirFS(directory)
		known, before := judged(t, tree, reference)
		tried := map[kindOfThing]map[thing]bool{}
		scripts := make([]*project.Script, 0, len(known.Project.Scripts))
		for _, scriptPath := range known.Project.ScriptPaths() {
			scripts = append(scripts, known.Project.Scripts[scriptPath])
		}
		sources := map[string]*project.Script{}
		for _, script := range scripts {
			sources[script.File] = script
		}
		for _, named := range known.everythingNamed(scripts).mentions {
			script, isInAScript := sources[named.where.File]
			if !isInAScript || len(tried[named.kind]) >= renamesOfEachKind || tried[named.kind][named.thing] {
				continue
			}
			{
				if tried[named.kind] == nil {
					tried[named.kind] = map[thing]bool{}
				}
				tried[named.kind][named.thing] = true
				offset := named.where.Position.Offset
				written, _ := known.References(script.File, script.Source, offset)
				renaming, err := known.Rename(script.File, script.Source, offset, named.name+suffixOfANewName)
				if err != nil {
					reason := whyRefused.FindString(err.Error())
					if refusals[reason]++; refusals[reason] <= examplesOfEachRefusal {
						t.Logf("refused in %s: %v", directory, err)
					}
					continue
				}
				changes := renaming.Changes
				if len(renaming.LeftAlone) > 0 {
					leftAlone++
				}
				renames++
				where := fmt.Sprintf("%s: %s in %s", directory, named.described(), script.File)
				after := changedFiles{base: tree, changed: fstest.MapFS{}}
				byFile := map[string][]Change{}
				for _, change := range changes {
					byFile[change.Where.File] = append(byFile[change.Where.File], change)
				}
				for file, ofTheFile := range byFile {
					content, err := fs.ReadFile(tree, file)
					if err != nil {
						t.Fatalf("%s: %v", where, err)
					}
					changed, err := Apply(content, ofTheFile)
					if err != nil {
						t.Errorf("%s: %v", where, err)
						continue
					}
					after.changed[file] = &fstest.MapFile{Data: changed}
				}
				renamed, found := judged(t, after, reference)
				if !slices.Equal(found, before) {
					t.Errorf("%s: the checks find %d problems after the rename and found %d before; only before: %q, only after: %q",
						where, len(found), len(before), onlyIn(before, found), onlyIn(found, before))
				}
				first := changes[0].Where
				content, _ := fs.ReadFile(after, first.File)
				if nowWritten, _ := renamed.References(first.File, content, offsetAfter(changes, changes[0])); len(nowWritten) != len(written) {
					t.Errorf("%s: written in %d places after the rename and in %d before", where, len(nowWritten), len(written))
				}
			}
		}
		projects++
	}
	t.Logf("%d renames in %d projects, %d of them leaving the same text alone somewhere, and these refused: %v", renames, projects, leftAlone, refusals)
}

package analysis

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"testing/fstest"

	"github.com/Lanterna-Games/defold-tools/internal/engine"
	"github.com/Lanterna-Games/defold-tools/internal/project"
)

func problemsRemembering(t *testing.T, tree fs.FS, readBefore *project.Parsed) []string {
	t.Helper()
	result, err := Run(tree, engine.Versions{Announce: func(string) {}}, Options{ReadBefore: readBefore})
	if err != nil {
		t.Fatal(err)
	}
	said := make([]string, 0, len(result.Problems))
	for _, problem := range result.Problems {
		said = append(said, fmt.Sprintf("%s:%d: %s [%s]", problem.Location.File, problem.Location.Position.Line, problem.Message, problem.Check))
	}
	slices.Sort(said)
	return said
}

// An editor asks for a check after every pause in the typing, and one file
// changed since the last. What was read from the others is kept, so that only
// the one that changed is read again. The answer has to be the one a check
// that remembers nothing gives, and a file that is gone has to be forgotten.
func TestACheckReadsAgainOnlyWhatChanged(t *testing.T) {
	tree := gameWithAServer(map[string]string{".defignore": "/server\n"})
	readBefore := project.NewParsed()

	first := problemsRemembering(t, tree, readBefore)
	readAtFirst := readBefore.ReadAnew
	second := problemsRemembering(t, tree, readBefore)

	if len(first) != 0 || !slices.Equal(first, second) || readAtFirst != 4 || readBefore.ReadAnew != readAtFirst || readBefore.Reused != readAtFirst {
		t.Fatalf("the first check found %q after reading %d files, the second %q after reading %d more and keeping %d; want the same answer with nothing read again", first, readAtFirst, second, readBefore.ReadAnew-readAtFirst, readBefore.Reused)
	}

	tree["main/ball.script"] = &fstest.MapFile{Data: []byte("function init(self)\n    msg.post(\"#gone\", \"disable\")\nend\n")}
	afterAChange := problemsRemembering(t, tree, readBefore)
	if want := problemsRemembering(t, tree, nil); len(afterAChange) != 1 || !slices.Equal(afterAChange, want) || readBefore.ReadAnew != readAtFirst+1 {
		t.Errorf("after one file changed the check found %q, having read %d files again; a check that remembers nothing finds %q", afterAChange, readBefore.ReadAnew-readAtFirst, want)
	}

	delete(tree, "main/ball.script")
	withoutTheFile := problemsRemembering(t, tree, readBefore)
	tree["main/ball.script"] = &fstest.MapFile{Data: []byte("function init(self)\n    msg.post(\"#gone\", \"disable\")\nend\n")}
	readBeforeItCameBack := readBefore.ReadAnew
	backAgain := problemsRemembering(t, tree, readBefore)
	if slices.Equal(withoutTheFile, afterAChange) || !slices.Equal(backAgain, afterAChange) || readBefore.ReadAnew != readBeforeItCameBack+1 {
		t.Errorf("without the script the check found %q, and with it back %q after reading %d files; want the script forgotten while it was gone and read once when it came back", withoutTheFile, backAgain, readBefore.ReadAnew-readBeforeItCameBack)
	}
}

// Run with DEFOLD_TOOLS_SAMPLES set to a directory of real Defold projects. Each
// is checked three times: remembering nothing, then twice with what was read
// kept between the two. The three have to say the same, and the third has to
// read no file again. The projects are not part of the repository, so without
// the variable the test is skipped.
func TestRememberingChangesNothingInRealProjects(t *testing.T) {
	root := os.Getenv("DEFOLD_TOOLS_SAMPLES")
	if root == "" {
		t.Skip("DEFOLD_TOOLS_SAMPLES is not set")
	}
	var directories []string
	_ = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err == nil && !entry.IsDir() && entry.Name() == ProjectFile {
			directories = append(directories, filepath.Dir(path))
		}
		return nil
	})
	projects, problems, kept := 0, 0, 0
	for _, directory := range directories {
		tree := os.DirFS(directory)
		readBefore := project.NewParsed()
		plain := problemsRemembering(t, tree, nil)
		first := problemsRemembering(t, tree, readBefore)
		readAtFirst := readBefore.ReadAnew
		second := problemsRemembering(t, tree, readBefore)
		if !slices.Equal(plain, first) || !slices.Equal(plain, second) {
			t.Errorf("%s: %d problems remembering nothing, %d the first time and %d the second", directory, len(plain), len(first), len(second))
		}
		if readBefore.ReadAnew != readAtFirst || readBefore.Reused != readAtFirst {
			t.Errorf("%s: %d files read at first, then %d read again and %d kept", directory, readAtFirst, readBefore.ReadAnew-readAtFirst, readBefore.Reused)
		}
		projects, problems, kept = projects+1, problems+len(plain), kept+readBefore.Reused
	}
	t.Logf("%d projects with %d problems said the same three times, %d files kept from one check to the next", projects, problems, kept)
}

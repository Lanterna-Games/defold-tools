package ignore

import (
	"testing"
	"testing/fstest"

	"github.com/Lanterna-Games/defold-tools/internal/engine"
)

var (
	withPatterns   = engine.Version{Major: 1, Minor: 14, Patch: 0}
	beforePatterns = engine.Version{Major: 1, Minor: 13, Patch: 2}
)

func rulesFrom(t *testing.T, content string, defold engine.Version) Rules {
	t.Helper()
	return RulesOf(fstest.MapFS{FileName: &fstest.MapFile{Data: []byte(content)}}, defold)
}

// The table Defold gave with the change that brought patterns to .defignore, in
// the notes of 1.14.0. Its build tool and its editor share one matcher, and
// this has to agree with it line by line.
func TestPatternsMatchAsDefoldSaysTheyDo(t *testing.T) {
	cases := []struct {
		pattern string
		hidden  []string
		kept    []string
	}{
		{"/levels", []string{"/levels", "/levels/1/map.tilemap"}, []string{"/levelsX", "/Levels"}},
		{"/a.b", []string{"/a.b", "/a.b/m.txt"}, []string{"/aXb"}},
		{"/levels/*/tiled", []string{"/levels/1/tiled", "/levels/1/tiled/a.txt"}, []string{"/levels/tiled", "/levels/1/tiledX", "/levels/1/other.txt"}},
		{"/levels/*", []string{"/levels/a.txt", "/levels/a/b.txt"}, []string{"/levels"}},
		{"/assets/temp_??.png", []string{"/assets/temp_01.png"}, []string{"/assets/temp_1.png", "/assets/temp_001.png"}},
		{"/dir(1)/*.png", []string{"/dir(1)/a.png"}, []string{"/dir1/a.png", "/dir(1)/b.txt"}},
		{"/**/generated", []string{"/generated", "/a/b/generated/n.txt"}, []string{"/a/b/ungenerated"}},
		{"/levels/**", []string{"/levels", "/levels/a/b/c.txt"}, []string{"/levelsX"}},
		{"/a/**/b", []string{"/a/b", "/a/x/y/b"}, []string{"/a/xb"}},
	}
	for _, testCase := range cases {
		t.Run(testCase.pattern, func(t *testing.T) {
			rules := rulesFrom(t, testCase.pattern+"\n", withPatterns)
			for _, resourcePath := range testCase.hidden {
				if !rules.Hides(resourcePath) {
					t.Errorf("%s does not hide %s", testCase.pattern, resourcePath)
				}
			}
			for _, resourcePath := range testCase.kept {
				if rules.Hides(resourcePath) {
					t.Errorf("%s hides %s", testCase.pattern, resourcePath)
				}
			}
		})
	}
}

// Defold's build tool reads the file a line at a time and takes only the lines
// that start with a slash: there are no comments, and a line written without
// one names nothing. A slash at the end is dropped, and so is the line ending
// of a file saved on Windows.
func TestOnlyLinesThatStartWithASlashAreRules(t *testing.T) {
	rules := rulesFrom(t, "# generated\ndist\n\n/server/\r\n/assets/raw\n/\n", withPatterns)

	for resourcePath, wantHidden := range map[string]bool{
		"/server":          true,
		"/server/main.go":  true,
		"/assets/raw/a.go": true,
		"/dist/index.html": false,
		"/assets/ball.png": false,
		"/main/ball.go":    false,
	} {
		if rules.Hides(resourcePath) != wantHidden {
			t.Errorf("Hides(%q) = %v, want %v", resourcePath, !wantHidden, wantHidden)
		}
	}
}

// Before 1.14.0 a star was a character like any other, so a line with one named
// a path that had a star in it, which is to say nothing. A project that targets
// such a version is read the way its Defold reads it.
func TestBeforeDefoldHadPatternsALineIsAPath(t *testing.T) {
	rules := rulesFrom(t, "/levels/*/tiled\n/server\n", beforePatterns)

	if rules.Hides("/levels/1/tiled") {
		t.Error("a pattern was applied for a Defold that has none")
	}
	if !rules.Hides("/levels/*/tiled/a.txt") || !rules.Hides("/server/main.go") {
		t.Error("a line is not read as the path it spells")
	}
}

func TestAProjectWithoutTheFileIgnoresNothing(t *testing.T) {
	rules := RulesOf(fstest.MapFS{"game.project": &fstest.MapFile{}, "server/main.go": &fstest.MapFile{}}, withPatterns)

	if rules.Hides("/server/main.go") || rules.Hides("/game.project") {
		t.Error("something is ignored in a project that ignores nothing")
	}
}

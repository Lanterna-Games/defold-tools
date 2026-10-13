package check

import (
	"testing"
	"testing/fstest"

	"github.com/Lanterna-Games/defold-tools/internal/engine"
	"github.com/Lanterna-Games/defold-tools/internal/project"
)

func problemsWith(t *testing.T, files map[string]string, options Options) []Problem {
	t.Helper()
	tree := fstest.MapFS{}
	for name, content := range files {
		tree[name] = &fstest.MapFile{Data: []byte(content)}
	}
	loaded, err := project.Load(tree)
	if err != nil {
		t.Fatal(err)
	}
	reference, err := engine.Bundled()
	if err != nil {
		t.Fatal(err)
	}
	return Run(loaded, reference, options)
}

const scriptWithTwoBrokenAddresses = `function init(self)
    msg.post("#sprit", "disable")
    msg.post("#nowhere", "disable")
end
`

func TestACheckThatIsTurnedOffReportsNothing(t *testing.T) {
	files := pongWithBallScript(scriptWithTwoBrokenAddresses)

	assertNoProblems(t, problemsWith(t, files, Options{TurnedOff: map[string]bool{unresolvedURLCheck: true}}))
	if got := problemsWith(t, files, Options{TurnedOff: map[string]bool{missingResourceCheck: true}}); len(got) != 2 {
		t.Errorf("got %d problems with another check turned off, want 2", len(got))
	}
}

func TestAProblemIsIgnoredWhereTheSettingsSay(t *testing.T) {
	files := pongWithBallScript(scriptWithTwoBrokenAddresses)
	ignores := map[string]struct {
		ignore Ignore
		want   int
	}{
		"the file":          {ignore: Ignore{Check: unresolvedURLCheck, Path: "main/ball.script"}, want: 0},
		"its directory":     {ignore: Ignore{Check: unresolvedURLCheck, Path: "main/*"}, want: 0},
		"any depth":         {ignore: Ignore{Check: unresolvedURLCheck, Path: "**/*.script"}, want: 0},
		"everything":        {ignore: Ignore{Check: unresolvedURLCheck, Path: "**"}, want: 0},
		"another check":     {ignore: Ignore{Check: missingResourceCheck, Path: "**"}, want: 2},
		"another file":      {ignore: Ignore{Check: unresolvedURLCheck, Path: "main/game.script"}, want: 2},
		"another directory": {ignore: Ignore{Check: unresolvedURLCheck, Path: "levels/**"}, want: 2},
		"only one level":    {ignore: Ignore{Check: unresolvedURLCheck, Path: "*"}, want: 2},
	}
	for name, testCase := range ignores {
		t.Run(name, func(t *testing.T) {
			if got := problemsWith(t, files, Options{Ignores: []Ignore{testCase.ignore}}); len(got) != testCase.want {
				t.Errorf("got %d problems, want %d: %+v", len(got), testCase.want, got)
			}
		})
	}
}

func TestAProblemIsIgnoredOnTheLineThatSaysSoOrTheOneBelow(t *testing.T) {
	problems := problemsIn(t, pongWithBallScript(`function init(self)
    msg.post("#sprit", "disable") -- defold-vet:ignore unresolved-url the sprite is added by the level loader
    -- defold-vet:ignore unresolved-url,unknown-property added by the level loader too
    go.set("#glow", "strength", 1)

    msg.post("#nowhere", "disable")
end
`))

	assertOneProblem(t, problems, unresolvedURLCheck, "main/ball.script", 6, `"#nowhere"`)
}

// An ignore is a claim that something is wrong on purpose. One that gives no
// reason, names nothing real or silences nothing is itself a problem.
func TestIgnoresThatAreNotInOrder(t *testing.T) {
	ignores := map[string]struct {
		directive string
		want      string
		wantURL   bool
	}{
		"no reason":             {directive: `-- defold-vet:ignore unresolved-url`, want: `this ignore gives no reason: write why after "unresolved-url"`, wantURL: true},
		"no check":              {directive: `-- defold-vet:ignore`, want: "this ignore names no check", wantURL: true},
		"a check that is not":   {directive: `-- defold-vet:ignore unresolved-ulr because of the loader`, want: `"unresolved-ulr" is not a check (did you mean "unresolved-url"?)`, wantURL: true},
		"nothing to ignore":     {directive: `-- defold-vet:ignore unknown-animation because of the loader`, want: `nothing is reported here for "unknown-animation": this ignore can go`, wantURL: true},
		"one of two is unused":  {directive: `-- defold-vet:ignore unresolved-url,unbound-action because of the loader`, want: `nothing is reported here for "unbound-action"`},
		"a comment that is not": {directive: `-- we should defold-vet:ignore this some day`, wantURL: true},
	}
	for name, testCase := range ignores {
		t.Run(name, func(t *testing.T) {
			problems := problemsIn(t, pongWithBallScript("function init(self)\n    msg.post(\"#sprit\", \"disable\") "+testCase.directive+"\nend\n"))

			if got := len(problemsOf(problems, unresolvedURLCheck)) == 1; got != testCase.wantURL {
				t.Errorf("the broken address is reported: %v, want %v", got, testCase.wantURL)
			}
			about := problemsOf(problems, ignoreCheck)
			if testCase.want == "" {
				assertNoProblems(t, about)
				return
			}
			assertOneProblem(t, about, ignoreCheck, "main/ball.script", 2, testCase.want)
		})
	}
}

func TestAnIgnoreOfACheckThatIsTurnedOffIsNotCalledUnused(t *testing.T) {
	files := pongWithBallScript(`function init(self)
    msg.post("#sprit", "disable") -- defold-vet:ignore unresolved-url added by the level loader
end
`)

	assertNoProblems(t, problemsWith(t, files, Options{TurnedOff: map[string]bool{unresolvedURLCheck: true}}))
}

// An editor shows a mistake as an error and the rest as a warning. Three checks
// report what does no harm as it stands: a name that cannot be read, a file
// nothing uses, and a setting Defold no longer reads.
func TestOnlyWhatIsWrongInTheGameIsAMistake(t *testing.T) {
	for _, name := range append(Names(), ignoreCheck) {
		isOfUpkeep := name == unverifiableCheck || name == unusedFileCheck || name == removedSettingCheck
		if IsAMistake(name) == isOfUpkeep {
			t.Errorf("IsAMistake(%q) = %v", name, !isOfUpkeep)
		}
	}
}

func TestEveryCheckThatReportsIsNamed(t *testing.T) {
	for _, name := range Names() {
		if !IsACheck(name) {
			t.Errorf("%q is listed and is not a check", name)
		}
	}
	if IsACheck(ignoreCheck) || IsACheck("") {
		t.Error("the report about ignores is not a check that can be turned off or ignored")
	}
}

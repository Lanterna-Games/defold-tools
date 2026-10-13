package cli

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Lanterna-Games/defold-tools/internal/engine"
)

func writeProject(t *testing.T, files map[string]string) (directory string) {
	t.Helper()
	directory = t.TempDir()
	for name, content := range files {
		path := filepath.Join(directory, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return directory
}

const collectionWithOneBall = `name: "main"
instances {
  id: "ball"
  prototype: "/main/ball.go"
}
`

const settingsOfPong = "[project]\ntitle = Pong\n\n[bootstrap]\nmain_collection = /main/main.collectionc\n\n[input]\ngame_binding = /builtins/input/all.input_bindingc\n"

func TestCheckIsSilentOnAProjectWithoutProblems(t *testing.T) {
	directory := writeProject(t, map[string]string{
		"game.project":         settingsOfPong,
		"main/main.collection": collectionWithOneBall,
		"main/ball.go":         "",
	})
	var stdout, stderr bytes.Buffer

	exitCode := Run([]string{"check", directory}, strings.NewReader(""), &stdout, &stderr)

	if exitCode != exitSuccess || stdout.Len() > 0 || stderr.Len() > 0 {
		t.Errorf("exit code = %d, stdout = %q, stderr = %q; want 0 and silence", exitCode, stdout.String(), stderr.String())
	}
}

func TestCheckPrintsProblemsAsFileLineColumn(t *testing.T) {
	directory := writeProject(t, map[string]string{
		"game.project":         settingsOfPong,
		"main/main.collection": collectionWithOneBall,
		"main/bal.go":          "",
	})
	var stdout, stderr bytes.Buffer

	exitCode := Run([]string{"check", directory}, strings.NewReader(""), &stdout, &stderr)

	want := filepath.Join(directory, "main", "bal.go") + ":1:1: nothing in the project uses this file [unused-file]\n" +
		filepath.Join(directory, "main", "main.collection") +
		`:4:14: "/main/ball.go" does not exist (did you mean "/main/bal.go"?) [missing-resource]` + "\n"
	if exitCode != exitProblemsFound {
		t.Errorf("exit code = %d, want %d", exitCode, exitProblemsFound)
	}
	if stdout.String() != want {
		t.Errorf("stdout =\n%s\nwant\n%s", stdout.String(), want)
	}
}

func TestCheckRefusesADirectoryThatIsNotADefoldProject(t *testing.T) {
	directory := writeProject(t, map[string]string{"main/main.collection": collectionWithOneBall})
	var stdout, stderr bytes.Buffer

	exitCode := Run([]string{"check", directory}, strings.NewReader(""), &stdout, &stderr)

	if exitCode != exitCouldNotRun {
		t.Errorf("exit code = %d, want %d", exitCode, exitCouldNotRun)
	}
	assertOutput(t, "stdout", stdout.String(), "")
	assertOutput(t, "stderr", stderr.String(), "is not a Defold project: it has no game.project")
}

func TestCheckTakesAtMostOneDirectory(t *testing.T) {
	var stdout, stderr bytes.Buffer

	exitCode := Run([]string{"check", "a", "b"}, strings.NewReader(""), &stdout, &stderr)

	if exitCode != exitCouldNotRun {
		t.Errorf("exit code = %d, want %d", exitCode, exitCouldNotRun)
	}
	assertOutput(t, "stderr", stderr.String(), "check takes one directory, got 2 arguments")
}

func projectWithAMissingBall(settings string) map[string]string {
	files := map[string]string{
		"game.project":         settingsOfPong,
		"main/main.collection": collectionWithOneBall,
	}
	if settings != "" {
		files["defold-vet.toml"] = settings
	}
	return files
}

func TestCheckFollowsTheSettingsOfTheProject(t *testing.T) {
	newest, hasAny := engine.NewestBundled()
	if !hasAny {
		t.Fatal("this build carries no reference")
	}
	settings := map[string]struct {
		content  string
		wantExit int
	}{
		"none":                   {content: "", wantExit: exitProblemsFound},
		"the check turned off":   {content: "[checks]\nmissing-resource = false\n", wantExit: exitSuccess},
		"another check off":      {content: "[checks]\nunresolved-url = false\n", wantExit: exitProblemsFound},
		"the file ignored":       {content: "[[ignore]]\ncheck = \"missing-resource\"\npath = \"main/**\"\nreason = \"the ball is generated\"\n", wantExit: exitSuccess},
		"another file ignored":   {content: "[[ignore]]\ncheck = \"missing-resource\"\npath = \"levels/**\"\nreason = \"generated\"\n", wantExit: exitProblemsFound},
		"the version it bundles": {content: "defold = \"" + newest.String() + "\"\n", wantExit: exitProblemsFound},
		"an older version":       {content: "defold = \"1.6.0\"\n", wantExit: exitProblemsFound},
		"names it vouches for":   {content: "[known]\nmessages = [\"set_scale\"]\nproperties = [\"outline_width\"]\n", wantExit: exitProblemsFound},
	}
	for name, testCase := range settings {
		t.Run(name, func(t *testing.T) {
			directory := writeProject(t, projectWithAMissingBall(testCase.content))
			var stdout, stderr bytes.Buffer

			exitCode := Run([]string{"check", directory}, strings.NewReader(""), &stdout, &stderr)

			if exitCode != testCase.wantExit || stderr.Len() > 0 {
				t.Errorf("exit code = %d, stderr = %q; want %d and nothing on stderr", exitCode, stderr.String(), testCase.wantExit)
			}
		})
	}
}

// A setting that is not understood must not pass for one that is: a check that
// was meant to be off, or on, would run the other way without a word.
func TestCheckRefusesSettingsItDoesNotUnderstand(t *testing.T) {
	settings := map[string]struct {
		content string
		want    string
	}{
		"not TOML":                {content: "checks = [\n", want: "defold-vet.toml"},
		"an unknown setting":      {content: "strictness = 3\n", want: `"strictness" is not a setting`},
		"an unknown check":        {content: "[checks]\nmissing-resorce = false\n", want: `"missing-resorce" is not a check`},
		"an ignore with no why":   {content: "[[ignore]]\ncheck = \"missing-resource\"\npath = \"main/**\"\n", want: "ignore 1 needs a check, a path and a reason"},
		"an ignore of no check":   {content: "[[ignore]]\ncheck = \"everything\"\npath = \"**\"\nreason = \"tired\"\n", want: `"everything" is not a check`},
		"a version nobody made":   {content: "defold = \"9.9.9\"\n", want: "version 9.9.9 of Defold was never published"},
		"a version too old":       {content: "defold = \"1.5.0\"\n", want: "the oldest version defold-vet supports is 1.6.0"},
		"something for a version": {content: "defold = \"latest\"\n", want: `"latest" is not a Defold version`},
		"a setting of wrong type": {content: "[checks]\nmissing-resource = \"off\"\n", want: "defold-vet.toml"},
	}
	for name, testCase := range settings {
		t.Run(name, func(t *testing.T) {
			directory := writeProject(t, projectWithAMissingBall(testCase.content))
			var stdout, stderr bytes.Buffer

			exitCode := run([]string{"check", directory}, strings.NewReader(""), &stdout, &stderr, versionsWhereNothingIsPublished(t))

			if exitCode != exitCouldNotRun || stdout.Len() > 0 || !strings.Contains(stderr.String(), testCase.want) {
				t.Errorf("exit code = %d, stdout = %q, stderr = %q; want %d and %q on stderr", exitCode, stdout.String(), stderr.String(), exitCouldNotRun, testCase.want)
			}
		})
	}
}

func versionsWhereNothingIsPublished(t *testing.T) versionsWith {
	t.Helper()
	server := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(server.Close)
	return func(announce func(message string)) engine.Versions {
		return engine.Versions{
			Publisher: engine.Publisher{Tags: server.URL, Archive: server.URL},
			Client:    server.Client(),
			Announce:  announce,
		}
	}
}

func TestCheckCanBeAskedAboutAnotherDefoldVersion(t *testing.T) {
	directory := writeProject(t, map[string]string{
		"game.project":         settingsOfPong,
		"defold-vet.toml":      "defold = \"1.13.2\"\n",
		"main/main.collection": "name: \"main\"\ninstances {\n  id: \"game\"\n  prototype: \"/main/game.go\"\n}\n",
		"main/game.go":         "components {\n  id: \"game\"\n  component: \"/main/game.script\"\n}\n",
		"main/game.script":     "function on_message(self, message_id)\n    if message_id == hash(\"trigger_event\") then\n        self.hit = true\n    end\nend\n",
	})
	asked := map[string]struct {
		arguments []string
		wantExit  int
		want      string
	}{
		"the declared version knows the message": {arguments: []string{"check", directory}, wantExit: exitSuccess},
		"an older one does not":                  {arguments: []string{"check", "--defold", "1.6.0", directory}, wantExit: exitProblemsFound, want: `"trigger_event" is handled here, but nothing in the project posts it`},
		"a version too old":                      {arguments: []string{"check", "--defold=1.5.0", directory}, wantExit: exitCouldNotRun, want: "the oldest version defold-vet supports is 1.6.0"},
		"a flag it does not have":                {arguments: []string{"check", "--version-of-defold", "1.6.0", directory}, wantExit: exitCouldNotRun, want: "flag provided but not defined"},
	}
	for name, testCase := range asked {
		t.Run(name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer

			exitCode := run(testCase.arguments, strings.NewReader(""), &stdout, &stderr, versionsWhereNothingIsPublished(t))

			if exitCode != testCase.wantExit || !strings.Contains(stdout.String()+stderr.String(), testCase.want) {
				t.Errorf("exit code = %d, stdout = %q, stderr = %q; want %d and %q", exitCode, stdout.String(), stderr.String(), testCase.wantExit, testCase.want)
			}
		})
	}
}

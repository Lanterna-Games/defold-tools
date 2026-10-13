package analysis

import (
	"slices"
	"testing"
	"testing/fstest"
)

const sourceOfAServerInGo = "package main\n\nfunc main() {}\n"

// A game that keeps the Go source of its server next to it, which is the case
// Defold's manual gives for .defignore: to Defold, a file that ends in .go is a
// game object.
func gameWithAServer(more map[string]string) fstest.MapFS {
	tree := fstest.MapFS{
		"game.project":         {Data: []byte("[project]\ntitle = Pong\n\n[bootstrap]\nmain_collection = /main/main.collectionc\n\n[input]\ngame_binding = /builtins/input/all.input_bindingc\n")},
		"defold-vet.toml":      {Data: []byte("strict = false\n")},
		"main/main.collection": {Data: []byte("name: \"main\"\ninstances {\n  id: \"ball\"\n  prototype: \"/main/ball.go\"\n}\n")},
		"main/ball.go":         {Data: []byte("components {\n  id: \"ball\"\n  component: \"/main/ball.script\"\n}\n")},
		"main/ball.script":     {Data: []byte("function init(self)\nend\n")},
		"server/main.go":       {Data: []byte(sourceOfAServerInGo)},
	}
	for name, content := range more {
		tree[name] = &fstest.MapFile{Data: []byte(content)}
	}
	return tree
}

// What .defignore names is not found by Defold's build tool when it looks
// through the project, and nothing is reported in it here: not what is wrong
// inside it, and not that nothing uses it.
func TestNothingIsReportedInWhatDefoldIsToldToIgnore(t *testing.T) {
	cases := map[string]struct {
		more map[string]string
		want []string
	}{
		"nothing is ignored": {
			want: []string{
				`server/main.go:1: nothing in the project uses this file [unused-file]`,
				`server/main.go:3: unexpected "{" [syntax]`,
			},
		},
		"the directory is ignored": {more: map[string]string{".defignore": "/server\n"}},
		"a pattern ignores it":     {more: map[string]string{".defignore": "/serv*\n"}},
		"a pattern, for a Defold that has none": {
			more: map[string]string{".defignore": "/serv*\n", "defold-vet.toml": "strict = false\ndefold = \"1.13.2\"\n"},
			want: []string{
				`server/main.go:1: nothing in the project uses this file [unused-file]`,
				`server/main.go:3: unexpected "{" [syntax]`,
			},
		},
	}
	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			if got := problemsOf(t, gameWithAServer(testCase.more)); !slices.Equal(got, testCase.want) {
				t.Errorf("problems = %q, want %q", got, testCase.want)
			}
		})
	}
}

// Being ignored keeps a file from being found, not from being built: Defold's
// build tool builds a game object that a collection places whether .defignore
// names it or not. That was seen with the build tool of 1.14.0, for a path, a
// file and a pattern. So such a file is not called missing where it is named,
// and what it uses is not called unused. What is wrong in it is still not
// reported: that is what the file asked for.
func TestWhatIsIgnoredAndStillNamedIsStillOfTheGame(t *testing.T) {
	cases := map[string]string{
		"the file as it is":             "components {\n  id: \"ball\"\n  component: \"/main/ball.script\"\n}\n",
		"the file with a mistake in it": "components {\n  id: \"ball\"\n  component: \"/main/ball.script\"\n}\ncomponents {\n  id: \"trail\"\n  component: \"/main/trial.script\"\n}\n",
	}
	for name, ball := range cases {
		t.Run(name, func(t *testing.T) {
			got := problemsOf(t, gameWithAServer(map[string]string{".defignore": "/server\n/main/ball.go\n", "main/ball.go": ball}))

			if len(got) != 0 {
				t.Errorf("problems = %q, want none", got)
			}
		})
	}
}

// .defunload is for Defold's editor alone: it keeps what the file names from
// being loaded there, and the build still takes all of it. The game has those
// files, so they are read and judged like the rest.
func TestWhatTheEditorIsToldNotToLoadIsStillRead(t *testing.T) {
	got := problemsOf(t, gameWithAServer(map[string]string{".defunload": "/server\n/main\n", ".defignore": "/server\n", "main/ball.go": "components {\n  id: \"ball\"\n  component: \"/main/bal.script\"\n}\n"}))

	want := []string{
		`main/ball.go:3: "/main/bal.script" does not exist (did you mean "/main/ball.script"?) [missing-resource]`,
		`main/ball.script:1: nothing in the project uses this file [unused-file]`,
	}
	if !slices.Equal(got, want) {
		t.Errorf("problems = %q, want %q", got, want)
	}
}

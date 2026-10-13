package lsp

import "testing"

func TestPathsAndURIsOfBothKindsOfSystem(t *testing.T) {
	cases := map[string]string{
		"file:///home/dev/pong":                  "/home/dev/pong",
		"file:///home/dev/my%20game/main.script": "/home/dev/my game/main.script",
		"file:///c%3A/Users/dev/Pong":            "C:/Users/dev/Pong",
		"file:///C:/Users/dev/Pong":              "C:/Users/dev/Pong",
	}
	for uri, want := range cases {
		got, isAFile := pathOf(uri)
		if !isAFile || got != want {
			t.Errorf("pathOf(%q) = %q, %v; want %q", uri, got, isAFile, want)
		}
		if again, _ := pathOf(uriOf(got)); again != got {
			t.Errorf("uriOf(%q) = %q, which does not lead back to it", got, uriOf(got))
		}
	}
	for _, uri := range []string{"untitled:Untitled-1", "git:/home/dev/pong/main.script", "", "::"} {
		if got, isAFile := pathOf(uri); isAFile {
			t.Errorf("pathOf(%q) = %q, want it not to be a file", uri, got)
		}
	}
}

func TestAPathIsToldApartFromItsProject(t *testing.T) {
	cases := []struct {
		root, file, want string
		inside           bool
	}{
		{root: "/home/dev/pong", file: "/home/dev/pong/main/ball.script", want: "main/ball.script", inside: true},
		{root: "/home/dev/pong/", file: "/home/dev/pong/game.project", want: "game.project", inside: true},
		{root: "/home/dev/pong", file: "/home/dev/pong2/main.script"},
		{root: "/home/dev/pong", file: "/home/dev/pong"},
		{root: "/home/dev/pong", file: "/home/dev/Pong/main.script"},
		{root: "C:/Users/dev/Pong", file: "C:/users/dev/pong/main/ball.script", want: "main/ball.script", inside: true},
	}
	for _, testCase := range cases {
		got, isInside := relativeTo(testCase.root, testCase.file)
		if isInside != testCase.inside || got != testCase.want {
			t.Errorf("relativeTo(%q, %q) = %q, %v; want %q, %v", testCase.root, testCase.file, got, isInside, testCase.want, testCase.inside)
		}
	}
}

func TestARangeCoversWhatTheProblemPointsAt(t *testing.T) {
	content := []byte("local a = 1\n    msg.post(\"#sprit\", 'disable')\n  default_animation: \\\"bal\\\"\nação = go.get(url, name)\n")
	cases := []struct {
		name         string
		line, column int
		want         textRange
	}{
		{name: "a quoted string", line: 2, column: 14, want: textRange{Start: position{1, 13}, End: position{1, 21}}},
		{name: "a string in single quotes", line: 2, column: 24, want: textRange{Start: position{2 - 1, 23}, End: position{1, 32}}},
		{name: "a name", line: 4, column: 22, want: textRange{Start: position{3, 19}, End: position{3, 23}}},
		{name: "after letters that take two bytes", line: 4, column: 10, want: textRange{Start: position{3, 7}, End: position{3, 13}}},
		{name: "a line that is not there", line: 9, column: 3, want: textRange{Start: position{8, 0}, End: position{8, 0}}},
		{name: "a column past the end", line: 1, column: 90, want: textRange{Start: position{0, 11}, End: position{0, 11}}},
	}
	for _, testCase := range cases {
		if got := rangeAt(content, testCase.line, testCase.column); got != testCase.want {
			t.Errorf("%s: range = %+v, want %+v", testCase.name, got, testCase.want)
		}
	}
}

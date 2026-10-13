package config

import (
	"testing"
	"testing/fstest"
)

func TestAProjectWithoutSettingsGetsTheDefaults(t *testing.T) {
	loaded, err := Load(fstest.MapFS{"game.project": &fstest.MapFile{}})
	if err != nil {
		t.Fatal(err)
	}

	options := loaded.Options()

	if loaded.Defold != "" || len(options.TurnedOff) != 0 || len(options.Ignores) != 0 {
		t.Errorf("settings = %+v, options = %+v; want nothing set", loaded, options)
	}
}

func TestStrictModeIsOnUntilTheProjectLeavesIt(t *testing.T) {
	cases := map[string]bool{
		"":                                 false,
		"strict = true\n":                  false,
		"strict = false\n":                 true,
		"[checks]\nunverifiable = false\n": true,
		"strict = true\n[checks]\nunverifiable = false\n": true,
	}
	for content, wantOff := range cases {
		loaded, err := Load(fstest.MapFS{FileName: &fstest.MapFile{Data: []byte(content)}})
		if err != nil {
			t.Fatal(err)
		}
		if got := loaded.Options().TurnedOff["unverifiable"]; got != wantOff {
			t.Errorf("%q: the report of what cannot be verified is off = %v, want %v", content, got, wantOff)
		}
	}
}

func TestSettingsBecomeOptionsOfTheChecks(t *testing.T) {
	loaded, err := Load(fstest.MapFS{FileName: &fstest.MapFile{Data: []byte(`defold = "1.13.2"

[checks]
unknown-collision-group = false
unresolved-url = true

[[ignore]]
check = "missing-resource"
path = "assets/generated/**"
reason = "written by the build"
`)}})
	if err != nil {
		t.Fatal(err)
	}

	options := loaded.Options()

	if loaded.Defold != "1.13.2" {
		t.Errorf("defold = %q, want 1.13.2", loaded.Defold)
	}
	if !options.TurnedOff["unknown-collision-group"] || options.TurnedOff["unresolved-url"] {
		t.Errorf("turned off = %v, want only unknown-collision-group", options.TurnedOff)
	}
	if len(options.Ignores) != 1 || options.Ignores[0].Check != "missing-resource" || options.Ignores[0].Path != "assets/generated/**" {
		t.Errorf("ignores = %+v, want the one of the file", options.Ignores)
	}
}

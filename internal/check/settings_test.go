package check

import (
	"fmt"
	"slices"
	"testing"
	"testing/fstest"

	"github.com/Lanterna-Games/defold-tools/internal/engine"
	"github.com/Lanterna-Games/defold-tools/internal/project"
)

func settingsProblemsIn(t *testing.T, files map[string]string) []string {
	t.Helper()
	return problemsOfTheSettingsIn(t, files, false)
}

// A build stops when the collection to start from or the input binding is not there.
// Neither has to be named: each has a path it falls back on, and that file is what has
// to exist. Bob says "the input.game_binding '/input/game.input_binding' can't be found".
func TestASettingABuildNeedsThatIsNotSetAndHasNothingToFallBackOn(t *testing.T) {
	cases := map[string]struct {
		files map[string]string
		want  []string
	}{
		"neither is set and neither file is there": {
			files: map[string]string{"game.project": "[project]\ntitle = Pong\n"},
			want: []string{
				`1:1: bootstrap.main_collection is not set, and "/logic/main.collection", which it falls back on, does not exist [unset-setting]`,
				`1:1: input.game_binding is not set, and "/input/game.input_binding", which it falls back on, does not exist [unset-setting]`,
			},
		},
		"neither is set and both files are where they are looked for": {
			files: map[string]string{"game.project": "[project]\ntitle = Pong\n", "logic/main.collection": "name: \"main\"\n", "input/game.input_binding": ""},
		},
		"both are set": {
			files: map[string]string{"game.project": "[bootstrap]\nmain_collection = /main/main.collectionc\n\n[input]\ngame_binding = /builtins/input/all.input_bindingc\n", "main/main.collection": "name: \"main\"\n"},
		},
	}
	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			if got := problemsOfTheSettingsIn(t, testCase.files, true); !slices.Equal(got, testCase.want) {
				t.Errorf("problems = %q, want %q", got, testCase.want)
			}
		})
	}
}

func problemsOfTheSettingsIn(t *testing.T, files map[string]string, judgesWhatIsNotSet bool) []string {
	t.Helper()
	reference, err := engine.Bundled()
	if err != nil {
		t.Fatal(err)
	}
	return problemsOfTheSettingsFor(t, reference, files, judgesWhatIsNotSet)
}

func problemsOfTheSettingsFor(t *testing.T, reference *engine.Reference, files map[string]string, judgesWhatIsNotSet bool) []string {
	t.Helper()
	tree := fstest.MapFS{}
	for name, content := range files {
		tree[name] = &fstest.MapFile{Data: []byte(content)}
	}
	loaded, err := project.Load(tree)
	if err != nil {
		t.Fatal(err)
	}
	var found []string
	for _, problem := range Run(loaded, reference, Options{TurnedOff: map[string]bool{unusedFileCheck: true, unsetSettingCheck: !judgesWhatIsNotSet}}) {
		if problem.Location.File == "game.project" {
			found = append(found, fmt.Sprintf("%d:%d: %s [%s]", problem.Location.Position.Line, problem.Location.Position.Column, problem.Message, problem.Check))
		}
	}
	return found
}

func TestSettingsTheEngineDoesNotHave(t *testing.T) {
	cases := map[string]struct {
		settings string
		want     []string
	}{
		"settings as the editor writes them": {
			settings: "[project]\ntitle = Pong\nversion = 1.0\n\n[display]\nwidth = 960\nheight = 640\nhigh_dpi = 1\n\n[physics]\nscale = 0.02\ngravity_y = -1000.0\n",
		},
		"a key that is almost one": {
			settings: "[display]\nwidht = 960\n",
			want:     []string{`2:1: "widht" is not a setting of [display] (did you mean "width"?) [unknown-setting]`},
		},
		"a section that is almost one": {
			settings: "[dispaly]\nwidth = 960\n",
			want:     []string{`1:2: "dispaly" is not a section of the settings (did you mean "display"?) [unknown-setting]`},
		},
		"a value of the wrong kind": {
			settings: "[display]\nwidth = wide\nhigh_dpi = yes\n\n[physics]\nscale = big\n",
			want: []string{
				`2:9: "wide" is not a whole number, which is what display.width takes [invalid-setting]`,
				`3:12: "yes" is not 0 or 1, which is what display.high_dpi takes [invalid-setting]`,
				`6:9: "big" is not a number, which is what physics.scale takes [invalid-setting]`,
			},
		},
		"a file the settings name that is not there": {
			settings: "[bootstrap]\nmain_collection = /main/mian.collectionc\n\n[input]\ngame_binding = /builtins/input/all.input_bindingc\n",
			want:     []string{`2:19: "/main/mian.collectionc" does not exist [missing-resource]`},
		},
		"a file the settings name by its compiled name": {
			settings: "[bootstrap]\nmain_collection = /main/main.collectionc\n",
		},
	}
	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			got := settingsProblemsIn(t, map[string]string{"game.project": testCase.settings, "main/main.collection": "name: \"main\"\n"})
			if !slices.Equal(got, testCase.want) {
				t.Errorf("problems = %q, want %q", got, testCase.want)
			}
		})
	}
}

// Defold 1.14.0 always shares one Lua state and took the setting that chose out
// of the schema, while the project templates had written it for years: it is in
// 389 of the 393 projects this was tried on. The line does nothing any more,
// and for the one project that had it at 0 the game now behaves differently.
// It is said to a project that targets a version without the setting, and only
// then: a version that still has it, or does not have it yet, is left alone.
func TestASettingDefoldRemovedIsReportedWhereItIsGone(t *testing.T) {
	cases := map[string]struct {
		defold   string
		settings string
		want     []string
	}{
		"removed in the version the project targets": {
			defold: "1.14.0", settings: "[script]\nshared_state = 1\n",
			want: []string{`2:1: "shared_state" is no longer a setting of [script]: Defold 1.14.0 removed it [removed-setting]`},
		},
		"still there in the version the project targets": {defold: "1.13.2", settings: "[script]\nshared_state = 1\n"},
		"removed several versions before": {
			defold: "1.14.0", settings: "[liveupdate]\nmount_on_start = 1\n",
			want: []string{`2:1: "mount_on_start" is no longer a setting of [liveupdate]: Defold 1.13.0 removed it [removed-setting]`},
		},
		"not there yet in the version the project targets": {defold: "1.13.2", settings: "[android]\nvkquality_database = /main/own.vkq\n"},
		"a section that only held what was removed":        {defold: "1.14.0", settings: "[script]\n"},
	}
	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			reference, err := engine.Versions{}.For(testCase.defold)
			if err != nil {
				t.Fatal(err)
			}
			got := problemsOfTheSettingsFor(t, reference, map[string]string{"game.project": testCase.settings, "main/own.vkq": ""}, false)
			if !slices.Equal(got, testCase.want) {
				t.Errorf("problems = %q, want %q", got, testCase.want)
			}
		})
	}
}

// A collection holds up to 1,048,576 game objects from Defold 1.14.0 on, and
// 65,534 before. A number is judged against the bounds of the version targeted.
func TestTheBoundsOfASettingAreThoseOfTheVersion(t *testing.T) {
	settings := map[string]string{"game.project": "[collection]\nmax_instances = 70000\n"}
	cases := map[string][]string{
		"1.14.0": nil,
		"1.13.2": {`2:17: "70000" is above the maximum of 65534, which is what collection.max_instances takes [invalid-setting]`},
	}
	for defold, want := range cases {
		t.Run(defold, func(t *testing.T) {
			reference, err := engine.Versions{}.For(defold)
			if err != nil {
				t.Fatal(err)
			}
			if got := problemsOfTheSettingsFor(t, reference, settings, false); !slices.Equal(got, want) {
				t.Errorf("problems = %q, want %q", got, want)
			}
		})
	}
}

// A project file gathers keys over the years, and extensions add their own. Each of
// these was found in real projects, and none of them is a mistake of whoever wrote it.
func TestSettingsThatAreNotMistakesAreLeftAlone(t *testing.T) {
	cases := map[string]string{
		"a section an extension reads":                 "[admob]\napp_id = ca-app-pub\n",
		"a key Defold had before the versions carried": "[windows]\niap_provider = GooglePlay\n",
		"a key an older version had":                   "[ios]\nlaunch_image_320x480 = /icons/launch.png\n",
		"an icon of a size Defold no longer asks for":  "[ios]\napp_icon_57x57 = /icons/57.png\n\n[android]\napp_icon_32x32 = /icons/32.png\n",
		"a key the extensions add to a section":        "[android]\ngms_app_id = 1:123:android:abc\n",
		"a list written one item to a line":            "[project]\ndependencies#0 = https://example.com/a.zip\ndependencies#1 = https://example.com/b.zip\n",
		"a setting left without a value":               "[display]\nwidth =\n",
	}
	for name, settings := range cases {
		t.Run(name, func(t *testing.T) {
			if got := settingsProblemsIn(t, map[string]string{"game.project": settings}); len(got) != 0 {
				t.Errorf("problems = %q, want none", got)
			}
		})
	}
}

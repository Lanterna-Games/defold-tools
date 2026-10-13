package engine

import (
	"maps"
	"slices"
	"testing"
)

func referenceWithSettings(version string, keysOf map[string][]string) *Reference {
	reference := &Reference{Defold: version, Settings: map[string]SettingsSection{}}
	for section, keys := range keysOf {
		settings := map[string]Setting{}
		for _, key := range keys {
			settings[key] = Setting{}
		}
		reference.Settings[section] = SettingsSection{Settings: settings}
	}
	return reference
}

// A setting can be missing from a version for three reasons, and only one of
// them is worth a word to the author of a project. Defold took it out: every
// later version lacks it too. It has not arrived yet: only later versions have
// it. Or the schema of that one version has a hole: it is there before and
// after. The versions are handed over out of order, as nothing promises one.
func TestASettingIsRemovedWhenNoLaterVersionHasIt(t *testing.T) {
	references := []*Reference{
		referenceWithSettings("1.10.0", map[string][]string{"script": {"shared_state"}, "display": {"width", "vsync"}, "android": {"proguard"}}),
		referenceWithSettings("1.9.0", map[string][]string{"script": {"shared_state"}, "display": {"width", "vsync"}, "android": {"proguard"}}),
		referenceWithSettings("1.12.0", map[string][]string{"display": {"width", "vsync"}, "android": {"vkquality"}}),
		referenceWithSettings("1.11.0", map[string][]string{"display": {"width"}, "android": {}}),
	}

	RememberSettingsOfOtherVersions(references)

	removedIn := map[string]map[string]map[string]string{}
	for _, reference := range references {
		removedIn[reference.Defold] = reference.SettingsRemoved
	}
	want := map[string]map[string]map[string]string{
		"1.9.0":  {},
		"1.10.0": {},
		"1.11.0": {"script": {"shared_state": "1.11.0"}, "android": {"proguard": "1.11.0"}},
		"1.12.0": {"script": {"shared_state": "1.11.0"}, "android": {"proguard": "1.11.0"}},
	}
	for version, removed := range want {
		if !maps.EqualFunc(removedIn[version], removed, maps.Equal[map[string]string]) {
			t.Errorf("removed in %s = %v, want %v", version, removedIn[version], removed)
		}
	}

	newest := references[2]
	if version, wasRemoved := newest.WhenTheSettingWasRemoved("script", "shared_state"); !wasRemoved || version != "1.11.0" {
		t.Errorf("shared_state in 1.12.0 = %q, %v; want it removed in 1.11.0", version, wasRemoved)
	}
	withTheHole := references[3]
	if _, wasRemoved := withTheHole.WhenTheSettingWasRemoved("display", "vsync"); wasRemoved {
		t.Error("vsync is there before and after 1.11.0, and was called removed in it")
	}
	if !withTheHole.IsASettingOfAnotherVersion("display", "vsync") || !withTheHole.IsASettingOfAnotherVersion("android", "vkquality") {
		t.Errorf("settings of other versions for 1.11.0 = %v, want vsync and vkquality among them", withTheHole.SettingsElsewhere)
	}
	if _, wasRemoved := references[1].WhenTheSettingWasRemoved("android", "vkquality"); wasRemoved {
		t.Error("vkquality arrives in 1.12.0, and was called removed in 1.9.0")
	}
	if got := slices.Sorted(maps.Keys(references[0].SettingsRemoved)); len(got) != 0 {
		t.Errorf("removed in 1.10.0 = %v, want nothing", got)
	}
}

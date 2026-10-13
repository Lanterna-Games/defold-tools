package engine

import (
	"slices"
	"testing"
)

func referenceWith(version string, messages []string, properties, fields map[string][]string, functions ...string) *Reference {
	reference := &Reference{Defold: version, Messages: messages, Properties: properties, MessageFields: fields, Functions: map[string][]Parameter{}}
	for _, function := range functions {
		reference.Functions[function] = []Parameter{{Name: "url", Types: []string{version}}}
	}
	return reference
}

// What a version documents before and after another one is taken to exist in
// between: the documentation lost it, the engine did not.
func TestAHoleBetweenTwoVersionsThatDocumentTheSameThingIsFilled(t *testing.T) {
	sprite := func(names ...string) map[string][]string { return map[string][]string{"sprite": names} }
	enter := map[string][]string{"trigger_response": {"enter", "other_id"}}
	first := referenceWith("1.0.0", []string{"enable", "set_camera", "trigger_response"}, sprite("size", "cursor"), enter, "go.get", "go.old")
	middle := referenceWith("1.1.0", []string{"enable", "trigger_response"}, sprite("size"), map[string][]string{"trigger_response": {"enter"}}, "go.new")
	later := referenceWith("1.2.0", []string{"enable", "set_camera", "trigger_response", "brand_new"}, sprite("size", "cursor", "slice"), enter, "go.get", "go.new")

	FillHoles([]*Reference{first, middle, later})

	if !middle.HasMessage("set_camera") || !slices.Contains(middle.Properties["sprite"], "cursor") || !slices.Contains(middle.MessageFields["trigger_response"], "other_id") {
		t.Errorf("middle = %+v, want the message, the property and the field it lost", middle)
	}
	if got := middle.Functions["go.get"]; len(got) != 1 || got[0].Types[0] != "1.0.0" {
		t.Errorf("go.get of the middle = %+v, want it as the earlier version documents it", got)
	}
	want := []string{"field trigger_response.other_id", "function go.get", "message set_camera", "property sprite.cursor"}
	if !slices.Equal(middle.Filled, want) {
		t.Errorf("filled = %v, want %v", middle.Filled, want)
	}
}

func TestOnlyHolesAreFilled(t *testing.T) {
	sprite := func(names ...string) map[string][]string { return map[string][]string{"sprite": names} }
	first := referenceWith("1.0.0", []string{"enable", "retired"}, sprite("size", "retired"), nil, "go.get", "go.retired")
	middle := referenceWith("1.1.0", []string{"enable"}, sprite("size"), nil, "go.get")
	later := referenceWith("1.2.0", []string{"enable", "brand_new"}, sprite("size", "slice"), nil, "go.get", "go.brand_new")

	FillHoles([]*Reference{first, middle, later})

	if len(first.Filled)+len(middle.Filled)+len(later.Filled) != 0 {
		t.Errorf("filled = %v, %v, %v; want nothing: what was retired stays retired and what is new does not reach back", first.Filled, middle.Filled, later.Filled)
	}
	if middle.HasMessage("retired") || middle.HasMessage("brand_new") || len(middle.Functions) != 1 {
		t.Errorf("middle = %+v, want it as it was", middle)
	}
}

// The two camera messages are in the documentation of 1.6.0 and of 1.13.2 and in
// none between 1.8.1 and 1.12.4. They never left the engine.
func TestTheCameraMessagesAreKnownInEveryVersionOfTheWindow(t *testing.T) {
	for _, version := range BundledVersions() {
		reference, err := bundledFor(version)
		if err != nil {
			t.Fatal(err)
		}
		if !reference.HasMessage("acquire_camera_focus") || !reference.HasMessage("release_camera_focus") {
			t.Errorf("Defold %s does not know the camera messages", version)
		}
	}
	filled, err := bundledFor(Version{1, 9, 0})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(filled.Filled, "message acquire_camera_focus") {
		t.Errorf("filled of 1.9.0 = %v, want it to say the message was put back", filled.Filled)
	}
}

package engine

import (
	"fmt"
	"slices"
	"testing"
)

func bundled(t *testing.T) *Reference {
	t.Helper()
	reference, err := Bundled()
	if err != nil {
		t.Fatal(err)
	}
	return reference
}

func TestBundledReferenceNamesItsDefoldVersion(t *testing.T) {
	reference := bundled(t)

	if reference.Defold == "" || len(reference.SHA1) != 40 || len(reference.Functions) < 1000 {
		t.Errorf("reference = Defold %q, sha1 %q, %d functions", reference.Defold, reference.SHA1, len(reference.Functions))
	}
}

func TestURLParameterIndexes(t *testing.T) {
	reference := bundled(t)

	tests := []struct {
		function string
		want     []int
	}{
		{function: "msg.post", want: []int{0}},
		{function: "sound.play", want: []int{0}},
		{function: "go.set_parent", want: []int{0, 1}},
		{function: "go.delete", want: []int{0}},
		// The value of a property may be a URL, a number or a vector: a string
		// passed there is not necessarily an address.
		{function: "go.set", want: []int{0}},
		{function: "go.property", want: nil},
		// A callback that receives a URL is not a place to write one.
		{function: "http.request", want: nil},
		{function: "no.such_function", want: nil},
	}
	for _, tt := range tests {
		t.Run(tt.function, func(t *testing.T) {
			if got := reference.URLParameterIndexes(tt.function); fmt.Sprint(got) != fmt.Sprint(tt.want) {
				t.Errorf("URLParameterIndexes(%q) = %v, want %v", tt.function, got, tt.want)
			}
		})
	}
}

func TestBundledReferenceKnowsTheEnginesMessages(t *testing.T) {
	reference := bundled(t)

	for _, message := range []string{"collision_response", "acquire_input_focus", "proxy_loaded", "play_sound", "layout_changed"} {
		if !reference.HasMessage(message) {
			t.Errorf("%q is a message of the engine, and the reference does not list it", message)
		}
	}
	for _, message := range []string{"ball_out", "colision_response", "msg.post", ""} {
		if reference.HasMessage(message) {
			t.Errorf("%q is not a message of the engine, and the reference lists it", message)
		}
	}
}

func TestPropertiesOfAComponent(t *testing.T) {
	reference := bundled(t)

	tests := []struct {
		componentType string
		has           []string
		lacks         []string
		documented    bool
	}{
		{componentType: GameObject, has: []string{"position", "rotation", "euler", "scale"}, lacks: []string{"tint", "size"}, documented: true},
		{componentType: "sprite", has: []string{"size", "cursor", "material", "texture0"}, lacks: []string{"position", "gain"}, documented: true},
		{componentType: "collisionobject", has: []string{"mass", "linear_velocity"}, lacks: []string{"material", "texture0"}, documented: true},
		{componentType: "sound", has: []string{"gain", "pan", "speed"}, lacks: []string{"volume"}, documented: true},
		// The reference documents no property for a mesh or the material of a model.
		{componentType: "mesh", has: []string{"vertices", "material", "texture1"}, documented: true},
		{componentType: "model", has: []string{"cursor", "texture7", "material"}, lacks: []string{"textureN", "texture"}, documented: true},
		{componentType: "spinemodel", documented: false},
	}
	for _, tt := range tests {
		t.Run(tt.componentType, func(t *testing.T) {
			properties, isDocumented := reference.PropertiesOf(tt.componentType)
			if isDocumented != tt.documented {
				t.Errorf("documented = %v, want %v", isDocumented, tt.documented)
			}
			for _, name := range tt.has {
				if !NamesTheProperty(properties, name) {
					t.Errorf("%q is a property of it, and was not found in %v", name, properties)
				}
			}
			for _, name := range tt.lacks {
				if NamesTheProperty(properties, name) {
					t.Errorf("%q is not a property of it, and was found in %v", name, properties)
				}
			}
		})
	}
}

func TestBuiltinMaterialsExposeTheTint(t *testing.T) {
	reference := bundled(t)

	if !NamesTheProperty(reference.MaterialProperties, "tint") || !NamesTheProperty(reference.EveryProperty(), "tint") {
		t.Errorf("material properties = %v, want them to hold tint", reference.MaterialProperties)
	}
	if !reference.IsDrawnWithAMaterial("sprite") || !reference.IsDrawnWithAMaterial("model") || reference.IsDrawnWithAMaterial("sound") {
		t.Error("a sprite and a model are drawn with a material, a sound is not")
	}
}

func TestPropertyWithoutItsElement(t *testing.T) {
	for written, want := range map[string]string{
		"position":   "position",
		"position.x": "position",
		"tint.w":     "tint",
		"euler.z":    "euler",
		"size.xy":    "size.xy",
		"a.b":        "a.b",
		".x":         "",
	} {
		if got := PropertyWithoutItsElement(written); got != want {
			t.Errorf("PropertyWithoutItsElement(%q) = %q, want %q", written, got, want)
		}
	}
}

func TestFunctionsThatTakeAPropertyOfAnAddress(t *testing.T) {
	reference := bundled(t)

	for function, want := range map[string]bool{
		"go.get": true, "go.set": true, "go.animate": true, "go.cancel_animations": true,
		// These take a property of a GUI node or of an editor node, not of an address.
		"gui.animate": false, "gui.set": false, "editor.get": false,
		"go.property": false, "msg.post": false,
	} {
		address, property, takesAProperty := reference.PropertyParameterIndexes(function)
		if takesAProperty != want || (want && (address != 0 || property != 1)) {
			t.Errorf("%s: address %d, property %d, takes a property %v; want %v", function, address, property, takesAProperty, want)
		}
	}
}

func TestBundledReferenceKnowsTheBindingThatComesWithTheEngine(t *testing.T) {
	actions := bundled(t).InputBindings["/builtins/input/all.input_binding"]

	for _, action := range []string{"key_space", "mouse_button_left", "gamepad_start", "touch"} {
		if !slices.Contains(actions, action) {
			t.Errorf("%q is bound by the builtin binding, and was not found among its %d actions", action, len(actions))
		}
	}
}

func TestFieldsOfWhatTheEngineSends(t *testing.T) {
	reference := bundled(t)

	fields, isSentByTheEngine := reference.FieldsOfWhatTheEngineSends("collision_response")
	if !isSentByTheEngine || !slices.Contains(fields, "other_id") || !slices.Contains(fields, "own_group") {
		t.Errorf("fields of collision_response = %v, sent by the engine = %v", fields, isSentByTheEngine)
	}
	// The engine still fills the field the reference stopped documenting.
	if !slices.Contains(fields, "group") {
		t.Errorf("fields of collision_response = %v, want the old group among them", fields)
	}
	for _, message := range []string{"play_animation", "load", "enable", "ball_out"} {
		if _, isSentByTheEngine := reference.FieldsOfWhatTheEngineSends(message); isSentByTheEngine {
			t.Errorf("%q is posted by scripts, and was taken for a message only the engine sends", message)
		}
	}
	for _, message := range messagesOnlyTheEngineSends {
		if !reference.HasMessage(message) {
			t.Errorf("%q is listed as sent by the engine, and the reference has no such message", message)
		}
	}
}

// The reference has gaps, and a project that knows of one can vouch for the name.
func TestAProjectCanVouchForWhatTheReferenceLeavesOut(t *testing.T) {
	reference := bundled(t)

	knowing := reference.Knowing([]string{"set_scale"}, []string{"outline_width"})

	if !knowing.HasMessage("set_scale") || reference.HasMessage("set_scale") {
		t.Error("the message is known to the copy only")
	}
	for _, componentType := range []string{GameObject, "sprite", "sound"} {
		if properties, _ := knowing.PropertiesOf(componentType); !NamesTheProperty(properties, "outline_width") {
			t.Errorf("a %s does not have the property the project vouches for", componentType)
		}
		if properties, _ := reference.PropertiesOf(componentType); NamesTheProperty(properties, "outline_width") {
			t.Errorf("the reference itself was changed for %s", componentType)
		}
	}
	if !NamesTheProperty(knowing.EveryProperty(), "outline_width") || !knowing.IsDrawnWithAMaterial("sprite") || knowing.IsDrawnWithAMaterial("sound") {
		t.Error("vouching for a property changed more than the names that are known")
	}
}

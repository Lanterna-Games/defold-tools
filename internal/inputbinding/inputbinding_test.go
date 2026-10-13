package inputbinding

import (
	"slices"
	"testing"

	"github.com/Lanterna-Games/defold-tools/internal/source"
	"github.com/Lanterna-Games/defold-tools/internal/textproto"
)

func TestActionsAreWhatEveryKindOfTriggerBinds(t *testing.T) {
	content := []byte(`key_trigger {
  input: KEY_SPACE
  action: "jump"
}
key_trigger {
  input: KEY_W
  action: "jump"
}
mouse_trigger {
  input: MOUSE_BUTTON_1
  action: "touch"
}
gamepad_trigger {
  input: GAMEPAD_LSTICK_LEFT
  action: "left"
}
touch_trigger {
  input: TOUCH_MULTI
  action: "multitouch"
}
text_trigger {
  input: TEXT
  action: "text"
}
`)
	document, syntaxErrors := textproto.Parse(content, source.NewLocator(content))
	if len(syntaxErrors) != 0 {
		t.Fatal(syntaxErrors)
	}

	got := Actions(document)

	want := []string{"jump", "left", "multitouch", "text", "touch"}
	if !slices.Equal(got, want) {
		t.Errorf("actions = %v, want %v", got, want)
	}
}

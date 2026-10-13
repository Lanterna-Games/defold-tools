package material

import (
	"slices"
	"testing"

	"github.com/Lanterna-Games/defold-tools/internal/source"
	"github.com/Lanterna-Games/defold-tools/internal/textproto"
)

func TestPropertyNamesAreWhatAScriptCanSetOnTheMaterial(t *testing.T) {
	content := []byte(`name: "sprite"
tags: "tile"
vertex_program: "/builtins/materials/sprite.vp"
vertex_constants {
  name: "view_proj"
  type: CONSTANT_TYPE_VIEWPROJ
}
fragment_constants {
  name: "tint"
  type: CONSTANT_TYPE_USER
}
fragment_constants {
  name: "glow"
  type: CONSTANT_TYPE_USER
}
attributes {
  name: "sprite_color"
}
samplers {
  name: "texture_sampler"
}
`)
	document, syntaxErrors := textproto.Parse(content, source.NewLocator(content))
	if len(syntaxErrors) != 0 {
		t.Fatal(syntaxErrors)
	}

	got := PropertyNames(document)

	want := []string{"glow", "sprite_color", "texture_sampler", "tint", "view_proj"}
	if !slices.Equal(got, want) {
		t.Errorf("names = %v, want %v", got, want)
	}
}

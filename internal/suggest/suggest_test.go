package suggest

import "testing"

func TestClosest(t *testing.T) {
	candidates := []string{"sprite", "collisionobject", "bounce_sound", "game"}

	tests := []struct {
		name      string
		input     string
		want      string
		wantFound bool
	}{
		{name: "one letter dropped", input: "bounce_sond", want: "bounce_sound", wantFound: true},
		// Swapping two neighbours is one slip of the fingers, so it counts as one edit.
		{name: "two letters swapped", input: "sprtie", want: "sprite", wantFound: true},
		{name: "two letters swapped in a short name", input: "gaem", want: "game", wantFound: true},
		// A suggestion that shares almost nothing with what was typed is noise.
		{name: "nothing close enough", input: "camera", wantFound: false},
		{name: "short names tolerate one edit", input: "ab", wantFound: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, found := Closest(tt.input, candidates)
			if found != tt.wantFound || got != tt.want {
				t.Errorf("Closest(%q) = %q, %v; want %q, %v", tt.input, got, found, tt.want, tt.wantFound)
			}
		})
	}
}

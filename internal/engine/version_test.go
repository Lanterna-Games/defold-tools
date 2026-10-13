package engine

import (
	"slices"
	"testing"
)

func TestParseVersion(t *testing.T) {
	for text, want := range map[string]Version{
		"1.6.0":   {1, 6, 0},
		"1.13.2":  {1, 13, 2},
		"1.2.185": {1, 2, 185},
	} {
		if got, isAVersion := ParseVersion(text); !isAVersion || got != want || got.String() != text {
			t.Errorf("ParseVersion(%q) = %v, %v; want %v", text, got, isAVersion, want)
		}
	}
	for _, text := range []string{"", "1.6", "1.6.0.1", "v1.6.0", "1.6.0-beta", "1.x.0", "latest", " 1.6.0"} {
		if got, isAVersion := ParseVersion(text); isAVersion {
			t.Errorf("ParseVersion(%q) = %v, want it refused", text, got)
		}
	}
}

// 1.10.0 comes after 1.9.8: versions are compared number by number, not as text.
func TestVersionsAreOrderedByTheirNumbers(t *testing.T) {
	inOrder := []Version{{1, 2, 185}, {1, 6, 0}, {1, 6, 4}, {1, 9, 8}, {1, 10, 0}, {1, 13, 2}, {2, 0, 0}}
	shuffled := []Version{inOrder[4], inOrder[0], inOrder[6], inOrder[3], inOrder[1], inOrder[5], inOrder[2]}

	slices.SortFunc(shuffled, Version.Compare)

	if !slices.Equal(shuffled, inOrder) {
		t.Errorf("sorted = %v, want %v", shuffled, inOrder)
	}
	if !inOrder[3].IsBefore(inOrder[4]) || inOrder[4].IsBefore(inOrder[3]) || inOrder[1].IsBefore(inOrder[1]) {
		t.Error("IsBefore does not agree with the order")
	}
}

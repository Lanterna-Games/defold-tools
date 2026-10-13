package conventions

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/Lanterna-Games/defold-tools/internal/annotations"
	"github.com/Lanterna-Games/defold-tools/internal/check"
	"github.com/Lanterna-Games/defold-tools/internal/docs/bundled"
)

const settingOfGlobals = "Lua.diagnostics.globals"

// A project opened for the first time has not been handed the annotations yet,
// and the Lua language server would underline every `go`, `msg` and `hash` as an
// undefined global, and every `init` and `update` as a global in lowercase. The
// extension therefore tells it, as a default that writes nothing in the project,
// which globals Defold gives a script. The list is written by hand in a file
// that has no build step, so this is what keeps it equal to the documentation.
func TestTheExtensionNamesEveryGlobalOfDefold(t *testing.T) {
	documentation, err := bundled.Documentation()
	if err != nil {
		t.Fatal(err)
	}
	expected := append(annotations.Globals(documentation), check.LifecycleFunctions()...)
	slices.Sort(expected)

	manifest, err := os.ReadFile(filepath.Join(repositoryRoot(t), "editors", "vscode", "package.json"))
	if err != nil {
		t.Fatal(err)
	}
	var written struct {
		Contributes struct {
			Defaults map[string]json.RawMessage `json:"configurationDefaults"`
		} `json:"contributes"`
	}
	if err := json.Unmarshal(manifest, &written); err != nil {
		t.Fatal(err)
	}
	var globals []string
	if err := json.Unmarshal(written.Contributes.Defaults[settingOfGlobals], &globals); err != nil {
		t.Fatalf("%s in editors/vscode/package.json: %v", settingOfGlobals, err)
	}

	if !slices.Equal(globals, expected) {
		asJSON, _ := json.Marshal(expected)
		t.Errorf("%s in editors/vscode/package.json is\n%v\nand the documentation gives\n%s", settingOfGlobals, globals, asJSON)
	}
}

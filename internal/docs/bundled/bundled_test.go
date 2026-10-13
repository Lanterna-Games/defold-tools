package bundled

import (
	"testing"

	"github.com/Lanterna-Games/defold-tools/internal/engine"
)

// The documentation carried is that of the newest version the checks know, so
// that a project which declares nothing has both without a network.
func TestTheDocumentationCarriedIsOfTheNewestVersion(t *testing.T) {
	newest, hasAny := engine.NewestBundled()
	version, hasOne := Version()
	if !hasAny || !hasOne || version != newest.String() {
		t.Fatalf("documentation of %q, newest reference %v", version, newest)
	}

	reference, err := Documentation()
	if err != nil {
		t.Fatal(err)
	}

	if reference.Version != version || len(reference.Modules) < 40 {
		t.Fatalf("documentation of %q with %d modules", reference.Version, len(reference.Modules))
	}
	for _, module := range reference.Modules {
		if module.Namespace != "msg" {
			continue
		}
		for _, function := range module.Functions {
			if function.Name == "msg.post" && len(function.Parameters) == 3 && function.Description != "" && function.Examples != "" {
				return
			}
		}
	}
	t.Error("the documentation does not describe msg.post in full")
}

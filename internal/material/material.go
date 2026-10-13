package material

import (
	"sort"

	"github.com/Lanterna-Games/defold-tools/internal/textproto"
)

const Extension = ".material"

var fieldsThatNameAProperty = []string{"vertex_constants", "fragment_constants", "attributes", "samplers"}

func PropertyNames(document *textproto.Message) []string {
	isNamed := map[string]bool{}
	for _, field := range fieldsThatNameAProperty {
		for _, declaration := range document.Messages(field) {
			if name := declaration.Text("name"); name != "" {
				isNamed[name] = true
			}
		}
	}
	names := make([]string, 0, len(isNamed))
	for name := range isNamed {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

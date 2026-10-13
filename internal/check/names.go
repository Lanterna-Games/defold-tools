package check

import (
	"fmt"
	"strings"

	"github.com/Lanterna-Games/defold-tools/internal/suggest"
)

func hintTowards(name string, known []string, format string) string {
	if closest, found := suggest.Closest(name, known); found {
		return fmt.Sprintf(" (did you mean %s?)", fmt.Sprintf(format, closest))
	}
	return ""
}

func quotedList(names []string) string {
	quoted := make([]string, 0, len(names))
	for _, name := range names {
		quoted = append(quoted, fmt.Sprintf("%q", name))
	}
	switch len(quoted) {
	case 0:
		return "none"
	case 1:
		return quoted[0]
	default:
		return strings.Join(quoted[:len(quoted)-1], ", ") + " and " + quoted[len(quoted)-1]
	}
}

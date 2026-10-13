package inputbinding

import (
	"sort"
	"strings"

	"github.com/Lanterna-Games/defold-tools/internal/source"
	"github.com/Lanterna-Games/defold-tools/internal/textproto"
)

const (
	Extension     = ".input_binding"
	triggerSuffix = "_trigger"
)

type Trigger struct {
	Action       string
	Position     source.Position
	TextPosition source.Position
}

func Triggers(document *textproto.Message) []Trigger {
	var triggers []Trigger
	for _, field := range document.Fields {
		if field.Message == nil || !strings.HasSuffix(field.Name, triggerSuffix) {
			continue
		}
		if action, isBound := field.Message.Scalar("action"); isBound && action.Text != "" {
			triggers = append(triggers, Trigger{Action: action.Text, Position: action.Position, TextPosition: action.TextPosition()})
		}
	}
	return triggers
}

func Actions(document *textproto.Message) []string {
	isBound := map[string]bool{}
	for _, trigger := range Triggers(document) {
		isBound[trigger.Action] = true
	}
	actions := make([]string, 0, len(isBound))
	for action := range isBound {
		actions = append(actions, action)
	}
	sort.Strings(actions)
	return actions
}

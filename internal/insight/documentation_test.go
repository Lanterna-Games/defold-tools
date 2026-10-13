package insight

import (
	"slices"
	"strings"
	"testing"

	"github.com/Lanterna-Games/defold-tools/internal/docs"
	"github.com/Lanterna-Games/defold-tools/internal/docs/bundled"
)

const offlineReference = "http://127.0.0.1:45565"

func insightWithTheDocumentation(t *testing.T, files map[string]string) Insight {
	t.Helper()
	documentation, err := bundled.Documentation()
	if err != nil {
		t.Fatal(err)
	}
	known := insightInto(t, files)
	known.Documentation = Documentation{Reference: documentation, Links: docs.Links{OfflineReference: offlineReference}}
	return known
}

func carriedVersion(t *testing.T) string {
	t.Helper()
	version, hasOne := bundled.Version()
	if !hasOne {
		t.Fatal("this build carries no documentation")
	}
	return version
}

func inUpdate(line string) string {
	return "function update(self, dt)\n    " + line + "\nend\n"
}

func inOnMessage(body string) string {
	return "function on_message(self, message_id, message, sender)\n" + body + "\nend\n"
}

func underTheMessage(message, line string) string {
	return inOnMessage("    if message_id == hash(\"" + message + "\") then\n        " + line + "\n    end")
}

// A name of the engine is explained by the reference of the project's version, and the
// same name can be documented by several kinds of component: "scale" is a number on a
// game object and a vector on a sprite. The address the name is used with decides.
func TestWhatTheReferenceSaysAboutAName(t *testing.T) {
	carried := carriedVersion(t)
	cases := map[string]struct {
		script string
		want   []string
		absent []string
	}{
		"a message posted to a component of its kind": {
			script: inUpdate(`msg.post("#bounce_sound", "play_|sound")`),
			want: []string{
				"Engine message `play_sound`.",
				"Post this message to a sound-component",
				"- `gain` *number, optional* — sound gain between 0 and 1",
				"**Examples:**",
				"```lua\nmsg.post(\"#sound\", \"play_sound\", {delay = 1, gain = 0.5})\n```",
				"[Offline reference](" + offlineReference + "/" + carried + "/sound#message-play_sound) · [defold.com](https://defold.com/ref/sound-lua#play_sound)",
			},
		},
		"a message any component takes, posted to a sprite": {
			script: inUpdate(`msg.post("#sprite", "ena|ble")`),
			want:   []string{"This message enables the receiving component.", "/" + carried + "/go#message-enable"},
			absent: []string{"collectionproxy"},
		},
		"the same message, posted to an address that is not written": {
			script: inUpdate(`msg.post(target, "ena|ble")`),
			want: []string{
				"- **Collection proxy** — tells a collection proxy to enable the referenced collection · [Offline reference](" + offlineReference + "/" + carried + "/collectionproxy#message-enable)",
				"- **Game object** — enables the receiving component",
			},
			absent: []string{"**Examples:**"},
		},
		"a message the engine sends": {
			script: inOnMessage("    if message_id == hash(\"collision_|response\") then end"),
			want: []string{
				"- `other_id` *hash* — the id of the instance the collision object collided with",
				"https://defold.com/ref/physics-lua#collision_response",
			},
			absent: []string{"Fields: `other_id`"},
		},
		"a property of a sprite": {
			script: inUpdate(`go.set("#sprite", "sca|le", 1)`),
			want:   []string{"Property `scale` of a sprite component.", "The non-uniform scale of the sprite.", "/" + carried + "/sprite#property-scale) · [defold.com](https://defold.com/ref/sprite-lua#scale)"},
			absent: []string{"game object"},
		},
		"one element of a property of the game object": {
			script: inUpdate(`go.set(".", "position|.x", 1)`),
			want:   []string{"Property `position.x` of a game object.", "The position of the game object.", "/" + carried + "/go#property-position"},
		},
		"a property of an address that is not written": {
			script: inUpdate(`go.get(target, "sca|le")`),
			want:   []string{"- **Game object** — ", "- **Label** — ", "- **Sprite** — "},
		},
		"a property the reference does not document": {
			script: inUpdate(`go.set("#sprite", "ti|nt", 1)`),
			want:   []string{"Property `tint` of a sprite component."},
			absent: []string{"defold.com"},
		},
		"a property of a component that is not there": {
			script: inUpdate(`go.get("#sprit", "si|ze")`),
			want:   []string{"Property `size`. `#sprit` leads to nothing that can be followed from here", "- **Label** — ", "- **Sprite** — "},
			absent: []string{"is not written"},
		},
		"a property that is not of the component": {
			script: inUpdate(`go.set("#sprite", "posi|tion", 1)`),
			absent: []string{"defold.com"},
		},
	}
	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			content, offset := at(t, testCase.script)

			said, _, _, _ := insightWithTheDocumentation(t, pong).Describe("main/ball.script", content, offset)

			for _, want := range testCase.want {
				if !strings.Contains(said, want) {
					t.Errorf("what is said lacks %q:\n%s", want, said)
				}
			}
			for _, absent := range testCase.absent {
				if strings.Contains(said, absent) {
					t.Errorf("what is said has %q:\n%s", absent, said)
				}
			}
		})
	}
}

// The documentation is loaded once, when the editor opens the project, and the version
// a project declares can change after that. What another version says is not shown as
// if it were about this one.
func TestTheDocumentationOfAnotherVersionIsLeftAside(t *testing.T) {
	known := insightWithTheDocumentation(t, pong)
	another := *known.Documentation.Reference
	another.Version = "1.6.0"
	known.Documentation.Reference = &another
	content, offset := at(t, inOnMessage("    if message_id == hash(\"collision_|response\") then end"))

	said, _, _, _ := known.Describe("main/ball.script", content, offset)

	if !strings.Contains(said, "Fields: `other_id`, `other_position`, `other_group`, `own_group`") || strings.Contains(said, "defold.com") {
		t.Errorf("what is said about a message of another version:\n%s", said)
	}
}

// Every message of the engine that is offered has to be found in the documentation
// under the same name, or a hover would say nothing about it without anyone noticing.
func TestEveryMessageOfTheEngineIsDocumented(t *testing.T) {
	known := insightWithTheDocumentation(t, pong)
	undocumented := map[string]bool{}
	for _, message := range known.Reference.Messages {
		if len(known.documentedMessages(message, kinds{})) == 0 {
			undocumented[message] = true
		}
	}
	if len(undocumented) > 0 {
		t.Errorf("the documentation says nothing about %v", sortedKeys(undocumented))
	}
}

func TestTheFieldsOfAMessageAreOfferedAfterTheDot(t *testing.T) {
	cases := map[string]struct {
		script   string
		want     []string
		replaced string
	}{
		"of a message the engine sends":           {script: underTheMessage("collision_response", "print(message.|)"), want: []string{"other_id", "other_position", "other_group", "own_group", "group"}},
		"of a message the project posts":          {script: underTheMessage("ball_out", "print(message.|)"), want: []string{"side"}},
		"with part of the name typed":             {script: underTheMessage("ball_out", "print(message.si|)"), want: []string{"side"}, replaced: "si"},
		"in the middle of a name":                 {script: underTheMessage("ball_out", "print(message.si|de)"), want: []string{"side"}, replaced: "side"},
		"alone on its line":                       {script: underTheMessage("ball_out", "message.|"), want: []string{"side"}},
		"when the argument has another name":      {script: "function on_message(self, id, data)\n    if id == hash(\"ball_out\") then\n        print(data.|)\n    end\nend\n", want: []string{"side"}},
		"under no message that can be read":       {script: inOnMessage("    print(message.|)"), want: nil},
		"of a message nobody posts":               {script: underTheMessage("unheard_of", "print(message.|)"), want: nil},
		"of something that is not the message":    {script: underTheMessage("ball_out", "print(sender.|)"), want: nil},
		"of a field of the message":               {script: underTheMessage("ball_out", "print(message.side.|)"), want: nil},
		"of the message of another table":         {script: underTheMessage("ball_out", "print(self.message.|)"), want: nil},
		"outside the handler":                     {script: inUpdate("print(message.|)"), want: nil},
		"in a comment":                            {script: underTheMessage("ball_out", "-- message.|"), want: nil},
		"in a string":                             {script: underTheMessage("ball_out", `print("message.|")`), want: nil},
		"after the dot of a number":               {script: underTheMessage("ball_out", "print(1.|)"), want: nil},
		"after the dots that join two strings":    {script: underTheMessage("ball_out", "print(message..|)"), want: nil},
		"of a message handled in another branch":  {script: inOnMessage("    if message_id == hash(\"ball_out\") then\n        print(1)\n    elseif message_id == hash(\"serve\") then\n        print(message.|)\n    end"), want: nil},
		"of either of two messages of the engine": {script: inOnMessage("    if message_id == hash(\"animation_done\") or message_id == hash(\"ball_out\") then\n        print(message.|)\n    end"), want: []string{"current_tile", "id", "side"}},
	}
	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			content, offset := at(t, testCase.script)

			candidates, start, end := insightWithTheDocumentation(t, pong).Complete("main/game.script", content, offset)

			if got := labelsOf(candidates); !slices.Equal(got, testCase.want) {
				t.Errorf("offered %q, want %q", got, testCase.want)
			}
			if got := string(content[start:end]); len(candidates) > 0 && got != testCase.replaced {
				t.Errorf("replaces %q, want %q", got, testCase.replaced)
			}
		})
	}
}

func TestAFieldSaysWhatItHoldsAndWhereItIsSet(t *testing.T) {
	known := insightWithTheDocumentation(t, pong)

	content, offset := at(t, underTheMessage("contact_point_response", "print(message.|)"))
	candidates, _, _ := known.Complete("main/game.script", content, offset)
	index := slices.IndexFunc(candidates, func(candidate Candidate) bool { return candidate.Label == "normal" })
	if index < 0 {
		t.Fatalf("normal is not offered among %q", labelsOf(candidates))
	}
	if offered := candidates[index]; offered.Detail != "vector3 · field of contact_point_response" || !strings.Contains(offered.Documentation, "`normal` *vector3* — normal in world space of the contact point") || offered.Role != RoleOfAField {
		t.Errorf("offered = %+v", offered)
	}

	content, offset = at(t, underTheMessage("contact_point_response", "print(message.nor|mal)"))
	said, start, end, _ := known.Describe("main/game.script", content, offset)
	if !strings.Contains(said, "Field of the engine message `contact_point_response`:") || !strings.Contains(said, "`normal` *vector3*") || string(content[start:end]) != "normal" {
		t.Errorf("said %q about %q", said, content[start:end])
	}

	content, offset = at(t, underTheMessage("ball_out", "print(message.si|de)"))
	said, _, _, _ = known.Describe("main/game.script", content, offset)
	if !strings.Contains(said, "Field `side` of `ball_out`.") || !strings.Contains(said, "- `main/ball.script` line 2") {
		t.Errorf("said %q", said)
	}
	if found := known.Define("main/game.script", content, offset); len(found) != 1 || found[0].File != "main/ball.script" || found[0].Position.Line != 2 {
		t.Errorf("the field leads to %+v, want the post in main/ball.script", found)
	}

	content, offset = at(t, underTheMessage("ball_out", "print(message.sid|es)"))
	if said, _, _, isSomewhere := known.Describe("main/game.script", content, offset); isSomewhere {
		t.Errorf("said %q about a field nothing sets", said)
	}
}

func TestEngineNamesAreOfferedWithWhatTheReferenceSays(t *testing.T) {
	known := insightWithTheDocumentation(t, pong)
	offered := func(script, label string) Candidate {
		t.Helper()
		content, offset := at(t, script)
		candidates, _, _ := known.Complete("main/ball.script", content, offset)
		index := slices.IndexFunc(candidates, func(candidate Candidate) bool { return candidate.Label == label })
		if index < 0 {
			t.Fatalf("%s is not offered among %q", label, labelsOf(candidates))
		}
		return candidates[index]
	}

	message := offered(inUpdate(`msg.post("#bounce_sound", "|")`), "play_sound")
	if !strings.Contains(message.Documentation, "- `gain` *number, optional*") || !strings.Contains(message.Documentation, "sound-lua#play_sound") || strings.Contains(message.Documentation, "**Examples:**") {
		t.Errorf("play_sound is offered with:\n%s", message.Documentation)
	}
	property := offered(inUpdate(`go.get("#sprite", "|")`), "scale")
	if !strings.Contains(property.Documentation, "The non-uniform scale of the sprite.") {
		t.Errorf("scale is offered with:\n%s", property.Documentation)
	}
	if ofTheProject := offered(inUpdate(`msg.post("/game#game", "|")`), "ball_out"); ofTheProject.Documentation != "" {
		t.Errorf("a message of the project is offered with:\n%s", ofTheProject.Documentation)
	}
}

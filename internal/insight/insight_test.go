package insight

import (
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/Lanterna-Games/defold-tools/internal/engine"
	"github.com/Lanterna-Games/defold-tools/internal/project"
)

const cursor = "|"

var pong = map[string]string{
	"game.project": "[project]\ntitle = Pong\n\n[input]\ngame_binding = /input/game.input_bindingc\n",
	"input/game.input_binding": `key_trigger {
  input: KEY_UP
  action: "p1_up"
}
mouse_trigger {
  input: MOUSE_BUTTON_1
  action: "touch"
}
`,
	"main/main.collection": `name: "main"
instances {
  id: "ball"
  prototype: "/main/ball.go"
}
instances {
  id: "game"
  prototype: "/main/game.go"
}
instances {
  id: "wall"
  prototype: "/main/wall.go"
}
`,
	"main/ball.go": `components {
  id: "ball"
  component: "/main/ball.script"
}
embedded_components {
  id: "sprite"
  type: "sprite"
  data: "default_animation: \"ball\"\n"
  "textures {\n"
  "  sampler: \"texture_sampler\"\n"
  "  texture: \"/main/ball.atlas\"\n"
  "}\n"
  ""
}
embedded_components {
  id: "bounce_sound"
  type: "sound"
  data: ""
}
`,
	"main/wall.go": `embedded_components {
  id: "collisionobject"
  type: "collisionobject"
  data: "group: \"wall\"\n"
  "mask: \"ball\"\n"
  ""
}
`,
	"main/game.go": `components {
  id: "game"
  component: "/main/game.script"
}
components {
  id: "hud"
  component: "/main/hud.gui"
}
`,
	"main/ball.atlas": `images {
  image: "/main/images/ball.png"
}
animations {
  id: "roll"
}
`,
	"main/hud.gui": `script: "/main/hud.gui_script"
textures {
  name: "ui"
  texture: "/main/ball.atlas"
}
nodes {
  id: "score"
  type: TYPE_TEXT
}
nodes {
  id: "restart"
  type: TYPE_TEMPLATE
  template: "/main/button.gui"
}
`,
	"main/button.gui":     "nodes {\n  id: \"label\"\n  type: TYPE_TEXT\n}\n",
	"main/hud.gui_script": "function init(self)\nend\n",
	"main/game.script": `go.property("score", 0)
go.property("winning_score", 5)

function on_message(self, message_id, message, sender)
    if message_id == hash("ball_out") then
        self.score = self.score + 1
    elseif message_id == hash("serve") then
        self.served = true
    end
end
`,
	"main/ball.script": `function update(self, dt)
    msg.post("/game#game", "ball_out", { side = 1 })
end
`,
	"main/ids.lua": "local M = {}\nM.GAME = \"/game#game\"\nreturn M\n",
}

func insightInto(t *testing.T, files map[string]string) Insight {
	t.Helper()
	tree := fstest.MapFS{}
	for name, content := range files {
		tree[name] = &fstest.MapFile{Data: []byte(content)}
	}
	loaded, err := project.Load(tree)
	if err != nil {
		t.Fatal(err)
	}
	reference, err := engine.Bundled()
	if err != nil {
		t.Fatal(err)
	}
	return Insight{Project: loaded, Reference: reference}
}

func at(t *testing.T, marked string) (content []byte, offset int) {
	t.Helper()
	offset = strings.Index(marked, cursor)
	if offset < 0 {
		t.Fatalf("the text has no cursor: %q", marked)
	}
	return []byte(strings.Replace(marked, cursor, "", 1)), offset
}

func labelsOf(candidates []Candidate) []string {
	labels := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		labels = append(labels, candidate.Label)
	}
	return labels
}

// The cursor is found by the text before it, so that a line still being typed,
// with its string and its call left open, is understood as well as a finished one.
func TestWhatTheCursorIsOn(t *testing.T) {
	cases := map[string]struct {
		line string
		want Role
	}{
		"an address being typed":          {line: `msg.post("#spr|`, want: RoleOfAnAddress},
		"an address in a finished line":   {line: `msg.post("#spr|ite", "disable")`, want: RoleOfAnAddress},
		"an address for go.get":           {line: `local x = go.get("|", "position")`, want: RoleOfAnAddress},
		"an address in msg.url":           {line: `self.game = msg.url("/ga|")`, want: RoleOfAnAddress},
		"an address behind a hash":        {line: `go.delete(hash("/ba|"))`, want: RoleOfAnAddress},
		"a message being posted":          {line: `msg.post("/game#game", "ball|`, want: RoleOfAMessagePosted},
		"a message behind a hash":         {line: `msg.post(self.game, hash("ball|"))`, want: RoleOfAMessagePosted},
		"a property":                      {line: `go.set("#sprite", "ti|", 1)`, want: RoleOfAProperty},
		"a property to animate":           {line: `go.animate(".", "posi|`, want: RoleOfAProperty},
		"a node":                          {line: `self.score = gui.get_node("sc|")`, want: RoleOfANode},
		"an animation":                    {line: `sprite.play_flipbook("#sprite", "ro|")`, want: RoleOfAnAnimation},
		"an animation in a message":       {line: `msg.post("#sprite", "play_animation", { id = hash("ro|") })`, want: RoleOfAnAnimation},
		"an animation of a node":          {line: `gui.play_flipbook(node, "ro|")`, want: RoleOfAnAnimationOfANode},
		"a group of a ray cast":           {line: `physics.raycast(from, to, { hash("wa|") })`, want: RoleOfAGroup},
		"the second group of a ray cast":  {line: `physics.raycast(from, to, { hash("wall"), hash("|") })`, want: RoleOfAGroup},
		"a group to set":                  {line: `physics.set_maskbit("#co", "wa|", true)`, want: RoleOfAGroup},
		"a group compared":                {line: `if message.other_group == hash("wa|") then end`, want: RoleOfAGroup},
		"the text of a label":             {line: `label.set_text("#label", "hel|lo")`, want: RoleOfNothing},
		"a string that is only a string":  {line: `local name = "ba|ll"`, want: RoleOfNothing},
		"a field of another table":        {line: `msg.post("#sprite", "other", { id = hash("ro|") })`, want: RoleOfNothing},
		"outside any string":              {line: `msg.post("#sprite", |"disable")`, want: RoleOfNothing},
		"in a comment":                    {line: `-- msg.post("#spr|`, want: RoleOfNothing},
		"after a string that was closed":  {line: `msg.post("#sprite")| `, want: RoleOfNothing},
		"a message with an escaped quote": {line: `msg.post("#s\"x", "ball|")`, want: RoleOfAMessagePosted},
	}
	reference, err := engine.Bundled()
	if err != nil {
		t.Fatal(err)
	}
	for name, testCase := range cases {
		content, offset := at(t, "function update(self, dt)\n    "+testCase.line+"\nend\n")

		place, isSomewhere := Locate(content, offset, reference)

		if place.Role != testCase.want || isSomewhere != (testCase.want != RoleOfNothing) {
			t.Errorf("%s: role = %d, somewhere = %v; want %d", name, place.Role, isSomewhere, testCase.want)
		}
	}
}

func TestWhatAHandlerComparesItsArgumentsWith(t *testing.T) {
	cases := map[string]struct {
		script string
		want   Role
	}{
		"the message id":             {script: "function on_message(self, message_id, message, sender)\n    if message_id == hash(\"ball|\") then\n", want: RoleOfAMessageHandled},
		"the message id, renamed":    {script: "function on_message(self, id, m)\n    if id ~= hash(\"ball|\") then return end\nend\n", want: RoleOfAMessageHandled},
		"the action":                 {script: "function on_input(self, action_id, action)\n    if action_id == hash(\"p1|\") and action.pressed then\n    end\nend\n", want: RoleOfAnAction},
		"the group of the message":   {script: "function on_message(self, message_id, message)\n    if message.group == hash(\"wa|\") then end\nend\n", want: RoleOfAGroup},
		"something else":             {script: "function on_message(self, message_id, message)\n    if message.name == hash(\"wa|\") then end\nend\n", want: RoleOfNothing},
		"the id in another function": {script: "function update(self, message_id)\n    if message_id == hash(\"ball|\") then end\nend\n", want: RoleOfNothing},
	}
	reference, err := engine.Bundled()
	if err != nil {
		t.Fatal(err)
	}
	for name, testCase := range cases {
		content, offset := at(t, testCase.script)

		if place, _ := Locate(content, offset, reference); place.Role != testCase.want {
			t.Errorf("%s: role = %d, want %d", name, place.Role, testCase.want)
		}
	}
}

func completions(t *testing.T, file, marked string) (labels []string, replaced string) {
	t.Helper()
	content, offset := at(t, marked)
	candidates, start, end := insightInto(t, pong).Complete(file, content, offset)
	if candidates == nil {
		return nil, ""
	}
	return labelsOf(candidates), string(content[start:end])
}

func TestWhatIsOfferedWhereANameIsTyped(t *testing.T) {
	inUpdate := func(line string) string { return "function update(self, dt)\n    " + line + "\nend\n" }
	cases := map[string]struct {
		file         string
		script       string
		want         []string
		unwanted     []string
		wantReplaced string
	}{
		"the components of the game object itself": {
			file: "main/ball.script", script: inUpdate(`msg.post("#|`),
			want: []string{"ball", "sprite", "bounce_sound"}, unwanted: []string{"game", "hud"}, wantReplaced: "",
		},
		"the components of another game object": {
			file: "main/ball.script", script: inUpdate(`msg.post("/game#h|", "x")`),
			want: []string{"game", "hud"}, unwanted: []string{"sprite"}, wantReplaced: "h",
		},
		"the game objects of the collection": {
			file: "main/ball.script", script: inUpdate(`go.delete("|")`),
			want: []string{"/ball", "/game", "/wall", "game", "#sprite", "."}, wantReplaced: "",
		},
		"the game objects by their absolute path": {
			file: "main/ball.script", script: inUpdate(`go.delete("/g|")`),
			want: []string{"/ball", "/game"}, unwanted: []string{"game", "#sprite"}, wantReplaced: "/g",
		},
		"another collection by its name": {
			file: "main/ball.script", script: inUpdate(`msg.post("main:/|")`),
			want: []string{"main:/ball", "main:/game"}, wantReplaced: "main:/",
		},
		"the messages the receiver handles": {
			file: "main/ball.script", script: inUpdate(`msg.post("/game#game", "|")`),
			want: []string{"ball_out", "serve"}, unwanted: []string{"enable"},
		},
		"the engine messages for a component that is not a script": {
			file: "main/ball.script", script: inUpdate(`msg.post("#sprite", "|")`),
			want: []string{"enable", "disable", "play_animation"}, unwanted: []string{"ball_out"},
		},
		"every message when the receiver is not written": {
			file: "main/ball.script", script: inUpdate(`msg.post(self.game, "|")`),
			want: []string{"ball_out", "serve", "enable"},
		},
		"the messages posted, for a handler": {
			file: "main/game.script", script: "function on_message(self, message_id)\n    if message_id == hash(\"|\") then end\nend\n",
			want: []string{"ball_out", "collision_response"}, unwanted: []string{"serve"},
		},
		"the properties of a game object": {
			file: "main/ball.script", script: inUpdate(`go.set(".", "|", 1)`),
			want: []string{"position", "rotation", "euler", "scale"}, unwanted: []string{"tint", "score"},
		},
		"the properties of a sprite, with those of its material": {
			file: "main/ball.script", script: inUpdate(`go.set("#sprite", "|", 1)`),
			want: []string{"tint", "size", "cursor"}, unwanted: []string{"position", "textureN"},
		},
		"the properties a script declares": {
			file: "main/ball.script", script: inUpdate(`go.get("/game#game", "|")`),
			want: []string{"score", "winning_score"}, unwanted: []string{"position"},
		},
		"the properties behind an address kept in a module": {
			file: "main/ball.script", script: "local ids = require(\"main.ids\")\nlocal GAME = ids.GAME\n" + inUpdate(`go.get(GAME, "|")`),
			want: []string{"score", "winning_score"},
		},
		"the nodes of the scene, with those of its templates": {
			file: "main/hud.gui_script", script: inUpdate(`gui.get_node("|")`),
			want: []string{"score", "restart", "restart/label"},
		},
		"the animations of the atlas of the sprite": {
			file: "main/ball.script", script: inUpdate(`sprite.play_flipbook("#sprite", "|")`),
			want: []string{"ball", "roll"},
		},
		"the animations of the textures of the scene": {
			file: "main/hud.gui_script", script: inUpdate(`gui.play_flipbook(node, "|")`),
			want: []string{"ball", "roll"},
		},
		"the actions that are bound": {
			file: "main/ball.script", script: "function on_input(self, action_id, action)\n    if action_id == hash(\"|\") then end\nend\n",
			want: []string{"p1_up", "touch"},
		},
		"the collision groups": {
			file: "main/ball.script", script: inUpdate(`physics.raycast(a, b, { hash("|") })`),
			want: []string{"wall"},
		},
	}
	for name, testCase := range cases {
		labels, replaced := completions(t, testCase.file, testCase.script)

		for _, want := range testCase.want {
			if !slices.Contains(labels, want) {
				t.Errorf("%s: %q is not offered; offered %v", name, want, labels)
			}
		}
		for _, unwanted := range testCase.unwanted {
			if slices.Contains(labels, unwanted) {
				t.Errorf("%s: %q is offered and does not belong", name, unwanted)
			}
		}
		if replaced != testCase.wantReplaced {
			t.Errorf("%s: what is replaced = %q, want %q", name, replaced, testCase.wantReplaced)
		}
	}
}

func TestNothingIsOfferedWhereNoNameGoes(t *testing.T) {
	if labels, _ := completions(t, "main/ball.script", "local name = \"ba|\"\n"); labels != nil {
		t.Errorf("offered %v inside a string that names nothing", labels)
	}
}

func definitions(t *testing.T, file, marked string) []string {
	t.Helper()
	content, offset := at(t, marked)
	var places []string
	for _, location := range insightInto(t, pong).Define(file, content, offset) {
		places = append(places, location.File+":"+string(rune('0'+location.Position.Line/10))+string(rune('0'+location.Position.Line%10)))
	}
	return places
}

func TestWhereANameIsDeclared(t *testing.T) {
	inUpdate := func(line string) string { return "function update(self, dt)\n    " + line + "\nend\n" }
	cases := map[string]struct {
		file   string
		script string
		want   []string
	}{
		"a component":              {file: "main/ball.script", script: inUpdate(`msg.post("#spr|ite", "disable")`), want: []string{"main/ball.go:06"}},
		"a component of another":   {file: "main/ball.script", script: inUpdate(`msg.post("/game#h|ud", "x")`), want: []string{"main/game.go:06"}},
		"a game object":            {file: "main/ball.script", script: inUpdate(`go.delete("/ga|me")`), want: []string{"main/main.collection:07"}},
		"the handler of a message": {file: "main/ball.script", script: inUpdate(`msg.post("/game#game", "ball|_out")`), want: []string{"main/game.script:05"}},
		"the posts of a message":   {file: "main/game.script", script: "function on_message(self, message_id)\n    if message_id == hash(\"ball_|out\") then end\nend\n", want: []string{"main/ball.script:02"}},
		"a property of a script":   {file: "main/ball.script", script: inUpdate(`go.get("/game#game", "winning_|score")`), want: []string{"main/game.script:02"}},
		"an element of a property": {file: "main/ball.script", script: inUpdate(`go.get("/game#game", "score.|x")`), want: []string{"main/game.script:01"}},
		"a node":                   {file: "main/hud.gui_script", script: inUpdate(`gui.get_node("sco|re")`), want: []string{"main/hud.gui:07"}},
		"a node of a template":     {file: "main/hud.gui_script", script: inUpdate(`gui.get_node("restart/la|bel")`), want: []string{"main/button.gui:02"}},
		"an animation":             {file: "main/ball.script", script: inUpdate(`sprite.play_flipbook("#sprite", "ro|ll")`), want: []string{"main/ball.atlas:05"}},
		"an image of the atlas":    {file: "main/ball.script", script: inUpdate(`sprite.play_flipbook("#sprite", "ba|ll")`), want: []string{"main/ball.atlas:02"}},
		"an action":                {file: "main/ball.script", script: "function on_input(self, action_id)\n    if action_id == hash(\"tou|ch\") then end\nend\n", want: []string{"input/game.input_binding:07"}},
		"a collision group":        {file: "main/ball.script", script: inUpdate(`physics.raycast(a, b, { hash("wa|ll") })`), want: []string{"main/wall.go:04"}},
		"a name that is not there": {file: "main/ball.script", script: inUpdate(`msg.post("#spr|it", "disable")`), want: nil},
		"a property of the engine": {file: "main/ball.script", script: inUpdate(`go.set(".", "posi|tion", 1)`), want: nil},
	}
	for name, testCase := range cases {
		if got := definitions(t, testCase.file, testCase.script); !slices.Equal(got, testCase.want) {
			t.Errorf("%s: declared at %v, want %v", name, got, testCase.want)
		}
	}
}

func TestWhatIsSaidAboutAName(t *testing.T) {
	inUpdate := func(line string) string { return "function update(self, dt)\n    " + line + "\nend\n" }
	cases := map[string]struct {
		file   string
		script string
		want   []string
	}{
		"a component":          {file: "main/ball.script", script: inUpdate(`msg.post("#spr|ite", "disable")`), want: []string{"**sprite** component `sprite` of `/ball`"}},
		"a script component":   {file: "main/ball.script", script: inUpdate(`msg.post("/game#ga|me", "x")`), want: []string{"**script** component `game` of `/game`", "`/main/game.script`", "Handles: `ball_out`, `serve`", "Properties: `score`, `winning_score`"}},
		"a game object":        {file: "main/ball.script", script: inUpdate(`go.delete("/ga|me")`), want: []string{"Game object `/game`, from `/main/game.go`", "- `#hud` gui · /main/hud.gui"}},
		"a broken address":     {file: "main/ball.script", script: inUpdate(`msg.post("#spr|it", "disable")`), want: []string{"`/ball` has no component `sprit`."}},
		"a message":            {file: "main/ball.script", script: inUpdate(`msg.post("/game#game", "ball|_out")`), want: []string{"Message `ball_out`.", "Handled in:", "- `main/game.script` line 5", "Posted from:", "- `main/ball.script` line 2 with `side`"}},
		"a message nobody has": {file: "main/ball.script", script: inUpdate(`msg.post("/game#game", "bal|_out")`), want: []string{"No script in the project handles it."}},
		"an engine message":    {file: "main/ball.script", script: "function on_message(self, message_id)\n    if message_id == hash(\"collision_|response\") then end\nend\n", want: []string{"Engine message `collision_response`.", "Fields: `other_id`, `other_position`, `other_group`, `own_group`"}},
		"a property":           {file: "main/ball.script", script: inUpdate(`go.set("#sprite", "ti|nt", 1)`), want: []string{"Property `tint` of a sprite component."}},
		"a node":               {file: "main/hud.gui_script", script: inUpdate(`gui.get_node("sco|re")`), want: []string{"GUI node `score`, declared in:", "- `main/hud.gui` line 7"}},
		"an action":            {file: "main/ball.script", script: "function on_input(self, action_id)\n    if action_id == hash(\"tou|ch\") then end\nend\n", want: []string{"Input action `touch`", "`input/game.input_binding` line 7"}},
		"a group nobody is in": {file: "main/ball.script", script: inUpdate(`physics.raycast(a, b, { hash("flo|or") })`), want: []string{"Collision group `floor`. Nothing in the project declares it."}},
	}
	for name, testCase := range cases {
		content, offset := at(t, testCase.script)

		said, start, end, isSomewhere := insightInto(t, pong).Describe(testCase.file, content, offset)

		if !isSomewhere || start >= end {
			t.Errorf("%s: nothing is said", name)
			continue
		}
		for _, want := range testCase.want {
			if !strings.Contains(said, want) {
				t.Errorf("%s: what is said lacks %q:\n%s", name, want, said)
			}
		}
	}
}

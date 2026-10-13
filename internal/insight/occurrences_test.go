package insight

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/Lanterna-Games/defold-tools/internal/check"
	"github.com/Lanterna-Games/defold-tools/internal/project"
)

var arena = map[string]string{
	"game.project": "[project]\ntitle = Arena\n\n[input]\ngame_binding = /input/game.input_bindingc\n",
	"input/game.input_binding": `key_trigger {
  input: KEY_SPACE
  action: "jump"
}
gamepad_trigger {
  input: GAMEPAD_RPAD_DOWN
  action: "jump"
}
key_trigger {
  input: KEY_LEFT
  action: "left"
}
`,
	"main/main.collection": `name: "main"
instances {
  id: "hero"
  prototype: "/main/hero.go"
}
instances {
  id: "enemy"
  prototype: "/main/enemy.go"
}
`,
	"main/hero.go": `components {
  id: "hero"
  component: "/main/hero.script"
}
embedded_components {
  id: "body"
  type: "collisionobject"
  data: "type: COLLISION_OBJECT_TYPE_KINEMATIC\n"
  "group: \"hero\"\n"
  "mask: \"enemy\"\n"
  "mask: \"wall\"\n"
  ""
}
`,
	"main/enemy.go": `components {
  id: "enemy"
  component: "/main/enemy.script"
}
components {
  id: "body"
  component: "/main/enemy.collisionobject"
}
`,
	"main/enemy.collisionobject": "type: COLLISION_OBJECT_TYPE_KINEMATIC\ngroup: \"enemy\"\nmask: \"hero\"\n",
	"main/level.tilesource":      "collision_groups: \"wall\"\nconvex_hulls {\n  collision_group: \"wall\"\n}\n",
	"main/ids.lua":               "local M = {}\nM.HIT = hash(\"hit\")\nM.ENEMY = hash(\"enemy\")\nreturn M\n",
	"main/hero.script": `local ids = require("main.ids")
local JUMP = hash("jump")

function on_input(self, action_id, action)
    if action_id == JUMP then
        msg.post("/enemy#enemy", ids.HIT, { damage = 1 })
    elseif action_id == hash("left") then
        msg.post("/enemy#enemy", "taunt")
    end
end

function on_message(self, message_id, message, sender)
    if message_id == hash("collision_response") and message.other_group == ids.ENEMY then
        msg.post(sender, ids.HIT, { damage = 2 })
    end
end
`,
	"main/enemy.script": `local ids = require("main.ids")

function on_message(self, message_id, message, sender)
    if message_id == ids.HIT then
        print(message.damage)
    elseif message_id == hash("taunt") then
        print("hit") -- a string that is no message
    end
end

function update(self, dt)
    physics.raycast(a, b, { hash("hero"), hash("wall") })
end
`,
}

func problemsOf(t *testing.T, files map[string]string) []string {
	t.Helper()
	known := insightInto(t, files)
	var found []string
	for _, problem := range check.Run(known.Project, known.Reference, check.Options{}) {
		found = append(found, fmt.Sprintf("%s:%d [%s]", problem.Location.File, problem.Location.Position.Line, problem.Check))
	}
	slices.Sort(found)
	return found
}

func written(locations []project.Location) []string {
	places := make([]string, 0, len(locations))
	for _, location := range locations {
		places = append(places, fmt.Sprintf("%s:%d:%d", location.File, location.Position.Line, location.Position.Column))
	}
	slices.Sort(places)
	return places
}

func cursorIn(t *testing.T, files map[string]string, file, before string) (content []byte, offset int) {
	t.Helper()
	index := strings.Index(files[file], before)
	if index < 0 {
		t.Fatalf("%s does not hold %q", file, before)
	}
	return []byte(files[file]), index + len(before)
}

func TestEveryPlaceThatWritesAName(t *testing.T) {
	cases := map[string]struct {
		file   string
		before string
		want   []string
	}{
		"a message kept in a module, from the module": {
			file: "main/ids.lua", before: `M.HIT = hash("hi`,
			want: []string{"main/ids.lua:2:15"},
		},
		"a message, from one of its posts": {
			file: "main/hero.script", before: `"/enemy#enemy", "tau`,
			want: []string{"main/enemy.script:6:32", "main/hero.script:8:35"},
		},
		"an action bound by two triggers and kept in a constant": {
			file: "main/hero.script", before: `local JUMP = hash("ju`,
			want: []string{"input/game.input_binding:3:12", "input/game.input_binding:7:12", "main/hero.script:2:20"},
		},
		"an action, from where it is compared": {
			file: "main/hero.script", before: `action_id == hash("le`,
			want: []string{"input/game.input_binding:11:12", "main/hero.script:7:31"},
		},
		"a group, from the list of a ray cast": {
			file: "main/enemy.script", before: `{ hash("hero"), hash("wa`,
			want: []string{"main/enemy.script:12:49", "main/hero.go:11:12", "main/level.tilesource:1:20", "main/level.tilesource:3:21"},
		},
		"a group written inside an embedded component": {
			file: "main/enemy.script", before: `{ hash("he`,
			want: []string{"main/enemy.collisionobject:3:8", "main/enemy.script:12:35", "main/hero.go:9:13"},
		},
	}
	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			content, offset := cursorIn(t, arena, testCase.file, testCase.before)

			locations, _ := insightInto(t, arena).References(testCase.file, content, offset)

			if got := written(locations); !slices.Equal(got, testCase.want) {
				t.Errorf("written in %q, want %q", got, testCase.want)
			}
		})
	}
}

func offsetAfter(changes []Change, moved Change) int {
	offset := moved.Where.Position.Offset
	for _, change := range changes {
		if change.Where.File == moved.Where.File && change.Where.Position.Offset < moved.Where.Position.Offset {
			offset += len(change.New) - len(change.Old)
		}
	}
	return offset
}

func renamed(t *testing.T, files map[string]string, file, before, newName string) (after map[string]string, changes []Change) {
	t.Helper()
	content, offset := cursorIn(t, files, file, before)
	renaming, err := insightInto(t, files).Rename(file, content, offset, newName)
	if err != nil {
		t.Fatal(err)
	}
	changes = renaming.Changes
	byFile := map[string][]Change{}
	for _, change := range changes {
		byFile[change.Where.File] = append(byFile[change.Where.File], change)
	}
	after = maps.Clone(files)
	for changedFile, ofTheFile := range byFile {
		changed, err := Apply([]byte(files[changedFile]), ofTheFile)
		if err != nil {
			t.Fatal(err)
		}
		after[changedFile] = string(changed)
	}
	return after, changes
}

// A rename is right when the project reads the same after it: the checks find what
// they found before, on the same lines, the new name is written wherever the old one
// was, and the old one is written nowhere.
func TestARenameLeavesTheProjectAsItWas(t *testing.T) {
	cases := map[string]struct {
		files   map[string]string
		file    string
		before  string
		newName string
		changed map[string][]string
	}{
		"a message posted through a module and handled through it": {
			file: "main/ids.lua", before: `M.HIT = hash("hi`, newName: "struck",
			changed: map[string][]string{"main/ids.lua": {`M.HIT = hash("struck")`}},
		},
		"a message written where it is posted and where it is handled": {
			file: "main/hero.script", before: `"/enemy#enemy", "tau`, newName: "mocked",
			changed: map[string][]string{
				"main/hero.script":  {`msg.post("/enemy#enemy", "mocked")`},
				"main/enemy.script": {`elseif message_id == hash("mocked") then`},
			},
		},
		"a message hashed without parentheses": {
			files: map[string]string{
				"main/enemy.script": "function on_message(self, message_id, message, sender)\n    if message_id == hash \"taunt\" then\n    end\nend\n",
			},
			file: "main/hero.script", before: `"/enemy#enemy", "tau`, newName: "mocked",
			changed: map[string][]string{
				"main/hero.script":  {`msg.post("/enemy#enemy", "mocked")`},
				"main/enemy.script": {`if message_id == hash "mocked" then`},
			},
		},
		"an action": {
			file: "main/hero.script", before: `action_id == hash("le`, newName: "move_left",
			changed: map[string][]string{
				"input/game.input_binding": {"input: KEY_LEFT\n  action: \"move_left\""},
				"main/hero.script":         {`elseif action_id == hash("move_left") then`},
			},
		},
		"a group, in files, embedded components, masks and scripts": {
			file: "main/enemy.script", before: `{ hash("he`, newName: "player",
			changed: map[string][]string{
				"main/hero.go":               {`"group: \"player\"\n"`, `"mask: \"enemy\"\n"`},
				"main/enemy.collisionobject": {"group: \"enemy\"\nmask: \"player\"\n"},
				"main/enemy.script":          {`{ hash("player"), hash("wall") }`},
			},
		},
	}
	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			files := with(arena, testCase.files)
			content, offset := cursorIn(t, files, testCase.file, testCase.before)
			before, oldName := insightInto(t, files).References(testCase.file, content, offset)

			after, changes := renamed(t, files, testCase.file, testCase.before, testCase.newName)

			if len(changes) == 0 {
				t.Fatal("nothing was changed")
			}
			if got, want := problemsOf(t, after), problemsOf(t, files); !slices.Equal(got, want) {
				t.Errorf("the checks find %q after the rename, and found %q before", got, want)
			}
			for changedFile, fragments := range testCase.changed {
				for _, fragment := range fragments {
					if !strings.Contains(after[changedFile], fragment) {
						t.Errorf("%s lacks %q:\n%s", changedFile, fragment, after[changedFile])
					}
				}
			}
			for unchangedFile, content := range files {
				if _, isExpected := testCase.changed[unchangedFile]; testCase.changed != nil && !isExpected && after[unchangedFile] != content {
					t.Errorf("%s was changed:\n%s", unchangedFile, after[unchangedFile])
				}
			}
			known := insightInto(t, after)
			first := changes[0].Where
			nowWritten, newName := known.References(first.File, []byte(after[first.File]), offsetAfter(changes, changes[0]))
			if newName != testCase.newName || len(nowWritten) != len(before) {
				t.Errorf("%q is written in %d places, and %q was in %d", newName, len(nowWritten), oldName, len(before))
			}
		})
	}
}

func with(files map[string]string, others map[string]string) map[string]string {
	together := maps.Clone(files)
	maps.Copy(together, others)
	return together
}

func TestARenameThatWouldBreakSomethingIsRefused(t *testing.T) {
	inOnInput := func(action string) string {
		return "function on_input(self, action_id, action)\n    if action_id == hash(\"" + action + "\") then\n    end\nend\n"
	}
	cases := map[string]struct {
		files   map[string]string
		file    string
		before  string
		newName string
		want    string
	}{
		"a message of the engine":             {file: "main/hero.script", before: `hash("collision_res`, newName: "bump", want: "is a message of the engine"},
		"onto a message that exists":          {file: "main/hero.script", before: `"/enemy#enemy", "tau`, newName: "hit", want: `another message is already named "hit"`},
		"onto a message of the engine":        {file: "main/hero.script", before: `"/enemy#enemy", "tau`, newName: "enable", want: `another message is already named "enable"`},
		"onto a group that exists":            {file: "main/enemy.script", before: `{ hash("he`, newName: "wall", want: `another collision group is already named "wall"`},
		"onto an action that exists":          {file: "main/hero.script", before: `action_id == hash("le`, newName: "jump", want: `another action is already named "jump"`},
		"to nothing":                          {file: "main/hero.script", before: `"/enemy#enemy", "tau`, newName: "", want: "is empty"},
		"to a name that would end the string": {file: "main/hero.script", before: `"/enemy#enemy", "tau`, newName: `mo"cked`, want: "cannot name a message"},
		"where there is no name":              {file: "main/enemy.script", before: `print("hi`, newName: "x", want: "no name of the project here"},
		"an action of the engine's binding": {
			files: map[string]string{
				"game.project":      "[project]\ntitle = Arena\n\n[input]\ngame_binding = /builtins/input/all.input_bindingc\n",
				"main/enemy.script": inOnInput("touch"),
			},
			file: "main/enemy.script", before: `action_id == hash("tou`, newName: "tap", want: "is bound in /builtins/input/all.input_binding, which is the engine's",
		},
		"an action no binding binds": {
			files: map[string]string{"main/enemy.script": inOnInput("fly")},
			file:  "main/enemy.script", before: `action_id == hash("fl`, newName: "glide", want: "is bound in no input binding of the project",
		},
		"a message compared in a way the rename does not follow": {
			files: map[string]string{
				"main/enemy.script": "function on_message(self, message_id, message, sender)\n    if hash(\"taunt\") == message_id then\n    end\nend\n",
			},
			file: "main/hero.script", before: `"/enemy#enemy", "tau`, newName: "mocked", want: `main/enemy.script uses "taunt" in 1 places and only 0 of them could be found to rename`,
		},
		"a text that is also the id of a GUI node": {
			files: map[string]string{
				"main/hud.gui":        "script: \"/main/hud.gui_script\"\nnodes {\n  id: \"score\"\n  type: TYPE_TEXT\n}\n",
				"main/hud.gui_script": "local SCORE = hash(\"score\")\n\nfunction on_message(self, message_id, message, sender)\n    if message_id == SCORE then\n        gui.set_text(gui.get_node(SCORE), message.text)\n    end\nend\n",
			},
			file: "main/hud.gui_script", before: `SCORE = hash("sco`, newName: "points", want: `"score" at main/hud.gui_script:1 is also used as something other than a message`,
		},
	}
	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			files := with(arena, testCase.files)
			content, offset := cursorIn(t, files, testCase.file, testCase.before)

			renaming, err := insightInto(t, files).Rename(testCase.file, content, offset, testCase.newName)

			if err == nil || !strings.Contains(err.Error(), testCase.want) || len(renaming.Changes) != 0 {
				t.Errorf("renaming = %+v, error = %v, want it refused because %q", renaming, err, testCase.want)
			}
		})
	}
}

// An edit is only offered against the text it was computed from. A file that changed
// since, by a line or by a letter, refuses the whole change.
func TestAChangeIsRefusedWhereTheTextIsNoLongerThere(t *testing.T) {
	content, offset := cursorIn(t, arena, "main/hero.script", `"/enemy#enemy", "tau`)
	renaming, err := insightInto(t, arena).Rename("main/hero.script", content, offset, "mocked")
	if err != nil {
		t.Fatal(err)
	}
	ofTheEnemy := slices.DeleteFunc(slices.Clone(renaming.Changes), func(change Change) bool { return change.Where.File != "main/enemy.script" })

	if _, err := Apply([]byte("-- a line added since\n"+arena["main/enemy.script"]), ofTheEnemy); err == nil || !strings.Contains(err.Error(), `does not hold "taunt" any more`) {
		t.Errorf("error = %v, want the change refused", err)
	}
	if changed, err := Apply([]byte(arena["main/enemy.script"]), ofTheEnemy); err != nil || !strings.Contains(string(changed), `hash("mocked")`) {
		t.Errorf("error = %v on the text the change was made for", err)
	}
}

// A name can reach a comparison through a script property: the action a paddle answers
// to is a property set in the collection. Nothing says that such a value is an action,
// so a rename cannot change it, and it must not pass over it in silence either.
func TestARenameSaysWhereItLeftTheSameTextAlone(t *testing.T) {
	files := with(arena, map[string]string{
		"main/main.collection": arena["main/main.collection"] + `instances {
  id: "paddle"
  prototype: "/main/paddle.go"
  component_properties {
    id: "paddle"
    properties {
      id: "up_action"
      value: "left"
      type: PROPERTY_TYPE_HASH
    }
  }
}
`,
		"main/paddle.go":     "components {\n  id: \"paddle\"\n  component: \"/main/paddle.script\"\n}\n",
		"main/paddle.script": "go.property(\"up_action\", hash(\"left\"))\n\nfunction on_input(self, action_id, action)\n    if action_id == self.up_action then\n        print(\"left\")\n    end\nend\n",
	})
	content, offset := cursorIn(t, files, "main/hero.script", `action_id == hash("le`)

	renaming, err := insightInto(t, files).Rename("main/hero.script", content, offset, "move_left")

	if err != nil || len(renaming.Changes) != 2 {
		t.Fatalf("renaming = %+v, error = %v, want the binding and the comparison changed", renaming, err)
	}
	if got, want := written(renaming.LeftAlone), []string{"main/main.collection:17:14", "main/paddle.script:1:32", "main/paddle.script:5:16"}; !slices.Equal(got, want) {
		t.Errorf("left alone %q, want %q", got, want)
	}
}

var hud = map[string]string{
	"main/main.collection": arena["main/main.collection"] + `instances {
  id: "hud"
  prototype: "/main/hud.go"
}
`,
	"main/hud.go": "components {\n  id: \"hud\"\n  component: \"/main/hud.gui\"\n}\n",
	"main/hud.gui": `script: "/main/hud.gui_script"
nodes {
  id: "panel"
  type: TYPE_BOX
}
nodes {
  id: "score"
  type: TYPE_TEXT
  parent: "panel"
}
nodes {
  id: "restart"
  type: TYPE_TEMPLATE
  template: "/main/button.gui"
  parent: "panel"
}
nodes {
  id: "restart/label"
  type: TYPE_TEXT
  parent: "restart"
  template_node_child: true
}
layouts {
  name: "Landscape"
  nodes {
    id: "score"
    parent: "panel"
  }
}
`,
	"main/button.gui": "nodes {\n  id: \"label\"\n  type: TYPE_TEXT\n}\n",
	"main/hud.gui_script": `function init(self)
    self.score = gui.get_node("score")
    self.label = gui.get_node("restart/label")
    gui.set_parent(self.score, gui.get_node("panel"))
end
`,
}

// The id of a node is written whole where it is declared and as a part of longer ids
// where a template brings it into another scene. A rename has to change the part and
// leave what is around it.
func TestGUINodesAreRenamedThroughTemplates(t *testing.T) {
	files := with(arena, hud)
	cases := map[string]struct {
		file    string
		before  string
		newName string
		changed map[string][]string
	}{
		"a node, where it is declared, where it is a parent and in a layout": {
			file: "main/hud.gui_script", before: `gui.get_node("pan`, newName: "frame",
			changed: map[string][]string{
				"main/hud.gui":        {"nodes {\n  id: \"frame\"\n  type: TYPE_BOX", "type: TYPE_TEXT\n  parent: \"frame\"", "template: \"/main/button.gui\"\n  parent: \"frame\"", "    id: \"score\"\n    parent: \"frame\""},
				"main/hud.gui_script": {`gui.get_node("frame"))`},
			},
		},
		"a node that brings a template in": {
			file: "main/hud.gui_script", before: `gui.get_node("rest`, newName: "again",
			changed: map[string][]string{
				"main/hud.gui":        {"id: \"again\"\n  type: TYPE_TEMPLATE", "id: \"again/label\"\n  type: TYPE_TEXT\n  parent: \"again\""},
				"main/hud.gui_script": {`gui.get_node("again/label")`},
			},
		},
		"a node of the template, from the scene that uses it": {
			file: "main/hud.gui_script", before: `gui.get_node("restart/la`, newName: "caption",
			changed: map[string][]string{
				"main/button.gui":     {"id: \"caption\""},
				"main/hud.gui":        {"id: \"restart/caption\"\n  type: TYPE_TEXT\n  parent: \"restart\""},
				"main/hud.gui_script": {`gui.get_node("restart/caption")`},
			},
		},
	}
	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			after, changes := renamed(t, files, testCase.file, testCase.before, testCase.newName)

			if got, want := problemsOf(t, after), problemsOf(t, files); !slices.Equal(got, want) {
				t.Errorf("the checks find %q after the rename, and found %q before", got, want)
			}
			for changedFile, fragments := range testCase.changed {
				for _, fragment := range fragments {
					if !strings.Contains(after[changedFile], fragment) {
						t.Errorf("%s lacks %q:\n%s", changedFile, fragment, after[changedFile])
					}
				}
			}
			for file, content := range files {
				if _, isExpected := testCase.changed[file]; !isExpected && after[file] != content {
					t.Errorf("%s was changed:\n%s", file, after[file])
				}
			}
			if len(changes) == 0 {
				t.Error("nothing was changed")
			}
		})
	}
}

var bodies = map[string]string{
	"main/main.collection": `name: "main"
instances {
  id: "hero"
  prototype: "/main/hero.go"
  component_properties {
    id: "hero"
    properties {
      id: "target"
      value: "/enemy#body"
      type: PROPERTY_TYPE_URL
    }
  }
}
instances {
  id: "enemy"
  prototype: "/main/enemy.go"
  component_properties {
    id: "body"
    properties {
      id: "mass"
      value: "2.0"
      type: PROPERTY_TYPE_NUMBER
    }
  }
}
`,
	"main/hero.script": `go.property("target", msg.url())

function init(self)
    msg.post("#body", "disable")
    msg.post("/enemy#body", "disable")
    msg.post("enemy#body", "enable")
    msg.post(msg.url(nil, "/enemy", "body"), "enable")
end
`,
	"main/enemy.script": `function init(self)
    msg.post("#body", "disable")
    msg.post("/hero#body", "apply_force", { force = vmath.vector3(), position = go.get_position() })
    msg.post(msg.url(nil, nil, "body"), "enable")
    msg.post(msg.url(nil, "/hero", hash("body")), "enable")
end
`,
}

// Two game objects can each have a component called "body". An address names one of
// them, and which one depends on the game object the address leads to, not on the text.
func TestAComponentIsRenamedWhereAddressesLeadToIt(t *testing.T) {
	files := with(arena, bodies)

	after, _ := renamed(t, files, "main/enemy.script", `msg.post("#bo`, "hitbox")

	if got, want := problemsOf(t, after), problemsOf(t, files); !slices.Equal(got, want) {
		t.Errorf("the checks find %q after the rename, and found %q before", got, want)
	}
	wanted := map[string][]string{
		"main/enemy.go":        {"id: \"hitbox\"\n  component: \"/main/enemy.collisionobject\""},
		"main/enemy.script":    {`msg.post("#hitbox", "disable")`, `msg.post("/hero#body", "apply_force"`, `msg.url(nil, nil, "hitbox")`, `msg.url(nil, "/hero", hash("body"))`},
		"main/hero.script":     {`msg.post("#body", "disable")`, `msg.post("/enemy#hitbox", "disable")`, `msg.post("enemy#hitbox", "enable")`, `msg.url(nil, "/enemy", "hitbox")`},
		"main/main.collection": {"value: \"/enemy#hitbox\"", "component_properties {\n    id: \"hitbox\"\n    properties {\n      id: \"mass\""},
	}
	for file, fragments := range wanted {
		for _, fragment := range fragments {
			if !strings.Contains(after[file], fragment) {
				t.Errorf("%s lacks %q:\n%s", file, fragment, after[file])
			}
		}
	}
	if after["main/hero.go"] != files["main/hero.go"] {
		t.Errorf("the body of the hero was renamed too:\n%s", after["main/hero.go"])
	}
}

func TestANameThatMeansTwoThingsIsNotRenamed(t *testing.T) {
	shared := "function init(self)\n    msg.post(\"#body\", \"disable\")\nend\n"
	files := with(arena, map[string]string{
		"main/hero.go":       strings.Replace(arena["main/hero.go"], "/main/hero.script", "/main/shared.script", 1),
		"main/enemy.go":      strings.Replace(arena["main/enemy.go"], "/main/enemy.script", "/main/shared.script", 1),
		"main/shared.script": shared,
	})
	content, offset := cursorIn(t, files, "main/shared.script", `msg.post("#bo`)

	renaming, err := insightInto(t, files).Rename("main/shared.script", content, offset, "hitbox")

	want := `main/shared.script:2 names the component "body" of /main/hero.go where its script runs for one owner and the component "body" of /main/enemy.go for another`
	if err == nil || !strings.Contains(err.Error(), want) || len(renaming.Changes) != 0 {
		t.Errorf("renaming = %+v, error = %v, want it refused because %s", renaming, err, want)
	}
}

// "enemy" is at once a game object of the collection, a collision group and the id of
// a script component. Renaming the game object changes the paths that lead to it and
// leaves the other two as they are. A module runs for no game object, so only its
// addresses that start from the root can be followed: a relative one is left alone.
func TestAGameObjectIsRenamedInThePathsThatLeadToIt(t *testing.T) {
	files := with(arena, bodies)
	files["main/main.collection"] = strings.Replace(files["main/main.collection"], "  prototype: \"/main/hero.go\"\n", "  prototype: \"/main/hero.go\"\n  children: \"enemy\"\n", 1)
	files["main/taunts.lua"] = "local M = {}\n\nfunction M.taunt()\n    msg.post(\"main:/enemy#enemy\", \"taunt\")\n    msg.post(\"/enemy\", \"taunt\")\n    msg.post(\"enemy\", \"taunt\")\nend\n\nreturn M\n"

	after, _ := renamed(t, files, "main/hero.script", `msg.post("/ene`, "villain")

	if got, want := problemsOf(t, after), problemsOf(t, files); !slices.Equal(got, want) {
		t.Errorf("the checks find %q after the rename, and found %q before", got, want)
	}
	wanted := map[string][]string{
		"main/taunts.lua":      {`msg.post("main:/villain#enemy", "taunt")`, `msg.post("/villain", "taunt")`, `msg.post("enemy", "taunt")`},
		"main/main.collection": {"instances {\n  id: \"villain\"\n  prototype: \"/main/enemy.go\"", "children: \"villain\"", "value: \"/villain#body\""},
		"main/hero.script":     {`msg.post("/villain#body", "disable")`, `msg.post("villain#body", "enable")`, `msg.url(nil, "/villain", "body")`},
	}
	for file, fragments := range wanted {
		for _, fragment := range fragments {
			if !strings.Contains(after[file], fragment) {
				t.Errorf("%s lacks %q:\n%s", file, fragment, after[file])
			}
		}
	}
	for _, untouched := range []string{"main/hero.go", "main/enemy.go", "main/enemy.collisionobject", "main/ids.lua", "main/enemy.script"} {
		if after[untouched] != files[untouched] {
			t.Errorf("%s was changed:\n%s", untouched, after[untouched])
		}
	}
}

var animated = map[string]string{
	"main/hero.go": arena["main/hero.go"] + `embedded_components {
  id: "sprite"
  type: "sprite"
  data: "default_animation: \"idle\"\n"
  "textures {\n"
  "  sampler: \"texture_sampler\"\n"
  "  texture: \"/main/hero.atlas\"\n"
  "}\n"
  ""
}
`,
	"main/enemy.go": arena["main/enemy.go"] + `embedded_components {
  id: "sprite"
  type: "sprite"
  data: "default_animation: \"idle\"\n"
  "textures {\n"
  "  sampler: \"texture_sampler\"\n"
  "  texture: \"/main/enemy.atlas\"\n"
  "}\n"
  ""
}
`,
	"main/hero.atlas":      "images {\n  image: \"/main/images/portrait.png\"\n}\nanimations {\n  id: \"idle\"\n}\nanimations {\n  id: \"run\"\n}\n",
	"main/enemy.atlas":     "animations {\n  id: \"idle\"\n}\n",
	"main/dust.particlefx": "emitters {\n  tile_source: \"/main/hero.atlas\"\n  animation: \"run\"\n}\n",
	"main/hud.go":          "components {\n  id: \"hud\"\n  component: \"/main/hud.gui\"\n}\n",
	"main/hud.gui": `script: "/main/hud.gui_script"
textures {
  name: "hero"
  texture: "/main/hero.atlas"
}
nodes {
  id: "face"
  type: TYPE_BOX
  texture: "hero/idle"
}
`,
	"main/hud.gui_script": "function init(self)\n    gui.play_flipbook(gui.get_node(\"face\"), \"run\")\nend\n",
	"main/hero.script": `function init(self)
    sprite.play_flipbook("#sprite", "idle")
    sprite.play_flipbook("/enemy#sprite", hash("idle"))
    msg.post("#sprite", "play_animation", { id = hash("run") })
    sprite.play_flipbook("#sprite", "portrait")
end
`,
}

// Two atlases can each have an animation called "idle". Which one a script plays
// depends on the sprite the address leads to.
func TestAnAnimationIsRenamedInItsAtlasAndWhereItIsPlayed(t *testing.T) {
	files := with(arena, animated)
	cases := map[string]struct {
		file    string
		before  string
		newName string
		changed map[string][]string
	}{
		"an animation another atlas has too": {
			file: "main/hero.script", before: `play_flipbook("#sprite", "id`, newName: "stand",
			changed: map[string][]string{
				"main/hero.atlas":  {"animations {\n  id: \"stand\"\n}\nanimations {\n  id: \"run\""},
				"main/hero.go":     {`"default_animation: \"stand\"\n"`},
				"main/hud.gui":     {`texture: "hero/stand"`},
				"main/hero.script": {`sprite.play_flipbook("#sprite", "stand")`, `sprite.play_flipbook("/enemy#sprite", hash("idle"))`},
			},
		},
		"an animation a particle effect and a node play": {
			file: "main/hero.script", before: `{ id = hash("ru`, newName: "sprint",
			changed: map[string][]string{
				"main/hero.atlas":      {"animations {\n  id: \"sprint\"\n}"},
				"main/dust.particlefx": {`animation: "sprint"`},
				"main/hud.gui_script":  {`gui.play_flipbook(gui.get_node("face"), "sprint")`},
				"main/hero.script":     {`{ id = hash("sprint") }`},
			},
		},
	}
	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			after, _ := renamed(t, files, testCase.file, testCase.before, testCase.newName)

			if got, want := problemsOf(t, after), problemsOf(t, files); !slices.Equal(got, want) {
				t.Errorf("the checks find %q after the rename, and found %q before", got, want)
			}
			for changedFile, fragments := range testCase.changed {
				for _, fragment := range fragments {
					if !strings.Contains(after[changedFile], fragment) {
						t.Errorf("%s lacks %q:\n%s", changedFile, fragment, after[changedFile])
					}
				}
			}
			for file, content := range files {
				if _, isExpected := testCase.changed[file]; !isExpected && after[file] != content {
					t.Errorf("%s was changed:\n%s", file, after[file])
				}
			}
		})
	}

	content, offset := cursorIn(t, files, "main/hero.script", `play_flipbook("#sprite", "portr`)
	if _, err := insightInto(t, files).Rename("main/hero.script", content, offset, "face"); err == nil || !strings.Contains(err.Error(), `"portrait" is an image of /main/hero.atlas and takes its name from the file`) {
		t.Errorf("error = %v, want an image refused", err)
	}
}

var withSpeed = map[string]string{
	"main/main.collection": `name: "main"
instances {
  id: "hero"
  prototype: "/main/hero.go"
  component_properties {
    id: "hero"
    properties {
      id: "speed"
      value: "120.0"
      type: PROPERTY_TYPE_NUMBER
    }
  }
}
instances {
  id: "enemy"
  prototype: "/main/enemy.go"
}
`,
	"main/enemy.go": `components {
  id: "enemy"
  component: "/main/enemy.script"
  properties {
    id: "speed"
    value: "40.0"
    type: PROPERTY_TYPE_NUMBER
  }
}
`,
	"main/hero.script": `go.property("speed", 100)
go.property("direction", vmath.vector3())

function update(self, dt)
    local step = self.speed * dt
    self["speed"] = step
    go.set("#", "speed", 10)
    go.animate("#hero", "direction.x", go.PLAYBACK_ONCE_FORWARD, 1, go.EASING_LINEAR, 1)
    go.set("/enemy#enemy", "speed", go.get("/hero#hero", "speed"))
    factory.create("#factory", nil, nil, { speed = 5 })
end
`,
	"main/enemy.script": `go.property("speed", 50)

function update(self, dt)
    self.position = self.speed * dt
end
`,
	"main/moves.lua": "local M = {}\n\nfunction M.advance(self, dt)\n    return self.speed * dt\nend\n\nfunction M.turn(url)\n    return go.get(url, \"direction\"), go.get(url, \"speed\")\nend\n\nreturn M\n",
}

// An address kept in a variable says nothing about where it leads. A property asked of
// it can still be only one thing when a single script of the project declares it, which
// is how the check judges it too; when two scripts do, it is left alone and listed.
func TestAPropertyAskedOfAnAddressThatIsNotWrittenIsTheOnlyOneOfItsName(t *testing.T) {
	files := with(arena, withSpeed)
	delete(files, "main/enemy.collisionobject")

	after, _ := renamed(t, files, "main/hero.script", `go.property("direc`, "heading")

	if got, want := problemsOf(t, after), problemsOf(t, files); !slices.Equal(got, want) {
		t.Errorf("the checks find %q after the rename, and found %q before", got, want)
	}
	if !strings.Contains(after["main/moves.lua"], `go.get(url, "heading"), go.get(url, "speed")`) || !strings.Contains(after["main/hero.script"], `go.animate("#hero", "heading.x"`) {
		t.Errorf("the property was not renamed where only it can be meant:\n%s\n%s", after["main/moves.lua"], after["main/hero.script"])
	}
}

// Two scripts can each declare a property called "speed". Which one a name means is
// told by the script that holds it, for self.speed, and by the component an address
// leads to, for go.get and for what a collection sets.
func TestAScriptPropertyIsRenamedWhereItsScriptIsMeant(t *testing.T) {
	files := with(arena, withSpeed)
	delete(files, "main/enemy.collisionobject")

	after, changes := renamed(t, files, "main/hero.script", `go.property("spe`, "pace")

	if got, want := problemsOf(t, after), problemsOf(t, files); !slices.Equal(got, want) {
		t.Errorf("the checks find %q after the rename, and found %q before", got, want)
	}
	wanted := map[string][]string{
		"main/hero.script": {
			`go.property("pace", 100)`, `local step = self.pace * dt`, `self["pace"] = step`, `go.set("#", "pace", 10)`,
			`go.set("/enemy#enemy", "speed", go.get("/hero#hero", "pace"))`, `{ speed = 5 }`,
		},
		"main/main.collection": {"id: \"hero\"\n    properties {\n      id: \"pace\""},
	}
	for file, fragments := range wanted {
		for _, fragment := range fragments {
			if !strings.Contains(after[file], fragment) {
				t.Errorf("%s lacks %q:\n%s", file, fragment, after[file])
			}
		}
	}
	for _, untouched := range []string{"main/enemy.go", "main/enemy.script", "main/moves.lua"} {
		if after[untouched] != files[untouched] {
			t.Errorf("%s was changed:\n%s", untouched, after[untouched])
		}
	}

	content, offset := cursorIn(t, files, "main/hero.script", `go.property("spe`)
	renaming, err := insightInto(t, files).Rename("main/hero.script", content, offset, "pace")
	if err != nil || len(renaming.Changes) != len(changes) {
		t.Fatalf("renaming = %+v, error = %v", renaming, err)
	}
	if got, want := written(renaming.LeftAlone), []string{"main/enemy.script:4:26", "main/hero.script:10:44", "main/moves.lua:4:17", "main/moves.lua:8:51"}; !slices.Equal(got, want) {
		t.Errorf("left alone %q, want %q: the module that reads self.speed, the enemy that has its own, and what the factory is given", got, want)
	}

	fromAField, _ := renamed(t, files, "main/hero.script", `local step = self.spe`, "pace")
	if fromAField["main/hero.script"] != after["main/hero.script"] || fromAField["main/main.collection"] != after["main/main.collection"] {
		t.Errorf("renaming from self.speed gives another result:\n%s", fromAField["main/hero.script"])
	}

	ofTheEnemy, _ := renamed(t, files, "main/hero.script", `go.set("/enemy#enemy", "spe`, "pace")
	if !strings.Contains(ofTheEnemy["main/enemy.script"], `go.property("pace", 50)`) || !strings.Contains(ofTheEnemy["main/enemy.go"], "properties {\n    id: \"pace\"") || !strings.Contains(ofTheEnemy["main/hero.script"], `go.property("speed", 100)`) {
		t.Errorf("the property of the enemy was not the one renamed:\n%s\n%s", ofTheEnemy["main/enemy.script"], ofTheEnemy["main/enemy.go"])
	}
}

func TestAPropertyKeepsTheNameOfAField(t *testing.T) {
	files := with(arena, withSpeed)
	cases := map[string]struct {
		file    string
		newName string
		want    string
	}{
		"a name that is no field":             {file: "main/enemy.script", newName: "top speed", want: `"top speed" cannot name a script property`},
		"the name of another property":        {file: "main/hero.script", newName: "direction", want: `/main/hero.script already has a property or a field of self named "direction"`},
		"the name of a field the script uses": {file: "main/enemy.script", newName: "position", want: `/main/enemy.script already has a property or a field of self named "position"`},
	}
	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			content, offset := cursorIn(t, files, testCase.file, `go.property("spe`)
			if _, err := insightInto(t, files).Rename(testCase.file, content, offset, testCase.newName); err == nil || !strings.Contains(err.Error(), testCase.want) {
				t.Errorf("error = %v, want %s", err, testCase.want)
			}
		})
	}
}

var nestedGame = map[string]string{
	"game.project": "[project]\ntitle = Nested\n\n[bootstrap]\nmain_collection = /main/main.collectionc\n",
	"main/main.collection": `name: "main"
instances {
  id: "hero"
  prototype: "/main/hero.go"
}
collection_instances {
  id: "area"
  collection: "/main/area.collection"
}
collection_instances {
  id: "other"
  collection: "/main/area.collection"
}
`,
	"main/area.collection": `name: "area"
instances {
  id: "gate"
  prototype: "/main/gate.go"
}
collection_instances {
  id: "yard"
  collection: "/main/yard.collection"
}
`,
	"main/yard.collection": "name: \"yard\"\ninstances {\n  id: \"tree\"\n  prototype: \"/main/gate.go\"\n}\n",
	"main/hero.go":         "components {\n  id: \"script\"\n  component: \"/main/hero.script\"\n}\n",
	"main/gate.go":         "components {\n  id: \"script\"\n  component: \"/main/gate.script\"\n}\n",
	"main/hero.script": `function init(self)
    msg.post("/area/gate#script", "open")
    msg.post("area/gate", "open")
    msg.post("main:/area/yard/tree#script", "open")
    msg.post("/other/gate", "open")
    msg.post(msg.url(nil, "/area/yard/tree", "script"), "open")
    msg.post("/area/gone", "open")
    print("area/nothing")
end

function on_message(self, message_id, message, sender)
    if message_id == hash("open") then
    end
end
`,
	"main/gate.script": `function init(self)
    msg.post("yard/tree", "open")
    msg.post("gate", "open")
end

function on_message(self, message_id, message, sender)
    if message_id == hash("open") then
    end
end
`,
}

// A collection placed inside another gives its id to the path of everything it
// holds. Renaming it changes that part of every address that goes through it,
// whether the address starts from the root or from a game object beside it,
// and leaves alone the same collection placed a second time under another id.
// An address that goes through it to nothing is changed too: it was wrong
// before and is wrong in the same way after, where leaving it would have made
// a second mistake of it. A real project had two such addresses, into a
// collection of a library that had not been fetched.
// The collection it holds in turn is named in the middle of a path, and by the
// scripts that run inside the first.
func TestACollectionPlacedInAnotherIsRenamedInThePathsThroughIt(t *testing.T) {
	cases := map[string]struct {
		file    string
		before  string
		newName string
		wanted  map[string][]string
		same    []string
	}{
		"the one the root places, from a script": {
			file: "main/hero.script", before: `msg.post("/ar`, newName: "zone",
			wanted: map[string][]string{
				"main/main.collection": {"collection_instances {\n  id: \"zone\"\n  collection: \"/main/area.collection\"", "id: \"other\""},
				"main/hero.script": {
					`msg.post("/zone/gate#script", "open")`, `msg.post("zone/gate", "open")`, `msg.post("main:/zone/yard/tree#script", "open")`,
					`msg.post("/other/gate", "open")`, `msg.url(nil, "/zone/yard/tree", "script")`, `msg.post("/zone/gone", "open")`, `print("area/nothing")`,
				},
			},
			same: []string{"main/area.collection", "main/yard.collection", "main/gate.script"},
		},
		"the one inside it, from a script that runs beside it": {
			file: "main/gate.script", before: `msg.post("ya`, newName: "garden",
			wanted: map[string][]string{
				"main/area.collection": {"collection_instances {\n  id: \"garden\"\n  collection: \"/main/yard.collection\""},
				"main/gate.script":     {`msg.post("garden/tree", "open")`, `msg.post("gate", "open")`},
				"main/hero.script":     {`msg.post("main:/area/garden/tree#script", "open")`, `msg.url(nil, "/area/garden/tree", "script")`, `msg.post("/area/gate#script", "open")`},
			},
			same: []string{"main/main.collection", "main/yard.collection"},
		},
		"from the collection that places it": {
			file: "main/main.collection", before: "collection_instances {\n  id: \"ar", newName: "zone",
			wanted: map[string][]string{
				"main/main.collection": {"id: \"zone\"\n  collection: \"/main/area.collection\""},
				"main/hero.script":     {`msg.post("/zone/gate#script", "open")`},
			},
			same: []string{"main/area.collection", "main/gate.script"},
		},
	}
	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			after, _ := renamed(t, nestedGame, testCase.file, testCase.before, testCase.newName)

			if got, want := problemsOf(t, after), problemsOf(t, nestedGame); !slices.Equal(got, want) {
				t.Errorf("the checks find %q after the rename, and found %q before", got, want)
			}
			for file, fragments := range testCase.wanted {
				for _, fragment := range fragments {
					if !strings.Contains(after[file], fragment) {
						t.Errorf("%s lacks %q:\n%s", file, fragment, after[file])
					}
				}
			}
			for _, untouched := range testCase.same {
				if after[untouched] != nestedGame[untouched] {
					t.Errorf("%s was changed:\n%s", untouched, after[untouched])
				}
			}
		})
	}
}

func TestACollectionIsNotRenamedToWhatItsNeighboursAreCalled(t *testing.T) {
	known := insightInto(t, nestedGame)
	content, offset := cursorIn(t, nestedGame, "main/hero.script", `msg.post("/ar`)

	for _, taken := range []string{"hero", "other"} {
		if _, err := known.Rename("main/hero.script", content, offset, taken); err == nil || !strings.Contains(err.Error(), "already named") {
			t.Errorf("renaming it to %q gave %v, want it refused: the collection that places it has that id", taken, err)
		}
	}
	if _, err := known.Rename("main/hero.script", content, offset, "a/b"); err == nil {
		t.Error("a name with a slash in it was taken for the id of a collection")
	}
	renaming, err := known.Rename("main/hero.script", content, offset, "zone")
	if err != nil {
		t.Fatal(err)
	}
	if left := written(renaming.LeftAlone); !slices.Equal(left, []string{"main/hero.script:8:12"}) {
		t.Errorf("left alone and said so: %q, want the path that leads nowhere", left)
	}
	if places, name := known.References("main/hero.script", content, offset); name != "area" || len(places) != 6 {
		t.Errorf("%q is written in %q, want the one place that declares it and the five addresses through it", name, written(places))
	}
}
